# Tenant Provisioning (control plane)

How the control plane **onboards and offboards a company as a fully isolated
tenant** (`003-tenant-provisioning`). A **tenant is a Keycloak realm**: creating
one is a Keycloak **Admin API** operation; populating it with users is done via
**invitations**, **OIDC/SAML brokering**, and **SCIM** directory sync. The
**gateway is unchanged** — it still derives the org from the request Host +
token issuer.

> SCIM has no "create realm" operation. Realm creation (Half A) is Admin-API
> driven; SCIM is one of the user-provisioning mechanisms (Half B), used *after*
> the realm exists. See the spec/plan under `specs/003-tenant-provisioning/`.

## Architecture

- **Platform realm + `platform-admin` role** authorize the cross-tenant operator
  API — distinct from any tenant's `admin` token, so a tenant token can never
  reach the platform API (HC-1).
- A **privileged Keycloak service account** (`mcp-provisioner`, a master client)
  is held only by the control plane (secret in Vault; dev: `MCP_KEYCLOAK_ADMIN_SECRET`).
  Operators never hold it — their token only authorizes the endpoint.
- Provisioning is an **idempotent saga with compensation**: realm → console +
  MCP clients (+ audience/role mappers) → `admin` role → admin user. Any failure
  rolls back (deletes the realm), leaving no "ghost" realm.
- New control-plane packages: `tenants` (registry + lifecycle, platform-scoped),
  `idp` (Keycloak Admin client), `invites`, `brokering`, `scimbridge` (org-scoped,
  RLS like `mcp_servers`).

## Operator (platform) API — `/v1/platform/tenants`

Auth: a bearer from the **platform realm** with the **`platform-admin`** role.

| Method | Path | Action |
|---|---|---|
| `POST` | `/v1/platform/tenants` | provision `{slug, display_name, admin_email}` → `202 {tenant, job}` |
| `GET` | `/v1/platform/tenants` | list all tenants |
| `GET` | `/v1/platform/tenants/{slug}` | tenant detail |
| `GET` | `/v1/platform/tenants/{slug}/jobs/{id}` | provisioning job status |
| `POST` | `/v1/platform/tenants/{slug}/suspend` | disable the realm (tokens stop validating) |
| `POST` | `/v1/platform/tenants/{slug}/resume` | re-enable |
| `DELETE` | `/v1/platform/tenants/{slug}` | purge realm/clients/creds/servers; **retain WORM audit ≥ 1 year** then purge |

## Tenant-admin API (org-scoped, `admin` role)

- **Invitations** — `POST/GET/DELETE /v1/orgs/{org}/invitations`, public
  `POST /v1/invitations:accept` (the invite token authenticates; the user is
  created in that realm only).
- **Brokering** — `PUT/GET/DELETE /v1/orgs/{org}/identity-providers/{alias}`
  (OIDC/SAML; the IdP secret goes to Vault, never echoed).
- **Directory sync (SCIM)** — `PUT/GET/POST :rotate/DELETE
  /v1/orgs/{org}/directory-sync` issues a per-tenant SCIM bearer (shown once).
  The SCIM 2.0 endpoint is at `/scim/v2/*` (bearer-authenticated, org resolved
  from the bearer). `active:false` disables the user → gateway access removed by
  their next token.

## Run it locally

```sh
make dev-up                                  # infra + base seed
make seed-platform                           # _platform realm + operator + provisioner (prints the secret)

# Start the control-plane with provisioning enabled (paste the printed secret):
MCP_KEYCLOAK_ADMIN_CLIENT_ID=mcp-provisioner MCP_KEYCLOAK_ADMIN_SECRET=<printed> \
MCP_KEYCLOAK_ADMIN_URL=http://localhost:8081 \
MCP_PLATFORM_REALM=_platform MCP_PLATFORM_AUDIENCE=https://platform.mcp.example.com \
  make run-control-plane

# Provision a tenant (operator operator/operator on the _platform realm):
make provision-tenant SLUG=globex NAME='Globex' ADMIN_EMAIL=ops@globex.example
# add a hosts entry so the new tenant routes to the gateway:
echo "127.0.0.1 globex.mcp.example.com" | sudo tee -a /etc/hosts
```

Verify isolation: an `acme` token is rejected (`401`) on the platform API, and a
`globex` token is rejected by the gateway for `acme` (and vice-versa).

## Scope (v1)

Operator-initiated provisioning; realm-per-tenant at tens–low-hundreds scale; all
three user-provisioning mechanisms (invites + brokering + SCIM). **Self-service
signup is deferred.** See `specs/003-tenant-provisioning/` for the full spec, plan,
contracts, and quickstart (the isolation walkthrough is the release gate).
