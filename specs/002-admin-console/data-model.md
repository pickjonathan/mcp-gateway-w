# Data Model — Admin Console (view models)

The console stores nothing persistently; these are **view models** derived from
the control-plane API (`001`). Field names mirror the API where known.

## AdminSession
Derived from the verified token — not fetched.
- `org` (string) — from the host `{org}.{base}` and token; scopes everything.
- `userId` (sub), `roles` (string[]), `displayName`, `expiresAt`.
- Invariant: if `roles` lacks `admin` → render Forbidden, fetch nothing.

## OrgContext
- `org`, `connectionEndpoint` = `https://{org}.{base}/mcp` (derived, for the
  "share with users" copy action, FR-018).

## ServerListItem  ← `GET /v1/orgs/{org}/servers`
- `id`, `slug`, `type` (`remote_http` | `stdio`), `enabled` (bool),
  `health` (`healthy` | `unhealthy` | `unknown`), `credentialMode`
  (`none` | `org_shared` | `per_user`), `allowedRoles` (string[]).
- Derived: `accessScope` = `allowedRoles.length ? "restricted" : "open"`.

## ServerDetail  ← `GET /v1/orgs/{org}/servers/{id}`
- All of ServerListItem plus: `endpointUrl` (remote), `command`/`args`/`env`
  (stdio), `healthDetail`, `createdAt`.

## AddServerInput / EditServerInput  → `POST` / `PATCH .../servers[/{id}]`
- Common: `slug`, `type`, `credentialMode`, `allowedRoles`, `enabled`.
- Remote: `endpointUrl` (required, must be a URL).
- Stdio: `command` (required), `args` (string[]), `env` (key→value map).
- Validation (client mirrors server): non-empty `slug` unique per org
  (server returns slug-taken → inline error, FR-010); remote requires
  `endpointUrl`; stdio requires `command`.

## CredentialStatus / CredentialInput  → `PUT`/`DELETE .../credentials[/me]`
- **Status only** (no value ever): `scope` (`org` | `me`), `isSet` (bool),
  `lastUpdated`. The list/get of a server conveys mode; "is set" is inferred from
  the most recent write or a status flag.
- `CredentialInput`: a key→value map written via `PUT`; cleared via `DELETE`.
- Invariant (FR-013): values are write-only — never stored, displayed, copied,
  exported, or logged by the console.

## AccessConfig (RBAC)  → part of `PATCH .../servers/{id}` (`allowed_roles`)
- `allowedRoles` (string[]); empty = open to all org members.

## AuditEventView  ← `GET /v1/orgs/{org}/audit`
- `time`, `actor`, `action` (e.g. `server.create`, `credentials.put`,
  `auth.denied`, `authz.denied`), `target`, `metadata` (non-secret), `seq`.
- `ChainStatus` (record integrity): `verified` | `tampered` — surfaced as a banner.

## RateLimits  (read-only — Clarifications 2026-06-13)
- `orgPerMin` (int), `userPerMin` (int). `0` = unlimited. **Display only** in v1
  (read via a read-only quotas endpoint); not editable.

## DashboardSummary (composed client-side)
- `serverCount`, `healthBreakdown` {healthy, unhealthy, unknown} ← servers list.
- `recentActivity` ← latest audit events.
- `denialIndicator` ← count of recent `auth.denied`/`authz.denied` from audit.
- `usageTrends` (request/error rates) ← **metrics source** (see Dependencies).

## State transitions

```
Server:   (add) → created/enabled ⇄ disabled        → deleted
                         │ (kill-switch terminates running instances on disable/delete)
Credential: not-set → set → (rotate) set' → (clear) not-set
```

## Entity → API map

| View model | Endpoint(s) |
|---|---|
| ServerListItem | `GET /v1/orgs/{org}/servers` |
| ServerDetail | `GET …/servers/{id}` |
| Add/Edit | `POST …/servers`, `PATCH …/servers/{id}` |
| Enable/Disable | `PATCH …/servers/{id}` (`enabled`) |
| Delete | `DELETE …/servers/{id}` |
| Credentials (org) | `PUT`/`DELETE …/servers/{id}/credentials` |
| Credentials (per-user) | `PUT`/`DELETE …/servers/{id}/credentials/me` |
| RBAC | `PATCH …/servers/{id}` (`allowed_roles`) |
| Audit + chain | `GET /v1/orgs/{org}/audit` |

## Dependencies (resolved in Clarifications 2026-06-13)

1. **Quotas (FR-017)** — `001` configures rate limits via **env**
   (`MCP_RATE_ORG_PER_MIN` / `MCP_RATE_USER_PER_MIN`); no quota API exists.
   **Resolved: read-only display** — add a small **read-only** quotas endpoint that
   returns the configured values; the console only displays them. Editing is out of
   scope for v1 (no per-org quota store in `001`). FR-023 updated to permit this
   read-only accommodation.
2. **Usage-rate widgets (FR-005/FR-019)** — request/denial/error **rates** live in
   Prometheus (`/metrics` is exposition). **Resolved: query Prometheus** — the
   console queries the metrics system's query API for rate charts (reachable via the
   edge same-origin proxy or CORS); health/counts/denials come from servers+audit.
3. **Credential "is set" flag** — confirm the servers API exposes set/not-set
   status; if absent, add a tiny non-secret status field (write-only preserved).
   (Implementation detail for `/speckit-tasks`.)
