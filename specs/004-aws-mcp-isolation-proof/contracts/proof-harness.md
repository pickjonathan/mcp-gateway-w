# Contract: Proof Harness (entry point + report)

The single deliverable surface the operator and (future) CI consume. Lives in `tests/isolation-proof/`.

## Entry point (FR-015 / SC-001)

```sh
make prove-isolation            # one command: preflight → setup → US1..US3 → SC-010 → report → teardown
# passthrough flags (optional):
make prove-isolation ARGS="--keep --tenants alpha,beta --stress-seconds 60 --concurrency 10 --report out.json"
```

Equivalent direct invocation: `node tests/isolation-proof/dist/prove.js [flags]`.

**Flags**
| Flag | Default | Meaning |
|---|---|---|
| `--tenants a,b` | `alpha,beta` | tenant slugs to provision/use |
| `--concurrency N` | `10` | concurrent MCP sessions per tenant (US3) |
| `--stress-seconds S` | `60` | smoke-load duration (US3) |
| `--report PATH` | `tests/isolation-proof/report.json` | machine-readable result |
| `--keep` | off | skip teardown (debugging); default tears down |
| `--no-stress` | off | run US1/US2/SC-010 only |

## Preconditions (asserted by the harness, fail loud if absent)
- `MCP_SANDBOX_RUNTIME=gvisor` and a reachable gVisor Docker daemon (Lima VM on macOS).
- `ministack` reachable at `MCP_PROOF_AWS_ENDPOINT` (default `http://localhost:4566`) **and** on the
  `mcp-sandbox-egress` network from the sandbox daemon.
- Control-plane `:8090` up with platform provisioning enabled; Keycloak `:8081` seeded (`seed-platform`).
- `MCP_SANDBOX_EGRESS_NETWORK=mcp-sandbox-egress` set on the running gateway.

## Preflights (run before any proof claim — FR-018)
1. `endpoint_override` — a `call_aws s3 ls` (or direct SDK call) provably hits the emulator, not real AWS.
2. `s3_per_account_isolation` — create a bucket under account `111111111111`; with account `222222222222`
   credentials it must be **inaccessible**. **If accessible → abort, non-zero exit, no proof emitted.**

Any failed preflight ⇒ `overall.passed=false`, exit ≠ 0, message naming the unmet precondition.

## Report schema (FR-014) — `report.json`

```json
{
  "started_at": "ISO-8601", "finished_at": "ISO-8601", "duration_s": 0,
  "environment": { "sandbox_runtime": "gvisor", "aws_endpoint": "http://localhost:4566",
                   "tenants": ["alpha","beta"], "egress_network": "mcp-sandbox-egress" },
  "preflights": [ { "name": "s3_per_account_isolation", "passed": true, "detail": "" } ],
  "checks": [
    { "id": "US1", "ref": ["FR-007"], "name": "alpha put/get/list own bucket", "passed": true,
      "detail": "", "evidence": "inspector tools/call ok; object roundtrip verified" }
  ],
  "stress": {
    "alpha": { "sessions": 10, "duration_s": 60, "calls": 0, "errors_nonquota": 0,
               "error_rate": 0.0, "p95_ms": 0, "quota_responses": 0 },
    "beta":  { "sessions": 10, "duration_s": 60, "calls": 0, "errors_nonquota": 0,
               "error_rate": 0.0, "p95_ms": 0, "quota_responses": 0 }
  },
  "leakage": { "scanned_artifacts": 0, "hits": 0 },
  "overall": { "passed": true }
}
```

## Exit codes
- `0` — `overall.passed == true` (all preflights, US1, US2, US3, SC-010 passed; leakage hits = 0).
- `1` — a proof check failed.
- `2` — a precondition/preflight failed (environment not ready or FR-018 emulator gate tripped).
- Teardown runs on **all** exits unless `--keep`.

## Determinism (SC-009)
Three consecutive `make prove-isolation` runs on an unchanged system MUST all exit `0`. The harness uses
fixed tenant slugs/account ids/bucket names and idempotent setup (re-provision is a no-op or clean
recreate) so repeated runs converge.
