# Contract: SCIM 2.0 bridge (per-tenant directory sync)

A control-plane-hosted **SCIM 2.0 subset** (RFC 7643/7644) served per tenant at
`/{org}.{base}/scim/v2` (or `/scim/v2` with the org from Host). The customer's IdP
(Okta/Entra/Google) is the SCIM **client**; the bridge is the **server**, translating
operations to the Keycloak Admin API for that org's realm. Scope is intentionally the
**subset real IdPs emit** (research §6), not full RFC 7644.

## Auth
`Authorization: Bearer <per-tenant SCIM bearer>` (issued by `PUT /v1/orgs/{org}/directory-sync`,
stored write-only in Vault). The bearer is **bound to one org** → all operations act only on that
org's realm (HC-1). A bearer for org A presented on org B's SCIM URL ⇒ `401`/`403`.

## Resources & operations (the supported subset)

### Users — `/scim/v2/Users`
| Op | Maps to | Notes |
|---|---|---|
| `POST` | create Keycloak user (realm=org) | `userName`→username, emails, name; `active:true` |
| `GET /{id}` , `GET ?filter=userName eq "…"` | read | filtering limited to `userName eq` (what IdPs use) |
| `PUT /{id}` | replace user attrs | |
| `PATCH /{id}` | partial update | **`active:false` ⇒ disable user** ⇒ gateway access removed by next token (SC-005, FR-017) |
| `DELETE /{id}` | deactivate (soft) | treated as `active:false`, not hard delete |

### Groups — `/scim/v2/Groups`
| Op | Maps to | Notes |
|---|---|---|
| `POST` / `PATCH` members | group membership | group → realm role via the connection's `group_role_mappings` |
| `GET` | read | |

### Discovery
`/scim/v2/ServiceProviderConfig`, `/Schemas`, `/ResourceTypes` advertise the supported subset
(patch supported; filtering = `userName eq`; bulk = off) so conformant clients negotiate correctly.

## Semantics
- **Deactivation is the security-critical path**: `active:false` (PATCH or DELETE) disables the
  Keycloak user; the gateway then rejects that user on their **next token** (≤ 15 min, SC-005).
- **Idempotent**: re-applying a create/replace converges (match by `userName`).
- **Group→role**: only roles in the connection's mapping are assigned; unmapped groups are ignored.
- **Bounded responsiveness**: a sync operation reflects at the gateway within one token lifetime.

## Errors
SCIM error envelope (`{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"status":"…","detail":"…"}`):
`401` bad/missing bearer, `404` unknown id, `409` `userName` conflict, `400` unsupported filter/op.

## Contract tests (Principle V)
- `POST /Users` then a login for that user works in `org`'s realm only.
- `PATCH active:false` → the user's next gateway token is rejected (deactivation propagates).
- Group add mapped to `aws-users` → the user gains exactly those tools (RBAC parity with 001).
- A bearer for `acme` used against `globex`'s SCIM URL → `401`/`403` (cross-tenant denial).
