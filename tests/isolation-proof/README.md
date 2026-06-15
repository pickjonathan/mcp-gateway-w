# Two-Tenant AWS-MCP Isolation Proof (harness)

Acceptance-proof harness for `specs/004-aws-mcp-isolation-proof/`. Drives the live
`/mcp` data plane through the MCP Inspector + an MCP-SDK load driver, proves two
tenants cannot cross each other's boundary, and emits a machine-readable report.

## Run

```sh
# from repo root (preferred — handles deps + flags):
make prove-isolation
make prove-isolation ARGS="--keep --concurrency 10 --stress-seconds 60"

# or directly:
npm install && npm start -- --tenants alpha,beta
```

See `../../specs/004-aws-mcp-isolation-proof/quickstart.md` for prerequisites
(gVisor/Lima VM, `make dev-up`, `make seed-platform`, `make run-gateway-proof`,
`ministack` reachable on the sandbox egress network).

## What each check proves (FR/SC → check)

| Story | Check | Spec |
|---|---|---|
| preflight | `endpoint_override`, `s3_per_account_isolation` (aborts loudly) | FR-001, **FR-018** |
| US1 | tools/list, server writes+reads own bucket, creds write-only | FR-006/007, SC-007 |
| US2 | V1 token@wrong-endpoint+audit · V2 catalog isolation · V3 cross-org admin denied · V4 foreign-bucket denied · V6 no secret leak | FR-009/010/011, SC-003/007 |
| SC-010 | sandbox reaches emulator only; control-plane/metadata/internet blocked | FR-017, SC-010 |
| US3 | smoke load error<1% & p95≤2s; quota independence; zero leakage under load | FR-012/013, SC-004/005/006 |
| US4 | one-command flow; machine-readable report+exit codes; teardown clean | FR-014/015, SC-001/008 |

Exit codes: `0` pass · `1` a check failed · `2` precondition/preflight failed.

## Determinism (SC-009)
```sh
for i in 1 2 3; do make prove-isolation || { echo "run $i FAILED"; break; }; done
```

## Notes / live-tuning
This harness must be validated against a live gVisor stack; a few integration
specifics may need tuning on first bring-up and are intentionally env-overridable
(`MCP_PROOF_*`): the AWS MCP server's console-script name and its `call_aws` arg
name, the admin-audience client used for password grants, and the exact Inspector
CLI stdout shape. The FR-018 preflight is a hard gate — it aborts (exit 2) rather
than emit a PASS on an emulator that does not isolate S3 per account.
