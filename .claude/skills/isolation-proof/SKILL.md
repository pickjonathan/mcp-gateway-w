---
name: isolation-proof
description: Guide setting up and running the Two-Tenant AWS-MCP Isolation Proof (spec 004) end-to-end, then open the dashboard that shows the result. Verifies prerequisites (Docker, Go, Node, Lima/gVisor), brings up the dev stack + ministack emulator, builds the sandbox image with the AWS MCP server, starts the gateway in gVisor+egress-allowlist mode, runs the proof harness (Inspector + stress + adversarial checks), and opens the generated report.html dashboard (+ Grafana for live metrics). Use when you want to run, demo, or debug the tenant-isolation proof locally.
metadata:
  author: mcp-runtime
  scope: project
---

# Two-Tenant AWS-MCP Isolation Proof — setup, run, and dashboard

Drives the [`004-aws-mcp-isolation-proof`](../../../specs/004-aws-mcp-isolation-proof/) proof
end-to-end: **prerequisites → dev stack + emulator → sandbox image → gVisor gateway → run the
proof → open the dashboard.** It proves two tenants, each with its own gVisor-sandboxed stdio AWS
MCP server + its own AWS account/creds + S3 bucket (in a local `ministack`), cannot cross each
other's boundary — under load, all attempts fail closed and audited.

Background: `docs/isolation-proof.md`, `specs/004-aws-mcp-isolation-proof/quickstart.md`.

## User Input

```text
$ARGUMENTS
```

Interpret `$ARGUMENTS` as a mode (default `all`):

- `doctor` — read-only: check prerequisites + whether the stack/gVisor/ministack are reachable.
- `setup` — bring up the stack, seed the platform realm, build the sandbox image, ensure ministack
  on the egress network. Installs nothing the user hasn't approved.
- `run` — start the gVisor gateway (proof mode) + run `make prove-isolation`.
- `dashboard` — open the generated `report.html` (+ Grafana) without re-running.
- `all` — `setup` → `run` → `dashboard`.

> **This is a multi-process flow.** The control-plane and gateway are long-running and must stay up
> while the proof runs. Start each in its own terminal (or background it). Tell the user when to open
> a new terminal; never block waiting on a foreground server inside this skill.

## Execution flow

### 1. Doctor (always run first; read-only)

Reuse the base environment check, then add the proof-specific checks:

```sh
bash .claude/skills/dev-setup/scripts/doctor.sh   # Go, Docker, Make, git, Lima/gVisor
node --version                                    # need Node >= 18 (built-in fetch)
docker info --format '{{.OSType}}/{{.ServerVersion}}'   # which daemon? (host vs Lima)
docker run --rm --runtime=runsc alpine true 2>/dev/null && echo "gVisor OK" || echo "gVisor MISSING"
```

Report a short table: each prerequisite OK/missing. The proof **requires gVisor** (clarified
decision) — if `runsc` is missing, the user must provision it (step 2b) before `run`.

### 2. Setup

**2a. Dev stack + platform realm.**

```sh
make dev-up            # postgres/redis/vault/keycloak/minio/observability + ministack (:4566)
make seed-platform     # creates _platform realm + operator; PRINTS MCP_KEYCLOAK_ADMIN_SECRET
```

Capture the printed `MCP_KEYCLOAK_ADMIN_*` values — the control-plane needs them to provision
tenants. Have the user export them (or note them for step 3).

**2b. gVisor sandbox VM + sandbox image (with the AWS MCP server baked in).**

```sh
bash .claude/skills/dev-setup/scripts/sandbox-up.sh    # Lima VM + runsc (macOS); no-op if present
make sandbox-image                                     # rebuilds acme/mcp-sandbox:dev w/ aws-api-mcp-server + AWS CLI
```

**2c. ministack + the egress network must live in the *sandbox* Docker daemon (research.md D4).**
Sandboxes launched by the gateway run in the gVisor daemon (the Lima VM on macOS), so `ministack`
and the `mcp-sandbox-egress` network must exist *there*, not only on the host. Point Docker at the
sandbox daemon before bringing up ministack:

```sh
# macOS/Lima example — set DOCKER_HOST to the Lima/Colima daemon, then:
docker compose -f deploy/dev/compose.yaml up -d ministack
docker network inspect mcp-sandbox-egress >/dev/null 2>&1 && echo "egress network OK"
```

On native Linux (single daemon) `make dev-up` already created both. Verify
`docker network inspect mcp-sandbox-egress` succeeds and `ministack` is running.

**2d. Harness deps.**

```sh
npm --prefix tests/isolation-proof install
go test ./services/gateway/internal/sandbox/   # fast: egress allowlist unit tests must pass
```

### 3. Run

**3a. Control-plane (own terminal)** — with provisioning enabled (values from `seed-platform`):

```sh
MCP_KEYCLOAK_ADMIN_CLIENT_ID=mcp-provisioner \
MCP_KEYCLOAK_ADMIN_SECRET=<printed-by-seed-platform> \
MCP_PLATFORM_REALM=_platform \
make run-control-plane
```

**3b. Gateway in PROOF mode (own terminal)** — gVisor + the emulator-only egress allowlist. Ensure
`DOCKER_HOST` points at the gVisor daemon so sandboxes launch there:

```sh
make run-gateway-proof    # MCP_SANDBOX_RUNTIME=gvisor + MCP_SANDBOX_EGRESS_NETWORK=mcp-sandbox-egress
```

**3c. Run the proof** (once 3a/3b are up):

```sh
make prove-isolation
# or, to keep the tenants for inspection: make prove-isolation ARGS="--keep"
```

The harness runs the **FR-018 preflight** first and **aborts (exit 2)** if `ministack` cannot
isolate S3 per account — it never emits a dishonest PASS. On success every check is green and it
writes `tests/isolation-proof/report.{json,html}`. Exit codes: `0` pass · `1` a check failed ·
`2` precondition/preflight failed. Report the final OVERALL line and the per-story results.

### 4. Dashboard (open the proof)

```sh
bash .claude/skills/isolation-proof/scripts/open-dashboard.sh
```

This opens the self-contained **`report.html`** dashboard (overall PASS/FAIL, preflights, every
check grouped by user story, per-tenant stress metrics, cross-tenant leakage count) and **Grafana**
at http://localhost:3000 (the "MCP Runtime Overview" dashboard shows per-tenant request traffic
during the stress run; Jaeger traces at :16686).

## Determinism / re-run (SC-009)

```sh
for i in 1 2 3; do make prove-isolation || { echo "run $i FAILED"; break; }; done
```

## Troubleshooting

- **Preflight `s3_per_account_isolation` FAILS (exit 2)** — the emulator isn't isolating S3 per
  account; the downstream-denial claim can't be honest (research.md D1). The gateway boundary
  (US2 V1–V3) still holds. Re-check the ministack image/config.
- **`endpoint_override` FAILS** — `ministack` isn't reachable at `MCP_PROOF_AWS_ENDPOINT`
  (default `http://localhost:4566`); confirm it's running and the port is published.
- **Sandbox can't reach ministack / SC-010 emulator check fails** — `ministack` and the
  `mcp-sandbox-egress` network aren't in the *sandbox* Docker daemon (step 2c), or
  `MCP_SANDBOX_EGRESS_NETWORK` isn't set on the gateway (use `make run-gateway-proof`).
- **SC-010 containment FAILS (sandbox reached control-plane/metadata)** — the egress network is not
  `internal: true`; check `deploy/dev/compose.yaml`.
- **Tokens rejected / no operator token** — run `make seed-platform` and start the control-plane
  with the printed `MCP_KEYCLOAK_ADMIN_*`; ensure `/etc/hosts` has
  `127.0.0.1 alpha.mcp.example.com beta.mcp.example.com`.
- **AWS MCP server fails to launch** — the image must be rebuilt with the server baked in
  (`make sandbox-image`); under egress-restricted gVisor it cannot fetch from PyPI at launch.
- Many specifics are env-overridable via `MCP_PROOF_*` (see `tests/isolation-proof/lib/config.ts`).
