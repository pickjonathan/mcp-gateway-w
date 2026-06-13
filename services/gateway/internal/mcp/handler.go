package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/aggregate"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/downstream"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/quota"
)

// ProtocolVersion is the MCP revision the gateway advertises (baseline).
const ProtocolVersion = "2025-03-26"

// Handler dispatches MCP JSON-RPC requests, aggregating and routing across the
// requesting principal's organization's downstream servers.
type Handler struct {
	catalog    *downstream.Catalog
	serverName string
	quota      *quota.Enforcer // nil = unlimited
	onDeny     DenyRecorder    // nil = no audit
}

// DenyRecorder records an authorization denial for audit (FR-010). target is the
// requested tool/server; reason is a short machine code.
type DenyRecorder func(ctx context.Context, org, user, target, reason string)

// Option configures a Handler.
type Option func(*Handler)

// WithQuota sets the per-org/per-user rate limiter (FR-017).
func WithQuota(e *quota.Enforcer) Option { return func(h *Handler) { h.quota = e } }

// WithDenyRecorder sets the audit hook invoked on an RBAC denial.
func WithDenyRecorder(r DenyRecorder) Option { return func(h *Handler) { h.onDeny = r } }

// NewHandler builds a dispatcher over the per-org catalog.
func NewHandler(catalog *downstream.Catalog, opts ...Option) *Handler {
	h := &Handler{catalog: catalog, serverName: "acme-mcp-gateway"}
	for _, o := range opts {
		o(h)
	}
	return h
}

// Dispatch handles one request and returns a response, or nil for a
// notification. Aggregation/routing is scoped to the principal's org (HC-1).
func (h *Handler) Dispatch(ctx context.Context, p *authz.Principal, req *Request) *Response {
	if req.JSONRPC != "2.0" && req.JSONRPC != "" {
		return newError(req.ID, CodeInvalidRequest, "unsupported jsonrpc version")
	}
	org, user, roles := "", "", []string(nil)
	if p != nil {
		org, user, roles = p.OrgID, p.UserID, p.Roles
	}
	switch req.Method {
	case MethodInitialize:
		return newResult(req.ID, map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": h.serverName, "version": "0.1.0"},
		})
	case MethodPing:
		return newResult(req.ID, map[string]any{})
	case MethodInitialized:
		return nil
	case MethodToolsList:
		return newResult(req.ID, map[string]any{"tools": h.aggregate(ctx, org, user, roles).Tools})
	case MethodToolsCall:
		return h.toolsCall(ctx, org, user, roles, req)
	default:
		if req.IsNotification() {
			return nil
		}
		return newError(req.ID, CodeMethodNotFound, "method not found: "+req.Method)
	}
}

func (h *Handler) toolsCall(ctx context.Context, org, user string, roles []string, req *Request) *Response {
	if !h.quota.Allow(org, user) {
		return newError(req.ID, CodeRateLimited, "rate limit exceeded")
	}
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
		return newError(req.ID, CodeInvalidParams, "invalid tools/call params")
	}
	slug, tool, ok := strings.Cut(p.Name, aggregate.Separator)
	reg := h.catalog.For(org)
	// Enforce RBAC on the call, not just listing (FR-009/FR-022). An unauthorized
	// or unknown server is reported identically so we never reveal its existence.
	if !ok || !reg.CanAccess(slug, roles) {
		if h.onDeny != nil {
			h.onDeny(ctx, org, user, p.Name, "forbidden_or_unknown_server")
		}
		return newError(req.ID, CodeMethodNotFound, "unknown tool: "+p.Name)
	}
	ds, err := reg.GetForUser(slug, user)
	if err != nil {
		if errors.Is(err, downstream.ErrNotFound) {
			return newError(req.ID, CodeInternal, "server unavailable: "+slug)
		}
		// Per-user instance could not be built — typically no credentials
		// configured for this user. Safe to name slug: RBAC already passed.
		return newError(req.ID, CodeInvalidParams, "credentials not configured for "+slug)
	}
	out, err := ds.CallTool(ctx, tool, p.Arguments)
	if err != nil {
		// Isolate downstream failure so one server's error never breaks others (FR-019).
		return newError(req.ID, CodeInternal, err.Error())
	}
	return newResult(req.ID, json.RawMessage(out))
}

// aggregate merges the org's servers' tools into one namespaced surface,
// skipping servers that fail to list (failure isolation). For per-user servers
// the instance is resolved with the caller's identity, so a user without
// credentials simply sees none of that server's tools.
func (h *Handler) aggregate(ctx context.Context, org, user string, roles []string) aggregate.Result {
	reg := h.catalog.For(org)
	var servers []aggregate.ServerTools
	for _, slug := range reg.VisibleSlugs(roles) {
		ds, err := reg.GetForUser(slug, user)
		if err != nil {
			continue
		}
		tools, err := ds.ListTools(ctx)
		if err != nil {
			continue
		}
		servers = append(servers, aggregate.ServerTools{Slug: slug, Tools: tools})
	}
	return aggregate.Aggregate(servers)
}
