# Phase 1 Data Model: Multi-Tenant MCP Server Runtime

**Date**: 2026-06-13 | **Plan**: [plan.md](./plan.md)

Persistent config/RBAC/audit entities live in PostgreSQL (every tenant-owned row carries `org_id`, enforced by app scoping + Row-Level Security). Ephemeral runtime state (sessions, routing, warm pools) lives in Redis / in-memory and is modeled here for completeness. Identities live in Keycloak (one realm per org) and are referenced by stable IDs.

## Entity overview & relationships

```text
Organization 1───* User                 Organization 1───* MCPServerDefinition
Organization 1───* Role                  MCPServerDefinition 1───* Credential
User *───* Role (via RoleBinding)        MCPServerDefinition 1───* PermissionBinding
Role/User *───* MCPServerDefinition (via PermissionBinding)
MCPServerDefinition 1───* ServerInstance (runtime)   User 1───* UserConnection (runtime)
ServerInstance 1───* Capability (Tool/Resource/Prompt, discovered)
Organization 1───* AuditEvent            Organization 1───* UsageRecord
```

---

## Organization (Tenant)
The primary, guaranteed isolation boundary (HC-1).

| Field | Type | Notes |
|---|---|---|
| `id` | UUID (PK) | |
| `slug` | string, unique | drives `{slug}.withwillow.ai`; DNS-safe; immutable after creation |
| `keycloak_realm` | string, unique | per-org realm name |
| `status` | enum | `active`, `suspended`, `offboarding` |
| `created_at` | timestamp | |

**Validation**: `slug` matches `^[a-z0-9]([a-z0-9-]{1,38}[a-z0-9])$`; unique across platform. **Offboarding** transition MUST cascade-purge servers, instances, secrets, sessions, and archive+close audit (FR-011, edge case "org offboarding").

## User
Member of one org; the best-effort secondary boundary (SG-1).

| Field | Type | Notes |
|---|---|---|
| `id` | UUID (PK) | maps to Keycloak subject (`sub`) within the org realm |
| `org_id` | UUID (FK) | |
| `external_idp` | string, nullable | set when federated from the org's IdP |
| `status` | enum | `active`, `disabled` |

**Validation**: `(org_id, keycloak_sub)` unique. Identity is platform-managed or federated via the central broker; always scoped to `org_id`.

## Role & RoleBinding
| Entity | Key fields | Notes |
|---|---|---|
| `Role` | `id`, `org_id`, `name` | org-defined roles; `(org_id, name)` unique |
| `RoleBinding` | `id`, `org_id`, `user_id`, `role_id` | assigns roles to users |

## PermissionBinding
Maps a principal (role or user) to a server it may use (FR-009).

| Field | Type | Notes |
|---|---|---|
| `id` | UUID (PK) | |
| `org_id` | UUID (FK) | |
| `server_id` | UUID (FK → MCPServerDefinition) | |
| `principal_type` | enum | `role`, `user` |
| `principal_id` | UUID | role_id or user_id |
| `allowed_tools` | string[] \| `*` | optional per-tool scoping (FR-009) |

**Validation**: principal and server MUST share `org_id` (cross-org binding rejected — HC-1). Changes MUST take effect on existing sessions promptly (FR-022).

## MCPServerDefinition
A registered server owned by an org.

| Field | Type | Notes |
|---|---|---|
| `id` | UUID (PK) | |
| `org_id` | UUID (FK) | |
| `slug` | string | namespacing prefix; `(org_id, slug)` unique (FR-003) |
| `type` | enum | `remote_http`, `stdio` |
| `endpoint_url` | string, nullable | required iff `type=remote_http` (Streamable HTTP) |
| `command` | string, nullable | required iff `type=stdio` (e.g. `npx`) |
| `args` | string[], nullable | e.g. `["-y","@modelcontextprotocol/server-sequential-thinking"]` |
| `env` | map<string,string>, nullable | non-secret env; secrets referenced via Credential |
| `credential_mode` | enum | `none`, `org_shared`, `per_user` (FR-016) |
| `egress_allowlist` | string[] | admin-approved external destinations (R7) |
| `enabled` | bool | |
| `health` | enum | `unknown`, `healthy`, `unreachable`, `auth_failed`, `startup_failed` (FR-008) |
| `health_detail` | string | last check message |

**Validation**: exactly one of (`endpoint_url`) / (`command`) set per `type`; `slug` DNS-safe and unique within org; disabling MUST stop offering it to users within 5 s (SC-004) and terminate in-flight use (FR-007/US7).

## Credential / Secret
Sensitive material for a server; never returned in API reads (write-only / reference).

| Field | Type | Notes |
|---|---|---|
| `id` | UUID (PK) | |
| `org_id` | UUID (FK) | |
| `server_id` | UUID (FK) | |
| `scope` | enum | `org` (shared) or `user` |
| `user_id` | UUID, nullable | set iff `scope=user` |
| `vault_path` | string | pointer into Vault; value never stored in Postgres |
| `version` | int | rotation applies on next instance start (FR-015) |

**Validation**: `scope` MUST match the server's `credential_mode`; value injected only at runtime, never logged or exposed to the gateway response path or other sandboxes (HC-3).

## ServerInstance / RuntimeSession *(runtime — Redis/in-memory)*
A running/connected instance.

| Field | Type | Notes |
|---|---|---|
| `id` | string | |
| `org_id` / `server_id` | refs | |
| `user_id` | nullable | set when granularity is per-`(org,user,server)` (R2) |
| `transport` | enum | `remote_http`, `sandbox_stdio` |
| `sandbox_id` | string, nullable | microVM handle for stdio |
| `state` | enum | `starting`, `healthy`, `idle`, `stopping`, `stopped`, `failed` |
| `last_used_at` | timestamp | drives idle reclamation (FR-018) |

**State transitions**: `starting → healthy → idle → stopped` (idle reclaim, FR-018); any → `failed` on crash/timeout (FR-019/FR-020), with transparent restart on next use. Instances are keyed so that **org_id is always part of the key** (HC-1) and never shared across orgs.

## UserConnection / Session *(runtime)*
An authorized client connection.

| Field | Type | Notes |
|---|---|---|
| `id` | string | |
| `org_id` / `user_id` / `roles` | from validated token | |
| `token_aud` | string | validated == org endpoint (HC-1) |
| `routing_table` | map<serverSlug, instanceRef> | aggregated permitted servers |
| `expires_at` | timestamp | token expiry; renew/re-auth on expiry (edge case) |

**Validation**: established only after token validation + audience check (FR-002/FR-023); revocation/permission change enforced on next request (FR-022).

## Capability (Tool / Resource / Prompt) *(runtime, discovered)*
| Field | Type | Notes |
|---|---|---|
| `namespaced_name` | string | `serverSlug__name` (FR-003) |
| `source_server_id` | ref | |
| `kind` | enum | `tool`, `resource`, `prompt` |
| `schema` | object | passed through from downstream |

## AuditEvent
Append-only; tamper-evident archive (FR-010, SOC 2).

| Field | Type | Notes |
|---|---|---|
| `id` | UUID | |
| `org_id` | UUID | |
| `actor` | string | admin/user/system |
| `action` | string | e.g. `server.create`, `server.disable`, `rbac.grant`, `auth.denied` |
| `target` | string | affected entity |
| `metadata` | jsonb | non-secret context |
| `created_at` | timestamp | |

**Retention**: ≥1 year; archived to object storage with Object Lock; never mutated/deleted before retention expiry.

## UsageRecord / Quota
| Field | Type | Notes |
|---|---|---|
| `org_id` / `user_id` | refs | |
| `window` | timestamp range | |
| `tool_calls` / `cpu_ms` / `egress_bytes` | counters | metering + rate limiting |
| `quota_limit` / `rate_limit` | config | per-org / per-user (FR-017) |

**Validation**: exceeding quota throttles/stops the offending principal without affecting other tenants (SC-009, FR-017).
