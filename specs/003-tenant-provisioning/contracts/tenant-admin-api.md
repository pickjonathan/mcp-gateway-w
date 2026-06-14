# Contract: Tenant-Admin API (org admin — users & federation)

Org-scoped operations for a tenant's **admin**. Served by the control plane under
`/v1/orgs/:org/…`, reusing the existing `requireAdmin(validator, AdminAudience)`
middleware (per-org realm token + `admin` role + path-org binding, HC-1) and the
`withOrg` RLS context. Extends the `002-admin-console`-consumed surface. All mutations
audited; **secret values are write-only** (returned once at creation, never echoed).

## Invitations (US2, FR-013..015)

### `POST /v1/orgs/{org}/invitations`
```json
{ "email": "dana@globex.example", "roles": ["member"] }
```
`201` → `{ "id", "email", "roles", "status": "pending", "expires_at" }` (the raw accept
token is emailed to the invitee, **never** in the response). `422` invalid email/role.

### `GET /v1/orgs/{org}/invitations` → `200` list (org-scoped, RLS).
### `DELETE /v1/orgs/{org}/invitations/{id}` → `204` revoke (FR-015).

### `POST /v1/invitations:accept` — **public** (no org token; the invite token authenticates)
```json
{ "token": "<raw-invite-token>", "password": "…" }
```
`200` → creates the user in **that org's realm only** with the assigned roles (FR-014);
`410 Gone` if expired/revoked/used.

## SSO brokering (US4, FR-016)

### `PUT /v1/orgs/{org}/identity-providers/{alias}`
```json
{ "type": "oidc", "config": { "issuer": "...", "clientId": "..." },
  "secret": "<idp-client-secret>", "role_mappings": { "groups.engineering": "aws-users" } }
```
`200` → `{ alias, type, enabled, role_mappings }` (the `secret` is stored in Vault, **never
returned**). Configures a Keycloak IdP + mappers with first-login JIT (FR-016).
### `GET …/identity-providers` → list (non-secret config only). `DELETE …/{alias}` → `204`.

## Directory sync — SCIM connection (US4, FR-017)

### `PUT /v1/orgs/{org}/directory-sync`
```json
{ "group_role_mappings": { "Engineering": "aws-users", "Admins": "admin" } }
```
`200` → `{ "status": "active", "scim_base_url": "https://{org}.{base}/scim/v2",
"bearer": "<shown-once>" }` — the **per-tenant SCIM bearer** is returned **once** then stored
write-only in Vault. The customer configures their IdP with that URL + bearer. See
[scim.md](./scim.md) for the SCIM endpoint itself.
### `GET …/directory-sync` → `{ status, scim_base_url, last_sync_at }` (no bearer).
### `POST …/directory-sync:rotate` → new bearer (shown once). `DELETE …/directory-sync` → `204`.

## Contract tests (Principle V)
- Invite + accept → user exists in `org`'s realm with the role; **absent** from any other realm.
- Expired/revoked invite → `410`.
- Brokering config never returns the IdP secret; SCIM config returns the bearer exactly once.
- An `org=acme` admin token cannot read/modify `org=globex` invitations/idp/scim (RLS + path binding).
