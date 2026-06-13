package admin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestPostgresStore_RoundTrip runs only when MCP_TEST_POSTGRES_DSN is set
// (e.g. the dev Postgres). Keeps `go test ./...` hermetic by default.
func TestPostgresStore_RoundTrip(t *testing.T) {
	dsn := os.Getenv("MCP_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set MCP_TEST_POSTGRES_DSN to run the Postgres integration test")
	}
	ctx := context.Background()
	s, err := NewPostgresStore(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer s.Close()
	// RLS is forced even for the owner, so the cleanup must run under the org context.
	if err := s.withOrg(ctx, "itest-pg", func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM mcp_servers WHERE org_id=$1`, "itest-pg")
		return err
	}); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	created, err := s.Create(Server{
		OrgID: "itest-pg", Slug: "echo", Type: TypeRemoteHTTP, EndpointURL: "http://x",
		AllowedRoles: []string{"eng"}, Enabled: true, Health: HealthHealthy, CreatedAt: time.Now(),
	})
	if err != nil || created.ID == "" {
		t.Fatalf("create: %v %+v", err, created)
	}

	got, err := s.Get("itest-pg", created.ID)
	if err != nil || got.Slug != "echo" || len(got.AllowedRoles) != 1 || got.AllowedRoles[0] != "eng" {
		t.Fatalf("get: %v %+v", err, got)
	}
	if _, err := s.Create(Server{OrgID: "itest-pg", Slug: "echo", Type: TypeRemoteHTTP, EndpointURL: "http://y", CreatedAt: time.Now()}); err != ErrSlugTaken {
		t.Fatalf("want ErrSlugTaken, got %v", err)
	}
	if _, err := s.Get("other-org", created.ID); err != ErrNotFound {
		t.Fatalf("cross-org get should be ErrNotFound (HC-1), got %v", err)
	}
	if l := s.List("itest-pg"); len(l) != 1 {
		t.Fatalf("list: want 1, got %d", len(l))
	}

	created.Enabled = false
	if _, err := s.Update(created); err != nil {
		t.Fatalf("update: %v", err)
	}
	if g, _ := s.Get("itest-pg", created.ID); g.Enabled {
		t.Fatal("update did not persist enabled=false")
	}
	if err := s.Delete("itest-pg", created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get("itest-pg", created.ID); err != ErrNotFound {
		t.Fatalf("after delete want ErrNotFound, got %v", err)
	}
}

// TestPostgresStore_RLS_OrgIsolation proves org isolation is enforced by the
// database itself (HC-1 defense in depth): a query by exact id with NO org
// filter still returns nothing across orgs, because row-level security hides
// rows whose org_id != app.current_org.
//
// RLS is bypassed by superusers (and the dev POSTGRES_USER is one), so — exactly
// as in production — the guarantee is exercised through a restricted, non-
// superuser application role. The owner connection runs migrations and
// provisions that role; the app connection is what RLS actually constrains.
func TestPostgresStore_RLS_OrgIsolation(t *testing.T) {
	dsn := os.Getenv("MCP_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set MCP_TEST_POSTGRES_DSN to run the Postgres RLS test")
	}
	ctx := context.Background()

	owner, err := NewPostgresStore(ctx, dsn) // migrates: enables RLS + policy
	if err != nil {
		t.Fatalf("connect owner: %v", err)
	}
	defer owner.Close()

	const role, pw = "mcp_rls_test", "rlspw"
	for _, stmt := range []string{
		`DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='` + role + `') THEN
		   CREATE ROLE ` + role + ` LOGIN PASSWORD '` + pw + `' NOSUPERUSER NOBYPASSRLS; END IF; END $$;`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON mcp_servers TO ` + role + `;`,
	} {
		if _, err := owner.pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("provision restricted role: %v", err)
		}
	}

	// Connect as the restricted role — this is the connection RLS constrains.
	appCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	appCfg.ConnConfig.User = role
	appCfg.ConnConfig.Password = pw
	appPool, err := pgxpool.NewWithConfig(ctx, appCfg)
	if err != nil {
		t.Fatalf("connect app role: %v", err)
	}
	defer appPool.Close()
	app := &PostgresStore{pool: appPool}

	for _, org := range []string{"rls-a", "rls-b"} {
		if err := app.withOrg(ctx, org, func(tx pgx.Tx) error {
			_, e := tx.Exec(ctx, `DELETE FROM mcp_servers WHERE org_id=$1`, org)
			return e
		}); err != nil {
			t.Fatalf("cleanup %s: %v", org, err)
		}
	}

	srvA, err := app.Create(Server{OrgID: "rls-a", Slug: "a", Type: TypeRemoteHTTP, EndpointURL: "http://a", CreatedAt: time.Now()})
	if err != nil {
		t.Fatalf("create rls-a: %v", err)
	}
	t.Cleanup(func() { _ = app.Delete("rls-a", srvA.ID) })
	srvB, err := app.Create(Server{OrgID: "rls-b", Slug: "secret", Type: TypeRemoteHTTP, EndpointURL: "http://b", CreatedAt: time.Now()})
	if err != nil {
		t.Fatalf("create rls-b: %v", err)
	}
	t.Cleanup(func() { _ = app.Delete("rls-b", srvB.ID) })

	// Count a row by exact id, with no org filter, under a given org context.
	countByID := func(asOrg, id string) int {
		var n int
		if err := app.withOrg(ctx, asOrg, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM mcp_servers WHERE id=$1`, id).Scan(&n)
		}); err != nil {
			t.Fatalf("count as %s: %v", asOrg, err)
		}
		return n
	}

	// Per-org isolation: rls-a cannot see rls-b's row even by exact id.
	if n := countByID("rls-a", srvB.ID); n != 0 {
		t.Fatalf("RLS breach: org rls-a saw %d rows of org rls-b's data (want 0)", n)
	}
	if n := countByID("rls-b", srvB.ID); n != 1 {
		t.Fatalf("org rls-b should see its own row, got %d (want 1)", n)
	}
	// Service context '*' (the gateway's full-catalog reconcile) sees every org.
	if countByID("*", srvA.ID) != 1 || countByID("*", srvB.ID) != 1 {
		t.Fatal("'*' service context must see all orgs' rows (gateway reconcile)")
	}
}
