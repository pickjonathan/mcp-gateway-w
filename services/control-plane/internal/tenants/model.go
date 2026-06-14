package tenants

import "time"

// Status is a tenant lifecycle state (data-model.md).
type Status string

const (
	StatusProvisioning Status = "provisioning"
	StatusActive       Status = "active"
	StatusSuspended    Status = "suspended"
	StatusDeleting     Status = "deleting"
	StatusDeleted      Status = "deleted"
	StatusFailed       Status = "failed"
)

// Tenant is the platform-scoped registry record for an org (= a Keycloak realm).
// It is never read by the gateway on the request path (org is derived from Host +
// issuer); it exists for lifecycle, display, and audit.
type Tenant struct {
	Slug                string     `json:"slug"`
	DisplayName         string     `json:"display_name"`
	Status              Status     `json:"status"`
	AdminEmail          string     `json:"admin_email"`
	RealmName           string     `json:"realm_name"`
	SubdomainReady      bool       `json:"subdomain_ready"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	SuspendedAt         *time.Time `json:"suspended_at,omitempty"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty"`
	AuditRetentionUntil *time.Time `json:"audit_retention_until,omitempty"`
}

// JobAction is the lifecycle action a ProvisioningJob performs.
type JobAction string

const (
	ActionProvision JobAction = "provision"
	ActionSuspend   JobAction = "suspend"
	ActionResume    JobAction = "resume"
	ActionDelete    JobAction = "delete"
)

// JobStatus is the saga state of a ProvisioningJob.
type JobStatus string

const (
	JobPending     JobStatus = "pending"
	JobRunning     JobStatus = "running"
	JobSucceeded   JobStatus = "succeeded"
	JobFailed      JobStatus = "failed"
	JobCompensated JobStatus = "compensated"
)

// Step is one saga step with its state, enabling idempotent resume + reverse
// compensation on failure (FR-006).
type Step struct {
	Name   string `json:"name"`
	State  string `json:"state"` // pending | done | failed | compensated
	Detail string `json:"detail,omitempty"`
}

// ProvisioningJob records a lifecycle action and its per-step progress.
type ProvisioningJob struct {
	ID         string    `json:"id"`
	TenantSlug string    `json:"tenant_slug"`
	Action     JobAction `json:"action"`
	Status     JobStatus `json:"status"`
	Steps      []Step    `json:"steps"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
