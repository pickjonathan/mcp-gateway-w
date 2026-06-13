# Multi-tenant identity & MCP access (Keycloak)

How organizations (tenants), users, roles, and MCP access map onto Keycloak —
and the exact steps to onboard a company and invite users with guardrails.

## Model: one realm per organization

Each customer org is a Keycloak **realm** (`acme`, `globex`, …). This is the
tenant boundary — users, clients, and roles are realm-scoped, so two companies
are fully isolated.

```
 user's MCP client ──▶ https://{org}.mcp.example.com/mcp        (gateway, data plane)
 admin's browser   ──▶ https://{org}.mcp.example.com/admin      (console)  ──▶ control-plane API
                              │
                              ▼ token issued by …/realms/{org}
                       Keycloak realm  "{org}"
                         ├─ client  mcp-admin-console   (admin SPA login;  aud = https://api.mcp.example.com)
                         ├─ client  mcp-client          (MCP clients;      aud = https://{org}.mcp.example.com/mcp)
                         ├─ role    admin               (manage via console)
                         ├─ role    …custom…            (gate specific servers)
                         └─ users   alice, bob, …       (realm members; roles travel in realm_access.roles)
```

The gateway resolves the org from the request **Host** (`{org}.{base-domain}`) and
**rejects any token whose issuer isn't `…/realms/{org}`** — so a `globex` token
can never reach `acme` data. "Two applications for two companies" = **two realms**,
each with its own `mcp-client`.

> In Keycloak terms an "application" is a *client*. The thing that separates
> companies here is the **realm**, not the client.

## What is actually enforced (your guardrails are real)

| Guardrail | Where you set it | Enforced by |
|---|---|---|
| Tenant isolation | the realm | gateway: token issuer must equal `…/realms/{org}`; catalog scoped to the org |
| Audience binding | `mcp-client` audience mapper | gateway: token `aud` must equal `https://{org}.{base}/mcp` |
| Who can administer | `admin` realm role | control-plane: the admin API requires `admin` |
| Who can use a server's tools | per-server `allowed_roles` (console → server → **Access**) vs the user's realm roles | gateway: enforced on **tools/list AND tools/call** |
| `mcp:tools` / `mcp:resources` / `mcp:prompts` | client scopes (optional) | **Advertised only** in discovery — *not* gated by the gateway today (roles are). See note. |

**Empty `allowed_roles` = open** → every member of the org can use that server.
Set one or more roles to restrict it.

## 1) Onboard a new organization (company)

**Dev — one command.** The seed is parameterized by `REALM` / `BASE_DOMAIN`:

```sh
REALM=globex bash deploy/dev/seed-keycloak.sh
# creates realm globex + clients mcp-admin-console & mcp-client
# + the admin role + an admin/admin user. Idempotent.
```

The gateway then routes `globex.mcp.example.com/mcp` → realm `globex`
automatically (no gateway change). Add `127.0.0.1 globex.mcp.example.com` to
`/etc/hosts` for local DNS, like `acme`.

**Production** — same steps via `kcadm`, Terraform, or `keycloak-config-cli`:

1. **Create the realm** (and pick sane token lifetimes; do **not** use `sslRequired=NONE` — that is dev-only):
   ```sh
   kcadm create realms -s realm=globex -s enabled=true
   ```
2. **Create the two clients** (public, Auth-Code + PKCE):
   - `mcp-admin-console` — redirect to the console origin, audience mapper → the admin-API audience, realm-roles in the ID token.
   - `mcp-client` — loopback redirects, audience mapper → `https://globex.<base>/mcp`.
   (The exact `kcadm create clients … protocol-mappers …` calls are in
   [`deploy/dev/seed-keycloak.sh`](../deploy/dev/seed-keycloak.sh) — copy them, swap the realm/base.)
3. **Create the `admin` role and the first admin user** with it (below). That user is the org's owner and invites everyone else.

## 2) Create the MCP scopes (client scopes)

The gateway advertises `mcp:tools`, `mcp:resources`, `mcp:prompts` in its
protected-resource metadata. Create them as realm **client scopes** so tokens
can carry them and clients can request least privilege:

```sh
# inside the keycloak container, after: kcadm config credentials … --realm master
for s in mcp:tools mcp:resources mcp:prompts; do
  kcadm create client-scopes -r globex \
    -s name="$s" -s protocol=openid-connect \
    -s 'attributes={"include.in.token.scope":"true","consent.screen.text":"Access MCP","display.on.consent.screen":"true"}'
done
# attach to mcp-client (default = always; optional = client must request it)
MCID=$(kcadm get clients -r globex -q clientId=mcp-client --fields id --format csv --noquotes)
SID=$(kcadm get client-scopes -r globex -q name=mcp:tools --fields id --format csv --noquotes)
kcadm update "clients/$MCID/default-client-scopes/$SID" -r globex
```

> **Note — scopes are advisory today.** The gateway *lists* these scopes for
> discovery but authorizes by **role** (per-server `allowed_roles`), not by
> scope. Creating them is forward-looking. If you want true scope-gated access
> (e.g. a token without `mcp:tools` can't call tools), that's a small gateway
> change — ask and it can be added.

## 3) Roles & guardrails

- **`admin`** — manage servers, credentials, RBAC, and audit in the console.
- **member** (no special role) — can use **open** servers via `/mcp`.
- **custom role** (e.g. `aws-users`) — gate a server to a subset:
  ```sh
  kcadm create roles -r globex -s name=aws-users
  ```
  then in the console open the server → **Access** tab → add `aws-users` → Save.
  Now only users with `aws-users` see or can call that server's tools.

## 4) Invite / create users

Users belong to the org's realm. Pick whichever fits:

- **Console (dev):** http://localhost:8081 → switch to realm `{org}` → **Users → Add user** → **Credentials** (set password) → **Role mapping** (assign `admin` and/or custom roles).
- **kcadm (scriptable):**
  ```sh
  kcadm create users -r globex -s username=alice -s enabled=true -s email=alice@globex.test -s emailVerified=true
  kcadm set-password -r globex --username alice --new-password 'change-me'
  kcadm add-roles -r globex --uusername alice --rolename aws-users   # guardrail
  ```
- **Self-service registration:** realm **Settings → Login → User registration = On**. People sign themselves up; an admin then assigns roles.
- **Email invitations:** configure realm **SMTP** (Realm settings → Email), then invite from the Users screen (or via Keycloak **Organizations** invitations on 26+). Requires a working mail server.

**Production invite flow (recommended):** the app exposes an "Invite user"
action that calls Keycloak's Admin REST API using a **service account** whose
realm-management roles are **scoped to that one realm** — so an org admin can
create/invite users in their realm but never touch another tenant. Send either
an invitation email or an "Update password" required-action link.

## 5) End-to-end (what a user does)

1. Org admin onboards the org (realm) and invites a user with role(s).
2. User points an MCP client (Inspector, Claude Desktop, Cursor, `mcp-remote`) at `https://{org}.{base}/mcp`.
3. The client gets `401` + `WWW-Authenticate` → reads `/.well-known/oauth-protected-resource` → discovers the realm → logs in via **PKCE** with `mcp-client` → receives a token (`aud = https://{org}.{base}/mcp`, roles in `realm_access.roles`).
4. The gateway validates issuer + audience, scopes the catalog to the org, and filters tools by the user's roles vs each server's `allowed_roles`.

## Quick reference (the `acme` dev tenant)

| Thing | Value |
|---|---|
| Realm | `acme` |
| Console client | `mcp-admin-console` (aud `https://api.mcp.example.com`) |
| MCP client | `mcp-client` (aud `https://acme.mcp.example.com/mcp`) |
| MCP endpoint | `https://acme.mcp.example.com/mcp` (dev: `http://acme.mcp.example.com:8080/mcp`) |
| First admin | `admin` / `admin` (realm role `admin`) |
| Onboard another org | `REALM=<org> bash deploy/dev/seed-keycloak.sh` |
