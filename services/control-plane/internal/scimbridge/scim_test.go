package scimbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/idp"
)

// fakeDir records the realm each operation targets, proving per-tenant scoping.
type fakeDir struct {
	created  []string // realm/username
	disabled []string // realm/id
	enabled  []string // realm/id
	roles    []string // realm/role
}

func (f *fakeDir) FindUserByUsername(_ context.Context, realm, username string) (string, bool, error) {
	return "", false, nil
}
func (f *fakeDir) CreateUser(_ context.Context, realm string, u idp.User) (string, error) {
	f.created = append(f.created, realm+"/"+u.Username)
	return "uid-" + u.Username, nil
}
func (f *fakeDir) SetUserEnabled(_ context.Context, realm, id string, en bool) error {
	if en {
		f.enabled = append(f.enabled, realm+"/"+id)
	} else {
		f.disabled = append(f.disabled, realm+"/"+id)
	}
	return nil
}
func (f *fakeDir) AssignRealmRole(_ context.Context, realm, id, role string) error {
	f.roles = append(f.roles, realm+"/"+role)
	return nil
}
func (f *fakeDir) RemoveRealmRole(_ context.Context, realm, id, role string) error { return nil }

type fakeOrgV struct{ role string }

func (f fakeOrgV) ValidateForOrg(_ context.Context, _, org, _ string) (*authz.Principal, error) {
	return &authz.Principal{OrgID: org, UserID: "admin-1", Roles: []string{f.role}}, nil
}

func newServer(dir idp.Directory) (*echo.Echo, Store) {
	store := NewMemStore()
	h := NewHandlers(store, dir, nil, "mcp.example.com")
	e := echo.New()
	RegisterRoutes(e, h, fakeOrgV{role: "admin"}, "https://api.mcp.example.com")
	return e, store
}

func req(e *echo.Echo, method, path, bearer, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("Authorization", "Bearer "+bearer)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, r)
	return rec
}

// configure issues a bearer for org via the org-admin endpoint and returns it.
func configure(t *testing.T, e *echo.Echo, org string) string {
	t.Helper()
	rec := req(e, http.MethodPut, "/v1/orgs/"+org+"/directory-sync", "admintoken",
		`{"group_role_mappings":{"Engineering":"aws-users"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("configure status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Bearer string `json:"bearer"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Bearer == "" {
		t.Fatal("no bearer issued")
	}
	return out.Bearer
}

func TestSCIM_CreateUserScopedToOrg(t *testing.T) {
	dir := &fakeDir{}
	e, _ := newServer(dir)
	bearer := configure(t, e, "globex")
	rec := req(e, http.MethodPost, "/scim/v2/Users", bearer,
		`{"userName":"erin@globex.example","active":true,"groups":[{"value":"Engineering"}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(dir.created) != 1 || dir.created[0] != "globex/erin@globex.example" {
		t.Errorf("user not created in globex realm: %v", dir.created)
	}
	if len(dir.roles) != 1 || dir.roles[0] != "globex/aws-users" {
		t.Errorf("group->role mapping not applied: %v", dir.roles)
	}
}

// SC-005: PATCH active:false disables the user (gateway access removed next token).
func TestSCIM_DeactivateDisablesUser(t *testing.T) {
	dir := &fakeDir{}
	e, _ := newServer(dir)
	bearer := configure(t, e, "globex")
	rec := req(e, http.MethodPatch, "/scim/v2/Users/uid-1", bearer,
		`{"Operations":[{"op":"replace","path":"active","value":false}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(dir.disabled) != 1 || dir.disabled[0] != "globex/uid-1" {
		t.Errorf("deactivation did not disable the user: %v", dir.disabled)
	}
}

// Adversarial: a forged/invalid bearer is rejected (401); no directory op runs.
func TestSCIM_InvalidBearerRejected(t *testing.T) {
	dir := &fakeDir{}
	e, _ := newServer(dir)
	_ = configure(t, e, "globex")
	rec := req(e, http.MethodPost, "/scim/v2/Users", "globex.WRONGSECRET", `{"userName":"x"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
	rec = req(e, http.MethodPost, "/scim/v2/Users", "acme.anything", `{"userName":"x"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown-org bearer status=%d want 401", rec.Code)
	}
	if len(dir.created) != 0 {
		t.Errorf("no user should be created under an invalid bearer: %v", dir.created)
	}
}

// The issued bearer acts only on its own org's realm (structural isolation).
func TestSCIM_BearerBoundToItsOrg(t *testing.T) {
	dir := &fakeDir{}
	e, _ := newServer(dir)
	acme := configure(t, e, "acme")
	_ = configure(t, e, "globex")
	// Use acme's bearer — every op must target the acme realm, never globex.
	rec := req(e, http.MethodPost, "/scim/v2/Users", acme, `{"userName":"u@acme.example","active":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d", rec.Code)
	}
	if len(dir.created) != 1 || !strings.HasPrefix(dir.created[0], "acme/") {
		t.Errorf("acme bearer acted outside acme realm: %v", dir.created)
	}
}

func TestSCIM_GetConfigNoBearerLeak(t *testing.T) {
	e, _ := newServer(&fakeDir{})
	_ = configure(t, e, "globex")
	rec := req(e, http.MethodGet, "/v1/orgs/globex/directory-sync", "admintoken", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "bearer") {
		t.Errorf("GetConfig leaked a bearer: %s", rec.Body.String())
	}
}
