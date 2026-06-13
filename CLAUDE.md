<!-- SPECKIT START -->
## Active feature: Admin Console UI — Carbon Design System (`002-admin-console`)

Plan: `specs/002-admin-console/plan.md` (see also `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`).

A **presentation-only admin console SPA** (React + TypeScript + Vite; Carbon Design System **vendored from the project-root handoff**; OAuth 2.1 + PKCE against the per-org Keycloak realm) that surfaces every admin-facing capability of `001-mcp-server-runtime` — server CRUD, write-only credentials, RBAC, audit, quotas/health — by consuming the existing control-plane API. **No new backend** (only CORS config). Lives under `web/admin-console/`.

**Builds on**: `001-mcp-server-runtime` (Go gateway / control-plane / sandbox-supervisor; Keycloak per-org realms; PostgreSQL + Redis + Vault; OpenTelemetry) — see `specs/001-mcp-server-runtime/`.

**Hard constraints (MUST, inherited)**: organization isolation, secret confidentiality (secrets are write-only — never displayed/copied/logged), frictionless admin. The console MUST NOT weaken these for UX (Constitution IV).
<!-- SPECKIT END -->

## Project overview

A **multi-tenant MCP gateway runtime**. End users of an organization connect to
`{org}.{base-domain}/mcp` over OAuth 2.1; org admins register **remote (HTTP)**
or **stdio** MCP servers, which the gateway aggregates and proxies. Go module:
`github.com/acme-corp/mcp-runtime` (Go 1.25). Full docs in **`docs/`** (also
published via GitHub Pages — see `docs/index.html`).

### Repository layout

| Path | What |
|---|---|
| `services/gateway/` | Data plane: OAuth-protected `/mcp`, aggregation, routing, sandbox/remote downstreams, quotas, SSRF guard, audit, metrics/traces |
| `services/control-plane/` | Admin API: per-org server CRUD, write-only credentials, audit query; publishes change events |
| `services/sandbox-supervisor/` | Sandbox warm pool (on-demand assignment, scale-to-zero) |
| `pkg/` | Shared libs: `config` (env singleton), `logging` (zerolog+redaction), `authz` (JWT/JWKS), `secrets` (Vault), `serverevents` (Redis bus), `audit` (hash chain + S3 WORM), `telemetry` (metrics/traces/redaction) |
| `deploy/dev/` | `compose.yaml` dev stack + Prometheus/Grafana/Jaeger config + Postgres init |
| `specs/001-mcp-server-runtime/` | Spec, plan, research, data-model, contracts, tasks (source of truth for scope) |
| `docs/` | Architecture, features, security, observability, solution comparison + GitHub Pages site |

### Build / test / run

```sh
go build ./...                 # build everything
go test ./...                  # hermetic suite (integration tests skip without MCP_TEST_* env)
go vet ./...
make run                       # gateway on :8080 (exec sandbox = UNSANDBOXED dev only)
/dev-setup                     # skill: verify prereqs + bring up the dev stack
```

Live integration tests are guarded by env vars (run against the dev stack):
`MCP_TEST_POSTGRES_DSN`, `MCP_TEST_REDIS_ADDR`, `MCP_TEST_VAULT_ADDR`, `MCP_TEST_S3_ENDPOINT`.

### Architecture at a glance

- **Control plane** persists server definitions to Postgres (RLS-enforced org
  isolation) and **propagates** changes to the gateway via **Redis pub/sub**;
  the gateway **reconciles from Postgres on startup** as the durability backstop.
- **Data plane** (gateway) authenticates per-org (audience-bound JWT, per-org
  Keycloak realm), enforces RBAC + quotas, injects credentials (org-shared or
  per-user, with rotation), and routes to **remote HTTP** clients or **sandboxed
  stdio** servers. Untrusted stdio code runs in a microVM/gVisor sandbox with no
  network; remote egress is SSRF-guarded.

### Conventions

- **Config**: all env (`MCP_*`) is loaded once into the `config.Config` singleton
  (`config.Get()`); pass it down, don't read env elsewhere.
- **HTTP**: Echo v4; **logging**: zerolog via `pkg/logging` (secrets auto-redacted
  at the writer). **Never log credential values.**
- **Isolation is the top invariant** (HC-1): every catalog/store lookup is
  org-scoped; add a cross-org test when touching that path.
- **Tests**: table-driven, hermetic by default; gate anything needing a backend
  behind an `MCP_TEST_*` env var and `t.Skip`.
- Mark task progress in `specs/001-mcp-server-runtime/tasks.md` (`[X]`/`[~]` with
  a short note) as work lands.
