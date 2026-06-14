---
description: "Task list for Two-Tenant AWS-MCP Isolation Proof"
---

# Tasks: Two-Tenant AWS-MCP Isolation Proof

**Input**: Design documents from `/specs/004-aws-mcp-isolation-proof/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Test tasks are **REQUIRED** for this feature. The deliverable *is* an acceptance proof, and
the one production change (sandbox egress allowlist) touches isolation/sandbox code → Constitution V
mandates written-first adversarial tests (the constitution explicitly requires `/speckit-tasks` to emit
them for this project).

**Organization**: By user story (US1 functional → US2 adversarial isolation → US3 stress → US4
one-command/teardown), each an independently runnable increment of the proof harness.

## Format: `[ID] [P?] [Story] Description`
- **[P]**: different files, no dependency on an incomplete task → parallelizable
- File paths are exact; harness lives in `tests/isolation-proof/` (Node/TS), the one code change in `pkg/`/`services/gateway/`.

## Two independent tracks in Foundational
- **Track A (Go, code change)**: T006 → T007 → T008 (egress allowlist + written-first adversarial tests).
- **Track B (Node, harness libs)**: T009–T018. Track A and Track B can proceed fully in parallel.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: dev-stack and harness scaffolding

- [X] T001 [P] Add `ministack` service + a `mcp-sandbox-egress` (`internal: true`) Docker network to `deploy/dev/compose.yaml` (ministack on `:4566`, joined to both `mcp-runtime-dev` and `mcp-sandbox-egress`; `SERVICES=s3,iam,sts`) per `contracts/sandbox-egress.md`
- [X] T002 [P] Extend `deploy/sandbox-images/Dockerfile` to pre-bake `awslabs.aws-api-mcp-server` + the AWS CLI (so the stdio server launches under egress-restricted gVisor with no PyPI fetch); verify `docker build` succeeds
- [X] T003 [P] Scaffold `tests/isolation-proof/` — `package.json` (deps: `@modelcontextprotocol/inspector`, `@modelcontextprotocol/sdk`, an S3 client, `tsx`/`typescript`), `tsconfig.json`, `README.md` (FR→check map)
- [X] T004 [P] Add Makefile targets: `sandbox-image` (build the extended image), `run-gateway-proof` (run gateway with `MCP_SANDBOX_RUNTIME=gvisor` + `MCP_SANDBOX_EGRESS_NETWORK=mcp-sandbox-egress`), and `prove-isolation` (run the harness) 
- [X] T005 [P] Create harness config module `tests/isolation-proof/lib/config.ts` (tenant slugs `alpha`/`beta`, account ids `111111111111`/`222222222222`, bucket names, endpoints, base-domain, env knobs from `contracts/proof-harness.md`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**⚠️ CRITICAL**: No user-story work can begin until this phase is complete. (Track A and Track B run in parallel.)

### Track A — sandbox egress allowlist (Go; written-first)

- [X] T006 Write **FAILING** adversarial tests in `services/gateway/internal/sandbox/egress_test.go`: (unit) `Select` with empty egress → `buildArgs` contains `--network none`; with `mcp-sandbox-egress` → `--network mcp-sandbox-egress`; (live, gated `MCP_TEST_SANDBOX_DOCKER`) a probe in-sandbox reaches `ministack:4566` but connecting to control-plane / `169.254.169.254` / a public host fails (FR-017/SC-010). Must fail before T007/T008.
- [X] T007 Add `MCP_SANDBOX_EGRESS_NETWORK` (string, default `""`) to `pkg/config/config.go` → `Config.SandboxEgressNetwork`
- [X] T008 Wire it through: set `ContainerRuntime{Network: egressNetwork}` in `services/gateway/internal/sandbox/exec.go` `Select(...)`, and pass `cfg.SandboxEgressNetwork` from `services/gateway/cmd/gateway/main.go`; run T006 → now **passes**. Confirm `--network none` is byte-for-byte unchanged when unset (FR-016)

### Track B — proof-harness libraries (Node/TS)

- [X] T009 [P] Implement per-realm headless token minting (OAuth password grant via `mcp-client`) in `tests/isolation-proof/lib/tokens.ts` (D6)
- [X] T010 [P] Implement tenant provisioning + status polling (platform API `POST/GET /v1/platform/tenants`) in `tests/isolation-proof/lib/tenants.ts`
- [X] T011 [P] Implement MCP Inspector CLI wrapper (`--cli --transport http --header "Authorization: Bearer …"`, `tools/list`, `tools/call`, JSON stdout parse, content-based assertions) in `tests/isolation-proof/lib/inspector.ts`
- [X] T012 [P] Implement S3/AWS helper (createBucket, put/get/list with explicit per-account creds against `:4566`) in `tests/isolation-proof/lib/aws.ts`
- [X] T013 [P] Implement audit query helper (`GET /v1/orgs/:org/audit`, find `auth.denied`/`authz.denied`, assert no secret in `Metadata`) in `tests/isolation-proof/lib/audit.ts`
- [X] T014 [P] Implement report writer (JSON schema + exit codes 0/1/2) in `tests/isolation-proof/lib/report.ts` per `contracts/proof-harness.md`
- [X] T015 [P] Implement per-tenant setup orchestration (provision 2 tenants → register stdio `aws` server → `PUT` write-only creds → create own bucket) in `tests/isolation-proof/lib/setup.ts` (uses T009–T012) per `contracts/tenant-aws-setup.md`
- [X] T016 [P] Implement teardown + clean post-check (delete creds/servers/tenants/buckets; assert 0 residual) in `tests/isolation-proof/lib/teardown.ts` (uses T010, T012)
- [X] T017 [P] Implement **preflight** (FR-018) in `tests/isolation-proof/lib/preflight.ts`: (1) `endpoint_override` hits emulator; (2) `s3_per_account_isolation` — bucket under acct A is **inaccessible** with acct B creds; on failure abort with exit 2 (uses T012)
- [X] T018 Implement orchestrator skeleton `tests/isolation-proof/prove.ts`: parse flags → run preflight → setup → (story hooks) → write report → teardown (depends on T014–T017)

**Checkpoint**: egress works under gVisor; both tenants can be provisioned/credentialed/bucketed; harness runs end-to-end skeleton with preflight gating.

---

## Phase 3: User Story 1 - Two tenants each operate their own AWS MCP server (Priority: P1) 🎯 MVP

**Goal**: Each tenant, via the MCP Inspector against its own endpoint, lists tools and reads/writes objects in its own bucket with its own credentials.

**Independent Test**: Run setup + US1; both tenants `tools/list` and put/get/list an object in their own bucket; write-only creds never returned.

- [X] T019 [P] [US1] Implement functional check in `tests/isolation-proof/lib/checks/us1.ts`: per tenant `tools/list` includes `call_aws`; `call_aws` put → list/get round-trips an object in that tenant's **own** bucket (FR-007); uses `inspector.ts`/`aws.ts`
- [X] T020 [P] [US1] Add write-only-credential assertion to `us1.ts`: after `PUT …/credentials`, the response returns no value and `credential_set` flips true; no credential value appears in the tool output (FR-006)
- [X] T021 [US1] Wire US1 checks into `prove.ts` and record per-check results into the report

**Checkpoint**: MVP — both tenants demonstrably use their own AWS bucket end-to-end via the Inspector.

---

## Phase 4: User Story 2 - Cross-tenant access is impossible, and proven (Priority: P1)

**Goal**: For both ordered tenant pairs, every cross-tenant vector fails closed and (where applicable) is audited; no secret/foreign data leaks.

**Independent Test**: Run setup + US2; all of V1–V6 deny/hide as specified and audit records exist; 0 secret hits.

- [X] T022 [P] [US2] Implement V1 (src token → dst `/mcp` rejected, no dst data) + audit assertion (`auth.denied`) in `tests/isolation-proof/lib/checks/us2.ts`
- [X] T023 [P] [US2] Implement V2 (dst's `aws` server absent from src's `tools/list`) and V3 (invoking dst's server from src denied/looks-absent) + audit assertion (`authz.denied`) in `us2.ts`
- [X] T024 [P] [US2] Implement V4 (via src's own server+creds, `s3 ls`/`cp` on dst's bucket → NoSuchBucket/denied; dst bucket unchanged) for both ordered pairs in `us2.ts` (FR-009c)
- [X] T025 [P] [US2] Implement V6 secret/foreign-data leak scan over all US2 stdout/stderr + gateway/server logs (FR-011) in `us2.ts`
- [X] T026 [US2] Implement SC-010 / V5 egress-containment harness check (from a sandbox on `mcp-sandbox-egress`: emulator reachable; control-plane/`169.254.169.254`/other infra blocked) in `tests/isolation-proof/lib/checks/egress.ts` (complements Go test T006)
- [X] T027 [US2] Wire US2 + SC-010 checks into `prove.ts`, recording denial evidence + audit record refs into the report

**Checkpoint**: cross-tenant access proven impossible and audited; sandbox egress contained.

---

## Phase 5: User Story 3 - Isolation holds under concurrent load (Priority: P2)

**Goal**: Smoke load on both tenants simultaneously meets the error/latency budget, quotas isolate tenants, and zero cross-tenant leakage occurs under contention.

**Independent Test**: Run setup + US3; per-tenant error_rate < 1% and p95 ≤ 2 s; alpha throttled doesn't affect beta; 0 leakage hits during the run.

- [X] T028 [P] [US3] Implement concurrent smoke-load driver (MCP SDK Streamable-HTTP, ~10 sessions/tenant, ~60 s, both tenants at once, looping S3 ops on own bucket) in `tests/isolation-proof/lib/stress.ts` (FR-012)
- [X] T029 [P] [US3] Implement continuous cross-tenant leakage assertion sampled during the load run in `tests/isolation-proof/lib/checks/us3.ts` (SC-004)
- [X] T030 [US3] Implement quota-independence check in `us3.ts`: drive alpha past `MCP_RATE_ORG_PER_MIN` → alpha gets JSON-RPC `-32000`; beta success rate unchanged; classify `-32000` as expected, excluded from the error budget (D8, FR-013/SC-006)
- [X] T031 [US3] Compute per-tenant metrics (calls, non-quota error_rate < 1%, p95 ≤ 2 s) and wire US3 into `prove.ts`/report (SC-005)

**Checkpoint**: isolation holds under load with independent quotas and zero leakage.

---

## Phase 6: User Story 4 - One-command, reproducible setup, proof, and teardown (Priority: P3)

**Goal**: `make prove-isolation` runs the entire proof to a clear machine-readable pass/fail and leaves a clean environment, deterministically.

**Independent Test**: From a clean env, one command runs green < 15 min, no manual steps; teardown leaves 0 residual; 3 consecutive runs all exit 0.

- [X] T032 [US4] Finalize the single-entry sequence in `prove.ts` (preflight → setup → US1 → US2 → US3 → SC-010 → report → teardown), no manual steps, < 15 min budget (FR-015/SC-001)
- [X] T033 [US4] Finalize machine-readable `report.json` (full schema) + exit codes (0 pass / 1 check-failed / 2 precondition-or-preflight) in `report.ts`/`prove.ts` (FR-014)
- [X] T034 [US4] Implement teardown verification post-check asserting 0 residual tenants/servers/creds/buckets in `teardown.ts`; wire into `prove.ts` (SC-008)
- [X] T035 [US4] Determinism gate: script/document 3 consecutive `make prove-isolation` runs all exit 0 in `tests/isolation-proof/README.md` (SC-009)

**Checkpoint**: reproducible, self-contained, one-command isolation proof.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T036 [P] Author `docs/isolation-proof.md` (what is proven, the two boundaries, how to run, the FR-018 caveat) and link it from `README.md`
- [X] T037 [P] Reference this proof as the isolation acceptance gate in `specs/001-mcp-server-runtime/quickstart.md` (and note CI wiring deferred)
- [ ] T038 Run `quickstart.md` end-to-end on a gVisor environment; commit a sample `report.json` (sanitized) as evidence
- [X] T039 [P] Ensure `go build ./... && go test ./... && go vet ./...` pass for the gateway change (egress unit tests run hermetically; live tests skip without `MCP_TEST_SANDBOX_DOCKER`)

---

## Dependencies & Execution Order

### Phase dependencies
- **Setup (P1)**: no deps — start immediately.
- **Foundational (P2)**: needs Setup. **Track A (T006→T007→T008)** and **Track B (T009–T018)** are independent and parallel. BLOCKS all user stories.
- **US1 (P3)** → **US2 (P4)** → **US3 (P5)** → **US4 (P6)**: each needs Foundational. They share `prove.ts` + setup/teardown, so wiring tasks (T021, T027, T031, T032) are sequential on `prove.ts`; the per-story check files (`checks/us1.ts`, `us2.ts`, `us3.ts`, `egress.ts`) are independent and [P].
- **Polish (P7)**: after the stories you intend to ship.

### Critical-path note
The **FR-018 preflight (T017)** gates every proof claim, and **Track A (egress, T006–T008)** gates US1's S3 round-trip under gVisor. Front-load both.

### User-story independence
- US1 needs only Foundational. US2/US3 reuse US1's setup but assert different properties — each independently testable against a freshly set-up pair. US4 packages the whole flow.

---

## Parallel Execution Examples

**Foundational (two tracks at once):**
```
Track A (one dev): T006 (failing tests) → T007 → T008
Track B (another):  T009, T010, T011, T012, T013, T014 in parallel → then T015, T016, T017 [P] → T018
```

**User Story 2 (independent check files):**
```
T022, T023, T024, T025 all [P] (all edit checks/us2.ts? → split into us2_v1.ts.. OR sequence within one file)
T026 egress.ts [P] alongside the above
then T027 wires them into prove.ts
```
> If V1–V6 share `checks/us2.ts`, treat T022–T025 as sequential within that file; split into per-vector files to parallelize.

---

## Implementation Strategy

### MVP (recommended: US1 + US2 — both P1)
1. Phase 1 Setup → Phase 2 Foundational (both tracks).
2. Phase 3 US1 → **validate**: both tenants use their own bucket via the Inspector.
3. Phase 4 US2 → **validate**: the actual isolation proof (the point of the feature). Ship this as the meaningful MVP.

### Incremental delivery
US1 (functional) → US2 (adversarial proof) → US3 (stress) → US4 (one-command/teardown/determinism) → Polish. Each adds value without breaking prior stories.

---

## Notes
- **Written-first**: T006 must fail before T007/T008 (Principle V).
- **FR-018 is a hard gate**: T017 aborts (exit 2) rather than emitting a PASS on a non-isolating emulator.
- **No secret values** in any report field, log, or trace (FR-006/FR-011/SC-007) — enforced by T025 + report review.
- The only shipping-code change is T007/T008 (+T006 tests); everything else is dev infra + the Node harness.
- Run `go build ./... && go test ./... && go vet ./...` after Track A; commit in small increments.

---

## Implementation status (2026-06-14)

- **Track A — sandbox egress allowlist (T006–T008): DONE & VERIFIED.** Written-first
  `egress_test.go` confirmed RED (compile failure on the 3-arg `Select`), then GREEN after the
  config + `Select` + caller change. `go build ./...`, `go vet ./...`, and `go test ./...` (21
  packages) all pass; the live containment test (`TestSandboxEgressContainment`) is gated on
  `MCP_TEST_SANDBOX_DOCKER` and skips without a gVisor stack. This is the only production-code change.
- **Infra (T001–T005) + Node harness (T009–T035): CODE-COMPLETE & VALIDATED (short of a full stack).**
  All files written per the contracts. `npm install` + `npx tsc --noEmit` → **type-checks clean (exit 0)**.
  `npm start` **executes**: the ESM import graph + SDK subpaths (`@modelcontextprotocol/sdk`,
  `@aws-sdk/client-s3`) resolve at runtime, the report is written, and with no emulator it **fails closed
  at the FR-018 preflight (exit 2)** with a clear message — not a faked PASS. Running it also surfaced and
  **fixed a real bug** (the `endpoint_override` preflight was passing unconditionally; it now requires a
  genuine S3 response via `$metadata.httpStatusCode`). A few integration specifics stay env-overridable
  (`MCP_PROOF_*`) for first-bring-up tuning (AWS MCP console-script/arg names, admin-audience client,
  Inspector stdout shape).
- **T036 (docs page) + T037 (001 quickstart reference): DONE.** `docs/isolation-proof.md` added + linked
  from `README.md`; the 001 quickstart §6 references this proof as the deeper real-workload gate.
- **Remaining — T038 (live E2E PASS run):** `make prove-isolation` on a **gVisor host** (Lima VM +
  `ministack` + seeded platform realm) to produce a green `report.json`. This is the true acceptance
  gate and cannot be executed in a non-gVisor environment; the harness is validated up to this point.
