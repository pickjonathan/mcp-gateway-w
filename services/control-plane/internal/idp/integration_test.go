package idp

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// liveClient builds a RESTClient against a real Keycloak, or skips. Set:
//   MCP_TEST_KEYCLOAK_URL        e.g. http://localhost:8081
//   MCP_TEST_KEYCLOAK_ADMIN_CLIENT_ID  e.g. mcp-provisioner (master, service account)
//   MCP_TEST_KEYCLOAK_ADMIN_SECRET     its client secret
func liveClient(t *testing.T) *RESTClient {
	t.Helper()
	url := os.Getenv("MCP_TEST_KEYCLOAK_URL")
	cid := os.Getenv("MCP_TEST_KEYCLOAK_ADMIN_CLIENT_ID")
	sec := os.Getenv("MCP_TEST_KEYCLOAK_ADMIN_SECRET")
	if url == "" || cid == "" || sec == "" {
		t.Skip("set MCP_TEST_KEYCLOAK_URL/_ADMIN_CLIENT_ID/_ADMIN_SECRET to run live Keycloak integration")
	}
	return NewRESTClient(url, "master", cid, sec)
}

// TestIntegration_BootstrapAndLifecycle provisions a throwaway realm end-to-end
// (T018), exercises realm enable/disable (T037) and the directory deactivation
// path (T047) against a real Keycloak, then cleans up.
func TestIntegration_BootstrapAndLifecycle(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()
	realm := fmt.Sprintf("itest%d", time.Now().UnixNano()%1_000_000)
	spec := BuildBootstrapSpec(BootstrapParams{
		Slug: realm, AdminEmail: "it@" + realm + ".test",
		ConsoleOrigin: "http://localhost:5173", AdminAudience: "https://api.mcp.example.com",
		MCPResource: "https://" + realm + ".mcp.example.com/mcp",
		AccessTTL:   900, SSOIdle: 28800, SSOMax: 86400, SSLRequired: "none",
	})

	if err := c.CreateRealm(ctx, spec.Realm); err != nil {
		t.Fatalf("create realm: %v", err)
	}
	defer func() { _ = c.DeleteRealm(ctx, realm) }()

	cid, err := c.CreateClient(ctx, realm, spec.ConsoleClient)
	if err != nil {
		t.Fatalf("create console client: %v", err)
	}
	for _, m := range spec.ConsoleMappers {
		if err := c.AddProtocolMapper(ctx, realm, cid, m); err != nil {
			t.Fatalf("console mapper %s: %v", m.Name, err)
		}
	}
	mcid, err := c.CreateClient(ctx, realm, spec.MCPClient)
	if err != nil {
		t.Fatalf("create mcp client: %v", err)
	}
	for _, m := range spec.MCPMappers {
		if err := c.AddProtocolMapper(ctx, realm, mcid, m); err != nil {
			t.Fatalf("mcp mapper %s: %v", m.Name, err)
		}
	}
	if err := c.CreateRealmRole(ctx, realm, spec.AdminRole); err != nil {
		t.Fatalf("create role: %v", err)
	}
	uid, err := c.CreateUser(ctx, realm, spec.AdminUser)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := c.AssignRealmRole(ctx, realm, uid, spec.AdminRole); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	if exists, err := c.RealmExists(ctx, realm); err != nil || !exists {
		t.Fatalf("realm should exist: exists=%v err=%v", exists, err)
	}

	// Lifecycle: disable then re-enable (suspend/resume).
	if err := c.SetRealmEnabled(ctx, realm, false); err != nil {
		t.Fatalf("disable realm: %v", err)
	}
	if err := c.SetRealmEnabled(ctx, realm, true); err != nil {
		t.Fatalf("enable realm: %v", err)
	}

	// Directory: find the user and deactivate (SCIM active:false path).
	id, found, err := c.FindUserByUsername(ctx, realm, spec.AdminUser.Username)
	if err != nil || !found {
		t.Fatalf("find user: found=%v err=%v", found, err)
	}
	if err := c.SetUserEnabled(ctx, realm, id, false); err != nil {
		t.Fatalf("deactivate user: %v", err)
	}
}
