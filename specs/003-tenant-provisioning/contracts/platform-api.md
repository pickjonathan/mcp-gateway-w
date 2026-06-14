# Contract: Platform API (operator — tenant lifecycle)

Cross-tenant operations available **only to platform operators**. Served by the
control plane under `/v1/platform`. Auth: a bearer token from the **platform realm**
(`MCP_PLATFORM_REALM`) with audience `MCP_PLATFORM_AUDIENCE` and the **`platform-admin`**
realm role (middleware `requirePlatformAdmin`). A tenant/org token MUST be rejected
(`403`). Every mutation is audited (actor sub, action, tenant, outcome).

> Provisioning is **asynchronous**: mutating calls return `202` with a
> `ProvisioningJob`; poll the job for completion. Provisioning is idempotent and
> rollback-safe (FR-006).

## Endpoints

### `POST /v1/platform/tenants` — provision a tenant (US1, FR-001..008)
Request:
```json
{ "slug": "globex", "display_name": "Globex Corporation", "admin_email": "ops@globex.example" }
```
Responses:
- `202 Accepted` → `{ "tenant": { …Tenant }, "job": { …ProvisioningJob } }`
- `409 Conflict` — slug taken / reserved (no identity asset created — FR-008)
- `422 Unprocessable` — malformed slug or email
- `403` — not a platform admin

### `GET /v1/platform/tenants` — list all tenants
`200` → `{ "tenants": [ {slug, display_name, status, subdomain_ready, created_at} … ] }`
Supports `?status=` filter. (Reads with `app.current_org='*'`.)

### `GET /v1/platform/tenants/{slug}` — tenant detail
`200` → `{ …Tenant }` · `404` if unknown.

### `GET /v1/platform/tenants/{slug}/jobs/{jobId}` — provisioning status (FR-025)
`200` → `{ …ProvisioningJob }` with per-step state; `error` populated on failure (no secrets).

### `POST /v1/platform/tenants/{slug}:suspend` — suspend (US3, FR-019)
`202` → `{ job }`. Effect: realm disabled; new/refreshed tokens rejected at the gateway
within `< 1 min` (SC-006). Reversible. `409` if already suspended/deleting.

### `POST /v1/platform/tenants/{slug}:resume` — resume (FR-019)
`202` → `{ job }`. Restores access without data loss.

### `DELETE /v1/platform/tenants/{slug}` — delete (US3, FR-020..022)
`202` → `{ job }`. Effect: remove org server defs (existing events terminate instances +
revoke injected creds — FR-021), delete realm + clients, destroy stored credentials, free
the slug; **WORM audit retained until `audit_retention_until` (≥ 1y), then purged** (FR-020).
Idempotent; `404` if unknown.

## Errors
Standard problem shape `{ "error": "<code>", "message": "<human>" }`. Codes:
`forbidden`, `conflict`, `unprocessable`, `not_found`, `provisioning_failed` (job carries detail).

## Contract tests (Principle V)
- A platform-admin token provisions `globex`; tenant reaches `active`; job `succeeded`.
- A **tenant/org** token → `403` on every `/v1/platform/*` route (cross-tenant denial).
- Duplicate/reserved slug → `409` and **no** Keycloak realm created (verify via Admin API).
- Inject a mid-saga failure → tenant ends `failed`, job `compensated`, **no ghost realm**.
- Suspend → a freshly minted `globex` token is rejected at the gateway; `acme` unaffected.
- Delete → realm/clients/credentials gone, slug reusable, audit still retrievable.
