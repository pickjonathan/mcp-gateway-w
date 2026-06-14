package tenants

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a Store backed by PostgreSQL. The tenant registry is
// platform-scoped (no RLS): only the platform API reaches it.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore connects to dsn, verifies connectivity, and ensures the schema.
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	s := &PostgresStore{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the connection pool.
func (s *PostgresStore) Close() { s.pool.Close() }

func (s *PostgresStore) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS tenants (
  slug                  text PRIMARY KEY,
  display_name          text NOT NULL,
  status                text NOT NULL,
  admin_email           text NOT NULL,
  realm_name            text NOT NULL,
  subdomain_ready       boolean NOT NULL DEFAULT false,
  created_at            timestamptz NOT NULL,
  updated_at            timestamptz NOT NULL,
  suspended_at          timestamptz,
  deleted_at            timestamptz,
  audit_retention_until timestamptz
);`,
		`CREATE TABLE IF NOT EXISTS provisioning_jobs (
  id           text PRIMARY KEY,
  tenant_slug  text NOT NULL,
  action       text NOT NULL,
  status       text NOT NULL,
  steps        text NOT NULL DEFAULT '[]',
  error        text NOT NULL DEFAULT '',
  created_at   timestamptz NOT NULL,
  updated_at   timestamptz NOT NULL
);`,
	}
	for _, q := range stmts {
		if _, err := s.pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

const tenantCols = "slug, display_name, status, admin_email, realm_name, subdomain_ready, created_at, updated_at, suspended_at, deleted_at, audit_retention_until"

func (s *PostgresStore) CreateTenant(ctx context.Context, t Tenant) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO tenants (`+tenantCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		t.Slug, t.DisplayName, string(t.Status), t.AdminEmail, t.RealmName, t.SubdomainReady,
		t.CreatedAt, t.UpdatedAt, t.SuspendedAt, t.DeletedAt, t.AuditRetentionUntil)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrSlugTaken
	}
	return err
}

func (s *PostgresStore) GetTenant(ctx context.Context, slug string) (Tenant, error) {
	t, err := scanTenant(s.pool.QueryRow(ctx, `SELECT `+tenantCols+` FROM tenants WHERE slug=$1`, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	return t, err
}

func (s *PostgresStore) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+tenantCols+` FROM tenants ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Tenant, 0)
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *PostgresStore) UpdateTenant(ctx context.Context, t Tenant) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE tenants SET display_name=$2, status=$3, admin_email=$4, realm_name=$5, subdomain_ready=$6,
		 updated_at=$7, suspended_at=$8, deleted_at=$9, audit_retention_until=$10 WHERE slug=$1`,
		t.Slug, t.DisplayName, string(t.Status), t.AdminEmail, t.RealmName, t.SubdomainReady,
		t.UpdatedAt, t.SuspendedAt, t.DeletedAt, t.AuditRetentionUntil)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) CountTenants(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM tenants WHERE status <> 'deleted'`).Scan(&n)
	return n, err
}

func (s *PostgresStore) CreateJob(ctx context.Context, j ProvisioningJob) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO provisioning_jobs (id, tenant_slug, action, status, steps, error, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		j.ID, j.TenantSlug, string(j.Action), string(j.Status), stepsJSON(j.Steps), j.Error, j.CreatedAt, j.UpdatedAt)
	return err
}

func (s *PostgresStore) UpdateJob(ctx context.Context, j ProvisioningJob) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE provisioning_jobs SET status=$2, steps=$3, error=$4, updated_at=$5 WHERE id=$1`,
		j.ID, string(j.Status), stepsJSON(j.Steps), j.Error, j.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) GetJob(ctx context.Context, id string) (ProvisioningJob, error) {
	var j ProvisioningJob
	var action, status, steps string
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_slug, action, status, steps, error, created_at, updated_at FROM provisioning_jobs WHERE id=$1`, id).
		Scan(&j.ID, &j.TenantSlug, &action, &status, &steps, &j.Error, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProvisioningJob{}, ErrNotFound
	}
	if err != nil {
		return ProvisioningJob{}, err
	}
	j.Action, j.Status = JobAction(action), JobStatus(status)
	_ = json.Unmarshal([]byte(steps), &j.Steps)
	return j, nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanTenant(sc rowScanner) (Tenant, error) {
	var t Tenant
	var status string
	if err := sc.Scan(&t.Slug, &t.DisplayName, &status, &t.AdminEmail, &t.RealmName, &t.SubdomainReady,
		&t.CreatedAt, &t.UpdatedAt, &t.SuspendedAt, &t.DeletedAt, &t.AuditRetentionUntil); err != nil {
		return Tenant{}, err
	}
	t.Status = Status(status)
	return t, nil
}

func stepsJSON(steps []Step) string {
	b, err := json.Marshal(steps)
	if err != nil {
		return "[]"
	}
	return string(b)
}
