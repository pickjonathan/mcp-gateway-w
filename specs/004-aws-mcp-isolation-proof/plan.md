# Implementation Plan: Two-Tenant AWS-MCP Isolation Proof

**Branch**: `004-aws-mcp-isolation-proof` | **Date**: 2026-06-14 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/004-aws-mcp-isolation-proof/spec.md`

## Summary

Deliver the **end-to-end, adversarial, load-tested acceptance proof of tenant isolation** the
constitution calls for (Principle V), using a realistic workload: **two tenants**, each with its own
**stdio AWS MCP server** (`awslabs.aws-api-mcp-server`, pre-baked into the sandbox image) running
**under the gVisor microVM sandbox**, its own **AWS account + write-only credentials**, and its own
**S3 bucket** in a **`ministack`** emulator added to the dev stack. The proof drives the live `/mcp`
data plane through the **MCP Inspector**, shows each tenant using its own bucket, then proves —
positively, adversarially, and under a smoke load — that no tested path lets tenant A reach tenant B's
server, credentials, or bucket, and that every cross-tenant attempt fails closed and is audited.

**Technical approach.** Almost everything is **composition + a test harness**: reuse `003` tenant
provisioning, the existing server-CRUD + write-only-credentials APIs, per-org quotas, and the audit
trail. Three small additive pieces of real work are required, all surfaced by Phase 0 research:

1. **One additive, default-off gateway capability — a sandbox egress allowlist.** Today sandboxed
   stdio servers run `--network none` (Principle II's *default-deny*), with the *allowlist* half of the
   principle unimplemented. A gVisor-sandboxed AWS MCP server **must** reach the emulator, so we wire a
   new `MCP_SANDBOX_EGRESS_NETWORK` config into the already-present `ContainerRuntime.Network` field
   (default `""` → `none`, unchanged for everyone else). The proof attaches sandboxes to a Docker
   `internal: true` network whose only member is the emulator → reach emulator, nothing else (no control
   plane, no metadata, no internet). This *completes* Principle II's "default-deny **with an explicit
   allowlist**" rather than weakening it; SC-010 adversarially proves the allowlist is tight.
2. **Sandbox image extension.** Pre-bake `awslabs.aws-api-mcp-server` + the AWS CLI into
   `deploy/sandbox-images/Dockerfile` so the server launches under `--network`-restricted gVisor with no
   PyPI fetch; the AWS CLI honors `AWS_ENDPOINT_URL` to target the emulator.
3. **The proof harness** (`tests/isolation-proof/`, Node/TS) + a `make prove-isolation` entry point:
   an Inspector-driven functional + adversarial suite, a concurrent smoke-load driver, a per-account
   **preflight** that fails loudly if the emulator can't isolate S3 per account (FR-018), and a
   machine-readable pass/fail report. Plus **Go adversarial tests** for the egress allowlist (Principle
   V, written-first).

> **Plan refinement of FR-016.** The spec's FR-016 ("MUST NOT alter sandbox behavior; only adds the
> emulator, server registrations, per-tenant setup, and harness") is honored in spirit — **no isolation
> behavior is weakened and the default is unchanged** — but is **refined** here: the feature adds exactly
> one additive, default-off sandbox capability (the egress allowlist), because running a *real* AWS MCP
> server under gVisor (the clarified requirement) against the emulator is impossible with `--network
> none`. Read FR-016 as "must not **weaken**." Recorded in Complexity Tracking.

## Technical Context

**Language/Version**: Go 1.25 (module `github.com/acme-corp/mcp-runtime`) for the one gateway/config
change + adversarial tests; **Node 20 / TypeScript** for the proof harness (the MCP Inspector is a Node
tool); Bash + Make for orchestration; Docker Compose for the emulator + egress network.
**Primary Dependencies**: *Reused* — `pkg/config`, `services/gateway/internal/sandbox`, the
control-plane admin API (server CRUD, write-only credentials, audit query, quotas read), `003` platform
API, `pkg/audit`. *New (external, dev-only)* — `ministackorg/ministack` (AWS emulator, S3 on `:4566`);
`awslabs.aws-api-mcp-server` (stdio AWS MCP server, pre-baked in the sandbox image) + AWS CLI; `npx
@modelcontextprotocol/inspector` (`--cli`) as the functional/adversarial driver; `@modelcontextprotocol/sdk`
(TS) for the concurrent smoke-load driver.
**Storage**: `ministack` **S3**, **per-account-namespaced** buckets (one bucket per tenant, under that
tenant's 12-digit account id) · **Vault** for per-tenant AWS access keys (write-only, injected at launch)
· existing Postgres/Redis (tenants, servers, quota, audit) unchanged.
**Testing**: Go `testing` (table-driven, hermetic; live parts gated by `MCP_TEST_*`) for the egress
allowlist — **adversarial, written-first** (Principle V); a Node E2E proof harness for US1–US4 emitting
a machine-readable JSON report; the harness is the release acceptance gate (run locally).
**Target Platform**: the `deploy/dev` stack **plus a provisioned gVisor sandbox VM** (Lima on macOS via
`.claude/skills/dev-setup/scripts/sandbox-up.sh`, or native `runsc` on Linux). gVisor is a **hard
prerequisite** of the proof (clarified).
**Project Type**: an acceptance-proof harness + a minimal, additive gateway/infra extension. No new
service; no frontend.
**Performance Goals**: smoke load **~10 concurrent MCP sessions/tenant for ~1 min**, **non-quota error
rate < 1%**, **p95 tool-call latency ≤ 2 s** (SC-005); full proof, one command, **< 15 min** (SC-001).
**Constraints**: tenant isolation inviolable (HC-1); AWS creds **write-only**, never logged (FR-006/
SC-007); sandbox egress **default-deny preserved**, allowlist = emulator only (FR-017/SC-010); gVisor
required; **local-only** — no real cloud, no internet egress from the sandbox except the emulator.
**Scale/Scope**: exactly **two tenants** (`alpha`, `beta`); harness written to extend to N.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Verdict | Notes |
|---|---|---|
| **I. Tenant Isolation Inviolable** | **PASS** | The feature's *purpose* is to prove HC-1 end-to-end. No central tenant table is read on the data path; org is still derived from Host + issuer. The two tenants' AWS resources are partitioned by **distinct account ids + per-account S3 namespacing + write-only per-tenant creds**. Cross-tenant attempts must **fail closed and be audited** (FR-009/010). Adversarial proof is the deliverable (Principle V). |
| **II. Secure Execution of Any MCP** | **PASS (completes the principle)** | The AWS MCP server runs under **gVisor/microVM** (clarified). We add the **explicit egress allowlist** the principle already mandates ("default-deny **with an explicit allowlist**") — previously only `--network none` existed. Default stays `none`; when enabled, the sandbox reaches **only** the emulator (Docker `internal` network), never the control plane, metadata, or internet. SC-010 proves containment. See Complexity Tracking. |
| **III. Frictionless Self-Service** | **PASS (N/A)** | No change to admin self-service or client onboarding. The proof is an operator-run harness. |
| **IV. Hard Constraints Outrank Soft Goals** | **PASS** | We *spend* complexity (gVisor, an emulator, an egress network, a harness) to **uphold** a hard constraint (prove isolation). No hard constraint is traded for a soft goal. Constitution IV explicitly permits paying cost/latency for isolation. |
| **V. Test-First, Adversarially Verified** | **PASS (commitment)** | The egress-allowlist change ships with **written-first adversarial tests** (sandbox can reach *only* the emulator; reaching control-plane/metadata/other hosts fails) that must fail before implementation. The cross-tenant E2E suite (token-at-wrong-endpoint, foreign-server enumeration, foreign-bucket access) is the acceptance proof. |
| **VI. Observable & Auditable by Default** | **PASS** | Each denied cross-tenant attempt must produce a tamper-evident audit record (FR-010), asserted via `GET /v1/orgs/:org/audit` (`auth.denied`/`authz.denied`). Secrets never appear in responses/logs/traces (FR-006/FR-011/SC-007), scanned in the proof. |
| **VII. Simplicity with Justified Complexity** | **PASS w/ tracking** | New surface is deliberately small: **one** additive default-off config; a Dockerfile extension; one compose service; a Node harness. The emulator + harness exist solely because the user requires a *real-workload* isolation proof. Recorded below. |

**Gate result: PASS** (two items in Complexity Tracking; no unjustified violations).

## Project Structure

### Documentation (this feature)

```text
specs/004-aws-mcp-isolation-proof/
├── plan.md              # This file
├── research.md          # Phase 0 — decisions, rationale, alternatives (resolves the FR-018 risk)
├── data-model.md        # Phase 1 — entities (tenant, AWS account, bucket, server reg, proof run/report)
├── quickstart.md        # Phase 1 — one-command walkthrough + preflight (release acceptance gate)
├── contracts/           # Phase 1 — interface contracts
│   ├── proof-harness.md         # `make prove-isolation`, harness CLI, JSON report schema, exit codes
│   ├── sandbox-egress.md        # MCP_SANDBOX_EGRESS_NETWORK + Docker internal network + invariants
│   ├── tenant-aws-setup.md      # per-tenant account/creds/bucket + server-registration payloads (reuse APIs)
│   └── inspector-and-assertions.md  # exact Inspector CLI calls + cross-tenant vectors + audit/stress assertions
└── tasks.md             # Phase 2 — created by /speckit-tasks (NOT here)
```

### Source Code (repository root)

```text
pkg/config/config.go                    # + MCP_SANDBOX_EGRESS_NETWORK (string, default "" → "none")

services/gateway/internal/sandbox/
├── exec.go                             # Select(): set ContainerRuntime{Network: cfg.SandboxEgressNetwork}
├── container.go                        # UNCHANGED — buildArgs already honors r.Network ("" → "none")
└── egress_test.go                      # NEW — adversarial: allowlist tight; none → no egress (Principle V)

services/gateway/cmd/gateway/main.go    # pass cfg.SandboxEgressNetwork into sandbox.Select(...)

deploy/
├── dev/compose.yaml                    # + `ministack` service; + `mcp-sandbox-egress` internal network
└── sandbox-images/Dockerfile           # + pre-bake awslabs.aws-api-mcp-server + AWS CLI (uv/pipx)

tests/isolation-proof/                  # NEW — the Node/TS acceptance-proof harness (the deliverable)
├── package.json                        # @modelcontextprotocol/inspector, @modelcontextprotocol/sdk
├── prove.ts                            # orchestrator: preflight → setup → US1 → US2 → US3 → SC-010 → report → teardown
├── lib/{tenants,inspector,aws,stress,audit,report}.ts   # provisioning, Inspector CLI wrapper, S3 ops, load driver, audit query, JSON report
└── README.md                           # how to run; what each check proves (FR→check map)

Makefile                                # + prove-isolation (one-command, FR-015/SC-001); + sandbox-image build helper
```

**Structure Decision**: **No new service.** The only production code touched is a **single additive,
default-off gateway config** (`MCP_SANDBOX_EGRESS_NETWORK`) wired into the existing
`sandbox.ContainerRuntime.Network`. Everything else is **dev infrastructure** (one compose service, a
Dockerfile extension, one internal network) and a **self-contained Node proof harness** under
`tests/isolation-proof/`. The gateway/control-plane request paths, RLS, token validation, quotas, and
audit are **unchanged** — which is the point: the proof exercises them as-is.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| **Additive sandbox egress allowlist** (`MCP_SANDBOX_EGRESS_NETWORK`) — extends sandbox networking, in tension with the literal FR-016 "must not alter sandbox behavior" | The clarified requirement is to run the stdio AWS MCP server **under gVisor**; under `--network none` it cannot reach the emulator, so the happy path is impossible. The allowlist is **additive and default-off** (empty → `none`, today's behavior) and **implements** Constitution II's already-specified "explicit allowlist." It is the *minimal* change that makes a real-workload gVisor proof possible. | (a) Run **unsandboxed** (`exec`) — rejected: contradicts the clarification and would prove nothing about containment. (b) **Custom iptables/firewall** inside the sandbox — rejected: far more complex and error-prone than a Docker `internal` network; harder to test. (c) **Remote-HTTP AWS server** instead of stdio — rejected: the user explicitly requires stdio. |
| **Emulator + Node proof harness** (new compose service, sandbox-image extension, Node/TS suite — added surface vs. Principle VII) | The user's explicit requirement is a *real-workload*, Inspector-driven, stress-tested **proof** that isolation holds — not another unit test. `ministack` provides local S3 with no cloud/cost; the Inspector is the mandated driver (Node); the harness is the machine-readable acceptance gate (FR-014/015). | (a) **Pure Go harness** — rejected: FR-007 mandates the MCP Inspector, which is a Node CLI. (b) **k6/vegeta** for stress — rejected: not MCP/JSON-RPC-aware and can't assert cross-tenant leakage. (c) **Mock S3** — rejected: a real emulator with per-account namespacing is what makes the downstream-denial claim honest. |

*Post-Phase-1 re-check: see the foot of [research.md](./research.md) — design holds the gate; the one
production change is additive and adversarially tested; no new violations introduced.*
