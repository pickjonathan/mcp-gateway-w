# Contract: Client-facing MCP Endpoint + OAuth Discovery

**Surface**: what an end user's MCP client sees at `https://{org}.withwillow.ai/mcp`.
**Transport**: MCP **Streamable HTTP** (current revision; legacy HTTP+SSE deferred).
**Auth**: OAuth 2.1 Bearer access token issued by the org's Keycloak realm; the gateway is an OAuth 2.0 **protected resource server**.

## 1. Authorization discovery (per MCP authorization spec)

Unauthenticated request → `401 Unauthorized` with:

```
WWW-Authenticate: Bearer resource_metadata="https://{org}.withwillow.ai/.well-known/oauth-protected-resource"
```

| Endpoint | Returns |
|---|---|
| `GET /.well-known/oauth-protected-resource` | RFC 9728 metadata: `resource` = `https://{org}.withwillow.ai/mcp`, `authorization_servers` = [org realm issuer], `scopes_supported` = `["mcp:tools","mcp:resources","mcp:prompts"]` |
| `GET {issuer}/.well-known/oauth-authorization-server` | RFC 8414 metadata (served by Keycloak realm): authorization, token, registration endpoints; `code_challenge_methods_supported` includes `S256` |
| `POST {registration_endpoint}` | RFC 7591 Dynamic Client Registration (policy-gated) |

**Flow**: client discovers AS → (dynamic-registers if needed) → Authorization Code + **PKCE (S256)** → receives access token with `aud=https://{org}.withwillow.ai/mcp` and granted `mcp:*` scopes.

## 2. Token validation (gateway, every request)

1. `Authorization: Bearer <jwt>` present, signature valid against the org realm JWKS.
2. `aud` == this org's MCP URL (**reject otherwise — HC-1**, FR-023).
3. Host subdomain ↔ token realm cross-check (defense-in-depth).
4. Resolve `org_id`, `user_id` (`sub`), `roles`; not expired.
5. On failure → `401`/`403`, audit `auth.denied`, **no server surface exposed** (US1 scenario 3).

## 3. Proxied MCP methods (aggregated, namespaced)

All standard MCP methods are proxied bidirectionally (FR-004). Capability names are namespaced `serverSlug__name` (FR-003).

| Method | Gateway behavior |
|---|---|
| `initialize` | Negotiate; advertise aggregate capabilities of permitted servers |
| `tools/list`, `resources/list`, `prompts/list` | Merge across the user's permitted servers; namespaced names; RBAC-filtered (FR-009) |
| `tools/call`, `resources/read`, `prompts/get` | Route by namespace to the owning server; stream partial results back faithfully |
| `notifications/*` | Forwarded bidirectionally |
| `ping` | Gateway-local liveness |

**Routing**: `serverSlug__tool` → session routing table → remote-HTTP client *or* sandbox stdio bridge.

## 4. Error & isolation semantics

| Condition | Result |
|---|---|
| Downstream server down/slow/erroring | Error scoped to that call; other servers and the session keep working (FR-019); server `health` reflected |
| Tool name collision | Prevented by namespacing; no silent overwrite (US1 scenario 4) |
| Output exceeds size/time limit | Backpressure + truncation/abort per limits (FR-020) |
| Access revoked mid-session | Next call to that server denied promptly (FR-022) |
| Per-user credential missing | Call blocked/prompted, never silently uses another user's credential (US6 scenario 2) |
