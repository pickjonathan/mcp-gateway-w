<!-- SPECKIT START -->
## Active feature: Admin Console UI — Carbon Design System (`002-admin-console`)

Plan: `specs/002-admin-console/plan.md` (see also `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`).

A **presentation-only admin console SPA** (React + TypeScript + Vite; Carbon Design System **vendored from the project-root handoff**; OAuth 2.1 + PKCE against the per-org Keycloak realm) that surfaces every admin-facing capability of `001-mcp-server-runtime` — server CRUD, write-only credentials, RBAC, audit, quotas/health — by consuming the existing control-plane API. **No new backend** (only CORS config). Lives under `web/admin-console/`.

**Builds on**: `001-mcp-server-runtime` (Go gateway / control-plane / sandbox-supervisor; Keycloak per-org realms; PostgreSQL + Redis + Vault; OpenTelemetry) — see `specs/001-mcp-server-runtime/`.

**Hard constraints (MUST, inherited)**: organization isolation, secret confidentiality (secrets are write-only — never displayed/copied/logged), frictionless admin. The console MUST NOT weaken these for UX (Constitution IV).

### Active: Automated Tenant Provisioning (`003-tenant-provisioning`)

Docs: `docs/tenant-provisioning.md`. Spec/plan/tasks/contracts: `specs/003-tenant-provisioning/`. **US1–US4 implemented + tested (live-validated against the dev Keycloak); US5 (self-service signup) deferred.** Run: `make seed-platform`, then start the control-plane with `MCP_KEYCLOAK_ADMIN_CLIENT_ID=mcp-provisioner` + the printed `MCP_KEYCLOAK_ADMIN_SECRET` (+ `MCP_PLATFORM_REALM=_platform`), then `make provision-tenant SLUG=… NAME=… ADMIN_EMAIL=…`.

A **control-plane capability to provision/populate/deprovision isolated tenants**. A tenant **is** a Keycloak realm; bootstrap (Half A) uses the **Keycloak Admin API** (an in-house `net/http` client in `idp/`, the programmatic `seed-keycloak.sh`), and user provisioning (Half B) ships **invites + OIDC/SAML brokering + SCIM** in v1. Operator-initiated (self-service signup deferred); realm-per-tenant (tens–low-hundreds); delete purges identity/creds/servers (kill-switch via `serverevents`) but **retains WORM audit ≥ 1y** (Constitution VI). New control-plane packages (`tenants`, `idp`, `invites`, `brokering`, `scimbridge`) + a **platform realm/`platform-admin`** authz (`authz.ValidateForRealm`) + a privileged Keycloak service-account credential in Vault. **Gateway unchanged** (org still derived from Host + issuer; suspend = realm-disable).

### Active: Two-Tenant AWS-MCP Isolation Proof (`004-aws-mcp-isolation-proof`)

Plan: `specs/004-aws-mcp-isolation-proof/plan.md` (see also `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`, `tasks.md`). Docs: `docs/isolation-proof.md`.

**Status (PR #12): 38/39 tasks done.** The only production change — the egress allowlist — is **verified** (Go build/vet/test green on 21 pkgs; written-first `egress_test.go` red→green). The Node harness (`tests/isolation-proof/`) **type-checks (`tsc` exit 0) and runs** (fails closed at the FR-018 preflight, exit 2, without a stack). **Remaining: T038** — the live gVisor PASS run. Run: `bash .claude/skills/dev-setup/scripts/sandbox-up.sh` → `make dev-up && make seed-platform && make sandbox-image` → `make run-gateway-proof` → `make prove-isolation`.

The **end-to-end, adversarial, load-tested acceptance proof of tenant isolation** (Constitution V) using a real workload: **two tenants** (`alpha`/`beta`), each with its own **stdio AWS MCP server** (`awslabs.aws-api-mcp-server`, pre-baked in `deploy/sandbox-images/Dockerfile`) run **under gVisor**, its own **AWS account + write-only creds**, and its own **S3 bucket** in a **`ministack`** emulator added to `deploy/dev/compose.yaml`. A Node harness (`tests/isolation-proof/`, `make prove-isolation`) drives `npx @modelcontextprotocol/inspector` for functional + adversarial checks, an MCP-SDK driver for the smoke stress run, queries audit for denials, and emits a machine-readable pass/fail report. **Mostly composition** of `001`/`003` APIs; the **only production change** is one additive, default-off gateway config **`MCP_SANDBOX_EGRESS_NETWORK`** (wired into `sandbox.ContainerRuntime.Network`, default `""`→`none`) that completes Constitution II's "default-deny **with an explicit allowlist**" so the gVisor sandbox can reach **only** the emulator (Docker `internal` network) — shipped with **written-first adversarial egress tests**. **FR-018 gate**: a preflight aborts loudly if `ministack` can't isolate S3 per account (it isolates by 12-digit-access-key = account-id namespacing, not IAM denial). Run locally as the release gate; CI deferred.
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
go build ./... && go test ./... && go vet ./...   # hermetic (integration tests skip without MCP_TEST_*)
make dev-up                    # docker infra (postgres/redis/vault/keycloak/minio/prometheus/grafana/jaeger) + seeds Keycloak
make run-control-plane         # admin API :8090           (own terminal)
make run-gateway               # data plane /mcp :8080      (exec sandbox = UNSANDBOXED dev only; `make run` aliases this)
cd web/admin-console && npm run dev   # admin console :5173
```

Full walkthrough + dashboards table in the **README "Run it locally"**. Dashboards:
console :5173 · Jaeger (traces) :16686 · Grafana :3000 · Prometheus :9090 · MinIO :9001 · Keycloak :8081.
Dev logins (seeded by `deploy/dev/seed-keycloak.sh`): `admin`/`admin` (org admin), `alice`/`alice` (member).
The `run-*` targets bake in the dev env (local Keycloak issuer, OTLP→Jaeger, exec sandbox).

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

### Connecting MCP clients (data plane)

- Clients (MCP Inspector, Claude Desktop, `mcp-remote`) connect to
  `{org}.{base-domain}/mcp` (dev: `http://acme.mcp.example.com:8080/mcp` + a
  `127.0.0.1 acme.mcp.example.com` hosts entry) via OAuth 2.1 + PKCE against the
  org's Keycloak realm.
- `deploy/dev/seed-keycloak.sh` registers two per-realm OAuth clients:
  `mcp-admin-console` (console; admin-API audience) and `mcp-client` (data plane;
  audience = the MCP resource URL).
- The gateway advertises the MCP **resource** (RFC 9728) and requires it as the
  token audience. `MCP_RESOURCE_TEMPLATE` overrides the canonical
  `https://{org}.{base}/mcp`; dev uses `http://%s.mcp.example.com:8080/mcp` so the
  Inspector's OAuth flow works over plain HTTP (set its **Client ID `mcp-client`**
  to skip Dynamic Client Registration).
- Tool visibility/use is gated by per-server `allowed_roles` × the user's realm
  roles. See `docs/multi-tenant-keycloak.md` and `docs/mcp-inspector-rbac.md`.

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
