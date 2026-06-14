# Contract: Keycloak Admin-API provisioning (per-tenant objects)

The exact Keycloak objects the control plane creates/updates per tenant via the Admin
API — the **programmatic, idempotent equivalent of `deploy/dev/seed-keycloak.sh`**. Each
step is check-then-create (idempotent) and recorded as a saga step with a compensation
(delete) for rollback (FR-006).

## Privileged credential the control plane uses
A **service-account client** (client-credentials) in the master/platform realm with the
minimum roles: `create-realm`, and on managed realms `manage-realm`, `manage-clients`,
`manage-users`, `manage-identity-providers`. Secret in Vault (`MCP_KEYCLOAK_ADMIN_SECRET_REF`),
never logged, rotatable. Distinct from operator user tokens (research §4).

## Provisioning steps (per tenant `slug`)

| # | Object | Settings (mirrors seed script) | Compensation |
|---|---|---|---|
| 1 | **Realm** `slug` | `enabled=true`; access-token TTL 15m; SSO idle 8h / max 24h; (`sslRequired=NONE` dev only) | delete realm |
| 2 | **Role** `admin` | realm role → reaches token via `realm_access.roles` | delete role |
| 3 | **Client** `mcp-admin-console` | public, PKCE S256, standard flow; redirect/web-origins = console origin for `{slug}.{base}` | delete client |
| 4 | mappers on (3) | `admin-api-audience` (audience = `MCP_ADMIN_AUDIENCE`); `realm-roles-id` (`realm_access.roles`) | n/a (cascade) |
| 5 | **Client** `mcp-client` | public, PKCE S256, standard + direct grants (dev); redirect `http://localhost:*`,`127.0.0.1:*` | delete client |
| 6 | mapper on (5) | audience = **`{slug}.{base}/mcp`** (the gateway's MCP resource — matches `MCP_RESOURCE_TEMPLATE`) | n/a |
| 7 | **Initial admin user** | username/email = `admin_email`; assigned `admin` role; required action: set password (invite email) | delete user |
| 8 | *(US4, if configured)* **IdentityProvider** + mappers | per `PUT …/identity-providers` (research §7) | delete IdP |
| 9 | *(US4, if configured)* **SCIM enablement** | register the per-tenant SCIM bridge connection (bearer in Vault) | disable connection |

After steps 1–7 the tenant is **usable** (US1 acceptance): the admin can sign in to the
`{slug}` console and a `{slug}` token is accepted by the gateway. Steps 8–9 are applied when
the tenant configures federation.

## Audience/issuer alignment (must hold — HC-1)
The gateway derives org from Host (`{slug}.{base}`), requires issuer `…/realms/{slug}` and
audience `{slug}.{base}/mcp`. Steps 1, 6 establish exactly these — the same alignment chain the
Inspector relies on today, now created programmatically rather than by the seed script.

## Suspend / resume / delete
- **Suspend**: `PUT realm {enabled:false}` → login/refresh fail; gateway rejects new tokens (research §10).
- **Resume**: `PUT realm {enabled:true}`.
- **Delete**: remove org server defs (events terminate instances), `DELETE` clients + realm; schedule
  audit purge at `audit_retention_until`.

## Idempotency & rollback tests (Principle V)
- Re-run provisioning for an existing `slug` → no duplicate realm/clients/roles/users (SC-008).
- Fail step 5 → steps 1–4 compensated (realm deleted); tenant `failed`; no ghost realm.
- Verify the privileged credential is never returned by any API nor written to logs/traces.
