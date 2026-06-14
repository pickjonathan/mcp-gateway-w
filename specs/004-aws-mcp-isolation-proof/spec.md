# Feature Specification: Two-Tenant AWS-MCP Isolation Proof

**Feature Branch**: `004-aws-mcp-isolation-proof`
**Created**: 2026-06-14
**Status**: Draft
**Input**: User description: "I want to setup 2 realms, each realm should have its own aws mcp using stdio installed and we should add a https://github.com/ministackorg/ministack to the docker compose, each of the realms should have its own 'account' with its own creds and create its own s3 bucket. this all should be done with automation test using npx @modelcontextprotocol/inspector and we should have proof (like stress test) and we should proof that the security isolation flow works!"

A **reproducible, automated proof that two isolated tenants cannot cross each other's
boundary** — demonstrated with a realistic, real-world MCP workload rather than a toy.
Two organizations are each provisioned as their own isolated tenant; each gets its own
**stdio AWS MCP server**, its own **cloud account + credentials**, and its own **object-storage
(S3) bucket** in a **self-contained local cloud emulator** (`ministack`) added to the dev stack.
The proof drives the live data plane end-to-end through the **MCP Inspector**, shows each
tenant can fully use *its own* AWS tools against *its own* bucket, and then proves — positively
and adversarially, including **under concurrent load** — that **no tested path lets tenant A
reach tenant B's server, credentials, or bucket**, and that every cross-tenant attempt **fails
closed and is audited**.

> **Why this feature exists.** Tenant isolation (Constitution I, HC-1) is the platform's
> primary, non-negotiable guarantee, and Constitution V requires that isolation be **proven by
> automated, adversarial tests — never merely asserted**. Today isolation is exercised by unit
> tests on individual layers (Postgres RLS, catalog scoping). This feature delivers the
> **end-to-end, black-box, load-tested acceptance proof** the constitution calls for, against a
> server that injects per-tenant credentials and touches a real downstream resource (object
> storage) — the case where a leak would be most damaging.

> **Builds on** `001-mcp-server-runtime` (the OAuth-protected `/mcp` data plane, per-org Keycloak
> realms, sandboxed **stdio** servers, write-only per-org/per-user credentials, per-org quotas,
> audit), `002-admin-console` (server registration UX), and `003-tenant-provisioning` (one-command
> tenant bootstrap). This feature **MUST NOT weaken** the inherited hard constraints — **tenant
> isolation** and **secret confidentiality** — and it changes **no isolation-enforcing behavior**;
> it only adds a downstream emulator to the dev stack, two registered AWS MCP servers, per-tenant
> cloud setup, and the automated proof harness.

## Clarifications

### Session 2026-06-14

- Q: What level of AWS-layer isolation must the proof demonstrate (beyond the gateway/tenant boundary)? → A: **Per-tenant credentials + bucket** — each tenant has its own access key/secret scoped to its own bucket; the proof MUST show tenant A's credentials are denied on tenant B's bucket (downstream boundary), in addition to the gateway/tenant boundary. True AWS multi-account/IAM is out of scope.
- Q: Under which execution runtime must the isolation proof run to count as valid? → A: **The hardware-isolated sandbox (gVisor/microVM).** The proof MUST launch the stdio AWS MCP servers under the gVisor microVM runtime so Constitution II egress containment is *proven*, not asserted; a provisioned gVisor sandbox VM (e.g. Lima) is therefore a prerequisite for running the proof.
- Q: What stress profile should the proof target? → A: **Light smoke** — ~10 concurrent MCP sessions per tenant for ~1 minute, non-quota error rate < 1%, and stable p95 tool-call latency ≤ 2 s across the run.
- Q: For v1, how strongly should the proof be wired into CI? → A: **Local-only** — the proof runs locally as the release acceptance gate and emits a machine-readable pass/fail result; wiring it as a blocking CI gate is deferred to a later iteration.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Two tenants each operate their own AWS MCP server, end-to-end (Priority: P1)

A platform operator brings up the dev stack (now including the local cloud emulator), provisions
**two tenants** (e.g. `alpha` and `beta`), and registers an **AWS MCP server (stdio type)** in each
tenant's catalog, scoped to that tenant's own cloud credentials. A user of each tenant connects to
**their own** `{org}.{base-domain}/mcp` endpoint through the **MCP Inspector** and can: list the
AWS tools, and call object-storage tools to **write, read, and list objects in their own bucket**.
Each tenant's calls succeed against **its own** bucket using **its own** account credentials.

**Why this priority**: This is the working substrate the entire proof depends on. Without two
tenants each successfully using a credentialed, real-world MCP server end-to-end, there is nothing
whose isolation can be proven. It is independently valuable on its own as the first automated
end-to-end exercise of the `/mcp` data plane with a credential-injecting stdio server.

**Independent Test**: From a clean stack, provision both tenants, register both AWS MCP servers,
seed each tenant's credentials and bucket, then run the Inspector against each tenant's endpoint to
`tools/list` and to put/get/list an object — and confirm both tenants succeed against their own
bucket. Fully automatable and demonstrable without any of the later stories.

**Acceptance Scenarios**:

1. **Given** the dev stack with the local cloud emulator running and two provisioned tenants each
   with a registered stdio AWS MCP server and its own credentials + bucket, **When** a `alpha` user
   runs the Inspector `tools/list` against `alpha`'s `/mcp` endpoint, **Then** the AWS server's tools
   are listed and the call is authorized by `alpha`'s realm token.
2. **Given** the same setup, **When** the `alpha` user calls an object-storage "put object" tool and
   then a "list/get object" tool, **Then** the object is written to and read back from **`alpha`'s
   bucket** using **`alpha`'s credentials**, and the same flow run by `beta` operates on **`beta`'s
   bucket**.
3. **Given** a registered AWS MCP server, **When** its credentials are set via the admin path,
   **Then** the credential values are stored write-only and are **never** returned, displayed, or
   logged (secret confidentiality preserved).

---

### User Story 2 - Cross-tenant access is impossible, and proven (Priority: P1)

The security-isolation proof. A suite of **adversarial, negative** checks demonstrates that tenant
`alpha` cannot — by any tested vector — reach tenant `beta`'s server, credentials, or bucket, and
vice versa. Each attempt is **denied (fails closed)** and produces an **audit record**. The vectors
exercised at minimum: presenting `alpha`'s token to `beta`'s endpoint (audience/issuer mismatch);
attempting to enumerate or invoke `beta`'s server from `alpha`'s session/catalog; and attempting,
through the AWS tools available to `alpha`, to read or write **`beta`'s bucket** (its credentials
have no access to it).

**Why this priority**: This is the actual point of the feature — the constitution's #1 invariant,
demonstrated rather than asserted. It is independently testable and is the release acceptance gate.

**Independent Test**: With both tenants set up (US1 substrate), run the negative suite: wrong-audience
token rejected with no data disclosed; cross-org server lookup returns "not found" (never another
org's definition); cross-bucket object operations denied by the downstream account boundary. Assert
every attempt is denied **and** that a corresponding audit event was recorded. Passing requires
**zero** cross-tenant successes.

**Acceptance Scenarios**:

1. **Given** a valid token minted by `alpha`'s realm, **When** it is presented to `beta`'s `/mcp`
   endpoint, **Then** the request is rejected (audience/issuer mismatch), no `beta` data is returned,
   and the rejection is audited.
2. **Given** an authenticated `alpha` session, **When** it attempts to list tools from or invoke
   `beta`'s server (by slug or by `beta`'s server identifier), **Then** the gateway behaves as if that
   server does not exist for `alpha` (fails closed), and the attempt is audited.
3. **Given** the AWS tools available to `alpha` (using `alpha`'s injected credentials), **When**
   `alpha` attempts an object operation targeting **`beta`'s bucket**, **Then** the operation is
   denied by the downstream account/credential boundary and no object in `beta`'s bucket is read,
   written, or enumerated.
4. **Given** any of the cross-tenant attempts above, **When** it is denied, **Then** **no secret
   value** (credentials, tokens) appears in any response, log, trace, or error message.
5. **Given** a tenant's AWS MCP server running inside the **gVisor/microVM** sandbox, **When** that
   sandboxed process attempts to reach the control plane, the other tenant's resources, or cloud
   metadata, **Then** every such egress attempt fails (default-deny), while reaching the emulator
   endpoint it is permitted to use succeeds.

---

### User Story 3 - Isolation holds under concurrent load (stress proof) (Priority: P2)

A stress run drives **sustained concurrent traffic against both tenants at the same time** — many
parallel MCP sessions and tool calls per tenant exercising the AWS object-storage tools. The proof
asserts two things simultaneously: (a) the system **sustains a target throughput** for both tenants
without errors beyond the tenants' own quota limits and without degradation that breaks isolation,
and (b) **zero cross-tenant leakage** occurs under contention — no response served to one tenant ever
contains another tenant's data, no credentials bleed across sandboxes, and per-tenant quotas are
enforced independently (one tenant exhausting its quota does not affect the other).

**Why this priority**: Isolation that holds only at low volume is not proven isolation; races,
shared caches, and credential reuse most often surface under concurrency. This converts the US2 proof
from "true in a quiet test" to "true under realistic pressure." It depends on US1/US2 and so is P2.

**Independent Test**: Run a load generator against both tenants' endpoints concurrently at a defined
concurrency/duration; collect throughput, error rate, and latency per tenant; and run the US2 leakage
assertions continuously during the run. Pass requires meeting the throughput/error targets **and**
zero cross-tenant artifacts throughout.

**Acceptance Scenarios**:

1. **Given** both tenants under the smoke load (~10 concurrent sessions/tenant for ~1 minute),
   **When** the run completes, **Then** each tenant sustains the load with a **non-quota error rate
   < 1%** and **stable p95 tool-call latency ≤ 2 s** across the run (no progressive degradation).
2. **Given** one tenant deliberately driven past its per-org quota, **When** it begins receiving
   quota-limit responses, **Then** the other tenant's throughput and success rate are unaffected.
3. **Given** continuous leakage assertions during the load run, **When** the run completes, **Then**
   no response, log line, or trace attributed to one tenant contains the other tenant's bucket name,
   object contents, credentials, or server identifiers.

---

### User Story 4 - One-command, reproducible setup, proof, and teardown (Priority: P3)

The entire environment and proof are reproducible by a documented automation: a single, documented
entry point brings up the emulator, provisions both tenants, registers both AWS MCP servers, creates
each tenant's account/credentials/bucket, runs the US1–US3 proofs, reports a clear pass/fail, and a
teardown returns the environment to a clean state (no residual buckets, credentials, realms, or
server definitions). The proof is runnable **locally as the release isolation acceptance gate** in v1;
wiring it as a blocking CI gate is deferred to a later iteration.

**Why this priority**: Repeatability turns a one-off demo into a durable, repeatable local acceptance
gate (Constitution V / quickstart acceptance gate; CI wiring deferred). It is valuable polish on top
of the proofs themselves, hence P3.

**Independent Test**: On a clean checkout, run the documented one-command setup-and-prove flow and
confirm it completes green; run teardown and confirm no feature-created resources remain; re-run from
clean to confirm determinism.

**Acceptance Scenarios**:

1. **Given** a clean dev environment, **When** the operator runs the documented setup-and-prove
   command, **Then** the emulator starts, both tenants and their AWS resources are created, and the
   US1–US3 proofs run to a clear pass/fail summary without manual steps.
2. **Given** a completed proof run, **When** the operator runs teardown, **Then** all
   feature-created tenants, credentials, buckets, and server definitions are removed and re-running
   setup succeeds from clean.

### Edge Cases

- **Emulator capability gap**: If the local cloud emulator cannot represent two independently
  credentialed accounts (separate access identities) and per-account bucket access control, the
  downstream cross-account denial in US2 cannot be honestly proven. The proof MUST detect and fail
  loudly in this case rather than silently degrade to "buckets separated by name only." (See
  Assumptions; the plan MUST validate the emulator's account/credential isolation before relying on
  it.)
- **Shared downstream endpoint**: Both tenants' AWS MCP servers talk to the *same* emulator endpoint;
  the proof MUST show isolation comes from **distinct per-tenant credentials/account scoping**, not
  from network separation, so that pointing both at one endpoint does not itself create a leak.
- **Credential misconfiguration**: A tenant's AWS MCP server registered without credentials, or with
  the wrong tenant's credentials, MUST be caught by the proof (it would otherwise be a silent
  isolation failure) — a server cannot "accidentally" run with another tenant's account.
- **Sandbox egress**: Running under the **gVisor/microVM** sandbox, the stdio AWS MCP server must reach
  the emulator but MUST NOT be able to reach the control plane, other tenants' resources, or cloud
  metadata (Constitution II); the proof actively verifies this (SC-010) and MUST NOT open a hole that a
  real deployment would not have.
- **Headless authentication**: The Inspector-driven automation must obtain a valid per-tenant token
  without interactive login; token acquisition for one tenant MUST NOT yield access usable against
  the other.
- **Suspended/deleted tenant**: An attempt to use a tenant after it is suspended/deleted (per `003`)
  MUST fail closed; a deleted tenant's bucket/credentials MUST NOT remain reachable.
- **Quota exhaustion vs. errors**: The stress proof must distinguish *expected* per-tenant quota
  responses from *unexpected* errors so that quota enforcement is not miscounted as failure.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a self-contained local cloud (AWS-compatible) emulator within
  the dev stack so the proof requires **no real cloud account or external network egress**.
- **FR-002**: The proof MUST provision **two distinct tenants**, each an isolated organization with
  its own realm/issuer and its own `{org}.{base-domain}` endpoint, reusing the existing tenant
  bootstrap from `003`.
- **FR-003**: Each tenant MUST have an **AWS MCP server registered as a `stdio`-type server** in that
  tenant's catalog, launched under the **hardware-isolated (gVisor/microVM) sandbox runtime** — not the
  dev exec (unsandboxed) runtime — via the existing sandboxed stdio execution path.
- **FR-004**: Each tenant MUST be given its **own cloud account identity and its own credentials**,
  distinct from the other tenant's, such that one tenant's credentials grant access only to that
  tenant's resources. Isolation is realized by **distinct, access-scoped credentials per tenant**;
  true AWS multi-account/IAM modeling is explicitly out of scope.
- **FR-005**: Each tenant MUST have its **own object-storage (S3) bucket**, created as part of setup,
  named/owned so that it is reachable only with that tenant's credentials.
- **FR-006**: Each AWS MCP server's credentials MUST be supplied through the existing **write-only**
  credential mechanism and injected only at server launch; credential values MUST NEVER be returned,
  displayed, logged, or traced (secret confidentiality, Constitution VI).
- **FR-007**: The proof MUST drive the **live `/mcp` data plane through the MCP Inspector**
  (`npx @modelcontextprotocol/inspector`) for each tenant, exercising at least `tools/list` and an
  object put + read/list against that tenant's bucket.
- **FR-008**: The Inspector-driven automation MUST authenticate **per tenant against that tenant's
  realm** (OAuth 2.1, audience-bound to that tenant's MCP resource) without interactive login.
- **FR-009**: The proof MUST include an **adversarial cross-tenant suite** demonstrating that, for
  each ordered pair of tenants, the following are **denied and fail closed**: (a) presenting one
  tenant's token to the other's endpoint; (b) enumerating or invoking the other tenant's server; and
  (c) using one tenant's AWS tools/credentials to access the other tenant's bucket.
- **FR-010**: Every denied cross-tenant attempt MUST produce a **tamper-evident audit record**
  attributing the actor, target, and outcome; the proof MUST assert these records exist.
- **FR-011**: No cross-tenant attempt may disclose any of the other tenant's data, secrets, server
  definitions, or existence in any response, error, log, or trace.
- **FR-012**: The proof MUST include a **stress run** that drives **concurrent** load against **both
  tenants simultaneously** at the dev-scale **smoke profile — ~10 concurrent MCP sessions per tenant
  for ~1 minute** — recording per-tenant throughput, error rate, and latency.
- **FR-013**: During and after the stress run, the proof MUST assert **zero cross-tenant leakage**
  (no foreign bucket data, credentials, or identifiers in any tenant's responses/logs/traces) and
  that **per-tenant quotas are enforced independently** (one tenant's quota exhaustion does not affect
  the other).
- **FR-014**: The proof MUST emit a clear, machine-readable **pass/fail result** summarizing each
  user story's checks, consumed **locally as the release acceptance gate in v1** (wiring it as a
  blocking CI gate is deferred to a later iteration).
- **FR-015**: The system MUST provide a **documented, single-entry-point automation** to set up,
  run the proof, and **tear down** the environment, leaving no feature-created tenants, credentials,
  buckets, or server definitions behind.
- **FR-016**: The feature MUST NOT alter any isolation-, authorization-, sandbox-, or
  secret-enforcing behavior of the existing gateway/control-plane; it only adds the emulator, the two
  server registrations, per-tenant cloud setup, and the proof harness.
- **FR-017**: The stdio AWS MCP servers MUST run under the **gVisor/microVM** sandbox, and the proof
  MUST **actively verify** that a tenant's sandboxed server can reach the emulator but **cannot** reach
  the control plane, another tenant's resources, or cloud metadata (egress containment proven, not
  assumed — Constitution II).
- **FR-018**: The proof MUST **fail loudly** (not silently pass) if a precondition for honest
  isolation is absent — e.g., the emulator cannot enforce per-account credential scoping, or a
  tenant's server is misconfigured with absent/foreign credentials.

### Key Entities *(include if feature involves data)*

- **Tenant (organization/realm)**: One of two isolated orgs in the proof; carries its own issuer,
  endpoint, catalog, quotas, and audit scope. (Reused from `001`/`003`.)
- **AWS MCP server (stdio)**: A per-tenant registered server definition of type `stdio` whose launch
  command runs the AWS MCP server; carries its command/args/env and a write-only credential set.
- **Cloud account + credentials**: A per-tenant access identity (e.g., access key/secret) in the
  emulator that scopes access to that tenant's resources only; stored write-only.
- **Object-storage bucket**: A per-tenant S3 bucket that only that tenant's credentials can access;
  the concrete downstream resource whose cross-tenant access must be denied.
- **Cloud emulator**: The single local AWS-compatible service in the dev stack hosting both tenants'
  accounts and buckets; isolation derives from credential/account scoping, not network separation.
- **Proof harness / report**: The automation that sets up, exercises (via the Inspector), stresses,
  and tears down the environment, producing the pass/fail acceptance result.
- **Audit record**: A tamper-evident event proving a cross-tenant attempt was denied and attributed.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: From a clean dev environment, a single documented command brings up the environment and
  runs the full proof to a clear pass/fail summary in **under 15 minutes**, with **no manual steps**.
- **SC-002**: Both tenants independently complete the end-to-end happy path (list tools + write/read
  an object in their own bucket) with a **100% success rate** in the proof run.
- **SC-003**: **100%** of adversarial cross-tenant attempts (token-at-wrong-endpoint, foreign-server
  enumeration/invocation, foreign-bucket access) are **denied and fail closed**, and **100%** of them
  produce a corresponding audit record — i.e., **zero** cross-tenant successes.
- **SC-004**: Across the entire proof — including the stress run — **zero** responses, logs, or traces
  attributed to one tenant contain any of the other tenant's bucket data, bucket name, credentials, or
  server identifiers (zero-leakage).
- **SC-005**: Under the smoke load (~10 concurrent sessions per tenant for ~1 minute), each tenant's
  **non-quota error rate stays < 1%** and **p95 tool-call latency stays ≤ 2 s** for the full run
  duration (no progressive degradation).
- **SC-006**: When one tenant is driven past its quota during the stress run, the other tenant's
  success rate stays at its pre-contention level (**no measurable cross-tenant impact**).
- **SC-007**: At **no point** does any credential value appear in a response, log, or trace
  (verified by scanning proof output and service logs for known secret values: zero hits).
- **SC-008**: Teardown removes **100%** of feature-created resources (tenants, credentials, buckets,
  server definitions), verified by a post-teardown check, and a subsequent clean re-run succeeds.
- **SC-009**: The proof is deterministic when run **repeatedly on a local environment**: it yields the
  same pass/fail outcome for an unchanged system with no flakiness across at least 3 consecutive runs.
  (CI wiring is deferred; see FR-014.)
- **SC-010**: While running under the gVisor/microVM sandbox, **100%** of a tenant server's attempts to
  reach the control plane, another tenant's resources, or cloud metadata are blocked (default-deny
  egress), verified by explicit connection attempts in the proof.

## Assumptions

- **Builds on existing capabilities**: The OAuth-protected `/mcp` data plane, per-org Keycloak realms,
  sandboxed stdio execution, write-only per-org/per-user credentials, per-org quotas, and audit from
  `001`, plus the one-command tenant bootstrap from `003`, all exist and work; this feature composes
  them and adds the proof — it does not reimplement isolation.
- **"Each realm has its own account"** is realized as a **distinct cloud access identity (its own
  credentials) scoped to its own bucket** within the emulator. True AWS-Organizations multi-account
  is not required; the isolation being proven is (1) the gateway/tenant boundary (the primary HC-1
  guarantee) and (2) that a tenant's downstream credentials grant access only to that tenant's bucket.
  The plan MUST confirm the chosen emulator (`ministack`) supports independently credentialed,
  access-scoped accounts; if it cannot, the emulator choice or the account model must be revised so
  the cross-account denial in US2 remains an honest proof (see Edge Cases / FR-018).
- **AWS MCP server choice**: A standard, publicly available AWS MCP server invoked as a stdio process
  (e.g. the AWS Labs AWS MCP server already referenced as a sample in the admin console) scoped to
  object-storage operations, pointed at the local emulator endpoint via its configuration/environment.
  Exact server package and tool surface are an implementation detail to be fixed in the plan, provided
  it exposes object put/get/list against a configurable endpoint with injected credentials.
- **Local-only, no real cloud**: All AWS interactions target the in-stack emulator; the proof requires
  no real AWS account, no internet egress, and incurs no cloud cost.
- **Two tenants are sufficient** to prove the boundary; the proof is written so additional tenants
  could be added but v1 fixes the count at two (named e.g. `alpha`/`beta`).
- **Headless tokens** for the Inspector are obtained programmatically per tenant (e.g. via that
  realm's token grant, as the existing dev seed/provision flows already do), not via interactive login.
- **Stress targets are dev-scale smoke**: The stress run is a **smoke profile — ~10 concurrent MCP
  sessions per tenant for ~1 minute** (non-quota error rate < 1%, p95 ≤ 2 s) — chosen to surface
  contention and races on a developer machine, not to benchmark production capacity.
- **Sandbox runtime is required**: The proof runs the stdio AWS MCP servers under the **gVisor/microVM**
  sandbox (not the dev exec/unsandboxed runtime), so egress containment (FR-017/SC-010) is proven
  rather than assumed. A provisioned gVisor sandbox VM (e.g. Lima) is therefore a **prerequisite** for
  running the proof locally.
- **Audit availability**: The existing tamper-evident audit trail (and its dev archive) is available
  so FR-010/SC-003 audit-record assertions can be made.
