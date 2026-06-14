package scimbridge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when an org has no directory-sync connection.
var ErrNotFound = errors.New("directory-sync connection not found")

// Connection is an org-scoped SCIM directory-sync configuration (one per org). The
// per-tenant bearer is stored only as a hash (shown once at issuance) — never the
// raw value — so it cannot leak from the datastore.
type Connection struct {
	OrgID             string            `json:"org_id"`
	BearerHash        string            `json:"-"`
	GroupRoleMappings map[string]string `json:"group_role_mappings"`
	Status            string            `json:"status"` // active | disabled
	CreatedAt         time.Time         `json:"created_at"`
	LastSyncAt        *time.Time        `json:"last_sync_at,omitempty"`
}

// Store persists org-scoped directory-sync connections (Postgres uses RLS).
type Store interface {
	Upsert(ctx context.Context, c Connection) error
	Get(ctx context.Context, org string) (Connection, error)
	Delete(ctx context.Context, org string) error
	Touch(ctx context.Context, org string, t time.Time) error
}

// NewBearer mints a per-tenant SCIM bearer "{org}.{secret}" and returns the raw
// value (shown once) and the hash to persist.
func NewBearer(org string) (raw, hash string) {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	secret := base64.RawURLEncoding.EncodeToString(b)
	return org + "." + secret, HashSecret(secret)
}

// HashSecret returns the hex SHA-256 of a bearer secret.
func HashSecret(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// ParseBearer splits "{org}.{secret}" (org slugs never contain '.').
func ParseBearer(raw string) (org, secret string, ok bool) {
	i := strings.IndexByte(raw, '.')
	if i <= 0 || i == len(raw)-1 {
		return "", "", false
	}
	return raw[:i], raw[i+1:], true
}

// MemStore is an in-memory Store for dev and tests.
type MemStore struct {
	mu sync.RWMutex
	m  map[string]Connection
}

// NewMemStore returns an empty in-memory store.
func NewMemStore() *MemStore { return &MemStore{m: map[string]Connection{}} }

func (s *MemStore) Upsert(_ context.Context, c Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[c.OrgID] = c
	return nil
}

func (s *MemStore) Get(_ context.Context, org string) (Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.m[org]
	if !ok {
		return Connection{}, ErrNotFound
	}
	return c, nil
}

func (s *MemStore) Delete(_ context.Context, org string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[org]; !ok {
		return ErrNotFound
	}
	delete(s.m, org)
	return nil
}

func (s *MemStore) Touch(_ context.Context, org string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.m[org]
	if !ok {
		return ErrNotFound
	}
	c.LastSyncAt = &t
	s.m[org] = c
	return nil
}

// PostgresStore is an org-scoped Store with row-level security.
type PostgresStore struct{ pool *pgxpool.Pool }

// NewPostgresStore connects and ensures the scim_connections schema + RLS.
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
		`CREATE TABLE IF NOT EXISTS scim_connections (
  org_id              text PRIMARY KEY,
  bearer_hash         text NOT NULL,
  group_role_mappings text NOT NULL DEFAULT '{}',
  status              text NOT NULL,
  created_at          timestamptz NOT NULL,
  last_sync_at        timestamptz
);`,
		`ALTER TABLE scim_connections ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE scim_connections FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS scim_connections_org_isolation ON scim_connections;`,
		`CREATE POLICY scim_connections_org_isolation ON scim_connections
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

func (s *PostgresStore) Upsert(ctx context.Context, c Connection) error {
	return s.withOrg(ctx, c.OrgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO scim_connections (org_id, bearer_hash, group_role_mappings, status, created_at, last_sync_at)
			 VALUES ($1,$2,$3,$4,$5,$6)
			 ON CONFLICT (org_id) DO UPDATE SET bearer_hash=$2, group_role_mappings=$3, status=$4`,
			c.OrgID, c.BearerHash, mapJSON(c.GroupRoleMappings), c.Status, c.CreatedAt, c.LastSyncAt)
		return err
	})
}

func (s *PostgresStore) Get(ctx context.Context, org string) (Connection, error) {
	var c Connection
	var roles string
	err := s.withOrg(ctx, org, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT org_id, bearer_hash, group_role_mappings, status, created_at, last_sync_at FROM scim_connections WHERE org_id=$1`, org).
			Scan(&c.OrgID, &c.BearerHash, &roles, &c.Status, &c.CreatedAt, &c.LastSyncAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Connection{}, ErrNotFound
	}
	if err != nil {
		return Connection{}, err
	}
	_ = json.Unmarshal([]byte(roles), &c.GroupRoleMappings)
	return c, nil
}

func (s *PostgresStore) Delete(ctx context.Context, org string) error {
	var n int64
	err := s.withOrg(ctx, org, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM scim_connections WHERE org_id=$1`, org)
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

func (s *PostgresStore) Touch(ctx context.Context, org string, t time.Time) error {
	return s.withOrg(ctx, org, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE scim_connections SET last_sync_at=$2 WHERE org_id=$1`, org, t)
		return err
	})
}

func mapJSON(m map[string]string) string {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}
