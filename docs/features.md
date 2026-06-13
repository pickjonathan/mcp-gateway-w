# Features

Organized by the seven user stories (US1–US7). Each lists what it delivers and
the key implementation surface.

## US1 — End user connects and uses tools

- OAuth-protected `POST /mcp` (JSON-RPC 2.0, MCP Streamable HTTP).
- `initialize`, `ping`, `tools/list`, `tools/call`, notifications.
- Unauthenticated requests get an **RFC 9728 401 challenge** and see **no server
  surface**.
- Tools from all of the org's servers are aggregated into one namespaced list
  (`serverSlug__tool`).

## US2 — Admin adds a remote (HTTP) MCP server

- Control-plane CRUD registers a `remote_http` server with an `endpoint_url`.
- The gateway speaks the MCP Streamable HTTP transport to it (session init, SSE +
  JSON responses, header injection).
- **SSRF protection**: outbound calls are refused to non-public IPs
  (loopback/private/link-local/metadata), checked at dial time (defeats DNS
  rebinding).

## US3 — Admin adds a stdio MCP server (untrusted)

- Register a `stdio` server with `{command, args, env}` (e.g. `npx -y …`).
- Runs in a **sandbox** (microVM/Kata/gVisor) with **no network**, dropped
  capabilities, read-only rootfs, and CPU/mem/pid limits.
- **Warm pool** with on-demand assignment and **scale-to-zero** keeps cold-start
  low without paying for idle capacity.
- Validated by a live **adversarial containment** suite.

## US4 — Secure execution of any MCP (hard constraint)

- Untrusted code cannot reach the cloud metadata endpoint, internal services,
  other tenants, or the host filesystem.
- Kill-switch: removing a server terminates its running instance(s).
- Remote and stdio egress are both constrained (SSRF guard + no-network sandbox).

## US5 — RBAC

- Servers carry `allowed_roles`; an empty set means visible to all org members.
- RBAC is enforced on **both** `tools/list` (filtering) and `tools/call`
  (an unauthorized server is indistinguishable from a missing one).
- Denials are audited (`authz.denied`).

## US6 — Credentials

- Three modes per server: `none`, `org_shared`, `per_user`.
- **Write-only** storage (Vault KV v2) — values are never echoed or logged.
- **Injection**: org-shared creds baked into the downstream at build; per-user
  creds resolved lazily into a per-`(org,user,server)` instance using the
  caller's own secret.
- **Rotation**: a credential change propagates so the next instance uses the new
  secret (per-user → targeted cache invalidation; org → instance rebuild).
- **Redaction**: a log-writer scrubs auth headers/tokens/keys regardless of how a
  field was added.

## US7 — Operations: quotas, audit, observability

- **Quotas / rate limits**: per-org and per-user fixed windows, independent so one
  tenant can't starve another; a **Redis-backed** limiter makes limits fleet-wide.
- **Audit**: tamper-evident SHA-256 **hash chain** with a durable **Object-Lock
  (WORM)** S3 archive; admin audit-query API; `auth.denied` / `authz.denied`
  events with anti-amplification rate-limiting.
- **Observability**: structured logs (zerolog), **metrics** (OTel → Prometheus),
  **traces** (OTel → Jaeger, propagated across services), and **Grafana
  dashboards + alert rules** (target-down, error rate, isolation-denial rate).

## Cross-cutting

- **Config**: single `MCP_*`-prefixed env singleton.
- **Org isolation** (HC-1): per-org catalog + audience-bound tokens + Postgres RLS.
- **Frictionless** (HC-2): one admin call to add a server; the platform runs it.
- **Propagation durability**: Redis live events + Postgres reconcile-on-startup.

For exact task-level status see `specs/001-mcp-server-runtime/tasks.md`.
