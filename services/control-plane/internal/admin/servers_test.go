package admin

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/config"
	"github.com/acme-corp/mcp-runtime/pkg/secrets"
)

type recordingSink struct {
	added, removed int
	credChanged    int
	credUsers      []string // userID per CredentialChanged call ("" = org-level)
}

func (r *recordingSink) Add(Server)    { r.added++ }
func (r *recordingSink) Remove(Server) { r.removed++ }
func (r *recordingSink) CredentialChanged(_ Server, userID string) {
	r.credChanged++
	r.credUsers = append(r.credUsers, userID)
}

func setupAdminAPI(t *testing.T) (base, adminTok, userTok string, sink *recordingSink, sec *secrets.MemStore, cleanup func()) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/acme/protocol/openid-connect/certs", func(w http.ResponseWriter, _ *http.Request) {
		eBytes := big.NewInt(int64(key.PublicKey.E)).Bytes()
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{
			"kty": "RSA", "kid": "test", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(eBytes),
		}}})
	})
	idp := httptest.NewServer(mux)
	cfg := &config.Config{
		BaseDomain:             "mcp.example.com",
		KeycloakIssuerTemplate: idp.URL + "/realms/%s",
		AdminAudience:          "https://api.mcp.example.com",
		ShutdownTimeout:        time.Second,
	}
	validator := authz.NewJWTValidator(cfg.BaseDomain, cfg.KeycloakIssuerTemplate, authz.NewJWKSKeySource())
	sink = &recordingSink{}
	sec = secrets.NewMemStore()
	api := NewAPI(cfg, zerolog.Nop(), NewMemStore(), sink, sec, audit.NewMemLogger(), validator)
	srv := httptest.NewServer(api.e)
	adminTok = mint(t, key, idp.URL+"/realms/acme", cfg.AdminAudience, []string{"admin"})
	userTok = mint(t, key, idp.URL+"/realms/acme", cfg.AdminAudience, []string{"user"})
	return srv.URL, adminTok, userTok, sink, sec, func() { srv.Close(); idp.Close() }
}

func TestAdmin_RequiresAdminRole(t *testing.T) {
	base, _, userTok, _, _, cleanup := setupAdminAPI(t)
	defer cleanup()
	if code, _ := do(t, http.MethodGet, base+"/v1/orgs/acme/servers", "", ""); code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", code)
	}
	if code, _ := do(t, http.MethodGet, base+"/v1/orgs/acme/servers", userTok, ""); code != http.StatusForbidden {
		t.Fatalf("non-admin: want 403, got %d", code)
	}
}

func TestAdmin_CRUD_RemoteHTTP(t *testing.T) {
	mcp := fakeMCP(t)
	defer mcp.Close()
	base, adminTok, _, sink, _, cleanup := setupAdminAPI(t)
	defer cleanup()

	code, b := do(t, http.MethodPost, base+"/v1/orgs/acme/servers", adminTok,
		`{"slug":"echo","type":"remote_http","endpoint_url":"`+mcp.URL+`"}`)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, b)
	}
	var created Server
	_ = json.Unmarshal(b, &created)
	if created.ID == "" || created.Health != HealthHealthy {
		t.Fatalf("unexpected created: %+v", created)
	}
	if sink.added != 1 {
		t.Fatalf("expected sink.Add called once, got %d", sink.added)
	}

	if code, _ := do(t, http.MethodPost, base+"/v1/orgs/acme/servers", adminTok,
		`{"slug":"x","type":"remote_http"}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("missing endpoint: want 422, got %d", code)
	}
	if code, _ := do(t, http.MethodPost, base+"/v1/orgs/acme/servers", adminTok,
		`{"slug":"echo","type":"remote_http","endpoint_url":"`+mcp.URL+`"}`); code != http.StatusConflict {
		t.Fatalf("dup slug: want 409, got %d", code)
	}

	if code, lb := do(t, http.MethodGet, base+"/v1/orgs/acme/servers", adminTok, ""); code != http.StatusOK || !bytes.Contains(lb, []byte("echo")) {
		t.Fatalf("list: %d %s", code, lb)
	}

	if code, _ := do(t, http.MethodDelete, base+"/v1/orgs/acme/servers/"+created.ID, adminTok, ""); code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", code)
	}
	if sink.removed != 1 {
		t.Fatalf("expected sink.Remove called once, got %d", sink.removed)
	}
	if code, _ := do(t, http.MethodGet, base+"/v1/orgs/acme/servers/"+created.ID, adminTok, ""); code != http.StatusNotFound {
		t.Fatalf("get after delete: want 404, got %d", code)
	}
}

func TestAdmin_HealthAuthFailed(t *testing.T) {
	unauth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer unauth.Close()
	base, adminTok, _, _, _, cleanup := setupAdminAPI(t)
	defer cleanup()

	code, b := do(t, http.MethodPost, base+"/v1/orgs/acme/servers", adminTok,
		`{"slug":"a","type":"remote_http","endpoint_url":"`+unauth.URL+`"}`)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, b)
	}
	var s Server
	_ = json.Unmarshal(b, &s)
	if s.Health != HealthAuthFailed {
		t.Fatalf("want auth_failed health, got %s", s.Health)
	}
}

func TestAdmin_Credentials_WriteOnly(t *testing.T) {
	mcp := fakeMCP(t)
	defer mcp.Close()
	base, adminTok, _, _, sec, cleanup := setupAdminAPI(t)
	defer cleanup()

	_, b := do(t, http.MethodPost, base+"/v1/orgs/acme/servers", adminTok,
		`{"slug":"s","type":"remote_http","endpoint_url":"`+mcp.URL+`"}`)
	var srv Server
	_ = json.Unmarshal(b, &srv)

	code, body := do(t, http.MethodPut, base+"/v1/orgs/acme/servers/"+srv.ID+"/credentials", adminTok, `{"API_KEY":"shh"}`)
	if code != http.StatusNoContent {
		t.Fatalf("put creds: want 204, got %d %s", code, body)
	}
	if bytes.Contains(body, []byte("shh")) {
		t.Fatal("credential value must never be echoed in the response")
	}
	got, err := sec.Get(context.Background(), secrets.OrgRef("acme", srv.ID))
	if err != nil || got["API_KEY"] != "shh" {
		t.Fatalf("secret not stored in backend: %v %v", got, err)
	}
}

// TestAdmin_CredentialChange_Propagates verifies the control plane signals the
// data plane on credential rotation (T079): org-level with userID "", per-user
// with the caller's id, so the gateway rebuilds the right instance(s).
func TestAdmin_CredentialChange_Propagates(t *testing.T) {
	mcp := fakeMCP(t)
	defer mcp.Close()
	base, adminTok, _, sink, _, cleanup := setupAdminAPI(t)
	defer cleanup()

	_, b := do(t, http.MethodPost, base+"/v1/orgs/acme/servers", adminTok,
		`{"slug":"s","type":"remote_http","endpoint_url":"`+mcp.URL+`"}`)
	var srv Server
	_ = json.Unmarshal(b, &srv)
	if sink.added != 1 {
		t.Fatalf("expected 1 upsert from create, got %d", sink.added)
	}

	if code, _ := do(t, http.MethodPut, base+"/v1/orgs/acme/servers/"+srv.ID+"/credentials", adminTok, `{"API_KEY":"k"}`); code != http.StatusNoContent {
		t.Fatalf("put org creds: %d", code)
	}
	if code, _ := do(t, http.MethodPut, base+"/v1/orgs/acme/servers/"+srv.ID+"/credentials/me", adminTok, `{"API_KEY":"mine"}`); code != http.StatusNoContent {
		t.Fatalf("put my creds: %d", code)
	}

	if sink.credChanged != 2 {
		t.Fatalf("expected 2 credential-changed signals, got %d", sink.credChanged)
	}
	if len(sink.credUsers) != 2 || sink.credUsers[0] != "" || sink.credUsers[1] != "u1" {
		t.Fatalf(`expected [org(""), user("u1")], got %v`, sink.credUsers)
	}
}

func TestAdmin_AuditTrail(t *testing.T) {
	mcp := fakeMCP(t)
	defer mcp.Close()
	base, adminTok, _, _, _, cleanup := setupAdminAPI(t)
	defer cleanup()

	_, b := do(t, http.MethodPost, base+"/v1/orgs/acme/servers", adminTok,
		`{"slug":"s","type":"remote_http","endpoint_url":"`+mcp.URL+`"}`)
	var srv Server
	_ = json.Unmarshal(b, &srv)
	do(t, http.MethodPut, base+"/v1/orgs/acme/servers/"+srv.ID+"/credentials", adminTok, `{"API_KEY":"shh"}`)

	code, ab := do(t, http.MethodGet, base+"/v1/orgs/acme/audit", adminTok, "")
	if code != http.StatusOK {
		t.Fatalf("audit query: %d %s", code, ab)
	}
	if !bytes.Contains(ab, []byte("server.create")) || !bytes.Contains(ab, []byte("credentials.put")) {
		t.Fatalf("audit missing expected actions: %s", ab)
	}
	if bytes.Contains(ab, []byte("shh")) {
		t.Fatal("audit log must never contain secret values")
	}
}

// --- helpers ---

func fakeMCP(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
}

func mint(t *testing.T, key *rsa.PrivateKey, iss, aud string, roles []string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss": iss, "aud": aud, "sub": "u1",
		"exp":          time.Now().Add(time.Hour).Unix(),
		"iat":          time.Now().Unix(),
		"realm_access": map[string]any{"roles": roles},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "test"
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func do(t *testing.T, method, url, token, body string) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = bytes.NewBufferString(body)
	}
	req, _ := http.NewRequest(method, url, r)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}
