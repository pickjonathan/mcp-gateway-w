# Two-Tenant AWS-MCP Isolation Proof

The **end-to-end, adversarial, load-tested proof** that the platform's #1 invariant —
**tenant isolation** (Constitution I / HC-1) — actually holds, demonstrated with a
realistic workload instead of a toy. Spec: [`specs/004-aws-mcp-isolation-proof/`](../specs/004-aws-mcp-isolation-proof/).
Harness: [`tests/isolation-proof/`](../tests/isolation-proof/).

## What it sets up

Two tenants (`alpha`, `beta`), each:

- provisioned as its own isolated org/realm (reusing [tenant provisioning](tenant-provisioning.md));
- with its own **stdio AWS MCP server** (`awslabs.aws-api-mcp-server`, pre-baked into the
  sandbox image) running **under the gVisor microVM sandbox**;
- with its own **AWS account + write-only credentials** (a distinct 12-digit access key =
  account id) and its own **S3 bucket** in a local [`ministack`](https://github.com/ministackorg/ministack)
  emulator added to the dev `compose.yaml`.

## What it proves (two boundaries)

1. **Gateway / tenant boundary (primary, HC-1) — emulator-independent.** `alpha`'s token is
   rejected at `beta`'s `/mcp` (audience/issuer mismatch); `alpha`'s catalog never reveals
   `beta`'s server; `alpha`'s admin token is denied on `beta`'s control-plane org. Every attempt
   **fails closed and is audited**.
2. **Downstream resource boundary (secondary) — emulator-backed.** Through `alpha`'s own AWS
   server (account A credentials), `beta`'s bucket is inaccessible (per-account S3 namespacing).

It also proves, under a **smoke load** (~10 concurrent sessions/tenant for ~1 min): per-tenant
error rate < 1% and p95 ≤ 2 s, **independent quotas** (one tenant throttled doesn't affect the
other), and **zero cross-tenant leakage**. And it proves **sandbox egress containment**: a server
in the sandbox reaches *only* the emulator — never the control plane, cloud metadata, or the
internet (SC-010).

## The one production change

Everything is composition + a test harness **except** one additive, default-off gateway config:
**`MCP_SANDBOX_EGRESS_NETWORK`**. Sandboxed stdio servers run `--network none` by default
(Constitution II's *default-deny*); this wires the *allowlist* half the principle already
specifies — the sandbox joins a Docker `internal: true` network whose only member is the emulator.
Default behavior is unchanged when the var is unset. It ships with **written-first adversarial
tests** (`services/gateway/internal/sandbox/egress_test.go`).

## Run it

Prerequisites and the full walkthrough: [`specs/004-aws-mcp-isolation-proof/quickstart.md`](../specs/004-aws-mcp-isolation-proof/quickstart.md).

```sh
bash .claude/skills/dev-setup/scripts/sandbox-up.sh   # gVisor/Lima VM (macOS)
make dev-up && make seed-platform                     # stack + platform realm
make sandbox-image                                    # sandbox image w/ AWS MCP server baked in
make run-gateway-proof                                # gateway: gVisor + egress allowlist
make prove-isolation                                  # the proof → report.json + pass/fail
```

Exit codes: `0` pass · `1` a check failed · `2` precondition/preflight failed.

## The FR-018 honesty gate

`ministack` isolates by **account namespacing**, not IAM denial. So before any downstream claim,
the harness runs a **preflight**: create a bucket under account A, then try to reach it with
account B's credentials. If B *can* reach it, S3 isn't per-account isolated in this emulator → the
harness **aborts loudly (exit 2)** rather than emit a dishonest PASS. The primary gateway boundary
is unaffected either way. See [research.md](../specs/004-aws-mcp-isolation-proof/research.md) D1.

## Status

The egress-allowlist code change is implemented and adversarially tested (green). The infra and
Node harness are code-complete; the end-to-end run is the operator's acceptance gate on a gVisor
host (run locally; CI wiring deferred).
