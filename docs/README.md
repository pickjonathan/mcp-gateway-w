# Multi-Tenant MCP Gateway Runtime

A platform that lets an organization connect its end users to MCP tooling through
a single OAuth-protected endpoint — `https://{org}.{base-domain}/mcp` — while org
admins add **remote (HTTP)** or **stdio** MCP servers (including untrusted ones
like `npx -y @modelcontextprotocol/server-sequential-thinking`) without operating
any infrastructure themselves.

> Go module `github.com/acme-corp/mcp-runtime`. This site documents the design and
> the implemented system. Scope of record lives in
> `specs/001-mcp-server-runtime/` (spec, plan, tasks).

## The problem, in one picture

```
 end users                          org admins
    │  OAuth 2.1 (per-org realm)        │  admin API (RBAC)
    ▼                                   ▼
 ┌─────────────────────┐         ┌──────────────────────┐
 │   Gateway (data)    │◀──Redis─│ Control plane (admin)│
 │  {org}.host/mcp     │  events │  server CRUD + creds │
 └─────────┬───────────┘         └──────────┬───────────┘
           │ aggregate + route               │ source of truth
   ┌───────┴────────┐                        ▼
   ▼                ▼                   PostgreSQL (RLS)
 remote HTTP     stdio sandbox          Vault (secrets)
 (SSRF-guarded)  (microVM/gVisor,       S3 WORM (audit)
                  no network)
```

## Design goals (and how they rank)

The system was evaluated against seven criteria. Three are **hard constraints**
that must always hold; four are **soft goals** that may be traded off to uphold
the hard ones.

| # | Criterion | Class | How it's met (short) |
|---|---|---|---|
| 1 | **Organization isolation** | **HARD** | Per-org catalog, per-org Keycloak realm + audience-bound tokens, Postgres **RLS** |
| 3 | **Frictionless admin** | **HARD** | Add a server via one API call; platform runs it — no infra for admins |
| 4 | **Secure any MCP** | **HARD** | Untrusted stdio in a no-network microVM/gVisor sandbox; remote egress SSRF-guarded |
| 2 | User isolation | soft | Per-user credentials + per-user downstream instances; RBAC |
| 5 | Scalability | soft | Stateless gateway, Redis-shared quotas, sandbox warm pool |
| 6 | Performance | soft | Warm pool (cold-start), connection reuse, in-process routing |
| 7 | Cost | soft | Scale-to-zero sandboxes, shared infra, microVM density |

See **[Solution Comparison](solution-comparison.md)** for how the chosen
architecture scores against alternatives on each of these verticals.

## Documentation

- **[Architecture](architecture.md)** — components, data/control planes, request flows.
- **[Features](features.md)** — the seven user stories and what each delivers.
- **[Security Model](security.md)** — isolation, sandboxing, secrets, SSRF, audit.
- **[Observability](observability.md)** — logs, metrics, traces, dashboards, alerts.
- **[Solution Comparison](solution-comparison.md)** — architectures & sandbox runtimes vs. the 7 criteria.
- **[Data Model](data-model.md)** — entities and relationships.
- **[Local Dev & Runbook](local-dev.md)** — bring up the stack, endpoints, integration tests.

## Status

Feature-complete across all seven user stories; the three hard constraints are
verified (including a live gVisor adversarial containment suite). Remaining work
is production infrastructure (k8s `RuntimeClass` sandbox backends, edge config)
tracked in `specs/001-mcp-server-runtime/tasks.md`.
