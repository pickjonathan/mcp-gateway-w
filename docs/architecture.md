# Architecture

The runtime separates a **control plane** (admin-facing, source of truth) from a
**data plane** (user-facing, hot path), connected by an event bus with a
reconcile-on-startup backstop.

## Components

| Component | Role |
|---|---|
| **Gateway** (`services/gateway`) | The data plane. Terminates per-org OAuth, exposes the aggregated MCP endpoint, routes `tools/call` to the right downstream, enforces RBAC + quotas, injects credentials, guards egress, emits telemetry + audit. |
| **Control plane** (`services/control-plane`) | The admin API. Per-org CRUD of server definitions, write-only credential storage, audit query. The source of truth (PostgreSQL). Publishes change events. |
| **Sandbox supervisor** (`services/sandbox-supervisor`) | Manages the pool of sandbox instances for stdio servers (warm pool, on-demand assignment, scale-to-zero). |
| **Keycloak** | Per-org realm OAuth 2.1 authorization server (PKCE, dynamic client registration, audience-mapped scopes), aligned to the MCP authorization spec. |
| **PostgreSQL** | Durable server definitions; org isolation enforced by Row-Level Security. |
| **Redis** | Control-plane → gateway change propagation (pub/sub) and fleet-wide rate-limit counters. |
| **Vault** | Downstream credentials (KV v2); write-only from the API's perspective. |
| **Object storage (S3/MinIO)** | Durable, Object-Lock'd (WORM) audit archive. |
| **OpenTelemetry sinks** | Prometheus (metrics), Jaeger/OTLP (traces); Grafana for dashboards + alerts. |

## Control plane vs. data plane

```
        ADMIN                                   END USER
          │ Bearer (admin role, per-org realm)    │ Bearer (audience = https://{org}.host/mcp)
          ▼                                        ▼
 ┌──────────────────────┐                 ┌──────────────────────────┐
 │     Control plane    │                 │         Gateway          │
 │  /v1/orgs/:org/...   │                 │  POST /mcp (JSON-RPC)    │
 │  - server CRUD       │                 │  - initialize/tools/*    │
 │  - credentials (W/O) │                 │  - RBAC + quota          │
 │  - audit query       │                 │  - credential injection  │
 └─────────┬────────────┘                 └───────────┬──────────────┘
           │ persist                                   │ resolve + route
           ▼                                           ▼
       PostgreSQL  ──(1) live: Redis pub/sub──▶  per-org catalog (in-memory)
       (RLS)       ──(2) startup: reconcile ───▶  (remote clients / sandbox bridges)
```

Two propagation paths keep the gateway's in-memory catalog correct:

1. **Live** — on every admin change the control plane publishes a
   `serverevents.Event` on Redis; the gateway applies it immediately (fast path).
2. **Reconcile** — on startup (and as a durability backstop for fire-and-forget
   pub/sub) the gateway reads the enabled servers from Postgres and rebuilds the
   catalog. A restart of *either* service converges to the correct state.

## Request flow — `tools/call`

```
client ──POST /mcp {tools/call serverSlug__tool}──▶ gateway
  1. RequireAuth: validate JWT (per-org JWKS), audience = https://{org}.host/mcp
  2. Dispatch: parse JSON-RPC; org/user/roles from the verified principal
  3. quota: per-org + per-user fixed window (Redis-shared across replicas)
  4. RBAC: registry.CanAccess(slug, roles)  → deny ⇒ audit authz.denied
  5. resolve downstream for (org, user):
       - fixed instance (none / org_shared creds), OR
       - per-user instance built lazily with the caller's own secret
  6. call:
       - remote_http → Streamable HTTP client (creds as headers, SSRF-guarded)
       - stdio       → sandbox bridge (creds as env, microVM/gVisor, no network)
  7. return result verbatim; record metrics + span
```

## Namespacing & aggregation

`tools/list` merges every visible server's tools into one surface, namespaced
`serverSlug__tool` (collision-safe). `tools/call` splits on the separator to find
the target server, then enforces RBAC on the call itself (not just on listing),
so an unauthorized or unknown server is reported identically — existence is never
revealed.

## Multi-tenancy boundaries (HC-1)

- **Catalog**: one `Registry` per org inside a `Catalog`; every lookup is keyed by
  org id — downstreams are never shared across orgs.
- **Identity**: org derived from the request host; the token's audience must match
  `https://{org}.host/mcp`, so a token minted for org A cannot be used at org B.
- **Database**: `mcp_servers` has Row-Level Security; the app sets
  `app.current_org` per transaction, so even a missing app-level filter cannot
  leak across tenants.

## Sandboxing model (HC-3)

stdio servers are arbitrary, possibly-hostile code. They run behind a pluggable
`SandboxRuntime`:

- `exec` — dev only, **UNSANDBOXED** (clearly labeled).
- `ContainerRuntime` — `docker run --runtime=runsc|kata --network none
  --cap-drop ALL --read-only --tmpfs /tmp --pids-limit --memory ...`.
- Production: microVM (Firecracker/Kata) or gVisor via a Kubernetes
  `RuntimeClass`, fronted by the sandbox supervisor's warm pool.

The local gVisor boundary was validated with an adversarial suite (separate guest
kernel, read-only rootfs, dropped caps, and metadata/internal/internet egress all
blocked). See **[Security Model](security.md)**.

## Technology choices

Go (single static binaries, strong concurrency, good ops story) with **Echo** for
HTTP and **zerolog** for structured logging. All configuration loads once into a
`config.Config` singleton. See **[Solution Comparison](solution-comparison.md)**
for the reasoning behind the larger architectural decisions.
