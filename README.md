# Multi-Tenant MCP Gateway Runtime

A platform that connects an organization's end users to MCP tooling through one
OAuth-protected endpoint — `https://{org}.{base-domain}/mcp` — while org admins
add **remote (HTTP)** or **stdio** MCP servers (including untrusted ones) with no
infrastructure work of their own.

Go module: `github.com/acme-corp/mcp-runtime` (Go 1.25).

## Quick start

```sh
go build ./... && go test ./...        # build + hermetic test suite
/dev-setup                             # bring up the dev stack (Claude Code skill)
make run                               # gateway on :8080 (dev: exec sandbox = UNSANDBOXED)
```

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
