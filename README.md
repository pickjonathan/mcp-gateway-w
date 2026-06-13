# Multi-Tenant MCP Gateway Runtime

A platform that connects an organization's end users to MCP tooling through one
OAuth-protected endpoint — `https://{org}.{base-domain}/mcp` — while org admins
add **remote (HTTP)** or **stdio** MCP servers (including untrusted ones) with no
infrastructure work of their own.

Go module: `github.com/acme-corp/mcp-runtime` (Go 1.25).

## Run it locally

**Prerequisites:** Docker Desktop, Go 1.25, Node 20+.

### 1 · Infrastructure + identity

```sh
make dev-up    # docker: postgres, redis, vault, keycloak, minio, prometheus, grafana, jaeger
               # then seeds the Keycloak `acme` realm — idempotent (see deploy/dev/seed-keycloak.sh)
```

### 2 · The services (each in its own terminal)

```sh
make run-control-plane    # admin API on :8090
make run-gateway          # data plane (/mcp) on :8080  — exec sandbox = UNSANDBOXED, dev only
cd web/admin-console && npm install && npm run dev   # admin console on :5173
```

These run your local code against the stack with the right dev env baked in
(local Keycloak issuer, traces → Jaeger). `make run` is an alias for `run-gateway`.

### 3 · Open everything

| UI | URL | Login |
|---|---|---|
| **Admin console** | http://localhost:5173 | `admin`/`admin` or `alice`/`alice` |
| **MCP Inspector** | `npx @modelcontextprotocol/inspector` → http://localhost:6274 | proxy token is printed |
| **Jaeger** — traces & spans | http://localhost:16686 | — (services: `mcp-gateway`, `mcp-control-plane`) |
| **Grafana** — "MCP Runtime Overview" | http://localhost:3000 | anonymous Admin |
| **Prometheus** | http://localhost:9090 | — |
| **MinIO** — audit WORM bucket | http://localhost:9001 | `minioadmin`/`minioadmin` |
| **Keycloak** | http://localhost:8081 | `admin`/`admin` |

### Logins & users

- **`admin`/`admin`** — org admin (manage servers, credentials, RBAC, audit in the console).
- **`alice`/`alice`** — a member (can use *open* servers; *gated* servers need a matching realm role).
- Add/invite users in Keycloak → realm `acme` → **Users**. Model + steps: [Multi-tenant (Keycloak)](docs/multi-tenant-keycloak.md).

### Connect an MCP client

Add `127.0.0.1 acme.mcp.example.com` to `/etc/hosts`, then point a client at
**`http://acme.mcp.example.com:8080/mcp`** (Transport: *Streamable HTTP*). For the
local HTTP endpoint, authenticate with a **Bearer token** (the OAuth *flow*
validates against the canonical `https://{org}.{base}/mcp`, which only matches
behind a TLS edge). Full walkthrough + per-user RBAC validation:
[MCP Clients & RBAC](docs/mcp-inspector-rbac.md).

### How the pieces fit (dev notes)

- The Go services run on the **host** so the OIDC issuer is
  `http://localhost:8081/realms/{org}` — the exact issuer browser/Inspector tokens
  carry. Prometheus (in Docker) scrapes them via `host.docker.internal`.
- Tracing is on by default (`MCP_OTLP_ENDPOINT=localhost:4318` → Jaeger). Generate
  a trace by making any `/mcp` or console request, then explore the span tree in Jaeger.
- `make dev-down` tears everything down (including volumes).

> **Hermetic build/test** (no stack needed): `go build ./... && go test ./...`.

## Documentation

Full docs live in [`docs/`](docs/) and render as a small site:

| Doc | |
|---|---|
| [Overview](docs/README.md) | What it is, goals, the 7 verticals |
| [Architecture](docs/architecture.md) | Control/data planes, components, request flows |
| [Features](docs/features.md) | The seven user stories |
| [Security Model](docs/security.md) | Isolation, sandboxing, secrets, SSRF, audit |
| [Observability](docs/observability.md) | Logs, metrics, traces, dashboards, alerts |
| [Solution Comparison](docs/solution-comparison.md) | Architectures & runtimes vs. the 7 criteria |
| [Data Model](docs/data-model.md) | Entities & relationships |
| [Local Dev & Runbook](docs/local-dev.md) | Stack, endpoints, integration tests |
| [Local gVisor Sandbox](docs/local-sandbox.md) | The HC-3 boundary locally |
| [Multi-tenant (Keycloak)](docs/multi-tenant-keycloak.md) | Realm-per-org, onboarding, MCP scopes, roles, invites |
| [MCP Clients & RBAC](docs/mcp-inspector-rbac.md) | Connect the Inspector/clients; validate per-user tool access |

### Viewing the docs site

The site is client-rendered (the Markdown files are the single source of truth),
so view it over HTTP, not `file://`:

```sh
cd docs && python3 -m http.server 8099   # then open http://localhost:8099
```

### Publishing to GitHub Pages

1. Push this repo to GitHub.
2. **Settings → Pages → Build and deployment**: Source = *Deploy from a branch*,
   Branch = `main`, Folder = **`/docs`**. Save.
3. The site publishes at `https://<user>.github.io/<repo>/` (entry: `docs/index.html`).

`docs/.nojekyll` is present so the `.md` files are served raw for the client-side
renderer.

## Repository layout

```
services/   gateway · control-plane · sandbox-supervisor
pkg/        config · logging · authz · secrets · serverevents · audit · telemetry
deploy/dev/ docker compose stack + Prometheus/Grafana/Jaeger + Postgres init
specs/      feature spec, plan, research, data-model, contracts, tasks (scope of record)
docs/       documentation + GitHub Pages site
```

See [`CLAUDE.md`](CLAUDE.md) for build/test/run conventions and architecture at a
glance.
