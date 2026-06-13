package server

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
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"github.com/acme-corp/mcp-runtime/pkg/config"
	"github.com/acme-corp/mcp-runtime/pkg/secrets"
	"github.com/acme-corp/mcp-runtime/pkg/serverevents"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/aggregate"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/downstream"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/remotehttp"
)

// setupAuthedGateway starts a gateway wired to a fake Keycloak JWKS endpoint for
// realm "acme" and returns the server, gateway URL, a valid token, and cleanup.
func setupAuthedGateway(t *testing.T) (s *Server, gwURL, token string, cleanup func()) {
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
		ShutdownTimeout:        time.Second,
	}
	s = New(cfg, zerolog.Nop())
	gw := httptest.NewServer(s.e)
	tok := mintToken(t, key, idp.URL+"/realms/acme", "https://acme.mcp.example.com/mcp")
	return s, gw.URL, tok, func() { gw.Close(); idp.Close() }
}

func TestMCP_EndToEnd_ListAndCall(t *testing.T) {
	s, gwURL, token, cleanup := setupAuthedGateway(t)
	defer cleanup()
	s.catalog.Add("acme", "demo", &downstream.Fake{
		Tools:   []aggregate.Tool{{Name: "echo"}},
		Results: map[string]json.RawMessage{"echo": json.RawMessage(`{"content":[{"type":"text","text":"hi"}]}`)},
	})

	out := postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	var listResp struct {
		Result struct {
			Tools []aggregate.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &listResp); err != nil {
		t.Fatalf("decode tools/list: %v (%s)", err, out)
	}
	if len(listResp.Result.Tools) != 1 || listResp.Result.Tools[0].Name != "demo__echo" {
		t.Fatalf("expected [demo__echo], got %+v", listResp.Result.Tools)
	}

	out = postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"demo__echo","arguments":{}}}`)
	if !bytes.Contains(out, []byte(`"text":"hi"`)) {
		t.Fatalf("tools/call passthrough missing: %s", out)
	}
}

func TestMCP_EndToEnd_RemoteDownstream(t *testing.T) {
	mcpSrv := fakeMCPServer(t)
	defer mcpSrv.Close()
	s, gwURL, token, cleanup := setupAuthedGateway(t)
	defer cleanup()
	s.catalog.Add("acme", "remote", remotehttp.New(mcpSrv.URL))

	out := postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if !bytes.Contains(out, []byte("remote__ping")) {
		t.Fatalf("expected remote__ping, got %s", out)
	}
	out = postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"remote__ping","arguments":{}}}`)
	if !bytes.Contains(out, []byte("pong")) {
		t.Fatalf("expected pong passthrough, got %s", out)
	}
}

// TestMCP_Propagation_ViaEvents proves a control-plane server-change event
// (upsert/remove) updates what a user sees through the gateway.
func TestMCP_Propagation_ViaEvents(t *testing.T) {
	mcpSrv := fakeMCPServer(t)
	defer mcpSrv.Close()
	s, gwURL, token, cleanup := setupAuthedGateway(t)
	defer cleanup()

	s.applyServerEvent(serverevents.Event{
		Action: serverevents.ActionUpsert, OrgID: "acme", Slug: "evt", Type: "remote_http", EndpointURL: mcpSrv.URL,
	})
	if out := postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`); !bytes.Contains(out, []byte("evt__ping")) {
		t.Fatalf("event-propagated server not visible: %s", out)
	}

	s.applyServerEvent(serverevents.Event{Action: serverevents.ActionRemove, OrgID: "acme", Slug: "evt"})
	if out := postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`); bytes.Contains(out, []byte("evt__ping")) {
		t.Fatalf("server still visible after remove event: %s", out)
	}
}

// TestMCP_OrgCredentialInjection proves an org-shared credential stored for a
// server is injected into the downstream request (US6 / FR-015).
func TestMCP_OrgCredentialInjection(t *testing.T) {
	gotAuth := make(chan string, 4)
	mcpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		select {
		case gotAuth <- r.Header.Get("Authorization"):
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","capabilities":{},"serverInfo":{"name":"f","version":"1"}}}`))
		case "tools/list":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"ping"}]}}`))
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		}
	}))
	defer mcpSrv.Close()

	s, gwURL, token, cleanup := setupAuthedGateway(t)
	defer cleanup()
	if err := s.secrets.Put(context.Background(), secrets.OrgRef("acme", "srv-cred"), map[string]string{"Authorization": "Bearer s3cret"}); err != nil {
		t.Fatal(err)
	}
	s.applyServerEvent(serverevents.Event{
		Action: serverevents.ActionUpsert, OrgID: "acme", ID: "srv-cred", Slug: "cred",
		Type: "remote_http", EndpointURL: mcpSrv.URL, CredentialMode: "org_shared",
	})

	_ = postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	select {
	case auth := <-gotAuth:
		if auth != "Bearer s3cret" {
			t.Fatalf("downstream did not receive injected credential, got %q", auth)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no request reached the downstream")
	}
}

// TestMCP_PerUserCredentialInjection proves per_user mode (US6): the server is
// invisible and uncontacted until the calling user configures credentials, after
// which the downstream receives that user's own credential.
func TestMCP_PerUserCredentialInjection(t *testing.T) {
	gotAuth := make(chan string, 4)
	mcpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		select {
		case gotAuth <- r.Header.Get("Authorization"):
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","capabilities":{},"serverInfo":{"name":"f","version":"1"}}}`))
		case "tools/list":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"ping"}]}}`))
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		}
	}))
	defer mcpSrv.Close()

	s, gwURL, token, cleanup := setupAuthedGateway(t)
	defer cleanup()
	s.applyServerEvent(serverevents.Event{
		Action: serverevents.ActionUpsert, OrgID: "acme", ID: "u-srv", Slug: "usrv",
		Type: "remote_http", EndpointURL: mcpSrv.URL, CredentialMode: "per_user",
	})

	// Before the user configures credentials: no tools, and no request reaches
	// the downstream (the provider errors before building a client).
	if out := postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`); bytes.Contains(out, []byte("usrv__ping")) {
		t.Fatalf("per-user server visible without credentials: %s", out)
	}
	select {
	case <-gotAuth:
		t.Fatal("downstream was contacted before credentials were configured")
	default:
	}

	// Configure this user's (sub=user-1) credentials; now the tool appears and
	// the downstream receives that user's own credential.
	if err := s.secrets.Put(context.Background(), secrets.UserRef("acme", "u-srv", "user-1"),
		map[string]string{"Authorization": "Bearer user1-key"}); err != nil {
		t.Fatal(err)
	}
	if out := postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`); !bytes.Contains(out, []byte("usrv__ping")) {
		t.Fatalf("per-user server not visible after credentials set: %s", out)
	}
	select {
	case auth := <-gotAuth:
		if auth != "Bearer user1-key" {
			t.Fatalf("downstream did not receive the user's credential, got %q", auth)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no request reached the downstream after credentials were set")
	}
}

// TestMCP_PerUserCredentialRotation proves rotation (T079): after the user's
// secret is rotated and a credential-changed event is delivered, the next
// request rebuilds the downstream instance and carries the new credential.
func TestMCP_PerUserCredentialRotation(t *testing.T) {
	gotAuth := make(chan string, 8)
	mcpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		select {
		case gotAuth <- r.Header.Get("Authorization"):
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","capabilities":{},"serverInfo":{"name":"f","version":"1"}}}`))
		case "tools/list":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"ping"}]}}`))
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		}
	}))
	defer mcpSrv.Close()

	s, gwURL, token, cleanup := setupAuthedGateway(t)
	defer cleanup()
	s.applyServerEvent(serverevents.Event{
		Action: serverevents.ActionUpsert, OrgID: "acme", ID: "rot", Slug: "rot",
		Type: "remote_http", EndpointURL: mcpSrv.URL, CredentialMode: "per_user",
	})

	put := func(val string) {
		if err := s.secrets.Put(context.Background(), secrets.UserRef("acme", "rot", "user-1"),
			map[string]string{"Authorization": val}); err != nil {
			t.Fatal(err)
		}
	}
	drain := func() {
		for {
			select {
			case <-gotAuth:
			default:
				return
			}
		}
	}
	// listAndAuth clears stale captures, drives one tools/list, and returns the
	// credential the downstream saw (initialize + list both carry the same value).
	listAndAuth := func(id string) string {
		drain()
		_ = postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":`+id+`,"method":"tools/list"}`)
		select {
		case a := <-gotAuth:
			return a
		case <-time.After(2 * time.Second):
			t.Fatal("no request reached the downstream")
			return ""
		}
	}

	put("Bearer k1")
	if a := listAndAuth("1"); a != "Bearer k1" {
		t.Fatalf("expected the original credential k1, got %q", a)
	}

	// Rotate the secret and deliver the credential-changed signal; the next
	// instance must be rebuilt and use k2.
	put("Bearer k2")
	s.applyServerEvent(serverevents.Event{
		Action: serverevents.ActionCredentialChanged, OrgID: "acme", Slug: "rot", UserID: "user-1",
	})
	if a := listAndAuth("2"); a != "Bearer k2" {
		t.Fatalf("rotation not applied: expected k2, got %q", a)
	}
}

type fakeSource struct{ evs []serverevents.Event }

func (f fakeSource) ListEnabled(context.Context) ([]serverevents.Event, error) { return f.evs, nil }

// TestReconcileOnStartup proves the gateway rebuilds its catalog from the source
// of truth without any live change events (persistence/durability backstop).
func TestReconcileOnStartup(t *testing.T) {
	mcpSrv := fakeMCPServer(t)
	defer mcpSrv.Close()
	s, gwURL, token, cleanup := setupAuthedGateway(t)
	defer cleanup()

	src := fakeSource{evs: []serverevents.Event{{
		Action: serverevents.ActionUpsert, OrgID: "acme", ID: "r1", Slug: "recon",
		Type: "remote_http", EndpointURL: mcpSrv.URL,
	}}}
	if err := s.Reconcile(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	out := postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if !bytes.Contains(out, []byte("recon__ping")) {
		t.Fatalf("reconciled server not visible: %s", out)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	s, gwURL, token, cleanup := setupAuthedGateway(t)
	defer cleanup()
	if s.metrics == nil {
		t.Fatal("metrics not initialized")
	}
	s.catalog.Add("acme", "demo", &downstream.Fake{
		Tools:   []aggregate.Tool{{Name: "echo"}},
		Results: map[string]json.RawMessage{"echo": json.RawMessage(`{"content":[{"type":"text","text":"hi"}]}`)},
	})

	// Drive traffic so both the request counter and the tool-call counter move.
	_ = postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"demo__echo","arguments":{}}}`)

	resp, err := http.Get(gwURL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{"mcp_requests_total", "mcp_tool_calls_total{", `outcome="ok"`} {
		if !bytes.Contains(body, []byte(want)) {
			t.Fatalf("metrics scrape missing %q:\n%s", want, body)
		}
	}
}

func TestMCP_NoToken_401Challenge(t *testing.T) {
	cfg := &config.Config{
		BaseDomain:             "mcp.example.com",
		KeycloakIssuerTemplate: "https://auth.mcp.example.com/realms/%s",
		ShutdownTimeout:        time.Second,
	}
	gw := httptest.NewServer(New(cfg, zerolog.Nop()).e)
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost, gw.URL+"/mcp",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Host = "acme.mcp.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
	if resp.Header.Get("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate challenge")
	}
}

// TestAuthDenied_Audited proves an unauthenticated request is recorded as a
// tamper-evident auth.denied audit event (FR-010), feeding the denial-rate alert.
func TestAuthDenied_Audited(t *testing.T) {
	cfg := &config.Config{
		BaseDomain:             "mcp.example.com",
		KeycloakIssuerTemplate: "https://auth.mcp.example.com/realms/%s",
		ShutdownTimeout:        time.Second,
	}
	s := New(cfg, zerolog.Nop())
	gw := httptest.NewServer(s.e)
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost, gw.URL+"/mcp",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Host = "acme.mcp.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}

	recs, _ := s.audit.List(context.Background(), "acme", 0)
	if len(recs) != 1 || recs[0].Action != "auth.denied" {
		t.Fatalf("expected one auth.denied record, got %+v", recs)
	}
	if recs[0].Metadata["reason"] != "missing_token" {
		t.Fatalf("expected reason=missing_token, got %v", recs[0].Metadata)
	}
	if ok, _ := s.audit.Verify(context.Background()); !ok {
		t.Fatal("audit chain must verify")
	}
}

// TestAuthzDenied_Audited proves an RBAC denial (a user calling a server scoped
// to a role they lack) is recorded as an authz.denied audit event.
func TestAuthzDenied_Audited(t *testing.T) {
	s, gwURL, token, cleanup := setupAuthedGateway(t)
	defer cleanup()
	// Token carries role "engineers"; this server requires "admins".
	s.catalog.AddScoped("acme", "adminonly", &downstream.Fake{
		Tools:   []aggregate.Tool{{Name: "x"}},
		Results: map[string]json.RawMessage{"x": json.RawMessage(`{}`)},
	}, []string{"admins"})

	out := postMCP(t, gwURL, token, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"adminonly__x","arguments":{}}}`)
	if !bytes.Contains(out, []byte("unknown tool")) {
		t.Fatalf("expected RBAC denial response, got %s", out)
	}

	recs, _ := s.audit.List(context.Background(), "acme", 0)
	found := false
	for _, r := range recs {
		if r.Action == "authz.denied" && r.Target == "adminonly__x" && r.Actor == "user-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an authz.denied record for adminonly__x by user-1, got %+v", recs)
	}
}

// TestAuthDenied_AuditRateLimited proves denial-audit writes are bounded
// (anti-amplification): with a limit of 2, four unauthenticated requests record
// only 2 audit events and the 2 drops are counted in mcp_audit_dropped_total.
func TestAuthDenied_AuditRateLimited(t *testing.T) {
	cfg := &config.Config{
		BaseDomain:             "mcp.example.com",
		KeycloakIssuerTemplate: "https://auth.mcp.example.com/realms/%s",
		ShutdownTimeout:        time.Second,
		AuditDenyPerMin:        2,
	}
	s := New(cfg, zerolog.Nop())
	gw := httptest.NewServer(s.e)
	defer gw.Close()

	for i := 0; i < 4; i++ {
		req, _ := http.NewRequest(http.MethodPost, gw.URL+"/mcp",
			bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Host = "acme.mcp.example.com"
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}

	recs, _ := s.audit.List(context.Background(), "acme", 0)
	if len(recs) != 2 {
		t.Fatalf("expected 2 recorded denials (rate-limited), got %d", len(recs))
	}

	mresp, err := http.Get(gw.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(mresp.Body)
	_ = mresp.Body.Close()
	if v := metricValue(body, "mcp_audit_dropped_total"); v != "2" {
		t.Fatalf("expected mcp_audit_dropped_total=2, got %q\n%s", v, body)
	}
}

// metricValue returns the value of the first non-comment Prometheus line whose
// metric name matches.
func metricValue(body []byte, name string) string {
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "#") || !strings.HasPrefix(line, name) {
			continue
		}
		if f := strings.Fields(line); len(f) >= 2 {
			return f[len(f)-1]
		}
	}
	return ""
}

// --- helpers ---

func fakeMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"fake","version":"1"}}}`))
		case "tools/list":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"ping"}]}}`))
		case "tools/call":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"pong"}]}}`))
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"nope"}}`))
		}
	}))
}

func postMCP(t *testing.T, base, token, body string) []byte {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+"/mcp", bytes.NewBufferString(body))
	req.Host = "acme.mcp.example.com"
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
	}
	return raw
}

func mintToken(t *testing.T, key *rsa.PrivateKey, iss, aud string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss":          iss,
		"aud":          aud,
		"sub":          "user-1",
		"exp":          time.Now().Add(time.Hour).Unix(),
		"iat":          time.Now().Unix(),
		"realm_access": map[string]any{"roles": []string{"engineers"}},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "test"
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
