# Connecting MCP clients & validating per-user tool access

How to point an MCP client (the official **MCP Inspector**, `mcp-remote`, Claude
Desktop, Cursor, …) at the gateway, and how to **prove that a user only sees the
tools their role allows**.

See also [Multi-tenant identity & MCP access](multi-tenant-keycloak.md) for the
realm/role model this builds on.

## Connection parameters

| Param | Value (dev) |
|---|---|
| MCP endpoint | `http://acme.mcp.example.com:8080/mcp` (prod: `https://acme.mcp.example.com/mcp`) |
| Transport | **Streamable HTTP** |
| Auth | OAuth 2.1 Bearer token; audience **must** be `https://acme.mcp.example.com/mcp` |
| Authorization server | `http://localhost:8081/realms/acme` (discovered automatically) |
| OAuth client | `mcp-client` (public, PKCE) |

The gateway resolves the org from the **Host** header, so the URL host must be
`acme.mcp.example.com`. For local use add a hosts entry:

```sh
echo "127.0.0.1 acme.mcp.example.com" | sudo tee -a /etc/hosts
```

A client discovers auth automatically: it hits `/mcp`, gets `401` +
`WWW-Authenticate: …resource_metadata="…/.well-known/oauth-protected-resource"`,
reads that document, and runs the OAuth flow against the `acme` realm.

### Getting a token by hand (dev)

For scripting/tests you can mint one directly (the `mcp-client` has dev
direct-grants enabled):

```sh
TOKEN=$(curl -s http://localhost:8081/realms/acme/protocol/openid-connect/token \
  -d grant_type=password -d client_id=mcp-client \
  -d username=alice -d password=alice -d scope=openid \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["access_token"])')
```

The token's `aud` includes `https://acme.mcp.example.com/mcp` and its
`realm_access.roles` carry the user's roles — both are what the gateway checks.

## Configure the MCP Inspector — manually

### CLI mode (scriptable)

```sh
npx @modelcontextprotocol/inspector --cli http://acme.mcp.example.com:8080/mcp \
  --transport http \
  --header "Authorization: Bearer $TOKEN" \
  --method tools/list
```

Call a tool:

```sh
npx @modelcontextprotocol/inspector --cli http://acme.mcp.example.com:8080/mcp \
  --transport http --header "Authorization: Bearer $TOKEN" \
  --method tools/call \
  --tool-name "awslabs.aws-api-mcp-server__suggest_aws_commands" \
  --tool-arg "query=list all S3 buckets"
```

Note the gateway **namespaces** each downstream server's tools as
`<server-slug>__<tool>`.

### UI mode

```sh
npx @modelcontextprotocol/inspector          # serves the UI on http://localhost:6274
```

Open the printed URL (it includes a one-time `MCP_PROXY_AUTH_TOKEN`), then set:

1. **Transport Type** → `Streamable HTTP`
2. **URL** → `http://acme.mcp.example.com:8080/mcp`
3. **Authentication** — either:
   - paste a token so the Inspector sends `Authorization: Bearer <token>`, **or**
   - use the built-in **OAuth** flow — the Inspector reads the gateway's
     protected-resource metadata, redirects you to the `acme` realm, you log in
     (e.g. `alice`/`alice`), and it captures the token via PKCE.
4. **Connect** → **List Tools**.

## Validate per-user tool access (RBAC)

Tool visibility is gated by **per-server `allowed_roles`** vs the user's realm
roles. Empty `allowed_roles` = open to all members; otherwise only holders of a
listed role see — or can call — that server's tools. The gateway enforces this
on **both** `tools/list` and `tools/call`.

### 1. Restrict the server to a role

- Create the realm role (Keycloak → realm `acme` → **Realm roles → Create role**, e.g. `aws-users`).
- In the admin console, open the server → **Access** tab → add `aws-users` → **Save**.
  (Equivalent API: `PATCH /v1/orgs/acme/servers/{id}` with `{"allowed_roles":["aws-users"]}`.)

### 2. A user WITHOUT the role sees nothing

`alice` has `default-roles-acme, offline_access, uma_authorization` — **not**
`aws-users`:

```
$ inspector --cli … --method tools/list
  alice (NO aws-users role) sees 0 tool(s): []

$ inspector --cli … --method tools/call --tool-name "awslabs.aws-api-mcp-server__suggest_aws_commands" …
  MCP error -32601: unknown tool: awslabs.aws-api-mcp-server__suggest_aws_commands
```

The gateway doesn't just refuse the call — it never lists the tool, so a user
can't discover servers they aren't entitled to.

### 3. Grant the role → access returns

Keycloak → realm `acme` → **Users → alice → Role mapping → Assign role →
`aws-users`** (or `kcadm add-roles -r acme --uusername alice --rolename aws-users`),
then the user gets a fresh token (re-login / re-mint):

```
$ inspector --cli … --method tools/list
  alice (WITH aws-users) now sees 2 tool(s):
    ['awslabs.aws-api-mcp-server__suggest_aws_commands',
     'awslabs.aws-api-mcp-server__call_aws']
```

Same user, same endpoint — the only change is the realm role. That is the
guardrail working end to end.

> Tokens are point-in-time: a role change only takes effect on the user's **next
> token** (re-login, or refresh). Revoking a role hides the tools on the next
> token as well.
