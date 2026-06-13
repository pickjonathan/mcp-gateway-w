package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/aggregate"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/downstream"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/quota"
)

var acme = &authz.Principal{OrgID: "acme"}

func newTestHandler() *Handler {
	cat := downstream.NewCatalog()
	cat.Add("acme", "demo", &downstream.Fake{
		Tools: []aggregate.Tool{{Name: "echo", Description: "echoes"}},
		Results: map[string]json.RawMessage{
			"echo": json.RawMessage(`{"content":[{"type":"text","text":"hi"}]}`),
		},
	})
	return NewHandler(cat)
}

func req(method, id, params string) *Request {
	r := &Request{JSONRPC: "2.0", Method: method}
	if id != "" {
		r.ID = json.RawMessage(id)
	}
	if params != "" {
		r.Params = json.RawMessage(params)
	}
	return r
}

func TestInitialize(t *testing.T) {
	resp := newTestHandler().Dispatch(context.Background(), acme, req("initialize", "1", ""))
	if resp == nil || resp.Error != nil {
		t.Fatalf("initialize failed: %+v", resp)
	}
	if m := resp.Result.(map[string]any); m["protocolVersion"] != ProtocolVersion {
		t.Fatalf("protocolVersion = %v", m["protocolVersion"])
	}
}

func TestToolsListNamespaced(t *testing.T) {
	resp := newTestHandler().Dispatch(context.Background(), acme, req("tools/list", "2", ""))
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]aggregate.Tool)
	if len(tools) != 1 || tools[0].Name != "demo__echo" {
		t.Fatalf("expected [demo__echo], got %+v", tools)
	}
}

func TestToolsListOrgIsolation(t *testing.T) {
	// A principal from another org sees none of acme's servers (HC-1).
	resp := newTestHandler().Dispatch(context.Background(), &authz.Principal{OrgID: "beta"}, req("tools/list", "9", ""))
	tools := resp.Result.(map[string]any)["tools"].([]aggregate.Tool)
	if len(tools) != 0 {
		t.Fatalf("cross-org leak: beta saw %+v", tools)
	}
}

func TestRBACFiltering(t *testing.T) {
	cat := downstream.NewCatalog()
	cat.AddScoped("acme", "eng", &downstream.Fake{Tools: []aggregate.Tool{{Name: "build"}}}, []string{"engineers"})
	h := NewHandler(cat)

	// A principal WITH the role sees the server.
	r := h.Dispatch(context.Background(), &authz.Principal{OrgID: "acme", Roles: []string{"engineers"}}, req("tools/list", "1", ""))
	if tools := r.Result.(map[string]any)["tools"].([]aggregate.Tool); len(tools) != 1 || tools[0].Name != "eng__build" {
		t.Fatalf("role holder should see eng__build, got %+v", tools)
	}
	// A principal WITHOUT the role sees nothing...
	r = h.Dispatch(context.Background(), &authz.Principal{OrgID: "acme", Roles: []string{"sales"}}, req("tools/list", "2", ""))
	if tools := r.Result.(map[string]any)["tools"].([]aggregate.Tool); len(tools) != 0 {
		t.Fatalf("non-role-holder must see nothing, got %+v", tools)
	}
	// ...and cannot call it (reported as unknown — existence not revealed).
	r = h.Dispatch(context.Background(), &authz.Principal{OrgID: "acme", Roles: []string{"sales"}}, req("tools/call", "3", `{"name":"eng__build","arguments":{}}`))
	if r.Error == nil || r.Error.Code != CodeMethodNotFound {
		t.Fatalf("non-role-holder call should be method-not-found, got %+v", r)
	}
}

func TestQuotaEnforced(t *testing.T) {
	cat := downstream.NewCatalog()
	cat.Add("acme", "demo", &downstream.Fake{
		Tools:   []aggregate.Tool{{Name: "echo"}},
		Results: map[string]json.RawMessage{"echo": json.RawMessage(`{}`)},
	})
	h := NewHandler(cat, WithQuota(quota.NewEnforcer(1, 0, time.Minute))) // org: 1/min
	p := &authz.Principal{OrgID: "acme", UserID: "u1"}
	call := func(id string) *Response {
		return h.Dispatch(context.Background(), p, req("tools/call", id, `{"name":"demo__echo","arguments":{}}`))
	}
	if r := call("1"); r.Error != nil {
		t.Fatalf("first call should pass, got %+v", r.Error)
	}
	if r := call("2"); r.Error == nil || r.Error.Code != CodeRateLimited {
		t.Fatalf("second call should be rate-limited, got %+v", r)
	}
}

func TestToolsCallRoutes(t *testing.T) {
	resp := newTestHandler().Dispatch(context.Background(), acme,
		req("tools/call", "3", `{"name":"demo__echo","arguments":{}}`))
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	if string(raw) != `{"content":[{"type":"text","text":"hi"}]}` {
		t.Fatalf("unexpected passthrough result: %s", raw)
	}
}

func TestToolsCallUnknown(t *testing.T) {
	h := newTestHandler()
	if resp := h.Dispatch(context.Background(), acme,
		req("tools/call", "4", `{"name":"echo","arguments":{}}`)); resp.Error == nil || resp.Error.Code != CodeMethodNotFound {
		t.Fatalf("expected method-not-found for un-namespaced name, got %+v", resp)
	}
	if resp := h.Dispatch(context.Background(), acme,
		req("tools/call", "5", `{"name":"ghost__x","arguments":{}}`)); resp.Error == nil {
		t.Fatalf("expected error for unknown slug, got %+v", resp)
	}
}

func TestNotificationNoResponse(t *testing.T) {
	if resp := newTestHandler().Dispatch(context.Background(), acme,
		req("notifications/initialized", "", "")); resp != nil {
		t.Fatalf("notification must yield no response, got %+v", resp)
	}
}
