# Phase 0 Research: Two-Tenant AWS-MCP Isolation Proof

Decisions that resolve the spec's open items (notably **FR-018 / the emulator-capability risk**) and
the technical unknowns surfaced while grounding the plan in the existing runtime. Each entry: **Decision
· Rationale · Alternatives considered**. File references are to the current `main`-line code.

---

## D1 — Emulator: `ministack`; isolation by per-account namespacing (with a fail-loud preflight)

**Decision.** Use `ministackorg/ministack` (Docker image, S3 on `:4566`). Model each tenant's "account"
as a **distinct 12-digit `AWS_ACCESS_KEY_ID`** — ministack treats a 12-digit access key as the **AWS
account id** and namespaces resources per account. Tenant `alpha` → account `111111111111`, tenant
`beta` → account `222222222222`; each creates **its own bucket under its own account**. The downstream
isolation claim (US2 / FR-009c) is therefore: **using tenant B's credentials (account B), tenant A's
bucket is not accessible (NoSuchBucket / not listed)** — partitioning by account namespace.

**This depends on ministack namespacing _S3_ per account, which the README does not guarantee
explicitly** (it says resources are "isolated per account at the data/naming level" but does not
document S3 access *denial*). Per **FR-018**, the harness therefore runs a **preflight** before any
proof claim:

> Create `s3://probe` under account `111111111111`; then, with account `222222222222` credentials,
> attempt `head-bucket`/`list-objects` on `probe`. **Expect failure (not found / denied).** If account B
> **can** see account A's bucket, S3 is a shared/global namespace in this emulator → the downstream
> denial is **not** an honest proof → **the harness aborts loudly** with a clear message and non-zero
> exit (no silent degrade to "buckets separated by name only").

**Rationale.** The user explicitly chose ministack and "each realm has its own account with its own
creds + bucket" (clarified: per-tenant creds + bucket). Account-namespacing is exactly that model and is
local/zero-cost. The preflight converts the README's ambiguity into a hard, automated gate, satisfying
FR-018 and keeping the proof honest. **Note the primary HC-1 guarantee — the gateway boundary — does not
depend on the emulator at all** (see D7); the emulator only backs the *secondary, downstream* layer.

**Alternatives considered.** *LocalStack* — rejected: the user chose ministack, and LocalStack's
community edition likewise does not enforce IAM by default. *Real AWS (two accounts)* — rejected: cost,
internet egress, and it would contradict the sandbox egress-allowlist (D3). *Mock/in-proc S3* — rejected:
a real emulator with account namespacing is what makes the downstream claim meaningful.

---

## D2 — AWS MCP server: `awslabs.aws-api-mcp-server` (stdio), endpoint via `AWS_ENDPOINT_URL`

**Decision.** Register `awslabs.aws-api-mcp-server` as each tenant's **stdio** server (it is the AWS MCP
server already referenced as a sample in the admin console). It exposes `call_aws`, which executes AWS
CLI commands — so S3 `mb`/`cp`/`ls`/`get-object` are all reachable. Point it at the emulator by injecting
**`AWS_ENDPOINT_URL=http://<ministack>:4566`** (the AWS CLI honors `AWS_ENDPOINT_URL`/`AWS_ENDPOINT_URL_S3`
globally), with `AWS_REGION` in the (non-secret) server `env`, and the **AWS access key/secret via the
write-only credential API** (injected as env at launch — `creds.go` `kvEnv`). `READ_OPERATIONS_ONLY`
stays **off** (we need `mb`/`put`).

**Validated assumption (preflight #2).** The server's README does not document `AWS_ENDPOINT_URL`, but it
shells out to the AWS CLI, which does honor it. The harness preflight confirms a `call_aws s3 ls` hits
the emulator (not real AWS) before relying on it.

**Rationale.** Matches the existing console sample and the user's "aws mcp"; one server type covers create
-bucket + object ops; credentials flow through the existing write-only path, demonstrating FR-006.

**Alternatives considered.** A **purpose-built minimal S3 stdio MCP server** (explicit endpoint config) —
kept as a **documented fallback** if preflight #2 shows the CLI endpoint override does not take effect in
the sandbox. Service-specific AWS MCP servers — unnecessary; `call_aws` already covers S3.

---

## D3 — Sandbox egress: additive `MCP_SANDBOX_EGRESS_NETWORK` → a Docker `internal` network

**Decision.** Add one config key **`MCP_SANDBOX_EGRESS_NETWORK`** (string, default `""`). In
`sandbox.Select(...)` set `ContainerRuntime{Network: cfg.SandboxEgressNetwork}`. `container.go`
`buildArgs` already maps empty → `--network none` (unchanged default) and a non-empty value →
`--network <name>`. The proof creates a Docker **`internal: true`** network `mcp-sandbox-egress` whose
only member is `ministack`; sandboxes join it and can reach **only** the emulator — no control plane, no
`169.254.169.254` metadata, no internet (an `internal` network has no external route).

Code touched (minimal):
- `pkg/config/config.go` — new key (`services/gateway/.../sandbox/exec.go:52-68` is the consumer).
- `services/gateway/internal/sandbox/exec.go` `Select()` — set `Network`.
- `services/gateway/cmd/gateway/main.go` — pass the config through.
- `container.go` — **no change** (`buildArgs` already honors `r.Network`, default `none`).

**Rationale.** This is the *smallest possible* change that lets a gVisor-sandboxed server reach the
emulator, and it **implements Constitution II's "default-deny with an explicit allowlist"** — the
allowlist half was previously unimplemented. Default behavior is byte-for-byte unchanged for anyone not
setting the key. Docker `internal` networks give "emulator-only, no metadata, no internet" for free,
avoiding bespoke firewall code.

**Alternatives considered.** *Per-server / per-tenant egress networks* (alpha-egress, beta-egress, each
with ministack) — stronger (no sandbox↔sandbox L2) but needs per-server network selection in the gateway;
**deferred as hardening** — for v1 a single shared `internal` network is acceptable because sandboxes run
**no listening services** (stdio only), drop all caps, and are read-only, so shared L2 is not a data path;
SC-010 still proves no sandbox reaches infra/metadata. *In-sandbox iptables* — rejected (complex, fragile,
hard to test). *Unsandboxed `exec`* — rejected (contradicts the gVisor clarification).

---

## D4 — Runtime & Docker context: gVisor via Lima; emulator co-located with sandboxes

**Decision.** Run the proof with **`MCP_SANDBOX_RUNTIME=gvisor`** (`runsc`). On macOS this requires the
**Lima VM** provisioned by `.claude/skills/dev-setup/scripts/sandbox-up.sh` (installs `runsc`, builds the
sandbox image); on Linux, native `runsc`. **`ministack` must run in the same Docker daemon that launches
sandboxes** (the Lima VM's Docker) and join `mcp-sandbox-egress`, so the sandboxed server can reach it by
name. The **control-plane and the proof harness** reach ministack via its **published port** (`:4566`) to
create/verify buckets out-of-band.

**Rationale.** The clarified requirement is gVisor; `dev-setup` already provisions it. Co-locating the
emulator with the sandbox Docker daemon is the only way `--network mcp-sandbox-egress` resolves for the
sandbox. The published port keeps harness/control-plane access simple.

**Alternatives considered.** Running ministack only on the host Docker (where `make dev-up`'s infra runs)
— rejected: the Lima-VM sandboxes couldn't reach it. Documented in quickstart as the key wiring step.

---

## D5 — Proof harness: Node/TS (Inspector + MCP SDK) + Go adversarial tests for the egress change

**Decision.** Two complementary layers:
- **Acceptance proof** — a **Node/TypeScript** harness in `tests/isolation-proof/`, run by
  `make prove-isolation`. It drives `npx @modelcontextprotocol/inspector --cli --transport http <url>
  --header "Authorization: Bearer <token>" --method tools/list|tools/call ...` for the **functional
  (US1)** and **adversarial (US2)** checks (satisfies FR-007/008), uses **`@modelcontextprotocol/sdk`**
  for the **concurrent smoke-load driver (US3)**, queries the audit API for denial records, scans for
  secret/cross-tenant leakage, and emits a **machine-readable JSON report** with an overall pass/fail and
  process **exit code** (0 pass / non-zero fail) — FR-014, consumed locally (FR-014 clarified: CI deferred).
- **Code-change proof** — **Go adversarial tests** (`services/gateway/internal/sandbox/egress_test.go`,
  gated `MCP_TEST_*` where they need Docker) that the allowlist is tight: default `""` → no egress;
  set → reaches *only* the named network; control-plane/metadata unreachable. **Written first** (Principle
  V), must fail before the `Select()` change lands.

**Rationale.** FR-007 mandates the Inspector (a Node CLI); the repo already has Node tooling
(`web/admin-console`). The Inspector CLI emits JSON for scripted assertions; because its exit-code/format
guarantees are thin, the harness asserts on **stdout content** (presence/absence of expected fields), not
exit code alone. The smoke driver uses the MCP SDK to issue many concurrent JSON-RPC calls cleanly
(per-process `npx` overhead would dominate at ~10 concurrent). Go tests cover the one isolation-sensitive
code change hermetically, as the constitution requires.

**Alternatives considered.** Pure-Go E2E (rejected — Inspector is mandated and is Node). N concurrent
`npx inspector` processes for stress (rejected — process overhead, opaque output; the SDK driver replays
the *same* protocol the Inspector speaks, so it is Inspector-equivalent traffic).

---

## D6 — Headless per-tenant tokens: Keycloak password grant via `mcp-client`

**Decision.** Obtain each tenant's data-plane token **per realm** via the OAuth **password grant** against
that realm's seeded **`mcp-client`** (which has `directAccessGrantsEnabled=true` and the MCP-resource
audience mapper), exactly as `make provision-tenant` mints the operator token:
`POST http://localhost:8081/realms/{slug}/protocol/openid-connect/token` with
`grant_type=password&client_id=mcp-client&username=<seeded>&password=<seeded>&scope=openid`. The token's
audience is `{slug}`'s MCP resource (dev: `http://{slug}.mcp.example.com:8080/mcp`).

**Rationale.** No interactive login (FR-008); reuses the seeded data-plane client and the existing dev
pattern. A token minted for `alpha` is, by construction, audience-bound to `alpha` — which is precisely
what the US2 "token at the wrong endpoint" vector exploits.

**Alternatives considered.** Client-credentials grant (rejected — no user/roles for RBAC checks);
Inspector's interactive OAuth/PKCE flow (rejected — not headless).

---

## D7 — The two isolation layers being proven (and which depends on the emulator)

**Decision.** State the proof as **two independent layers**, so a skeptic can see what rests on what:

1. **Gateway/tenant boundary (primary, HC-1) — emulator-independent.** Enforced by token
   audience/issuer binding (`pkg/authz/jwt.go`), Host→org derivation (`OrgFromHost`), per-org catalog
   scoping (`TestToolsListOrgIsolation`), and Postgres RLS. Vectors: `alpha`'s token rejected at `beta`'s
   `/mcp` (audience/issuer mismatch); `alpha` cannot list/invoke `beta`'s server (fails closed, looks
   absent). **Holds regardless of ministack.**
2. **Downstream resource boundary (secondary) — emulator-backed.** Via `alpha`'s own AWS server (account
   A creds), an attempt on `beta`'s bucket → NoSuchBucket (account-namespace partition, D1, preflighted).

**Rationale.** Makes the proof's foundation explicit and robust: even in the worst case where the emulator
can't isolate S3 (D1 preflight fails), the primary HC-1 guarantee is unaffected — the harness just refuses
to assert the secondary layer rather than faking it.

---

## D8 — Per-tenant quota independence (US3 / SC-006)

**Decision.** Run the gateway with a modest per-org limit (e.g. `MCP_RATE_ORG_PER_MIN`) for the proof.
During the smoke run, deliberately drive **one** tenant past its limit and assert: that tenant receives
JSON-RPC `error.code = -32000` ("rate limit exceeded", `services/gateway/internal/mcp/jsonrpc.go`), while
the **other tenant's success rate is unaffected**. The harness counts `-32000` as *expected* (excluded
from the <1% error budget), distinguishing it from unexpected errors (FR-013 / edge case "quota vs errors").

**Rationale.** Per-org enforcement already exists; this proves it isolates load between tenants. No code
change.

---

## D9 — Audit assertions for denials (FR-010 / SC-003)

**Decision.** After each cross-tenant attempt, query `GET /v1/orgs/{org}/audit` (admin token for that org)
and assert a matching record exists with action `auth.denied` or `authz.denied`
(`services/gateway/internal/server/audit.go`), correct `Actor`/`Target`, and **no secret values** in
`Metadata`. The hash-chained record (`pkg/audit`) is the tamper-evident proof.

**Rationale.** The audit query API and denial emit-points already exist; the proof only needs to read and
assert. Note the gateway's `MCP_AUDIT_DENY_PER_MIN` (default 600) anti-amplification cap is far above the
proof's denial volume.

---

## Post-Phase-1 Constitution re-check

After designing the data model, contracts, and quickstart, the gate **still holds**:
- The **only production change** is the additive, default-off `MCP_SANDBOX_EGRESS_NETWORK` (D3), shipped
  with **written-first adversarial tests** (Principle V) and **completing** Principle II's allowlist.
- No request-path, token, RLS, quota, or audit behavior changes — the proof exercises them unchanged
  (Principle I/VI upheld by construction).
- Secrets remain write-only and are scanned for in output (Principle VI; FR-006/SC-007).
- Complexity is bounded and justified (Principle VII; Complexity Tracking).
- The FR-018 risk is converted into a **hard preflight gate** (D1), so the proof cannot silently overclaim.

No new violations introduced; proceed to `/speckit-tasks`.
