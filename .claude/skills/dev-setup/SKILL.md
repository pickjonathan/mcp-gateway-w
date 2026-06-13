---
name: dev-setup
description: Verify and install local prerequisites (Go, Docker, Make, git; Lima/gVisor for the sandbox), build and test the Go services, bring up the dev service stack (Postgres/Redis/Vault/Keycloak + MinIO audit archive + Prometheus/Grafana/Jaeger observability), and optionally provision the gVisor sandbox VM. Use this when onboarding a new machine, or to check/repair a broken local environment for the multi-tenant MCP runtime.
metadata:
  author: mcp-runtime
  scope: project
---

# Local Dev Environment Setup

Bootstraps a working local environment for the multi-tenant MCP runtime:
**prerequisites → build/test → dev services → (optional) gVisor sandbox (HC-3).**

## User Input

```text
$ARGUMENTS
```

Interpret `$ARGUMENTS` as a mode (default `all`):

- `doctor` — read-only prerequisite + environment check (installs nothing).
- `services` — doctor, then install missing prerequisites and bring up the dev service stack.
- `sandbox` — provision the Lima Docker VM + gVisor (the HC-3 isolation boundary).
- `all` — `services` then `sandbox`.

## Execution flow

1. **Always start with `doctor` (read-only).** Run:

   ```sh
   .claude/skills/dev-setup/scripts/doctor.sh
   ```

   It prints a status table of prerequisites, the project files, and which dev
   services are running, and exits non-zero if a *required* tool is missing.
   Summarize the result to the user.

2. **Stop here if mode is `doctor`.**

3. **Before installing anything or booting a VM, confirm.** `setup.sh` installs
   OS packages (Homebrew on macOS, apt on Linux) and starts containers;
   `sandbox-up.sh` boots a Lima VM and installs gVisor. List exactly what will
   happen and get the user's go-ahead **unless they already approved**.

4. **Services** (mode `services`/`all`):

   ```sh
   .claude/skills/dev-setup/scripts/setup.sh
   ```

   Installs missing prerequisites, runs `go build ./...` + `go test ./...`, and
   brings up the dev stack via `deploy/dev/compose.yaml` — Postgres/Redis/Vault/
   Keycloak + MinIO (audit archive) + Prometheus/Grafana/Jaeger (observability) —
   waiting for readiness, then prints the dev UIs (Grafana :3000, Prometheus
   :9090, Jaeger :16686, MinIO console :9001, Keycloak :8081).

5. **Sandbox / HC-3** (mode `sandbox`/`all`, macOS):

   ```sh
   .claude/skills/dev-setup/scripts/sandbox-up.sh
   ```

   Creates a Lima Docker VM, installs gVisor (`runsc`) — **no nested virtualization
   required** — registers it as a Docker runtime, builds the base sandbox image,
   and verifies the gVisor kernel is active. See `docs/local-sandbox.md`.

6. **Report**: what was installed, what's running (with ports), and how to run
   the services (`make run` for the gateway; `MCP_SANDBOX_RUNTIME=gvisor` to use
   the sandbox). Surface any failures with the exact remediation command.

## Notes

- All scripts are **idempotent** — safe to re-run; they skip already-satisfied steps.
- `doctor.sh` is read-only. `setup.sh` and `sandbox-up.sh` modify the system.
- Required to build/run: **Go ≥ 1.25, Docker (daemon reachable), Make, git**.
- Sandbox path additionally needs **Lima** (macOS) or **gVisor** (Linux).
- Service ports: Postgres `5432` · Redis `6379` · Vault `8200` · Keycloak `8081` ·
  gateway `8080` · control-plane `8090`.
