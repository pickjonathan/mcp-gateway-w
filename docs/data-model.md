# Data Model

Core entities and where they live. The authoritative spec is
`specs/001-mcp-server-runtime/data-model.md`; this is the implemented summary.

## Entities

### Organization
A tenant. Not a heavy row — an org is realized by its **Keycloak realm**, its
slice of the catalog, and the `org_id` on every owned record. Org id is derived
from the request host (`{org}.{base-domain}`).

### Principal (user)
Derived from a verified JWT, never stored by the gateway:
`{ OrgID, UserID (sub), Roles (realm_access.roles) }`.

### MCP Server (`mcp_servers`)
An admin-registered downstream. The source-of-truth table:

| Column | Notes |
|---|---|
| `id` | `srv_…` primary key |
| `org_id` | tenant; part of `UNIQUE(org_id, slug)`; RLS key |
| `slug` | per-org name; used in tool namespacing `slug__tool` |
| `type` | `remote_http` \| `stdio` |
| `endpoint_url` | for `remote_http` |
| `command`, `args`, `env` | for `stdio` (`args`/`env` stored as JSON text) |
| `credential_mode` | `none` \| `org_shared` \| `per_user` |
| `allowed_roles` | RBAC scope (JSON text); empty = all org members |
| `enabled` | gateway reconcile reads `enabled = true` |
| `health`, `health_detail` | last probe result |
| `created_at` | ordering |

**RLS**: `ENABLE` + `FORCE ROW LEVEL SECURITY`, policy on `org_id =
current_setting('app.current_org')` (with a `'*'` read-only service context for
the gateway's full-catalog reconcile).

### Credential
Stored in **Vault KV v2**, never in Postgres. Reference paths:

- org-shared: `org/{org}/server/{serverID}`
- per-user: `org/{org}/server/{serverID}/user/{userID}`

Write-only via the admin API; the gateway reads them at downstream build time.

### Server change event (`serverevents.Event`)
Published on Redis to propagate catalog changes:
`{ Action (upsert|remove|credential_changed), OrgID, ID, Slug, Type, EndpointURL,
Command, Args, Env, AllowedRoles, CredentialMode, UserID }`.
Carries only **non-secret** config; the gateway resolves secrets itself.

### Audit record (`audit.Record`)
A sealed event in the hash chain:
`{ Event{Time, OrgID, Actor, Action, Target, Metadata}, Seq, PrevHash, Hash }`.
`Hash = sha256(Seq, PrevHash, fields…)`; durable copy is one immutable,
Object-Lock'd object per `Seq` in S3.

## Relationships

```
Organization 1───* MCP Server 1───* Credential (org-shared: 1; per-user: per user)
Organization 1───* Audit Record
MCP Server   1───* Server change events (lifecycle)
Principal     *──* MCP Server   (via allowed_roles / RBAC; per-user instances)
```

## Lifecycle: a server, end to end

```
admin POST ─▶ control plane ─▶ Postgres (insert, RLS)         [source of truth]
                     │
                     └─▶ Redis upsert event ─▶ gateway builds downstream  [live]
gateway restart ─▶ reconcile: SELECT enabled ─▶ rebuild catalog          [backstop]
admin sets creds ─▶ Vault write + credential_changed event ─▶ next instance uses new secret
admin deletes ─▶ Postgres delete + remove event ─▶ gateway kill-switch closes instance
```
