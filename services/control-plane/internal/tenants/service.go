package tenants

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/idp"
)

// ErrInvalidSlug / ErrConflict are returned for client errors (mapped to 422/409).
var (
	ErrInvalidSlug = errors.New("invalid tenant slug")
	ErrConflict    = errors.New("tenant in a conflicting state")
	ErrNotConfigured = errors.New("tenant provisioning not configured (no Keycloak admin client)")
)

// Config holds non-secret provisioning settings (from pkg/config).
type Config struct {
	BaseDomain         string
	ConsoleOrigin      string
	AdminAudience      string
	ResourceTemplate   string
	ReservedSlugs      string
	AuditRetentionDays int
	Ceiling            int
	AccessTTL          int
	SSOIdle            int
	SSOMax             int
	SSLRequired        string
}

// Service orchestrates tenant lifecycle: provision (idempotent saga with
// compensation), suspend/resume, and delete. Every action is audited.
type Service struct {
	store Store
	kc    idp.Keycloak // nil when provisioning is not configured
	audit audit.Logger
	log   zerolog.Logger
	cfg   Config
	now   func() time.Time
}

// NewService builds the service. kc may be nil in environments where provisioning
// is not configured (the mutating endpoints then return ErrNotConfigured).
func NewService(store Store, kc idp.Keycloak, auditLog audit.Logger, log zerolog.Logger, cfg Config) *Service {
	return &Service{store: store, kc: kc, audit: auditLog, log: log, cfg: cfg, now: time.Now}
}

// ProvisionRequest is the input to Provision.
type ProvisionRequest struct {
	Slug        string
	DisplayName string
	AdminEmail  string
	Actor       string // operator subject (audit)
}

func (s *Service) mcpResource(slug string) string {
	return authz.MCPResource(slug, s.cfg.BaseDomain, s.cfg.ResourceTemplate)
}

// Provision creates a fully isolated tenant (realm + clients + mappers + admin
// role + admin user) as an idempotent saga; on any failure it compensates
// (deletes the realm, which cascades sub-objects) leaving no ghost realm (FR-006).
func (s *Service) Provision(ctx context.Context, req ProvisionRequest) (Tenant, ProvisioningJob, error) {
	if s.kc == nil {
		return Tenant{}, ProvisioningJob{}, ErrNotConfigured
	}
	if err := ValidateSlug(req.Slug, s.cfg.ReservedSlugs); err != nil {
		return Tenant{}, ProvisioningJob{}, fmt.Errorf("%w: %v", ErrInvalidSlug, err)
	}
	if _, err := s.store.GetTenant(ctx, req.Slug); err == nil {
		return Tenant{}, ProvisioningJob{}, ErrSlugTaken
	} else if !errors.Is(err, ErrNotFound) {
		return Tenant{}, ProvisioningJob{}, err
	}
	exists, err := s.kc.RealmExists(ctx, req.Slug)
	if err != nil {
		return Tenant{}, ProvisioningJob{}, fmt.Errorf("check realm: %w", err)
	}
	if exists {
		return Tenant{}, ProvisioningJob{}, ErrSlugTaken
	}
	if n, err := s.store.CountTenants(ctx); err == nil && s.cfg.Ceiling > 0 && n >= s.cfg.Ceiling {
		s.log.Warn().Int("count", n).Int("ceiling", s.cfg.Ceiling).Msg("tenant ceiling reached (SC-009)")
	}

	now := s.now()
	t := Tenant{
		Slug: req.Slug, DisplayName: req.DisplayName, Status: StatusProvisioning,
		AdminEmail: req.AdminEmail, RealmName: req.Slug, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.store.CreateTenant(ctx, t); err != nil {
		return Tenant{}, ProvisioningJob{}, err
	}
	job := ProvisioningJob{ID: newJobID(), TenantSlug: req.Slug, Action: ActionProvision, Status: JobRunning, CreatedAt: now, UpdatedAt: now}
	_ = s.store.CreateJob(ctx, job)

	spec := idp.BuildBootstrapSpec(idp.BootstrapParams{
		Slug: req.Slug, DisplayName: req.DisplayName, AdminEmail: req.AdminEmail,
		ConsoleOrigin: s.cfg.ConsoleOrigin, AdminAudience: s.cfg.AdminAudience,
		MCPResource: s.mcpResource(req.Slug),
		AccessTTL:   s.cfg.AccessTTL, SSOIdle: s.cfg.SSOIdle, SSOMax: s.cfg.SSOMax, SSLRequired: s.cfg.SSLRequired,
	})

	realm := spec.Realm.Name
	var consoleID, mcpID, adminUserID string
	steps := []sagaStep{
		// The realm is the only step with an explicit undo: deleting it cascades
		// all sub-objects, so it is the catch-all compensation.
		{name: "realm", do: func() error { return s.kc.CreateRealm(ctx, spec.Realm) }, undo: func() error { return s.kc.DeleteRealm(ctx, realm) }},
		{name: "console-client", do: func() error {
			id, err := s.kc.CreateClient(ctx, realm, spec.ConsoleClient)
			if err != nil {
				return err
			}
			consoleID = id
			for _, m := range spec.ConsoleMappers {
				if err := s.kc.AddProtocolMapper(ctx, realm, consoleID, m); err != nil {
					return err
				}
			}
			return nil
		}},
		{name: "mcp-client", do: func() error {
			id, err := s.kc.CreateClient(ctx, realm, spec.MCPClient)
			if err != nil {
				return err
			}
			mcpID = id
			for _, m := range spec.MCPMappers {
				if err := s.kc.AddProtocolMapper(ctx, realm, mcpID, m); err != nil {
					return err
				}
			}
			return nil
		}},
		{name: "admin-role", do: func() error { return s.kc.CreateRealmRole(ctx, realm, spec.AdminRole) }},
		{name: "admin-user", do: func() error {
			id, err := s.kc.CreateUser(ctx, realm, spec.AdminUser)
			if err != nil {
				return err
			}
			adminUserID = id
			return s.kc.AssignRealmRole(ctx, realm, adminUserID, spec.AdminRole)
		}},
	}

	runErr := s.runSaga(ctx, &job, steps)
	t, _ = s.store.GetTenant(ctx, req.Slug)
	if runErr != nil {
		t.Status = StatusFailed
		t.UpdatedAt = s.now()
		_ = s.store.UpdateTenant(ctx, t)
		job.Status = JobCompensated
		job.Error = runErr.Error()
		job.UpdatedAt = s.now()
		_ = s.store.UpdateJob(ctx, job)
		s.record(ctx, req.Slug, req.Actor, "tenant.provision.failed", runErr.Error())
		return t, job, runErr
	}
	t.Status = StatusActive
	t.SubdomainReady = true
	t.UpdatedAt = s.now()
	_ = s.store.UpdateTenant(ctx, t)
	job.Status = JobSucceeded
	job.UpdatedAt = s.now()
	_ = s.store.UpdateJob(ctx, job)
	s.record(ctx, req.Slug, req.Actor, "tenant.provision.succeeded", "")
	return t, job, nil
}

// Suspend disables the tenant's realm (new/refreshed tokens fail at the gateway).
func (s *Service) Suspend(ctx context.Context, slug, actor string) (Tenant, error) {
	t, err := s.store.GetTenant(ctx, slug)
	if err != nil {
		return Tenant{}, err
	}
	if s.kc == nil {
		return Tenant{}, ErrNotConfigured
	}
	if t.Status == StatusDeleted || t.Status == StatusDeleting {
		return Tenant{}, ErrConflict
	}
	if err := s.kc.SetRealmEnabled(ctx, slug, false); err != nil {
		return Tenant{}, err
	}
	now := s.now()
	t.Status = StatusSuspended
	t.SuspendedAt = &now
	t.UpdatedAt = now
	if err := s.store.UpdateTenant(ctx, t); err != nil {
		return Tenant{}, err
	}
	s.record(ctx, slug, actor, "tenant.suspend", "")
	return t, nil
}

// Resume re-enables a suspended tenant's realm.
func (s *Service) Resume(ctx context.Context, slug, actor string) (Tenant, error) {
	t, err := s.store.GetTenant(ctx, slug)
	if err != nil {
		return Tenant{}, err
	}
	if s.kc == nil {
		return Tenant{}, ErrNotConfigured
	}
	if t.Status != StatusSuspended {
		return Tenant{}, ErrConflict
	}
	if err := s.kc.SetRealmEnabled(ctx, slug, true); err != nil {
		return Tenant{}, err
	}
	now := s.now()
	t.Status = StatusActive
	t.SuspendedAt = nil
	t.UpdatedAt = now
	if err := s.store.UpdateTenant(ctx, t); err != nil {
		return Tenant{}, err
	}
	s.record(ctx, slug, actor, "tenant.resume", "")
	return t, nil
}

// Delete removes the realm (cascading clients/users/creds) and records an audit
// retention deadline >= 1 year (Constitution VI). The WORM audit is retained
// until AuditRetentionUntil, then purged by a separate deferred job.
//
// TODO(T041): also emit pkg/serverevents removals for the org's servers so the
// gateway terminates running instances + revokes injected creds (kill-switch).
func (s *Service) Delete(ctx context.Context, slug, actor string) (Tenant, error) {
	t, err := s.store.GetTenant(ctx, slug)
	if err != nil {
		return Tenant{}, err
	}
	if s.kc == nil {
		return Tenant{}, ErrNotConfigured
	}
	now := s.now()
	t.Status = StatusDeleting
	t.UpdatedAt = now
	_ = s.store.UpdateTenant(ctx, t)
	if err := s.kc.DeleteRealm(ctx, slug); err != nil {
		t.Status = StatusFailed
		t.UpdatedAt = s.now()
		_ = s.store.UpdateTenant(ctx, t)
		return Tenant{}, err
	}
	retDays := s.cfg.AuditRetentionDays
	if retDays < 365 {
		retDays = 365 // Constitution VI floor
	}
	ret := now.AddDate(0, 0, retDays)
	t.Status = StatusDeleted
	t.DeletedAt = &now
	t.AuditRetentionUntil = &ret
	t.SubdomainReady = false
	t.UpdatedAt = now
	_ = s.store.UpdateTenant(ctx, t)
	s.record(ctx, slug, actor, "tenant.delete", "audit retained until "+ret.Format(time.RFC3339))
	return t, nil
}

type sagaStep struct {
	name string
	do   func() error
	undo func() error // optional; run in reverse on later failure
}

func (s *Service) runSaga(ctx context.Context, job *ProvisioningJob, steps []sagaStep) error {
	var done []sagaStep
	for _, st := range steps {
		job.Steps = append(job.Steps, Step{Name: st.name, State: "pending"})
		_ = s.store.UpdateJob(ctx, *job)
		if err := st.do(); err != nil {
			i := len(job.Steps) - 1
			job.Steps[i].State = "failed"
			job.Steps[i].Detail = err.Error()
			_ = s.store.UpdateJob(ctx, *job)
			for j := len(done) - 1; j >= 0; j-- {
				if done[j].undo != nil {
					_ = done[j].undo()
				}
			}
			return fmt.Errorf("step %s: %w", st.name, err)
		}
		job.Steps[len(job.Steps)-1].State = "done"
		_ = s.store.UpdateJob(ctx, *job)
		done = append(done, st)
	}
	return nil
}

func (s *Service) record(ctx context.Context, slug, actor, action, detail string) {
	if s.audit == nil {
		return
	}
	md := map[string]string{}
	if detail != "" {
		md["detail"] = detail
	}
	_ = s.audit.Record(ctx, audit.Event{Time: s.now(), OrgID: slug, Actor: actor, Action: action, Target: slug, Metadata: md})
}

func newJobID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "job_" + hex.EncodeToString(b)
}
