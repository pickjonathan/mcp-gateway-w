# Local Dev & Runbook

## Prerequisites

Go 1.25+, Docker (daemon running), Make, git. For the real stdio sandbox on
macOS: Lima + gVisor. The **`/dev-setup`** skill checks/install these and brings
up the stack.

```sh
/dev-setup            # all: verify prereqs → build/test → start dev stack
/dev-setup doctor     # read-only environment check
/dev-setup services   # install + start the dev stack only
/dev-setup sandbox    # provision the Lima VM + gVisor (HC-3 boundary)
```

Or manually:

```sh
go build ./... && go test ./...
docker compose -f deploy/dev/compose.yaml up -d postgres redis vault keycloak minio jaeger
docker compose -f deploy/dev/compose.yaml up -d --no-deps prometheus grafana
```

## Dev stack & endpoints

| Service | URL / addr | Credentials |
|---|---|---|
| Gateway | http://localhost:8080 (`make run`) | OAuth (Keycloak) |
| Control plane | http://localhost:8090 | OAuth (admin role) |
| Keycloak | http://localhost:8081 | admin / admin |
| PostgreSQL | localhost:5432 | mcp / mcp (runtime role: `mcp_app`) |
| Redis | localhost:6379 | — |
| Vault | http://localhost:8200 | token `dev-root` |
| MinIO | http://localhost:9001 (console) | minioadmin / minioadmin |
| Prometheus | http://localhost:9090 | — |
| Grafana | http://localhost:3000 | anon admin (dev) |
| Jaeger | http://localhost:16686 | — |

## Running the services

```sh
make run    # gateway on :8080 — uses the exec sandbox (UNSANDBOXED; dev only)
MCP_HTTP_ADDR=:8090 go run ./services/control-plane/cmd/control-plane
```

Key environment (all `MCP_`-prefixed, loaded into the config singleton):

| Var | Purpose | Dev default |
|---|---|---|
| `MCP_BASE_DOMAIN` | `{org}.{base}` routing | `mcp.example.com` |
| `MCP_POSTGRES_DSN` | server store | (compose: `mcp_app@postgres`) |
| `MCP_REDIS_ADDR` | events + fleet quotas | `localhost:6379` |
| `MCP_VAULT_ADDR` / `_TOKEN` | secrets | — / — |
| `MCP_KEYCLOAK_ISSUER_TEMPLATE` | per-org realm issuer | `…/realms/%s` |
| `MCP_SANDBOX_RUNTIME` | `exec`\|`gvisor`\|`kata` | `gvisor` |
| `MCP_OTLP_ENDPOINT` | traces export | unset (→ Jaeger `localhost:4318`) |
| `MCP_BLOCK_PRIVATE_EGRESS` | SSRF guard | off in dev / on in prod |
| `MCP_RATE_ORG_PER_MIN` / `_USER_PER_MIN` | quotas | 0 (unlimited) |
| `MCP_AUDIT_S3_ENDPOINT` … | durable audit archive | unset (→ in-memory) |
| `MCP_AUDIT_DENY_PER_MIN` | denial-audit cap | 600 |

## Tests

```sh
go test ./...    # hermetic; integration tests skip unless MCP_TEST_* is set
```

Live integration tests (run against the dev stack):

```sh
MCP_TEST_POSTGRES_DSN='postgres://mcp:mcp@localhost:5432/mcp?sslmode=disable' \
MCP_TEST_REDIS_ADDR=localhost:6379 \
MCP_TEST_VAULT_ADDR=http://localhost:8200 \
MCP_TEST_S3_ENDPOINT=localhost:9000 \
  go test ./...
```

> **RLS note:** Row-Level Security is enforced only for the non-superuser
> `mcp_app` role, created by `deploy/dev/postgres-init` on a **fresh** Postgres
> data dir. If you change that init, run `make dev-down` then bring the stack up
> again so Postgres is recreated.

## The gVisor sandbox (HC-3, macOS)

```sh
/dev-setup sandbox          # Lima VM + gVisor (runsc)
tests/security/adversarial.sh   # run the containment suite inside the VM
```

## Teardown

```sh
make dev-down               # stop & remove the compose stack
limactl stop mcp && limactl delete mcp   # remove the sandbox VM
```
