package brokering

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/secrets"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/idp"
)

type fakeBroker struct {
	upserts []string // realm/alias/clientSecret
	deletes []string
}

func (f *fakeBroker) UpsertIdentityProvider(_ context.Context, realm string, ip idp.IdentityProvider) error {
	f.upserts = append(f.upserts, realm+"/"+ip.Alias+"/"+ip.Config["clientSecret"])
	return nil
}
func (f *fakeBroker) DeleteIdentityProvider(_ context.Context, realm, alias string) error {
	f.deletes = append(f.deletes, realm+"/"+alias)
	return nil
}

type fakeOrgV struct {
	role string
	err  error
}

func (f fakeOrgV) ValidateForOrg(_ context.Context, _, org, _ string) (*authz.Principal, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &authz.Principal{OrgID: org, UserID: "admin-1", Roles: []string{f.role}}, nil
}

func newServer(v OrgValidator, broker idp.Broker) (*echo.Echo, Store, secrets.Store) {
	store := NewMemStore()
	sec := secrets.NewMemStore()
	h := NewHandlers(store, broker, sec, nil)
	e := echo.New()
	RegisterRoutes(e, h, v, "https://api.mcp.example.com")
	return e, store, sec
}

func req(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, r)
	return rec
}

// Adversarial: the IdP secret is applied to Keycloak and stored in the secret
// store, but is NEVER returned in the API response (secret confidentiality).
func TestBrokering_PutDoesNotLeakSecret(t *testing.T) {
	broker := &fakeBroker{}
	e, store, sec := newServer(fakeOrgV{role: "admin"}, broker)
	body := `{"type":"oidc","config":{"issuer":"https://corp.example"},"secret":"s3cr3t","role_mappings":{"groups.eng":"aws-users"}}`
	rec := req(e, http.MethodPut, "/v1/orgs/globex/identity-providers/corp", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "s3cr3t") || strings.Contains(rec.Body.String(), "secret_ref") {
		t.Errorf("response leaked secret/ref: %s", rec.Body.String())
	}
	// secret was applied to Keycloak and stored in the secret store
	if len(broker.upserts) != 1 || broker.upserts[0] != "globex/corp/s3cr3t" {
		t.Errorf("broker not called with secret: %v", broker.upserts)
	}
	vals, err := sec.Get(context.Background(), "idp/globex/corp")
	if err != nil || vals["clientSecret"] != "s3cr3t" {
		t.Errorf("secret not stored in secret store: %v %v", vals, err)
	}
	// config persisted (role mappings retrievable)
	l, err := store.Get(context.Background(), "globex", "corp")
	if err != nil || l.RoleMappings["groups.eng"] != "aws-users" {
		t.Errorf("link not persisted: %+v %v", l, err)
	}
}

func TestBrokering_InvalidType422(t *testing.T) {
	e, _, _ := newServer(fakeOrgV{role: "admin"}, &fakeBroker{})
	rec := req(e, http.MethodPut, "/v1/orgs/globex/identity-providers/corp", `{"type":"ldap"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422", rec.Code)
	}
}

func TestBrokering_NonAdminForbidden(t *testing.T) {
	e, _, _ := newServer(fakeOrgV{role: "member"}, &fakeBroker{})
	rec := req(e, http.MethodGet, "/v1/orgs/globex/identity-providers", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
}

func TestBrokering_Delete(t *testing.T) {
	broker := &fakeBroker{}
	e, _, _ := newServer(fakeOrgV{role: "admin"}, broker)
	_ = req(e, http.MethodPut, "/v1/orgs/globex/identity-providers/corp", `{"type":"oidc","config":{},"secret":"x"}`)
	rec := req(e, http.MethodDelete, "/v1/orgs/globex/identity-providers/corp", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rec.Code)
	}
	if len(broker.deletes) != 1 {
		t.Errorf("broker delete not called: %v", broker.deletes)
	}
}
