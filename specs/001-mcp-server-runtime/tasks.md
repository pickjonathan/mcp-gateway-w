# Tasks: Multi-Tenant MCP Server Runtime

**Input**: Design documents from `/specs/001-mcp-server-runtime/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: INCLUDED and **mandatory** per Constitution v1.0.0 Principle V — every external interface gets a contract test, and every change touching isolation/authorization/sandboxing/secrets gets a failing-first adversarial test. (This overrides the template's "tests optional" default.)

**Organization**: Tasks are grouped by user story (priorities from spec.md). Stories are independently testable increments. Paths follow the multi-service layout in plan.md (`services/`, `pkg/`, `deploy/`, `migrations/`, `tests/`).

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: parallelizable (different files, no dependency on an incomplete task)
- **[Story]**: user-story label (US1–US7); omitted for Setup/Foundational/Polish

---

## Implementation progress — 2026-06-13

First increment built & verified (`go build` / `vet` / `test` green; gateway boots and serves):
- **Setup**: single root Go module `github.com/acme-corp/mcp-runtime` (go.work split deferred), `Makefile`, `.golangci.yml`, `.gitignore`/`.dockerignore`, GitHub Actions CI, `deploy/dev/compose.yaml`, `services/gateway/Dockerfile`.
- **Config singleton** (`pkg/config`): all env loaded once into a shared `Config` via `sync.Once` (`config.Get()`); unit-tested.
- **Logging** (`pkg/logging`): structured **zerolog**, configured from `Config`.
- **Gateway** (`services/gateway`, **Echo**): health/readiness, RFC 9728 `/.well-known/oauth-protected-resource`, `WWW-Authenticate` 401 challenge on `/mcp` (US1 scenario 3), graceful shutdown.

**US1 is functionally complete & verified** (17 tests passing). The full path works end-to-end against a fake downstream: token validation (T010, `pkg/authz`) → middleware (T027) → namespacing/aggregation (T029) → JSON-RPC dispatch for `initialize`/`tools/list`/`tools/call` (T030/T032) with result passthrough. An end-to-end server test stands up a fake Keycloak JWKS endpoint, mints a token, and drives `tools/list`+`tools/call` over HTTP (T026).

Deferred within US1 (non-blocking): SSE/streaming responses (currently single JSON response) and Redis-backed session caching (currently per-request in-memory routing, T031). (The `auth.denied` audit event is now implemented — see T033.)

**US2 core delivered & verified**: a real Streamable-HTTP `downstream.Downstream` (`internal/remotehttp`) speaks MCP to a remote server (initialize + session id, tools/list/call, JSON + SSE parsing, result passthrough, per-request timeout). The gateway end-to-end test registers a remote server and proves `tools/list` (namespaced `remote__ping`) + `tools/call` (pong passthrough) over HTTP with real auth. **20 tests passing.**

**US2 complete** (control-plane delivered): a new **control-plane** service exposes org-scoped, admin-role-guarded server CRUD (`/v1/orgs/{org}/servers`) with validation, an MCP `initialize` **health probe** (healthy/unreachable/auth_failed), and a `Sink` that emits add/remove events for the data plane. **23 tests passing.**

**Control-plane → gateway propagation is wired (T040, Redis):** `pkg/serverevents` provides a `Bus` with a Redis pub/sub backend (prod) and an in-process `MemBus` (dev/test); the control-plane `busSink` publishes upsert/remove events and the gateway subscribes, building `remotehttp` clients into a **per-org catalog**. The gateway data plane is now correctly **org-scoped** (a user only ever sees their org's servers — HC-1, with a test proving no cross-org leak). Full-stack **docker-compose** (Postgres, Redis, Vault, Keycloak, gateway, control-plane) added and `docker compose config`-validated. **26 tests passing.**

**US3 stdio execution path delivered** (`services/gateway/internal/sandbox`): a pluggable `SandboxRuntime` (interface + `ExecRuntime` dev backend + `Select` factory) and an MCP **stdio bridge** (newline-delimited JSON-RPC: initialize + tools/list/call + passthrough). The gateway builds `sandbox.Server`s from stdio events into the per-org catalog; the control-plane publishes command/args/env. Verified over both an in-process pipe and a **real subprocess**. **29 tests passing.**

**Explicitly deferred to the Linux/prod isolation layer** (the HC-3 boundary; not runnable on this macOS host — see `sandbox-runtime-options.md`): the gVisor/Firecracker-Kata `Runtime` backends (T047), default-deny egress (T049), CPU/mem/pid/disk limits + timeouts (T050), warm pool / scale-to-zero (T048), the base sandbox image (T046), and the **adversarial containment suite** (T043 / US4 / SC-002–003) which must run on a real sandbox. The `exec` backend is dev-only and clearly labeled UNSANDBOXED. A DB-backed reconcile/persistence layer (T007) remains the propagation durability backstop.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Repo skeleton, tooling, dev stack.

- [X] T001 Create monorepo structure (`services/{gateway,control-plane,sandbox-supervisor,sandbox-agent}/`, `pkg/`, `deploy/`, `migrations/`, `tests/`) per plan.md — *gateway + pkg + deploy created; remaining service dirs added with their phases*
- [X] T002 Initialize Go workspace + modules — *single root module `github.com/acme-corp/mcp-runtime` (Go 1.25); go.work per-service split deferred until modules diverge*
- [X] T003 [P] Configure linting/formatting (`golangci-lint`) and `Makefile` targets in repo root
- [X] T004 [P] Add CI pipeline with build/lint/test in `.github/workflows/ci.yml` — *security-suite + contract-test gates wired in as US1/US3/US4 land*
- [X] T005 [P] Implement OpenTelemetry bootstrap (traces/metrics/logs) in `pkg/telemetry/` — *All three signals: **logs** (zerolog + secret redaction, T080); **metrics** (OTel→Prometheus `mcp_requests_total`/`mcp_tool_calls_total` at `/metrics` on both services, Prometheus in compose); **traces** (`telemetry.NewTracing`: OTLP/HTTP exporter via `MCP_OTLP_ENDPOINT`, no-op export when unset, global W3C TraceContext+Baggage propagator). Gateway emits a server span per request (`tracingMiddleware`: method/route/status/`mcp.org`, errors recorded) and propagates `traceparent` to remote downstreams (`remotehttp.applyHeaders`); spans flushed on shutdown; Jaeger added to dev compose (UI :16686). **Both services emit request spans** — the control-plane has the same `tracingMiddleware` (method/route/status/path-org), so a trace flows admin → (Redis) → gateway → downstream. Tests: `TestNewTracing_PropagatorRoundTrip`, gateway + control-plane `TestTracingMiddleware_RecordsSpan`.*
- [X] T006 Create dev stack (Keycloak, PostgreSQL, Redis, Vault) in `deploy/dev/compose.yaml` — *Envoy edge config follows in T020*

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure all stories depend on.

**⚠️ CRITICAL**: No user-story work begins until this phase is complete.

- [~] T007 PostgreSQL persistence — *`admin.PostgresStore` (pgx v5) persists **server definitions** (org-scoped `mcp_servers`, schema-on-init); wired via `MCP_POSTGRES_DSN`; **proven live** (`TestPostgresStore_RoundTrip`: CRUD + slug-uniqueness + cross-org isolation). Remaining: persist users/roles/credentials-refs/audit/usage + a real migrations framework.*
- [X] T008 Row-Level Security — *`mcp_servers` now has RLS **enabled + FORCEd** with policy `org_isolation` USING `org_id = current_setting('app.current_org')` (WITH CHECK exact-match on writes; `'*'` read-only service context for the gateway's full-catalog reconcile; unset GUC → NULL → no rows = fail-closed). `PostgresStore.withOrg` wraps every statement in a tx that sets the org GUC via `set_config(...,true)` (tx-local, pool-safe); all CRUD refactored through it. Gateway `PostgresSource.ListEnabled` uses the `'*'` context. **Proven live** by `TestPostgresStore_RLS_OrgIsolation` through a non-superuser role (a query by exact id with no org filter returns 0 cross-org rows; `'*'` sees all) — the app-layer `org_id` filters remain as the first layer. **Now active in dev too:** RLS is bypassed by superusers, so a non-superuser runtime role `mcp_app` is provisioned by a Postgres init script (`deploy/dev/postgres-init/01-app-role.sql`, granted `CREATE` so its migrations own the tables → `FORCE` applies to it) and both services connect as it (`deploy/dev/compose.yaml`). Verified on a fresh DB: `mcp_app` migrates + CRUDs as a non-superuser owner (`RoundTrip`), and a no-`WHERE` cross-org query under `mcp_app` returns 0 (org o1 cannot see org o2's row) — RLS genuinely enforced for the runtime role. (Takes effect on a fresh data dir: recreate the postgres container.) Prod must likewise run under a non-superuser role.*
- [ ] T009 [P] Implement MCP protocol library (Streamable HTTP transport, JSON-RPC framing, message/notification types) in `pkg/mcp/`
- [X] T010 [P] Implement OAuth2 resource-server token validation (JWKS, signature, **audience binding**, org/user/role claim resolution) in `pkg/authz/` — *RS256, per-org issuer+audience binding, JWKS fetch/cache; 5 unit tests incl. cross-org audience rejection*
- [X] FR-010 audit emission — *control-plane records `server.create/delete/update` + `credentials.put/delete` (values never recorded); verified by `TestAdmin_AuditTrail`*
- [ ] T011 [P] Implement RBAC evaluation library (principal → server/tool decisions) in `pkg/rbac/`
- [X] T012 [P] Implement audit library in `pkg/audit/` — *append-only, **hash-chained** `Logger` (`Record`/`List`/`Verify`) → tamper-evidence; `MemLogger` for dev. Tested (chain verify + tamper detection + org-scoped list). Durable Object-Lock archive behind the same interface delivered in T087 (`S3Archive`).*
- [ ] T013 Implement Keycloak per-org realm provisioning (realm template, `mcp:tools|resources|prompts` client scopes, audience mappers, dynamic-registration policies) in `deploy/keycloak/` and `services/control-plane/internal/idp/`
- [ ] T014 [P] Implement Vault secrets client (envelope encryption; org + per-user paths) in `pkg/secrets/`
- [ ] T015 [P] Implement Redis session/routing/quota accessors in `pkg/store/`
- [X] T016 Scaffold gateway service (cmd, config singleton, health, graceful shutdown) in `services/gateway/` — *Echo framework; OTel wiring with T005*
- [ ] T017 Scaffold control-plane service (cmd, REST router, admin authz middleware, health) in `services/control-plane/`
- [ ] T018 Scaffold sandbox-supervisor + gRPC server (EnsureSession/OpenStream/StopSession/Health per `contracts/sandbox-supervisor.md`) in `services/sandbox-supervisor/`
- [ ] T019 Scaffold sandbox-agent (in-VM init + control-channel client) in `services/sandbox-agent/`
- [ ] T020 Configure Envoy edge: wildcard TLS `*.withwillow.ai` + subdomain→gateway routing in `deploy/edge/`
- [ ] T021 [P] Create contract-test harness (MCP endpoint + admin API) in `tests/contract/harness.go`
- [ ] T022 [P] Create adversarial security harness (2-org/2-user fixtures + hostile MCP server images) in `tests/security/harness.go`

**Checkpoint**: Foundation ready — user stories can begin.

---

## Phase 3: User Story 1 - End user reaches and uses tools through one endpoint (Priority: P1) 🎯 MVP

**Goal**: An authorized client connects via OAuth and lists/calls aggregated, namespaced tools.
**Independent Test**: With one seeded fixture server, connect a client → OAuth → `tools/list` → call a tool → correct result.

### Tests for User Story 1

- [X] T023 [P] [US1] Contract test: OAuth discovery (401 + `WWW-Authenticate` + protected-resource metadata) — *verified via metadata endpoint + `server_test.go` 401-challenge test*
- [X] T024 [P] [US1] Contract test: `tools/list` aggregation + `serverSlug__name` namespacing incl. collision — *`aggregate` + `mcp` handler + `server` e2e tests*
- [X] T025 [P] [US1] Contract test: `tools/call` routing + passthrough — *`mcp` handler + `server` e2e tests; SSE streaming deferred*
- [X] T026 [P] [US1] Integration test: connect→list→call against a seeded fixture server — *implemented as `services/gateway/internal/server/server_test.go` (fake JWKS + minted token + HTTP)*

### Implementation for User Story 1

- [X] T027 [US1] Gateway auth middleware: validate token, audience, host↔realm check, resolve org/user/roles in `services/gateway/internal/auth/middleware.go` — *now wired to the real `pkg/authz` JWT validator; bogus/missing tokens → 401 challenge (smoke-verified)*
- [X] T028 [US1] Implement `/.well-known/oauth-protected-resource` + `WWW-Authenticate` challenge in `services/gateway/internal/auth/metadata.go`
- [X] T029 [US1] Capability aggregation + namespacing engine in `services/gateway/internal/aggregate/aggregate.go` — *`slug__tool` namespacing + routing table; collision/dedup unit-tested (FR-003)*
- [X] T030 [US1] MCP JSON-RPC dispatch + downstream routing in `services/gateway/internal/mcp/` (+ `internal/downstream`) — *initialize/tools.list/tools.call/ping/notifications + passthrough; SSE streaming of partial results deferred*
- [~] T031 [US1] Session + routing table in `services/gateway/internal/{downstream,mcp}` — *per-request in-memory routing via registry + aggregate; Redis-backed session caching deferred (scale optimization, SG-2)*
- [X] T032 [US1] `initialize` handshake + aggregate capability advertisement in `services/gateway/internal/mcp/handler.go` — *advertises protocolVersion 2025-03-26 + tools capability*
- [X] T033 [US1] Unauthorized → 401 with no surface exposed in `services/gateway/internal/auth/` — *401 + RFC 9728 challenge, no surface. The gateway now wires an `audit.Logger` (in-memory, or the durable `S3Archive` on its own `-gateway` stream when configured) and emits tamper-evident **`auth.denied`** events from `RequireAuth` (reason `missing_token`/`invalid_token`) and **`authz.denied`** events on RBAC denial (via `mcp.WithDenyRecorder`, actor + requested tool) — the cross-tenant/over-privilege signal feeding the HighAuthDenialRate alert. Tests: `TestAuthDenied_Audited`, `TestAuthzDenied_Audited` (chain verifies). **Anti-amplification**: denial-audit writes are rate-limited by a global fixed-window limiter (`MCP_AUDIT_DENY_PER_MIN`, default 600), so unauthenticated floods can't inflate audit storage regardless of how many hosts are forged; drops are counted in `mcp_audit_dropped_total` (observable, never silent — the `mcp_requests_total{code="401"}` metric still captures the full rate). Test: `TestAuthDenied_AuditRateLimited` (limit 2 → 2 recorded + 2 dropped).*

**Checkpoint**: US1 functional against a fixture server (MVP).

---

## Phase 4: User Story 2 - Admin adds a remote HTTP MCP server (Priority: P1)

**Goal**: Admin registers a remote MCP URL; it becomes usable by permitted users with no redeploy.
**Independent Test**: Add a reachable remote endpoint → permitted user sees its tools within target time → call one; other servers/users unaffected.

### Tests for User Story 2

- [X] T034 [P] [US2] Contract test: admin API `remote_http` server CRUD — *`control-plane/internal/admin/servers_test.go` (create/list/get/delete, 422/409, admin-role 401/403)*
- [X] T035 [P] [US2] Contract test: health-check state transitions (healthy/unreachable/auth_failed) — *covered in `servers_test.go` (healthy on create + `TestAdmin_HealthAuthFailed`)*
- [X] T036 [P] [US2] Integration test: add remote server → permitted user sees tools, no redeploy — *`server_test.go` `TestMCP_EndToEnd_RemoteDownstream` (registers a remote, drives list+call via the gateway)*

### Implementation for User Story 2

- [X] T037 [US2] Server-definition CRUD (remote_http + stdio config) in `services/control-plane/internal/admin/` — *org-scoped store + Echo CRUD (create/list/get/patch/delete) + validation + admin-role auth + `Sink` propagation; in-memory store (Postgres = T007)*
- [X] T038 [US2] Health-check engine (reachability/auth) + status surfacing in `services/control-plane/internal/admin/health.go` — *MCP `initialize` probe → healthy / unreachable / auth_failed, surfaced on the server record*
- [X] T039 [US2] Gateway remote-HTTP downstream client (egress-controlled) in `services/gateway/internal/remotehttp/client.go` — *MCP Streamable HTTP: initialize+session, tools/list/call, SSE+JSON response parsing, header injection; trace-context propagation. **SSRF protection** (`safedial.go`, `WithBlockPrivate`): a dial-time guard refuses connections to non-public IPs (loopback / RFC1918 / IPv6 ULA / link-local incl. cloud metadata `169.254.169.254` / multicast / unspecified), so an admin-supplied endpoint can't make the gateway hit internal services; checking at dial time also defeats DNS rebinding. Enabled via `MCP_BLOCK_PRIVATE_EGRESS` (on by default in prod, off in dev so loopback test servers work). Tests: `TestIsBlockedIP` (range table) + `TestWithBlockPrivate_BlocksLoopback` (end-to-end block vs allow).*
- [X] T040 [US2] Hot server-set propagation to gateway (no redeploy) — *`pkg/serverevents` Bus (Redis pub/sub `RedisBus` for prod + `MemBus` for dev/test); control-plane `busSink` publishes upsert/remove, gateway subscribes and builds `remotehttp` clients into a **per-org** catalog (HC-1). **Reconcile-on-startup** (`Server.Reconcile` + `PostgresSource`) rebuilds the catalog from the source of truth so a restarted gateway is correct without waiting for events — the durability backstop. Verified by `TestMCP_Propagation_ViaEvents` + `TestReconcileOnStartup`.*
- [~] T041 [US2] Per-server failure isolation — *per-request timeouts in the remote client + failing servers skipped during aggregation + downstream tool errors isolated (FR-019); circuit-breaker deferred*

**Checkpoint**: Admins can add remote servers; users use them live.

---

## Phase 5: User Story 3 - Admin adds a stdio MCP server (Priority: P1)

**Goal**: Admin registers `{command,args,env}`; the gateway runs it in a microVM sandbox and bridges stdio (HC-3 core).
**Independent Test**: Add `npx -y @modelcontextprotocol/server-sequential-thinking` → permitted user calls its tool; process confined to its sandbox.

### Tests for User Story 3

- [ ] T042 [P] [US3] Contract test: admin API `stdio` server CRUD in `tests/contract/admin_servers_stdio_test.go`
- [X] T043 [US3] **Adversarial** test: hostile stdio server contained — *`tests/security/adversarial.sh`; **all 7 checks passed live in the gVisor VM** (metadata/internet/internal egress blocked, isolated guest kernel, read-only rootfs, mount/cap-drop denied, no host-fs leak) with the production ContainerRuntime flags*
- [X] T044 [P] [US3] Integration test: launch a stdio MCP server → user calls tool — *`sandbox.TestExecRuntime_RealSubprocess` runs a real subprocess MCP server over OS pipes (list+call); `npx`-specific run deferred to the Linux image*
- [ ] T045 [P] [US3] Test: startup-timeout/hang → unhealthy (no leak); idle → reclaim → transparent restart in `tests/integration/us3_lifecycle_test.go`

### Implementation for User Story 3

- [X] T046 [US3] Build sandbox base image (Node 20/npx + Python/pipx) in `deploy/sandbox-images/Dockerfile` — *server command is the container command; run read-only under gVisor/Kata. Registry-mirror cache deferred (ties to egress allowlist T049)*
- [X] T047 [US3] Sandbox runtime lifecycle (launch/stop) in `services/gateway/internal/sandbox/` — *`Runtime` interface + `ExecRuntime` (dev) + **`ContainerRuntime`** (gVisor `runsc` / Kata via `docker run --runtime`) + `Select` factory. gVisor runs locally in a Linux VM with no nested virt — see `docs/local-sandbox.md`. Arg-builder unit-tested; sandboxed execution runs in the Linux VM. **Validated locally** (Lima VM + gVisor `runsc`): separate guest kernel (`4.19.0-gvisor` vs host `6.8.0`), `--read-only` rootfs enforced, metadata/internal egress blocked with the exact ContainerRuntime flags.*
- [~] T048 [US3] Warm pool + on-demand assignment + scale-to-zero in `services/sandbox-supervisor/internal/pool/pool.go` — *Generic warm `Pool`: `Prewarm` (pre-create MinWarm to cut cold start), `Acquire`/`Release` (reuse warm, create on demand up to MaxSize, `ErrExhausted` at cap), `Reap` (idle eviction back to MinWarm, **scale-to-zero** when MinWarm=0), `Stats`, `Close`. Decoupled from the backend via a `Factory`/`Instance` seam so it's hermetically tested (`TestPool_ReuseAndCapacity`, `_Prewarm`, `_ReapToMinWarm`, `_ScaleToZero`, `_Close` with an injected clock). Wiring the real Firecracker/Kata factory + the supervisor service binary is the remaining prod step.*
- [~] T049 [US3] Default-deny egress — *Two layers done: (1) **stdio sandboxes** launch with `--network none` (default-deny) + `--cap-drop ALL` + `--security-opt no-new-privileges`; (2) **remote-HTTP egress** has SSRF protection blocking non-public IPs at dial time (see T039, `remotehttp.WithBlockPrivate`). Still deferred: an egress **proxy with allowlist** for sandboxes that need limited outbound (e.g. the npm registry reachable while internal/metadata stays blocked).*
- [~] T050 [US3] Resource limits + timeouts — *`ContainerRuntime` sets `--memory`, `--pids-limit`, `--read-only`, `--tmpfs /tmp` (mem/pid/disk). CPU caps + startup/idle/request timeouts deferred*
- [X] T051 [US3] stdio bridge (newline-delimited JSON-RPC over the process's stdio) in `services/gateway/internal/sandbox/stdio.go` — *initialize handshake + tools/list/call + passthrough; tested over an in-process pipe and a real subprocess*
- [~] T052 [US3] Exec + MCP handshake + stdio relay — *done in-process via `ExecRuntime` + `sandbox.Server` (launch, initialize, relay); the in-VM agent shim is the Linux microVM piece (deferred)*
- [X] T053 [US3] Server-definition CRUD (stdio: command/args/env) in `services/control-plane/internal/admin/` — *stdio type validated (requires `command`); `busSink` publishes command/args/env to the data plane*
- [~] T054 [US3] Idle reclamation + transparent restart — *`Server.Close` stops the process and the next call re-launches (transparent restart); the idle timer + warm pool are deferred*
- [X] T055 [US3] Gateway routes stdio servers via the sandbox runtime — *`applyServerEvent` builds a `sandbox.Server` (using the configured runtime) into the per-org catalog on stdio upserts*

**Checkpoint**: Admins can run arbitrary stdio servers, sandboxed.

---

## Phase 6: User Story 4 - Organization isolation guaranteed; user isolation best-effort (Priority: P1)

**Goal**: Prove and harden the isolation boundary across all surfaces (HARD release gate — SC-001/002/003).
**Independent Test**: 2 orgs × 2 users; cross-org access always fails, hostile servers contained, audience-bound tokens non-transferable.

### Tests for User Story 4 (adversarial — must pass before any multi-tenant exposure)

- [X] T056 [P] [US4] Adversarial: cross-org access blocked — *gateway per-org catalog (`mcp.TestToolsListOrgIsolation`: org B sees none of org A's servers); control-plane is org-path + per-org-realm scoped*
- [X] T057 [P] [US4] Adversarial: org-A audience token rejected at org-B endpoint — *`authz.TestValidate_CrossOrgAudienceRejected` (issuer + audience binding)*
- [~] T058 [P] [US4] Adversarial: cross-user access (best-effort SG-1) — *org-shared servers are shared within an org by design; per-user credential isolation now implemented (T078: per-`(org,user,server)` instances via providers, proven isolated in `TestRegistry_PerUserProvider`). RBAC (US5) scopes by role. Not a release gate (SG-1 soft).*
- [X] T059 [P] [US4] Adversarial: hostile sandbox cannot reach other tenants/secrets/control-plane/metadata — *`tests/security/adversarial.sh`, **passed live in the gVisor VM** (egress to metadata/internal/internet all blocked, own kernel, read-only, caps dropped, no host-fs leak)*
- [X] T060 [US4] Mid-session revocation enforced on next request — *the gateway resolves the per-org catalog + RBAC per request, so a remove/disable or permission change applies on the next call (FR-022); covered by `TestMCP_Propagation_ViaEvents` (remove → gone) and the RBAC call-time check*

### Implementation for User Story 4

- [ ] T061 [US4] Enforce `org_id` scoping on every data access + verify RLS coverage across services in `services/*/internal/**`
- [ ] T062 [US4] Sandbox instance keys always include `org_id`; assert never shared across orgs in `services/sandbox-supervisor/internal/pool/key.go`
- [ ] T063 [US4] Host-subdomain ↔ token-realm cross-check in `services/gateway/internal/auth/tenant.go`
- [ ] T064 [US4] Live-session revocation/permission-change propagation (FR-022) in `services/gateway/internal/session/revoke.go`
- [ ] T065 [US4] Fail-closed denial + audit for any cross-tenant attempt in `pkg/authz/deny.go`

**Checkpoint**: Isolation guarantee verified — safe for multi-tenant exposure.

---

## Phase 7: User Story 5 - Admin governs access with RBAC (Priority: P2)

**Goal**: Scope servers/tools to roles/users; changes apply promptly.
**Independent Test**: Grant a server to one role → only that role sees it; revoke → disappears on next list/call.

### Tests for User Story 5

- [ ] T066 [P] [US5] Contract test: permission-bindings API (set/list, same-org constraint) in `tests/contract/admin_rbac_test.go`
- [X] T067 [P] [US5] Test: role scoping changes visibility on list/call — *`mcp.TestRBACFiltering` (role holder sees `eng__build`; non-holder sees nothing and gets method-not-found on call)*

### Implementation for User Story 5

- [~] T068 [US5] RBAC scoping CRUD — *per-server `allowed_roles` on the server definition (create + event + store) implemented; a richer separate PermissionBinding entity with per-tool grants is deferred*
- [X] T069 [US5] Gateway RBAC filtering of list/call by roles in `services/gateway/internal/downstream` + `internal/mcp/handler.go` — *`VisibleSlugs`/`CanAccess`; enforced on both list and call, unauthorized servers reported as unknown (no existence leak)*
- [X] T070 [US5] Enforce same-org scoping — *the gateway catalog is per-org and the control-plane API is org-path + per-org-realm-token scoped, so RBAC bindings can only ever reference same-org principals/servers (HC-1)*
- [X] T071 [US5] Propagate permission changes to live sessions — *role scope rides the server-change event; the gateway rebuilds visibility per request, so changes take effect on the next list/call (FR-022). Mid-flight push (`tools/list_changed`) deferred*
- [ ] T072 [US5] Audit `rbac.grant`/`rbac.revoke` events in `services/control-plane/internal/rbac/audit.go`

**Checkpoint**: RBAC governs visibility/use across new and existing sessions.

---

## Phase 8: User Story 6 - Secrets and downstream credentials managed securely (Priority: P3)

**Goal**: Org-level (default) and per-user (optional) credentials injected at runtime, never leaked.
**Independent Test**: Store a secret → permitted user's session uses it; unpermitted cannot; never in logs/other sandboxes; per-user-missing → blocked not silently shared.

### Tests for User Story 6

- [X] T073 [P] [US6] Contract test: write-only credential endpoints (org + `/me` per-user) — *`TestAdmin_Credentials_WriteOnly` (PUT → 204, value never echoed, stored in backend) + `secrets.TestMemStore_RoundTrip`*
- [~] T074 [US6] Security test: secret never in logs/responses/other sandboxes — *all three dimensions covered: **logs** by `TestLoggerRedactsSecrets`/`TestRedact` (T080 redaction); **responses** by `TestAdmin_Credentials_WriteOnly` (write-only, value never echoed); **other sandboxes** by the gVisor adversarial suite (`tests/security/adversarial.sh`, no cross-sandbox/host leakage). A single consolidated `secret_isolation_test.go` is still optional*
- [~] T075 [P] [US6] Integration test: per-user credential missing → blocked, not silent reuse — *covered at the gateway by `TestMCP_PerUserCredentialInjection` (before creds: server contributes no tools AND the downstream is never contacted → blocked, no silent reuse of any other credential); a dedicated `tests/integration/` file against live backends is still optional*

### Implementation for User Story 6

- [X] T076 [US6] Credential write/delete (org + per-user), never echo values — *`pkg/secrets` `Store` (MemStore + **VaultStore** KV v2, proven live against the dev Vault) + control-plane write-only PUT/DELETE `/credentials` and `/credentials/me`*
- [X] T077 [US6] Runtime secret injection into the downstream (stdio env / remote headers) — *gateway fetches the org-shared secret at downstream build and injects it (remote: `WithHeader`; stdio: env). E2E-verified (`TestMCP_OrgCredentialInjection`: downstream receives the injected `Authorization`). Per-user injection done in T078.*
- [X] T078 [US6] `credential_mode` handling + per-user escalation — *`none`/`org_shared` build one shared instance; `per_user` registers a `downstream.Provider` (`perUserProvider`) that lazily builds a per-`(org,user,server)` instance carrying the calling user's own secret (`secrets.UserRef`), cached per user and closed by the kill-switch. Routing resolves the right instance via `Registry.GetForUser` (used by both `tools/list` and `tools/call`). Tests: `TestMCP_PerUserCredentialInjection` (no creds → invisible + uncontacted; with creds → downstream gets that user's `Authorization`) and `TestRegistry_PerUserProvider` (per-user isolation, caching, provider error, kill-switch close).*
- [X] T079 [US6] Rotation (applies on next instance start) — *credential write/delete propagates to the data plane so the next instance uses the new secret: per-user → targeted `serverevents.ActionCredentialChanged{UserID}` → gateway `Catalog.Invalidate` drops+closes that user's cached instance (next `GetForUser` rebuilds with the rotated secret); org-level → re-emitted `ActionUpsert` rebuilds the shared instance. `Registry.AddScoped`/`AddProvider` now close the replaced instance(s) so rebuilds don't leak. Tests: `TestRegistry_Invalidate`, `TestRegistry_ReplaceClosesPrevious`, gateway e2e `TestMCP_PerUserCredentialRotation` (k1→rotate→k2 reaches downstream), control-plane `TestAdmin_CredentialChange_Propagates` (org→userID "", `/me`→caller id). A dedicated `rotate.go` was unnecessary — rotation is event-driven, not a separate component.*
- [X] T080 [US6] Secret redaction across logs/traces in `pkg/telemetry/redact.go` — *`Redact` scrubs sensitive JSON fields/arrays (keys containing authorization/token/secret/password/api_key/cookie/private_key), `key=value` env/query secrets, and bare `Bearer …` tokens (RE2, linear-time, no ReDoS). A `RedactingWriter` wraps the log sink so secrets are scrubbed regardless of how a field was added — wired into `pkg/logging` (`buildWithWriter`); reusable for OTLP span attributes when tracing lands. Tests: `TestRedact` (field/array/env/bearer + non-secret `credential_mode` preserved), `TestRedactingWriter`, `TestLoggerRedactsSecrets` (a secret passed as a zerolog field never reaches the sink).*

**Checkpoint**: Credential-requiring servers work safely; per-user isolation available.

---

## Phase 9: User Story 7 - Admin observes health and stops misbehaving servers (Priority: P3)

**Goal**: Per-server health/usage visibility + instant kill-switch + quotas.
**Independent Test**: Disable a server → unavailable ≤5s, others unaffected, action audited; quota breach → throttle/stop without cross-tenant impact.

### Tests for User Story 7

- [X] T081 [P] [US7] Test: disable → stops serving, others unaffected — *`downstream.TestRegistry_RemoveClosesDownstream` (Close on remove) + `server.TestMCP_Propagation_ViaEvents` (remove event → server gone on next list/call); disable emits a remove event*
- [X] T082 [P] [US7] Test: quota exceeded → rejected without cross-tenant impact — *`quota` enforcer tests (org/user limits + noisy-neighbor isolation) + `mcp.TestQuotaEnforced` (2nd call → CodeRateLimited)*

### Implementation for User Story 7

- [ ] T083 [US7] Per-org/per-server health + usage surfacing API in `services/control-plane/internal/servers/status.go`
- [X] T084 [US7] Audit query API — *admin-only `GET /v1/orgs/{org}/audit` returns org-scoped records, newest first (`ListAudit`). Reads through whichever `audit.Logger` is configured, so it queries the durable `S3Archive` (T087) directly when enabled.*
- [X] T085 [US7] Kill-switch (disable → terminate + stop serving) — *control-plane disable/delete emits a remove event; the gateway `Registry.Remove` drops the server from the per-org catalog **and Closes it** (terminating a stdio sandbox process). Takes effect on the next request.*
- [X] T086 [US7] Per-org/per-user quotas + rate limits in `services/gateway/internal/quota/` — *fixed-window `Enforcer` (org + user keys, independent so one tenant can't starve another), enforced on `tools/call` → `CodeRateLimited`; configurable via `MCP_RATE_ORG_PER_MIN`/`MCP_RATE_USER_PER_MIN`. **Fleet-wide backend done**: `Enforcer` is now backend-agnostic (a `limiter` interface) with a `RedisLimiter` (atomic INCR+PEXPIRE Lua fixed-window, shared across replicas) selected by the gateway when `MCP_REDIS_ADDR` is set (in-memory otherwise); **fails open** if Redis is unreachable so a soft goal can't take down the data plane; client closed on shutdown. Tests: `TestRedisLimiter_Unlimited`, `TestRedisLimiter_FailOpen` (hermetic), and live `TestRedisLimiter_SharedWindow` (two instances enforce one combined window; distinct keys independent — proven against dev Redis).*
- [X] T087 [US7] Tamper-evident audit archive — *tamper-evidence via the SHA-256 **hash chain** (any edit/insert/reorder breaks `Verify`) **plus** a durable WORM backend: `audit.S3Archive` (minio-go) writes each sealed record as an immutable object under **Object Lock COMPLIANCE** with a retention window (default 1y via `MCP_AUDIT_RETENTION`), recovers the chain tip on restart, and reads `List`/`Verify` back from storage. Wired into the control-plane (`MCP_AUDIT_S3_*`; in-memory dev logger when unset); MinIO added to dev compose. **Proven live** by `TestS3Archive_DurableAndWORM` against MinIO: records persist + chain verifies from storage + tip rebuilt on reopen + a locked record version cannot be deleted within retention. Single-writer chain (multi-writer sequence coordination is a follow-up).*
- [X] T088 [US7] OTel dashboards + alerts (health, isolation-denial rate) — *Prometheus scrapes both services (`deploy/dev/prometheus.yml`) and loads **5 alert rules** (`deploy/dev/alerts.yml`: gateway/control-plane target-down, 5xx rate, **auth/isolation-denial rate** on 401/403, tool-call error/quota rate) — validated with `promtool check rules/config` and confirmed live via `/api/v1/rules`. **Grafana** added to dev compose with provisioned datasource + the "MCP Runtime Overview" dashboard (request rate by code, auth denials, tools/call by outcome, targets-up) — verified live via Grafana API (datasource health OK, dashboard + 4 panels loaded). UI: Grafana :3000, Prometheus :9090.*
- [ ] T089 [US7] Emit usage metering records for cost accounting in `services/gateway/internal/quota/meter.go`

**Checkpoint**: Operable at scale with governance and kill-switch.

---

## Phase 10: Polish & Cross-Cutting Concerns

- [ ] T090 [P] Load tests at ~10k concurrent sessions (k6) verifying SC-008/010/011/012 in `tests/load/`
- [ ] T091 Warm-pool sizing + scale-to-zero validation for idle≈0 cost (SC-014) in `services/sandbox-supervisor/internal/pool/`
- [ ] T092 [P] Audit Object Lock + 1-year retention verification (SOC 2) in `tests/security/audit_retention_test.go`
- [ ] T093 [P] Operator runbooks + threat model in `docs/`
- [ ] T094 Run `quickstart.md` end-to-end validation (US1–US6 + adversarial US4)
- [ ] T095 [P] Security hardening pass (seccomp profiles, capability-drop review, dependency scan)
- [ ] T096 [P] Performance tuning to meet p95 targets (SC-010/011/012)
- [ ] T097 Cost-target instrumentation + report vs SC-015 business target in `services/control-plane/internal/audit/usage_report.go`

---

## Dependencies & Execution Order

- **Setup (P1)** → **Foundational (P2)** → **user stories**. Foundational blocks all stories.
- **US1 (P1)** after Foundational; uses a seeded fixture server for its independent test.
- **US2 (P1)** after Foundational; remote client builds on the US1 proxy/session layer.
- **US3 (P1)** after Foundational; heaviest — depends on supervisor scaffold (T018) + US1 proxy.
- **US4 (P1)** logically after US1–US3 exist (it adversarially verifies + hardens their surfaces). **HARD gate: US4 must be green before any real multi-tenant exposure**, even though US1/US2/US3 can be demoed earlier in single-tenant dev.
- **US5 (P2)** after US1 (filtering on aggregation) + control-plane.
- **US6 (P3)** after US3 (sandbox injection) + control-plane.
- **US7 (P3)** after US2/US3 (servers to operate) + control-plane.
- **Polish** after the desired stories.

### Within each story

- Tests written first and failing → models/libs → services → endpoints → integration.

### Parallel opportunities

- Foundational libs T009, T010, T011, T012, T014, T015 in parallel.
- All `[P]` test tasks within a story in parallel (e.g., T023–T026; T056–T059).
- After Foundational, US1/US2/US3 can be staffed in parallel (US3 is the long pole).

---

## Parallel Example: User Story 1

```bash
# Tests first (parallel):
Task: "Contract test OAuth discovery in tests/contract/mcp_discovery_test.go"
Task: "Contract test tools/list namespacing in tests/contract/mcp_aggregate_test.go"
Task: "Contract test tools/call streaming in tests/contract/mcp_call_test.go"
Task: "Integration connect→list→call in tests/integration/us1_connect_use_test.go"
```

---

## Implementation Strategy

### MVP

1. Phase 1 Setup → Phase 2 Foundational.
2. Phase 3 **US1** against a seeded fixture server → validate independently (technical MVP).
3. First externally useful increment: add **US2** (remote HTTP) and/or **US3** (stdio) so admins can register real servers.

### Hard-constraint gate

- **US4 (isolation) is a release gate.** No production/multi-tenant traffic until the adversarial suite (T056–T060) is green — per Constitution Principles I, II, V.

### Incremental delivery

US1 → US2 → US3 → **US4 (gate)** → US5 → US6 → US7 → Polish. Each story is an independently testable, demoable increment that does not break earlier ones.

---

## Notes

- Tests are mandatory here (Constitution Principle V), not optional — contract tests for interfaces and failing-first adversarial tests for every isolation/authz/sandbox/secrets change.
- `[P]` = different files, no incomplete dependency.
- Commit after each task or logical group; small reviewable increments (Constitution: Development Workflow).
- Total: **97 tasks** across Setup (6), Foundational (16), US1 (11), US2 (8), US3 (14), US4 (10), US5 (7), US6 (8), US7 (9), Polish (8).
