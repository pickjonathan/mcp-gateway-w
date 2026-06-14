package tenants

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func newTestService(kc *fakeKC) (*Service, Store) {
	store := NewMemStore()
	svc := NewService(store, kc, nil, zerolog.Nop(), Config{
		BaseDomain: "mcp.example.com", ConsoleOrigin: "http://localhost:5173",
		AdminAudience: "https://api.mcp.example.com", ReservedSlugs: "www,api,admin",
		AuditRetentionDays: 365, Ceiling: 200, AccessTTL: 900, SSOIdle: 28800, SSOMax: 86400, SSLRequired: "none",
	})
	return svc, store
}

func TestProvision_Success(t *testing.T) {
	kc := newFakeKC()
	svc, store := newTestService(kc)
	tn, job, err := svc.Provision(context.Background(), ProvisionRequest{Slug: "globex", DisplayName: "Globex", AdminEmail: "ops@globex.example"})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if tn.Status != StatusActive {
		t.Errorf("tenant status=%s want active", tn.Status)
	}
	if job.Status != JobSucceeded {
		t.Errorf("job status=%s want succeeded", job.Status)
	}
	if !kc.realms["globex"] {
		t.Error("realm not created")
	}
	// the full bootstrap ran: both clients + role + user
	for _, want := range []string{"CreateClient:globex:mcp-admin-console", "CreateClient:globex:mcp-client", "CreateRealmRole:admin", "CreateUser:ops@globex.example", "AssignRealmRole:admin"} {
		if !kc.called(want) {
			t.Errorf("missing bootstrap call %q", want)
		}
	}
	got, _ := store.GetTenant(context.Background(), "globex")
	if got.Status != StatusActive || !got.SubdomainReady {
		t.Errorf("stored tenant=%+v", got)
	}
}

func TestProvision_DuplicateSlug(t *testing.T) {
	kc := newFakeKC()
	svc, _ := newTestService(kc)
	if _, _, err := svc.Provision(context.Background(), ProvisionRequest{Slug: "globex", AdminEmail: "a@b.c"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := svc.Provision(context.Background(), ProvisionRequest{Slug: "globex", AdminEmail: "a@b.c"})
	if !errors.Is(err, ErrSlugTaken) {
		t.Errorf("got %v want ErrSlugTaken (idempotency / SC-008)", err)
	}
}

func TestProvision_ReservedSlug(t *testing.T) {
	kc := newFakeKC()
	svc, _ := newTestService(kc)
	_, _, err := svc.Provision(context.Background(), ProvisionRequest{Slug: "admin", AdminEmail: "a@b.c"})
	if !errors.Is(err, ErrInvalidSlug) {
		t.Errorf("got %v want ErrInvalidSlug", err)
	}
	if kc.called("CreateRealm:admin") {
		t.Error("no Keycloak object must be created for an invalid slug (FR-008)")
	}
}

// Adversarial: a mid-saga failure must compensate (delete the realm) — no ghost
// realm — and leave the tenant failed (FR-006 / SC-003).
func TestProvision_CompensatesNoGhostRealm(t *testing.T) {
	kc := newFakeKC()
	kc.failOn = "CreateUser" // realm + 2 clients + role succeed, then user fails
	svc, store := newTestService(kc)
	_, job, err := svc.Provision(context.Background(), ProvisionRequest{Slug: "globex", AdminEmail: "a@b.c"})
	if err == nil {
		t.Fatal("expected provision to fail")
	}
	if kc.realms["globex"] {
		t.Error("ghost realm: the realm must be deleted by compensation")
	}
	if !kc.called("DeleteRealm:globex") {
		t.Error("compensation did not delete the realm")
	}
	if job.Status != JobCompensated {
		t.Errorf("job status=%s want compensated", job.Status)
	}
	got, _ := store.GetTenant(context.Background(), "globex")
	if got.Status != StatusFailed {
		t.Errorf("tenant status=%s want failed", got.Status)
	}
}

func TestSuspendResume(t *testing.T) {
	kc := newFakeKC()
	svc, _ := newTestService(kc)
	if _, _, err := svc.Provision(context.Background(), ProvisionRequest{Slug: "globex", AdminEmail: "a@b.c"}); err != nil {
		t.Fatal(err)
	}
	tn, err := svc.Suspend(context.Background(), "globex", "op")
	if err != nil {
		t.Fatal(err)
	}
	if tn.Status != StatusSuspended || kc.enabled["globex"] {
		t.Errorf("suspend failed: status=%s enabled=%v", tn.Status, kc.enabled["globex"])
	}
	tn, err = svc.Resume(context.Background(), "globex", "op")
	if err != nil {
		t.Fatal(err)
	}
	if tn.Status != StatusActive || !kc.enabled["globex"] {
		t.Errorf("resume failed: status=%s enabled=%v", tn.Status, kc.enabled["globex"])
	}
}

// Delete removes the realm and retains the WORM audit >= 1 year (Constitution VI).
func TestDelete_RetainsAuditOneYear(t *testing.T) {
	kc := newFakeKC()
	svc, _ := newTestService(kc)
	if _, _, err := svc.Provision(context.Background(), ProvisionRequest{Slug: "tmp", AdminEmail: "a@b.c"}); err != nil {
		t.Fatal(err)
	}
	tn, err := svc.Delete(context.Background(), "tmp", "op")
	if err != nil {
		t.Fatal(err)
	}
	if tn.Status != StatusDeleted || kc.realms["tmp"] {
		t.Errorf("delete failed: status=%s realmExists=%v", tn.Status, kc.realms["tmp"])
	}
	if tn.DeletedAt == nil || tn.AuditRetentionUntil == nil {
		t.Fatal("deletedAt / auditRetentionUntil must be set")
	}
	minRetention := tn.DeletedAt.AddDate(1, 0, 0).Add(-time.Hour)
	if tn.AuditRetentionUntil.Before(minRetention) {
		t.Errorf("audit retention %v is < ~1 year after delete %v", tn.AuditRetentionUntil, tn.DeletedAt)
	}
}

type fakePurger struct{ purged []string }

func (f *fakePurger) PurgeOrg(_ context.Context, org string) (int, error) {
	f.purged = append(f.purged, org)
	return 1, nil
}

// Delete fires the kill-switch (purges the org's servers) before removing the realm.
func TestDelete_FiresKillSwitch(t *testing.T) {
	kc := newFakeKC()
	svc, _ := newTestService(kc)
	p := &fakePurger{}
	svc.SetServerPurger(p)
	if _, _, err := svc.Provision(context.Background(), ProvisionRequest{Slug: "tmp", AdminEmail: "a@b.c"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Delete(context.Background(), "tmp", "op"); err != nil {
		t.Fatal(err)
	}
	if len(p.purged) != 1 || p.purged[0] != "tmp" {
		t.Errorf("kill-switch not fired for the deleted tenant: %v", p.purged)
	}
}
