# Quickstart: Two-Tenant AWS-MCP Isolation Proof

The one-command proof and what it demonstrates. This walkthrough is the **release acceptance gate** for
tenant isolation (Constitution V) and must pass locally before merge (CI wiring deferred — clarified).

## What you'll prove
Two tenants (`alpha`, `beta`), each with its **own** gVisor-sandboxed AWS MCP server, **own** AWS account
+ write-only credentials, and **own** S3 bucket in a local `ministack` emulator — and that **neither can
reach the other's server, credentials, or bucket**, even under load, with every cross-tenant attempt
failing closed and audited.

## Prerequisites (one-time)
1. **gVisor sandbox VM** (the proof runs under gVisor — required):
   - macOS: `bash .claude/skills/dev-setup/scripts/sandbox-up.sh` (installs Lima + `runsc`, builds the
     sandbox image). Point the gateway's Docker at the Lima daemon (`DOCKER_HOST=…`, see `docs/local-sandbox.md`).
   - Linux: install `runsc` and restart Docker (same script detects Linux).
2. **Dev stack + platform realm**: `make dev-up` then `make seed-platform` (creates `_platform`, the
   `operator` user, and prints `MCP_KEYCLOAK_ADMIN_*` to export for the control plane).
3. **Build the extended sandbox image** (now includes `awslabs.aws-api-mcp-server` + AWS CLI):
   `make sandbox-image` (or the `sandbox-up.sh` build step).

## Run it
```sh
make prove-isolation
```
That single command (FR-015): starts `ministack`, runs preflights, provisions both tenants, registers
each AWS server, sets write-only creds, creates each bucket, runs the Inspector-driven functional +
adversarial suites, the smoke-load stress run, the egress-containment check, writes `report.json`, prints
a PASS/FAIL summary, and tears everything down. Target: **< 15 min**, **no manual steps** (SC-001).

## Preflight gate (FR-018 — fails loud, never fakes)
Before any claim, the harness verifies the emulator can actually isolate S3 **per account**:
> bucket created under account `111111111111` must be **inaccessible** with account `222222222222`
> credentials. If it is reachable, the downstream-denial proof would be dishonest → **the run aborts with
> a clear message and exit code 2**; no PASS is ever emitted on a non-isolating emulator.

## What success looks like
```
PREFLIGHT  s3_per_account_isolation … PASS   endpoint_override … PASS
US1  alpha put/get/list own bucket … PASS    beta … PASS
US2  V1 token@wrong-endpoint denied+audited (a→b,b→a) … PASS
     V2 foreign-server hidden … PASS   V3 foreign-invoke denied+audited … PASS
     V4 foreign-bucket NoSuchBucket (a→b,b→a) … PASS   V6 no-secret-leak … PASS
SC-010 sandbox reaches emulator only (control-plane/metadata blocked) … PASS
US3  smoke 10/tenant×60s  alpha err 0.0% p95 410ms  beta err 0.0% p95 430ms … PASS
     quota independence (alpha throttled, beta unaffected) … PASS
LEAKAGE scanned 1,284 artifacts, 0 hits … PASS
TEARDOWN clean (0 residual tenants/servers/creds/buckets) … PASS
OVERALL: PASS   report.json written   (exit 0)
```

## Determinism (SC-009)
```sh
for i in 1 2 3; do make prove-isolation || { echo "run $i failed"; break; }; done
```
All three must exit `0` on an unchanged system.

## Troubleshooting
- **Preflight `s3_per_account_isolation` FAILS** → this emulator build treats S3 as a global namespace.
  The honest downstream proof can't run; see `research.md` D1 (try a ministack flag that enforces
  per-account S3, or fall back to the documented purpose-built S3 MCP). The **gateway boundary** (US2
  V1–V3) is unaffected and still proven.
- **Sandbox can't reach `ministack`** → confirm `MCP_SANDBOX_EGRESS_NETWORK=mcp-sandbox-egress` on the
  gateway and that `ministack` + the `mcp-sandbox-egress` network exist **in the same Docker daemon that
  launches sandboxes** (the Lima VM), not only on the host (research.md D4).
- **SC-010 fails (sandbox reaches control plane/metadata)** → the egress network is not `internal: true`,
  or the sandbox joined the wrong network. Check `deploy/dev/compose.yaml` and `contracts/sandbox-egress.md`.
- **Tokens rejected on the happy path** → ensure the dev `127.0.0.1 {slug}.mcp.example.com` hosts entries
  exist and the gateway uses `MCP_RESOURCE_TEMPLATE='http://%s.mcp.example.com:8080/mcp'`.
- **`uvx`/network errors at server launch** → the AWS MCP server must be **pre-baked** in the sandbox
  image (no PyPI fetch under `--network`-restricted gVisor); rebuild via `make sandbox-image`.
