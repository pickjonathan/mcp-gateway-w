package server

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acme-corp/mcp-runtime/pkg/serverevents"
)

// PostgresSource is a ServerSource backed by the control-plane's mcp_servers
// table (a read-only projection of the source of truth).
type PostgresSource struct {
	pool *pgxpool.Pool
}

// NewPostgresSource connects to dsn for reconcile reads.
func NewPostgresSource(ctx context.Context, dsn string) (*PostgresSource, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PostgresSource{pool: pool}, nil
}

// Close releases the connection pool.
func (p *PostgresSource) Close() { p.pool.Close() }

// ListEnabled returns every enabled server, across all orgs, as an upsert event.
// This is the trusted full-catalog read: it runs in a transaction with the RLS
// service context ('*') so row-level security returns every org's rows.
func (p *PostgresSource) ListEnabled(ctx context.Context) ([]serverevents.Event, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_org', '*', true)`); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `SELECT id, org_id, slug, type, endpoint_url, command, args, env, credential_mode, allowed_roles
		FROM mcp_servers WHERE enabled = true`)
	if err != nil {
		return nil, err
	}
	var out []serverevents.Event
	for rows.Next() {
		var e serverevents.Event
		var argsS, envS, rolesS string
		if err := rows.Scan(&e.ID, &e.OrgID, &e.Slug, &e.Type, &e.EndpointURL, &e.Command,
			&argsS, &envS, &e.CredentialMode, &rolesS); err != nil {
			rows.Close()
			return nil, err
		}
		e.Action = serverevents.ActionUpsert
		if argsS != "" {
			_ = json.Unmarshal([]byte(argsS), &e.Args)
		}
		if envS != "" {
			_ = json.Unmarshal([]byte(envS), &e.Env)
		}
		if rolesS != "" {
			_ = json.Unmarshal([]byte(rolesS), &e.AllowedRoles)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	return out, tx.Commit(ctx)
}
