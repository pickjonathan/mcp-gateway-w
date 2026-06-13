package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a Store backed by PostgreSQL. JSON-valued fields (args, env,
// allowed_roles) are stored as text holding their JSON encoding.
type PostgresStore struct {
	pool *pgxpool.Pool
}

const serverCols = "id, org_id, slug, type, endpoint_url, command, args, env, credential_mode, allowed_roles, enabled, health, health_detail, created_at, credential_set"

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
	// Row-level security enforces org isolation at the database (HC-1 defense in
	// depth): every row is visible only when app.current_org matches its org_id —
	// set per transaction by withOrg. The '*' service context is used solely by
	// the trusted full-catalog read (gateway reconcile). FORCE makes the policy
	// apply even to the table owner; an unset GUC yields NULL → no rows (fail
	// closed).
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS mcp_servers (
  id              text PRIMARY KEY,
  org_id          text NOT NULL,
  slug            text NOT NULL,
  type            text NOT NULL,
  endpoint_url    text NOT NULL DEFAULT '',
  command         text NOT NULL DEFAULT '',
  args            text NOT NULL DEFAULT '',
  env             text NOT NULL DEFAULT '',
  credential_mode text NOT NULL DEFAULT '',
  allowed_roles   text NOT NULL DEFAULT '',
  enabled         boolean NOT NULL DEFAULT true,
  health          text NOT NULL DEFAULT '',
  health_detail   text NOT NULL DEFAULT '',
  created_at      timestamptz NOT NULL,
  UNIQUE (org_id, slug)
);`,
		`ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS credential_set boolean NOT NULL DEFAULT false;`,
		`ALTER TABLE mcp_servers ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE mcp_servers FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS org_isolation ON mcp_servers;`,
		`CREATE POLICY org_isolation ON mcp_servers
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

// withOrg runs fn inside a transaction with the RLS org context set to org, so
// row-level security scopes every statement to that org. Pass "*" only for the
// trusted full-catalog read.
func (s *PostgresStore) withOrg(ctx context.Context, org string, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful commit
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_org', $1, true)`, org); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "srv_" + hex.EncodeToString(b)
}

// Create inserts a new server, mapping a unique-violation to ErrSlugTaken.
func (s *PostgresStore) Create(srv Server) (Server, error) {
	srv.ID = newID()
	args, env, roles := jsonStr(srv.Args), jsonStr(srv.Env), jsonStr(srv.AllowedRoles)
	err := s.withOrg(context.Background(), srv.OrgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			`INSERT INTO mcp_servers (`+serverCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			srv.ID, srv.OrgID, srv.Slug, string(srv.Type), srv.EndpointURL, srv.Command, args, env,
			srv.CredentialMode, roles, srv.Enabled, string(srv.Health), srv.HealthDetail, srv.CreatedAt, srv.CredentialSet)
		return err
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Server{}, ErrSlugTaken
		}
		return Server{}, err
	}
	return srv, nil
}

// Get returns the server scoped to org (cross-org id → ErrNotFound).
func (s *PostgresStore) Get(org, id string) (Server, error) {
	var srv Server
	err := s.withOrg(context.Background(), org, func(tx pgx.Tx) error {
		var e error
		srv, e = scanServer(tx.QueryRow(context.Background(),
			`SELECT `+serverCols+` FROM mcp_servers WHERE org_id=$1 AND id=$2`, org, id))
		return e
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Server{}, ErrNotFound
	}
	if err != nil {
		return Server{}, err
	}
	return srv, nil
}

// List returns all of org's servers (oldest first).
func (s *PostgresStore) List(org string) []Server {
	out := make([]Server, 0)
	_ = s.withOrg(context.Background(), org, func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(),
			`SELECT `+serverCols+` FROM mcp_servers WHERE org_id=$1 ORDER BY created_at`, org)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if srv, err := scanServer(rows); err == nil {
				out = append(out, srv)
			}
		}
		return rows.Err()
	})
	return out
}

// Update replaces an existing server.
func (s *PostgresStore) Update(srv Server) (Server, error) {
	args, env, roles := jsonStr(srv.Args), jsonStr(srv.Env), jsonStr(srv.AllowedRoles)
	var affected int64
	err := s.withOrg(context.Background(), srv.OrgID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(context.Background(),
			`UPDATE mcp_servers SET slug=$3, type=$4, endpoint_url=$5, command=$6, args=$7, env=$8,
			 credential_mode=$9, allowed_roles=$10, enabled=$11, health=$12, health_detail=$13, credential_set=$14
			 WHERE org_id=$1 AND id=$2`,
			srv.OrgID, srv.ID, srv.Slug, string(srv.Type), srv.EndpointURL, srv.Command, args, env,
			srv.CredentialMode, roles, srv.Enabled, string(srv.Health), srv.HealthDetail, srv.CredentialSet)
		affected = tag.RowsAffected()
		return err
	})
	if err != nil {
		return Server{}, err
	}
	if affected == 0 {
		return Server{}, ErrNotFound
	}
	return srv, nil
}

// Delete removes the server scoped to org.
func (s *PostgresStore) Delete(org, id string) error {
	var affected int64
	err := s.withOrg(context.Background(), org, func(tx pgx.Tx) error {
		tag, err := tx.Exec(context.Background(), `DELETE FROM mcp_servers WHERE org_id=$1 AND id=$2`, org, id)
		affected = tag.RowsAffected()
		return err
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanServer(sc rowScanner) (Server, error) {
	var srv Server
	var typ, health, argsS, envS, rolesS string
	if err := sc.Scan(&srv.ID, &srv.OrgID, &srv.Slug, &typ, &srv.EndpointURL, &srv.Command,
		&argsS, &envS, &srv.CredentialMode, &rolesS, &srv.Enabled, &health, &srv.HealthDetail, &srv.CreatedAt, &srv.CredentialSet); err != nil {
		return Server{}, err
	}
	srv.Type = ServerType(typ)
	srv.Health = Health(health)
	jsonUnmarshal(argsS, &srv.Args)
	jsonUnmarshal(envS, &srv.Env)
	jsonUnmarshal(rolesS, &srv.AllowedRoles)
	return srv, nil
}

func jsonStr(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func jsonUnmarshal(s string, dst any) {
	if s == "" {
		return
	}
	_ = json.Unmarshal([]byte(s), dst)
}
