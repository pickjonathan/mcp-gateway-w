# Contract: Control-Plane Admin API

**Base**: `https://api.withwillow.ai/v1` (consumed by the existing admin panel).
**Auth**: Keycloak access token with an admin role for the target org; every route is org-scoped and RBAC-enforced. All writes emit an `AuditEvent` (FR-010).
**Conventions**: JSON; `404` for cross-org access attempts (never reveal another org's resources — HC-1); `409` on conflicts; `422` on validation errors.

## Server definitions (FR-007, FR-008)

| Method & path | Body / behavior |
|---|---|
| `POST /orgs/{org}/servers` | Create. `remote_http`: `{slug,type,endpoint_url,auth?,egress_allowlist?}`. `stdio`: `{slug,type,command,args[],env?,credential_mode,egress_allowlist?}`. Triggers async health-check; returns `201` with `health=unknown`. |
| `GET /orgs/{org}/servers` | List with current `health`/`enabled`. |
| `GET /orgs/{org}/servers/{id}` | Detail incl. `health_detail`. |
| `PATCH /orgs/{org}/servers/{id}` | Edit / `enabled` toggle; applies with no redeploy (FR-007); disabling stops serving within 5 s (SC-004). |
| `DELETE /orgs/{org}/servers/{id}` | Remove + reclaim instances + purge secrets. |
| `POST /orgs/{org}/servers/{id}/health-check` | Force re-check; returns status enum. |

**Validation**: exactly one of `endpoint_url`/`command` per `type`; `slug` unique in org & DNS-safe.

## RBAC (FR-009, FR-022)

| Method & path | Behavior |
|---|---|
| `PUT /orgs/{org}/servers/{id}/permissions` | Set bindings: `[{principal_type, principal_id, allowed_tools}]`. Principal must be same-org. |
| `GET /orgs/{org}/servers/{id}/permissions` | List bindings. |

Changes enforced on new **and** existing sessions promptly (FR-022).

## Secrets / credentials (FR-015, FR-016) — write-only

| Method & path | Behavior |
|---|---|
| `PUT /orgs/{org}/servers/{id}/credentials` | Store org-level secret(s) → Vault; response never echoes values. |
| `PUT /orgs/{org}/servers/{id}/credentials/me` | Caller stores their own per-user credential (when `credential_mode=per_user`). |
| `DELETE …/credentials[/me]` | Remove; rotation/version bump applies on next instance start. |

## Audit (FR-010, SOC 2)

| Method & path | Behavior |
|---|---|
| `GET /orgs/{org}/audit?from&to&actor&action` | Query recent events (org-scoped); paginated; archive (≥1y) queried for older windows. |

## Usage & quota (FR-017)

| Method & path | Behavior |
|---|---|
| `GET /orgs/{org}/usage` | Per-org/per-user tool-calls, cpu, egress. |
| `PUT /orgs/{org}/quotas` | Set per-org/per-user rate/quota limits. |
