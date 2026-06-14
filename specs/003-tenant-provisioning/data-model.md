# Phase 1 Data Model: Automated Tenant Provisioning

All new tables live in the control-plane's Postgres. **Scope** is either
*platform* (the cross-tenant registry; reachable only with `app.current_org='*'`
via the platform API) or *org* (per-tenant; `org_id` + RLS, same policy shape as
the existing `servers` table). Secrets (SCIM bearers, brokered-IdP client secrets,
the privileged Keycloak credential) are **never** stored here — only **Vault
references** are.

## Entities

### Tenant — *platform-scoped* (the org registry)

| Field | Type | Notes |
|---|---|---|
| `slug` | text PK | = subdomain = Keycloak realm name. DNS-label + realm-name valid; not reserved; unique |
| `display_name` | text | company name |
| `status` | text | `provisioning` · `active` · `suspended` · `deleting` · `deleted` · `failed` |
| `admin_email` | text | initial org admin |
| `realm_name` | text | = `slug` (explicit for clarity) |
| `subdomain_ready` | bool | derived: realm reachable + slug routable |
| `created_at` / `updated_at` | timestamptz | |
| `suspended_at` / `deleted_at` | timestamptz null | lifecycle stamps |
| `audit_retention_until` | timestamptz null | set on delete = `deleted_at` + `MCP_AUDIT_RETENTION_DAYS` (≥ 365) |

**Validation**: `slug` matches `^[a-z][a-z0-9-]{1,38}[a-z0-9]$`, ∉ reserved
(`www`,`api`,`admin`,`auth`,`app`,`_platform`, apex), unique. `admin_email` valid email.

### ProvisioningJob — *platform-scoped* (saga record)

| Field | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `tenant_slug` | text FK→tenants.slug | |
| `action` | text | `provision` · `suspend` · `resume` · `delete` |
| `status` | text | `pending` · `running` · `succeeded` · `failed` · `compensated` |
| `steps` | jsonb | ordered `[{name, state: pending/done/failed/compensated, detail}]` for idempotent resume + compensation |
| `error` | text null | actionable failure reason (no secrets) |
| `created_at` / `updated_at` | timestamptz | |

### Invitation — *org-scoped* (RLS by `org_id`)

| Field | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `org_id` | text | = tenant slug; RLS key |
| `email` | text | invitee |
| `roles` | text[] | realm roles to assign on accept (must exist in realm) |
| `token_hash` | text | hash of the single-use accept token (raw token emailed once, never stored) |
| `status` | text | `pending` · `accepted` · `revoked` · `expired` |
| `expires_at` | timestamptz | validity window |
| `created_by` | text | inviting admin (sub) |
| `created_at` / `accepted_at` | timestamptz | |

### IdentityProviderLink — *org-scoped* (brokering config)

| Field | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `org_id` | text | RLS key |
| `alias` | text | Keycloak IdP alias (unique per org) |
| `type` | text | `oidc` · `saml` |
| `config` | jsonb | **non-secret** IdP settings (issuer, endpoints, client_id) |
| `secret_ref` | text null | Vault ref for the IdP client secret (never the value) |
| `role_mappings` | jsonb | assertion/group → realm role |
| `enabled` | bool | |
| `created_at` / `updated_at` | timestamptz | |

### DirectorySyncConnection (SCIM) — *org-scoped*

| Field | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `org_id` | text | RLS key |
| `status` | text | `active` · `disabled` |
| `bearer_ref` | text | Vault ref for the per-tenant SCIM bearer (value shown once at creation, write-only) |
| `group_role_mappings` | jsonb | SCIM group → realm role |
| `last_sync_at` | timestamptz null | observability |
| `created_at` | timestamptz | |

> **Subdomain allocation** is folded into `Tenant` (`slug` + `subdomain_ready`) — no
> separate table (Principle VII; wildcard DNS makes it a record, not a resource).

## Relationships

```text
Tenant (slug) 1───* ProvisioningJob          (platform)
Tenant (slug = org_id) 1───* Invitation        (org-scoped)
                       1───* IdentityProviderLink
                       1───0..1 DirectorySyncConnection
Tenant 1───1 Keycloak Realm (external; name = slug)
Realm  1───2 OAuth Clients (mcp-admin-console, mcp-client)  + roles + users   (external)
```

## State machines

**Tenant.status**
```text
(none) ──provision──▶ provisioning ──success──▶ active
                          │                        │ suspend ⇄ resume
                          │ failure                ▼
                          ▼                     suspended
                        failed ──retry──▶ provisioning
active|suspended ──delete──▶ deleting ──success──▶ deleted
                                  │ failure
                                  ▼
                                failed
```
- `provisioning`/`deleting` are driven by a `ProvisioningJob`; failure leaves a
  `failed` tenant + a `failed`/`compensated` job (never a half-tenant — FR-006).
- `suspended` ⇄ `active` is reversible (FR-019). `deleted` is terminal; `slug` freed
  for reuse after the audit-retention window note is recorded.

**ProvisioningJob.status**: `pending → running → {succeeded | failed}`; a `failed`
`provision`/`delete` may transition to `compensated` after reverse-compensation.

**Invitation.status**: `pending → {accepted | revoked | expired}` (terminal).

## Postgres schema sketch (RLS)

```sql
-- Platform-scoped registry. No org RLS; reachable only via platform API
-- (which sets app.current_org='*'); a tenant token can never set that.
CREATE TABLE tenants (
  slug                   text PRIMARY KEY,
  display_name           text NOT NULL,
  status                 text NOT NULL,
  admin_email            text NOT NULL,
  realm_name             text NOT NULL,
  subdomain_ready        boolean NOT NULL DEFAULT false,
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  suspended_at           timestamptz,
  deleted_at             timestamptz,
  audit_retention_until  timestamptz
);
CREATE TABLE provisioning_jobs (
  id           uuid PRIMARY KEY,
  tenant_slug  text NOT NULL REFERENCES tenants(slug),
  action       text NOT NULL,
  status       text NOT NULL,
  steps        jsonb NOT NULL DEFAULT '[]',
  error        text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);

-- Org-scoped tables: identical RLS shape to `servers`.
CREATE TABLE invitations (
  id          uuid PRIMARY KEY,
  org_id      text NOT NULL,
  email       text NOT NULL,
  roles       text[] NOT NULL DEFAULT '{}',
  token_hash  text NOT NULL,
  status      text NOT NULL,
  expires_at  timestamptz NOT NULL,
  created_by  text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  accepted_at timestamptz
);
-- (idp_links, scim_connections: same org_id + RLS pattern)

ALTER TABLE invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE invitations FORCE ROW LEVEL SECURITY;
CREATE POLICY invitations_org_isolation ON invitations
  USING (org_id = current_setting('app.current_org', true)
         OR current_setting('app.current_org', true) = '*')
  WITH CHECK (org_id = current_setting('app.current_org', true));
```

## Config additions (`pkg/config`)

| Env | Purpose | Default |
|---|---|---|
| `MCP_KEYCLOAK_ADMIN_URL` | Admin API base | dev: `http://localhost:8081` |
| `MCP_KEYCLOAK_ADMIN_CLIENT_ID` | privileged service-account client | — |
| `MCP_KEYCLOAK_ADMIN_SECRET_REF` | Vault ref to its secret (never the value) | — |
| `MCP_PLATFORM_REALM` | realm issuing operator tokens | `_platform` |
| `MCP_PLATFORM_AUDIENCE` | audience for platform-admin tokens | — |
| `MCP_TENANT_RESERVED_SLUGS` | CSV reserved labels | `www,api,admin,auth,app` |
| `MCP_AUDIT_RETENTION_DAYS` | audit retention on delete (≥ 365) | `365` |
| `MCP_TENANT_CEILING` | warn threshold for realm count | `200` |

All loaded once into the `config.Config` singleton (existing convention).
