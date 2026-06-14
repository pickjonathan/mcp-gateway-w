package invites

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/idp"
)

// fakeKC implements idp.Keycloak; only CreateUser/AssignRealmRole are exercised.
type fakeKC struct{ users, roles []string }

func (f *fakeKC) RealmExists(context.Context, string) (bool, error)              { return true, nil }
func (f *fakeKC) CreateRealm(context.Context, idp.Realm) error                   { return nil }
func (f *fakeKC) UpdateRealm(context.Context, idp.Realm) error                   { return nil }
func (f *fakeKC) SetRealmEnabled(context.Context, string, bool) error            { return nil }
func (f *fakeKC) DeleteRealm(context.Context, string) error                      { return nil }
func (f *fakeKC) CreateClient(context.Context, string, idp.Client) (string, error) { return "cid", nil }
func (f *fakeKC) AddProtocolMapper(context.Context, string, string, idp.ProtocolMapper) error {
	return nil
}
func (f *fakeKC) CreateRealmRole(context.Context, string, string) error { return nil }
func (f *fakeKC) CreateUser(_ context.Context, realm string, u idp.User) (string, error) {
	f.users = append(f.users, realm+"/"+u.Email)
	return "uid-1", nil
}
func (f *fakeKC) AssignRealmRole(_ context.Context, realm, uid, role string) error {
	f.roles = append(f.roles, realm+"/"+role)
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

func newServer(v OrgValidator, kc idp.Keycloak) (*echo.Echo, Store, *string) {
	store := NewMemStore()
	captured := new(string)
	h := NewHandlers(store, kc, nil, zerolog.Nop(), func(_, raw string) { *captured = raw })
	e := echo.New()
	RegisterRoutes(e, h, v, "https://api.mcp.example.com")
	return e, store, captured
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

func TestInvite_CreateDoesNotLeakToken(t *testing.T) {
	e, _, captured := newServer(fakeOrgV{role: "admin"}, &fakeKC{})
	rec := req(e, http.MethodPost, "/v1/orgs/globex/invitations", `{"email":"dana@globex.example","roles":["member"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "token") || strings.Contains(rec.Body.String(), *captured) {
		t.Errorf("response leaked the invite token: %s", rec.Body.String())
	}
	if *captured == "" {
		t.Error("token was not delivered to the notifier")
	}
}

func TestInvite_AcceptCreatesUserInOrg(t *testing.T) {
	kc := &fakeKC{}
	e, _, captured := newServer(fakeOrgV{role: "admin"}, kc)
	_ = req(e, http.MethodPost, "/v1/orgs/globex/invitations", `{"email":"dana@globex.example","roles":["member","aws-users"]}`)
	rec := req(e, http.MethodPost, "/v1/invitations:accept", `{"token":"`+*captured+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(kc.users) != 1 || kc.users[0] != "globex/dana@globex.example" {
		t.Errorf("user not created in globex realm: %v", kc.users)
	}
	if len(kc.roles) != 2 {
		t.Errorf("roles not assigned: %v", kc.roles)
	}
}

func TestInvite_AcceptExpiredIsGone(t *testing.T) {
	kc := &fakeKC{}
	e, store, captured := newServer(fakeOrgV{role: "admin"}, kc)
	_ = req(e, http.MethodPost, "/v1/orgs/globex/invitations", `{"email":"x@globex.example","roles":[]}`)
	// expire it directly in the store
	list, _ := store.List(context.Background(), "globex")
	inv := list[0]
	inv.ExpiresAt = time.Now().Add(-time.Hour)
	_ = store.Update(context.Background(), inv)
	rec := req(e, http.MethodPost, "/v1/invitations:accept", `{"token":"`+*captured+`"}`)
	if rec.Code != http.StatusGone {
		t.Fatalf("status=%d want 410", rec.Code)
	}
}

func TestInvite_NonAdminForbidden(t *testing.T) {
	e, _, _ := newServer(fakeOrgV{role: "member"}, &fakeKC{})
	rec := req(e, http.MethodGet, "/v1/orgs/globex/invitations", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
}

func TestInvite_InvalidTokenUnauthorized(t *testing.T) {
	e, _, _ := newServer(fakeOrgV{err: errors.New("bad")}, &fakeKC{})
	rec := req(e, http.MethodGet, "/v1/orgs/globex/invitations", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

// Adversarial: an invitation created in one org is invisible to another (store
// scoping; RLS in Postgres). Mirrors the cross-tenant isolation invariant (HC-1).
func TestInvite_CrossOrgIsolation(t *testing.T) {
	store := NewMemStore()
	inv := Invitation{ID: "inv_1", OrgID: "acme", Email: "a@acme.example", Status: StatusPending, ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now()}
	if err := store.Create(context.Background(), inv); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "globex", "inv_1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("globex must not read acme's invitation: got %v", err)
	}
	if out, _ := store.List(context.Background(), "globex"); len(out) != 0 {
		t.Errorf("globex List must be empty, got %d", len(out))
	}
}
