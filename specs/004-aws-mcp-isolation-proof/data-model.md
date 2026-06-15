# Phase 1 Data Model: Two-Tenant AWS-MCP Isolation Proof

This feature introduces **no new persistent schema**. The "model" here is the set of **proof-time
entities** the harness constructs/consumes (most are existing runtime records reused as-is) plus the
**config**, **cross-tenant attempt matrix**, and **report** shapes. Persistent records (tenants, servers,
credentials, audit, quotas) live in the existing stores and are untouched.

---

## Entities

### Tenant *(reused — `003`; `tenants` store)*
- `slug` (e.g. `alpha`, `beta`) — also the Keycloak realm name and `{slug}.{base-domain}` subdomain.
- `display_name`, `admin_email`, `status` (`active` required for the proof).
- Data-plane token audience: `MCPResource(slug)` (dev `http://{slug}.mcp.example.com:8080/mcp`).
- **Created by**: `POST /v1/platform/tenants` (platform-admin token). Exactly two for v1.

### AwsAccount *(new proof-time concept; not persisted as a new table)*
- `tenant_slug` → owns one account.
- `account_id` — **12-digit string**, distinct per tenant (`alpha`=`111111111111`, `beta`=`222222222222`);
  ministack derives the account from `AWS_ACCESS_KEY_ID`.
- `access_key_id` (= `account_id`), `secret_access_key` (random) — **secret; stored write-only** in Vault
  via the credential API, never returned.
- `region` (e.g. `us-east-1`) — non-secret.
- **Invariant**: an account's credentials grant access **only** to that account's namespace (D1, preflighted).

### Bucket *(reused — `ministack` S3)*
- `name` (e.g. `alpha-data`, `beta-data`), owned by exactly one `AwsAccount`.
- **Created** under the tenant's own account (by the tenant's own AWS MCP server, `call_aws s3 mb`, or by
  the harness using that tenant's creds).
- **Invariant (secondary boundary)**: not accessible with another account's credentials (NoSuchBucket).

### ServerRegistration *(reused — control-plane `mcp_servers`; one per tenant)*
- `slug`: `aws` · `type`: `stdio` · `credential_mode`: `org` (org-shared per tenant).
- `command`/`args`: the pre-baked `awslabs.aws-api-mcp-server` entrypoint (see `sandbox-images/Dockerfile`).
- `env` (non-secret): `AWS_ENDPOINT_URL=http://<ministack>:4566`, `AWS_REGION=us-east-1`.
- `allowed_roles`: `[]` or a role the seeded user holds (so the user can call it).
- **Secret env injected at launch** (not in `env`): `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` — set via
  `PUT /v1/orgs/{slug}/servers/{id}/credentials` (write-only).
- **Created by**: `POST /v1/orgs/{slug}/servers` (that org's admin token).

### SandboxEgressNetwork *(new config)*
- `MCP_SANDBOX_EGRESS_NETWORK` (string, default `""`). `""` → `--network none` (unchanged). Set to
  `mcp-sandbox-egress` for the proof → sandbox joins that Docker `internal` network (emulator-only).
- A Docker network `mcp-sandbox-egress` with `internal: true`, members: `ministack` (+ launched sandboxes).

### CrossTenantAttempt *(proof-time matrix)*
For each ordered pair `(src, dst)` ∈ {(alpha,beta),(beta,alpha)}, the vectors and expected outcomes:

| # | Vector | Action | Expected (fail-closed) | Audit action |
|---|--------|--------|------------------------|--------------|
| V1 | Token at wrong endpoint | `src` token → `dst`'s `/mcp` `tools/list` | 401/403; **no `dst` data**; issuer/audience mismatch | `auth.denied` |
| V2 | Foreign-server enumeration | `src` session lists tools; `dst`'s `aws` server must be absent | `dst` server **not present** for `src` | (none expected; absence) |
| V3 | Foreign-server invocation | `src` calls `dst`'s server by slug/id | method-not-found / forbidden (looks absent) | `authz.denied` |
| V4 | Foreign-bucket access | via `src`'s AWS server (account `src`), `s3 ls`/`get` on `dst`'s bucket | NoSuchBucket / denied (account namespace) | (downstream; harness asserts denial) |
| V5 | Sandbox egress containment | from inside `src`'s sandbox, reach control-plane / `169.254.169.254` / `dst` infra | **all fail**; only emulator reachable | n/a (network-level) |
| V6 | Secret disclosure | scan all responses/logs/traces from V1–V5 | **no credential/token value** appears | n/a |

### ProofRun / Report *(new — harness output, FR-014)*
- `started_at`, `finished_at`, `duration_s`, `environment` (runtime=`gvisor`, emulator endpoint, tenants).
- `preflights[]`: `{name, passed, detail}` (e.g. `s3_per_account_isolation`, `endpoint_override`).
- `checks[]`: `{id (FR/SC ref), story (US1..US4/SC-010), name, passed, detail, evidence}`.
- `stress`: per-tenant `{sessions, duration_s, calls, errors_nonquota, error_rate, p95_ms, quota_responses}`.
- `leakage`: `{scanned_artifacts, hits}` (must be `hits=0`).
- `overall`: `passed` (bool). Process **exit code** = 0 iff `overall.passed`.

---

## State / lifecycle (proof run)

```
preflight ─(fail)→ ABORT LOUD (non-zero exit, FR-018; no proof claims made)
   │ pass
setup: provision alpha,beta → register aws server each → set creds (write-only) → create buckets
   │
US1 functional (Inspector: tools/list + put/get/list per tenant, own bucket)
   │ all pass
US2 adversarial (V1..V6 for both ordered pairs) + audit-record assertions
   │ all denied + audited
US3 smoke load (~10/tenant ~1 min) + continuous leakage + quota independence (D8)
   │ within budget + zero leakage
SC-010 egress containment (V5)
   │ contained
report (JSON, overall pass/fail) → teardown (remove tenants, creds, buckets, servers; verify clean)
```

A failure at any stage marks the run failed (non-zero exit) but **always** runs teardown.

---

## Requirement → check coverage map

| Spec item | Where proven |
|---|---|
| FR-001 (emulator) / FR-002 (2 tenants) | setup; `compose.yaml` ministack; `POST /v1/platform/tenants` ×2 |
| FR-003 (stdio under gVisor) | ServerRegistration; `MCP_SANDBOX_RUNTIME=gvisor`; SC-010 |
| FR-004/005 (own account/creds/bucket) | AwsAccount, Bucket; D1 |
| FR-006 / SC-007 (write-only creds) | credential API; leakage scan (V6) |
| FR-007/008 (Inspector, headless per-realm auth) | US1/US2 Inspector calls; D6 |
| FR-009 / SC-003 (adversarial deny) | V1–V4 both pairs |
| FR-010 (audit) | D9 audit-record assertions |
| FR-011 / SC-004 (no disclosure / zero leakage) | V6 + continuous leakage scan |
| FR-012 / SC-005 (stress) | US3 smoke driver |
| FR-013 / SC-006 (quota independence) | D8 |
| FR-014 (machine-readable pass/fail) | Report + exit code |
| FR-015 / SC-001 (one command < 15 min) | `make prove-isolation` |
| FR-016 (no weakening) | only additive default-off egress config (plan Complexity Tracking) |
| FR-017 / SC-010 (egress containment) | V5; `egress_test.go` (Go, written-first) |
| FR-018 (fail loud) | D1 preflight gate |
| SC-008 (teardown clean) | teardown stage + post-check |
| SC-009 (local determinism) | 3 consecutive `make prove-isolation` runs |
