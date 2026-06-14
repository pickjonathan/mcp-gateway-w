package invites

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

// ErrNotFound is returned when an invitation does not exist (in the given org).
var ErrNotFound = errors.New("invitation not found")

// Status is an invitation lifecycle state.
type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusRevoked  Status = "revoked"
	StatusExpired  Status = "expired"
)

// Invitation is an org-scoped pending grant to join a tenant with assigned roles.
// The raw accept token is emailed once and never stored — only its hash.
type Invitation struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"org_id"`
	Email      string     `json:"email"`
	Roles      []string   `json:"roles"`
	TokenHash  string     `json:"-"`
	Status     Status     `json:"status"`
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
}

// Store persists org-scoped invitations. Postgres enforces org isolation via RLS;
// the in-memory store mirrors the scoping for dev/tests.
type Store interface {
	Create(ctx context.Context, inv Invitation) error
	List(ctx context.Context, org string) ([]Invitation, error)
	Get(ctx context.Context, org, id string) (Invitation, error)
	GetByTokenHash(ctx context.Context, org, hash string) (Invitation, error)
	Update(ctx context.Context, inv Invitation) error
}

// MemStore is an in-memory Store for dev and tests.
type MemStore struct {
	mu sync.RWMutex
	m  map[string]Invitation // id -> invitation
}

// NewMemStore returns an empty in-memory invitation store.
func NewMemStore() *MemStore { return &MemStore{m: map[string]Invitation{}} }

func (s *MemStore) Create(_ context.Context, inv Invitation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[inv.ID] = inv
	return nil
}

func (s *MemStore) List(_ context.Context, org string) ([]Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Invitation{}
	for _, inv := range s.m {
		if inv.OrgID == org {
			out = append(out, inv)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *MemStore) Get(_ context.Context, org, id string) (Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inv, ok := s.m[id]
	if !ok || inv.OrgID != org {
		return Invitation{}, ErrNotFound
	}
	return inv, nil
}

func (s *MemStore) GetByTokenHash(_ context.Context, org, hash string) (Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, inv := range s.m {
		if inv.OrgID == org && inv.TokenHash == hash {
			return inv, nil
		}
	}
	return Invitation{}, ErrNotFound
}

func (s *MemStore) Update(_ context.Context, inv Invitation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[inv.ID]; !ok {
		return ErrNotFound
	}
	s.m[inv.ID] = inv
	return nil
}

// PostgresStore is an org-scoped Store backed by PostgreSQL with row-level
// security (same policy shape as mcp_servers): a row is visible only when
// app.current_org matches its org_id (set per transaction by withOrg).
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore connects, verifies, and ensures the invitations schema + RLS.
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
		`CREATE TABLE IF NOT EXISTS invitations (
  id          text PRIMARY KEY,
  org_id      text NOT NULL,
  email       text NOT NULL,
  roles       text NOT NULL DEFAULT '[]',
  token_hash  text NOT NULL,
  status      text NOT NULL,
  expires_at  timestamptz NOT NULL,
  created_by  text NOT NULL,
  created_at  timestamptz NOT NULL,
  accepted_at timestamptz
);`,
		`ALTER TABLE invitations ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE invitations FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS invitations_org_isolation ON invitations;`,
		`CREATE POLICY invitations_org_isolation ON invitations
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

const invCols = "id, org_id, email, roles, token_hash, status, expires_at, created_by, created_at, accepted_at"

func (s *PostgresStore) Create(ctx context.Context, inv Invitation) error {
	return s.withOrg(ctx, inv.OrgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO invitations (`+invCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			inv.ID, inv.OrgID, inv.Email, rolesJSON(inv.Roles), inv.TokenHash, string(inv.Status),
			inv.ExpiresAt, inv.CreatedBy, inv.CreatedAt, inv.AcceptedAt)
		return err
	})
}

func (s *PostgresStore) List(ctx context.Context, org string) ([]Invitation, error) {
	out := []Invitation{}
	err := s.withOrg(ctx, org, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+invCols+` FROM invitations WHERE org_id=$1 ORDER BY created_at`, org)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			inv, err := scanInv(rows)
			if err != nil {
				return err
			}
			out = append(out, inv)
		}
		return rows.Err()
	})
	return out, err
}

func (s *PostgresStore) Get(ctx context.Context, org, id string) (Invitation, error) {
	return s.one(ctx, org, `WHERE org_id=$1 AND id=$2`, org, id)
}

func (s *PostgresStore) GetByTokenHash(ctx context.Context, org, hash string) (Invitation, error) {
	return s.one(ctx, org, `WHERE org_id=$1 AND token_hash=$2`, org, hash)
}

func (s *PostgresStore) one(ctx context.Context, org, where string, args ...any) (Invitation, error) {
	var inv Invitation
	err := s.withOrg(ctx, org, func(tx pgx.Tx) error {
		var e error
		inv, e = scanInv(tx.QueryRow(ctx, `SELECT `+invCols+` FROM invitations `+where, args...))
		return e
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Invitation{}, ErrNotFound
	}
	return inv, err
}

func (s *PostgresStore) Update(ctx context.Context, inv Invitation) error {
	var n int64
	err := s.withOrg(ctx, inv.OrgID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE invitations SET status=$3, accepted_at=$4 WHERE org_id=$1 AND id=$2`,
			inv.OrgID, inv.ID, string(inv.Status), inv.AcceptedAt)
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

func scanInv(sc rowScanner) (Invitation, error) {
	var inv Invitation
	var roles, status string
	if err := sc.Scan(&inv.ID, &inv.OrgID, &inv.Email, &roles, &inv.TokenHash, &status,
		&inv.ExpiresAt, &inv.CreatedBy, &inv.CreatedAt, &inv.AcceptedAt); err != nil {
		return Invitation{}, err
	}
	inv.Status = Status(status)
	_ = json.Unmarshal([]byte(roles), &inv.Roles)
	return inv, nil
}

func rolesJSON(roles []string) string {
	b, err := json.Marshal(roles)
	if err != nil {
		return "[]"
	}
	return string(b)
}
