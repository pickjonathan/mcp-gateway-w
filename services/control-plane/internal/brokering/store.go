// Package brokering implements org-scoped enterprise SSO brokering config (US4):
// a tenant admin registers an external OIDC/SAML IdP (with group->role mappings)
// that the control plane applies to the tenant's Keycloak realm via idp.Broker.
// The IdP client secret is stored in the secret store (Vault) — only a ref is
// persisted here, never the value.
package brokering

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when an IdP link does not exist in the org.
var ErrNotFound = errors.New("identity provider link not found")

// Link is an org-scoped brokered-IdP configuration. Config holds only non-secret
// settings; SecretRef points at the secret store entry for the client secret.
type Link struct {
	ID           string            `json:"id"`
	OrgID        string            `json:"org_id"`
	Alias        string            `json:"alias"`
	Type         string            `json:"type"` // oidc | saml
	Config       map[string]string `json:"config"`
	SecretRef    string            `json:"-"`
	RoleMappings map[string]string `json:"role_mappings"`
	Enabled      bool              `json:"enabled"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// Store persists org-scoped IdP links (Postgres enforces org isolation via RLS).
type Store interface {
	Upsert(ctx context.Context, l Link) error
	List(ctx context.Context, org string) ([]Link, error)
	Get(ctx context.Context, org, alias string) (Link, error)
	Delete(ctx context.Context, org, alias string) error
}

// MemStore is an in-memory Store for dev and tests.
type MemStore struct {
	mu sync.RWMutex
	m  map[string]Link // org+"/"+alias -> link
}

// NewMemStore returns an empty in-memory store.
func NewMemStore() *MemStore { return &MemStore{m: map[string]Link{}} }

func key(org, alias string) string { return org + "/" + alias }

func (s *MemStore) Upsert(_ context.Context, l Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key(l.OrgID, l.Alias)] = l
	return nil
}

func (s *MemStore) List(_ context.Context, org string) ([]Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Link{}
	for _, l := range s.m {
		if l.OrgID == org {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
	return out, nil
}

func (s *MemStore) Get(_ context.Context, org, alias string) (Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.m[key(org, alias)]
	if !ok {
		return Link{}, ErrNotFound
	}
	return l, nil
}

func (s *MemStore) Delete(_ context.Context, org, alias string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key(org, alias)]; !ok {
		return ErrNotFound
	}
	delete(s.m, key(org, alias))
	return nil
}

// PostgresStore is an org-scoped Store with row-level security.
type PostgresStore struct{ pool *pgxpool.Pool }

// NewPostgresStore connects and ensures the idp_links schema + RLS.
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

// Close releases the pool.
func (s *PostgresStore) Close() { s.pool.Close() }

func (s *PostgresStore) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS idp_links (
  id            text PRIMARY KEY,
  org_id        text NOT NULL,
  alias         text NOT NULL,
  type          text NOT NULL,
  config        text NOT NULL DEFAULT '{}',
  secret_ref    text NOT NULL DEFAULT '',
  role_mappings text NOT NULL DEFAULT '{}',
  enabled       boolean NOT NULL DEFAULT true,
  created_at    timestamptz NOT NULL,
  updated_at    timestamptz NOT NULL,
  UNIQUE (org_id, alias)
);`,
		`ALTER TABLE idp_links ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE idp_links FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS idp_links_org_isolation ON idp_links;`,
		`CREATE POLICY idp_links_org_isolation ON idp_links
  USING (org_id = current_setting('app.current_org', true)
         OR current_setting('app.current_org', true) = '*')
  WITH CHECK (org_id = current_setting('app.current_org', true));`,
	}
	for _, q := range stmts {
		if _, err := s.pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) withOrg(ctx context.Context, org string, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_org', $1, true)`, org); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) Upsert(ctx context.Context, l Link) error {
	return s.withOrg(ctx, l.OrgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO idp_links (id, org_id, alias, type, config, secret_ref, role_mappings, enabled, created_at, updated_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			 ON CONFLICT (org_id, alias) DO UPDATE SET type=$4, config=$5, secret_ref=$6, role_mappings=$7, enabled=$8, updated_at=$10`,
			l.ID, l.OrgID, l.Alias, l.Type, mapJSON(l.Config), l.SecretRef, mapJSON(l.RoleMappings), l.Enabled, l.CreatedAt, l.UpdatedAt)
		return err
	})
}

func (s *PostgresStore) List(ctx context.Context, org string) ([]Link, error) {
	out := []Link{}
	err := s.withOrg(ctx, org, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, org_id, alias, type, config, secret_ref, role_mappings, enabled, created_at, updated_at FROM idp_links WHERE org_id=$1 ORDER BY alias`, org)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			l, err := scanLink(rows)
			if err != nil {
				return err
			}
			out = append(out, l)
		}
		return rows.Err()
	})
	return out, err
}

func (s *PostgresStore) Get(ctx context.Context, org, alias string) (Link, error) {
	var l Link
	err := s.withOrg(ctx, org, func(tx pgx.Tx) error {
		var e error
		l, e = scanLink(tx.QueryRow(ctx, `SELECT id, org_id, alias, type, config, secret_ref, role_mappings, enabled, created_at, updated_at FROM idp_links WHERE org_id=$1 AND alias=$2`, org, alias))
		return e
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	return l, err
}

func (s *PostgresStore) Delete(ctx context.Context, org, alias string) error {
	var n int64
	err := s.withOrg(ctx, org, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM idp_links WHERE org_id=$1 AND alias=$2`, org, alias)
		n = tag.RowsAffected()
		return err
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanLink(sc rowScanner) (Link, error) {
	var l Link
	var config, roles string
	if err := sc.Scan(&l.ID, &l.OrgID, &l.Alias, &l.Type, &config, &l.SecretRef, &roles, &l.Enabled, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return Link{}, err
	}
	_ = json.Unmarshal([]byte(config), &l.Config)
	_ = json.Unmarshal([]byte(roles), &l.RoleMappings)
	return l, nil
}

func mapJSON(m map[string]string) string {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}
