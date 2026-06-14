package tenants

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
)

// fakeValidator stands in for the platform-realm token validator.
type fakeValidator struct {
	roles []string
	err   error
}

func (f fakeValidator) ValidateForRealm(_ context.Context, _, _, _ string) (*authz.Principal, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &authz.Principal{OrgID: "_platform", UserID: "op-1", Roles: f.roles}, nil
}

func newTestServer(v RealmValidator) (*echo.Echo, Store) {
	store := NewMemStore()
	svc := NewService(store, newFakeKC(), nil, zerolog.Nop(), Config{
		BaseDomain: "mcp.example.com", ConsoleOrigin: "http://localhost:5173",
		AdminAudience: "https://api.mcp.example.com", ReservedSlugs: "www,api,admin",
		AuditRetentionDays: 365, Ceiling: 200, AccessTTL: 900, SSLRequired: "none",
	})
	e := echo.New()
	RegisterRoutes(e, NewHandlers(svc, store), v, "_platform", "https://platform")
	return e, store
}

func do(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
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

func TestPlatformAPI_CreateAndList(t *testing.T) {
	e, _ := newTestServer(fakeValidator{roles: []string{"platform-admin"}})
	rec := do(e, http.MethodPost, "/v1/platform/tenants", `{"slug":"globex","display_name":"Globex","admin_email":"ops@globex.example"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = do(e, http.MethodGet, "/v1/platform/tenants", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "globex") {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// Adversarial: a token without the platform-admin role (e.g. an org admin token)
// is forbidden on the platform API (HC-1 / Constitution I).
func TestPlatformAPI_RejectsNonPlatformAdmin(t *testing.T) {
	e, _ := newTestServer(fakeValidator{roles: []string{"admin"}})
	rec := do(e, http.MethodGet, "/v1/platform/tenants", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
}

func TestPlatformAPI_RejectsInvalidToken(t *testing.T) {
	e, _ := newTestServer(fakeValidator{err: errors.New("invalid")})
	rec := do(e, http.MethodGet, "/v1/platform/tenants", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestPlatformAPI_ReservedSlug422(t *testing.T) {
	e, _ := newTestServer(fakeValidator{roles: []string{"platform-admin"}})
	rec := do(e, http.MethodPost, "/v1/platform/tenants", `{"slug":"admin","admin_email":"a@b.c"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422", rec.Code)
	}
}

func TestPlatformAPI_DuplicateSlug409(t *testing.T) {
	e, _ := newTestServer(fakeValidator{roles: []string{"platform-admin"}})
	_ = do(e, http.MethodPost, "/v1/platform/tenants", `{"slug":"globex","admin_email":"a@b.c"}`)
	rec := do(e, http.MethodPost, "/v1/platform/tenants", `{"slug":"globex","admin_email":"a@b.c"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409", rec.Code)
	}
}
