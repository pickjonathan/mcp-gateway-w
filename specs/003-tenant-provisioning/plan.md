# Implementation Plan: Automated Tenant Provisioning

**Branch**: `003-tenant-provisioning` | **Date**: 2026-06-14 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-tenant-provisioning/spec.md`

## Summary

Add a **control-plane capability to provision, populate, and deprovision isolated
tenants** on the MCP gateway. A tenant **is** a Keycloak realm; bootstrapping one
(Half A) is a Keycloak **Admin API** operation, and populating it with users (Half
B) is done via **admin invitations**, **OIDC/SAML brokering**, and **SCIM directory
sync** — all three in v1. Provisioning is **platform-operator-initiated** (self-service
signup deferred), the isolation model stays **realm-per-tenant** (tens–low-hundreds
scale), and tenant deletion **purges identity/credentials/servers but retains the
tamper-evident audit ≥ 1 year** (Principle VI) before purging.

Technical approach: a new `tenants` package + a `idp` (Keycloak Admin) package in the
**control-plane**, a privileged Keycloak **service-account credential** held only by
the control plane (Vault), a **platform realm + `platform-admin` role** authorizing the
cross-tenant operator API (distinct from per-org admin tokens), a new **platform API**
(`/v1/platform/tenants…`) and **tenant-admin API** extensions (`/v1/orgs/:org/invitations`,
`…/identity-providers`, `…/directory-sync`), and a **control-plane-hosted SCIM bridge**
that translates SCIM 2.0 → Admin API. The **gateway is unchanged**: provisioning only
creates identity assets + a control-plane record; suspend = realm-disable (tokens fail
validation); delete reuses existing server-removed events for kill-switch parity.

## Technical Context

**Language/Version**: Go 1.25 (module `github.com/acme-corp/mcp-runtime`)
**Primary Dependencies**: Echo v4 (HTTP); a Keycloak **Admin REST client** (`Nerzal/gocloak`, new) behind an internal interface; `pkg/authz` (JWT/JWKS), `pkg/secrets` (Vault), `pkg/serverevents` (Redis bus), `pkg/audit` (hash-chain + S3 WORM), `pkg/telemetry`, `pkg/logging`, `pgx` (Postgres). SCIM bridge: stdlib + Echo (no new SCIM lib for v1 subset).
**Storage**: PostgreSQL (new tables, RLS) · HashiCorp Vault (privileged Keycloak credential; per-tenant SCIM bearer + brokering client secrets) · Redis (lifecycle events) · S3/MinIO (audit WORM, retention ≥ 1y)
**Testing**: Go `testing`, table-driven, hermetic by default; live integration behind `MCP_TEST_*` (adds `MCP_TEST_KEYCLOAK_URL`/`_ADMIN_*`); **adversarial** cross-tenant isolation + credential-containment tests (Principle V) gated and run against the dev stack
**Target Platform**: Linux server — extends the existing control-plane service (`:8090`); integrates with the dev `deploy/dev` Keycloak/Postgres/Vault/Redis/MinIO stack
**Project Type**: Backend web-service (control-plane extension) + an identity-platform integration. No new frontend in this feature (operator/admin UI is a later `002-admin-console` extension).
**Performance Goals**: provision a tenant end-to-end **< 5 min** (SC-001); invite→active **< 3 min** (SC-004); suspend propagation **< 1 min** (SC-006); SCIM/brokered deactivation removes gateway access **within one token lifetime (≤ 15 min)** (SC-005)
**Constraints**: tenant isolation is inviolable (HC-1); secrets write-only and never logged; idempotent + rollback-safe provisioning (no half-tenants); audit retained **≥ 1 year** even through deletion (Principle VI)
**Scale/Scope**: **tens to low hundreds** of tenants (realms) for v1; warn before a configured realm-count ceiling (SC-009)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Verdict | Notes |
|---|---|---|
| **I. Tenant Isolation Inviolable** | **PASS** | The feature's purpose is stronger isolation (a realm per tenant). New org-scoped tables (`invitations`, `idp_links`, `scim_connections`) carry `org_id` + RLS like `servers`. The `tenants` registry is **platform-scoped**, reachable only via the platform API (`app.current_org='*'`), never by a tenant token. The cross-tenant operator path is **explicit and gated** by a `platform-admin` role in a dedicated platform realm. Suspend **fails closed** (realm disabled → tokens rejected). Adversarial cross-tenant tests required (Principle V). |
| **II. Secure Execution of Any MCP** | **PASS (N/A)** | No change to sandbox execution. New invariant: the **privileged Keycloak credential** and per-tenant SCIM bearers live only in the control plane + Vault and MUST remain unreachable from any sandbox (existing network/egress rules already forbid sandbox→control-plane). |
| **III. Frictionless Self-Service** | **PASS w/ justified deviation** | In-tenant admin self-service (servers, invites) and MCP client onboarding (PKCE) are unchanged. **Tenant creation is operator-gated in v1** — a deliberate, temporary deviation (see Complexity Tracking): minting whole isolated companies is privileged by necessity; **self-service signup (US5) is the planned path** to restore frictionless tenant creation. |
| **IV. Hard Constraints Outrank Soft Goals** | **PASS** | No hard constraint weakened for any soft goal. Where suspend trades immediacy for simplicity (existing short-lived tokens expire naturally rather than forced revocation), that is a documented, bounded choice (≤ 15 min), not an isolation weakening. |
| **V. Test-First, Adversarially Verified** | **PASS (commitment)** | Adversarial tests written-first: (a) a token/admin/user of tenant A cannot read/affect tenant B; (b) the privileged Keycloak credential is never exposed via any API/log/sandbox; (c) a suspended/deleted tenant's tokens are rejected. Contract tests for the platform API, tenant-admin API, and SCIM bridge. |
| **VI. Observable & Auditable by Default** | **PASS** | Every provision/suspend/resume/delete and every user-provisioning action is audited (FR-012) via `pkg/audit` (tamper-evident). Deletion **retains the WORM audit ≥ 1 year** (sets the retention window), then purges. Provisioning emits metrics/traces + observable status (FR-025). |
| **VII. Simplicity with Justified Complexity** | **PASS w/ tracking** | New deps (Keycloak Admin client, platform realm/authz, **control-plane SCIM bridge**) are justified by the multi-tenant product requirement and the "all three user-sync mechanisms in v1" decision. The SCIM bridge is **scoped to the subset** real directories use (Users CRUD + Groups + `active=false`), not full RFC 7644 — see Complexity Tracking. |

**Gate result: PASS** (two justified items recorded in Complexity Tracking). No unjustified violations.

## Project Structure

### Documentation (this feature)

```text
specs/003-tenant-provisioning/
├── plan.md              # This file
├── research.md          # Phase 0 — decisions & rationale
├── data-model.md        # Phase 1 — entities, schema, RLS, state machines
├── quickstart.md        # Phase 1 — provision + isolation walkthrough (release gate)
├── contracts/           # Phase 1 — interface contracts
│   ├── platform-api.md          # operator: tenant CRUD + lifecycle
│   ├── tenant-admin-api.md       # org admin: invitations, brokering, SCIM config
│   ├── scim.md                   # SCIM 2.0 subset the bridge serves per tenant
│   └── keycloak-provisioning.md  # Admin-API objects created per tenant (= seed script, programmatic)
└── tasks.md             # Phase 2 — created by /speckit-tasks (NOT here)
```

### Source Code (repository root)

```text
services/control-plane/
├── cmd/control-plane/main.go          # wire new stores, idp client, platform+tenant-admin routes, SCIM bridge
└── internal/
    ├── admin/                          # EXISTING org-scoped admin API (servers, creds, audit, quotas)
    │   └── api.go                      # + register tenant-admin route groups (invitations, idp, scim)
    ├── tenants/                        # NEW — platform-scoped tenant registry + lifecycle
    │   ├── store.go / postgres.go      # tenants + provisioning_jobs tables, status state machine, RLS('*')
    │   ├── handlers.go                 # platform API handlers (POST/GET/DELETE, suspend/resume)
    │   ├── service.go                  # provision/suspend/resume/delete orchestration (saga + compensation)
    │   └── authz.go                    # requirePlatformAdmin (platform realm + platform-admin role)
    ├── idp/                            # NEW — Keycloak Admin API integration
    │   ├── keycloak.go                 # gocloak wrapper behind an interface (realm/client/role/user/IdP)
    │   ├── bootstrap.go                # create realm + 2 clients + mappers + admin role + admin user (idempotent)
    │   ├── brokering.go                # configure OIDC/SAML identity provider + role mappers
    │   └── scim_apply.go               # apply SCIM user/group changes via Admin API
    ├── invites/                        # NEW — invitation lifecycle (org-scoped)
    │   └── store.go / handlers.go
    └── scimbridge/                     # NEW — control-plane-hosted SCIM 2.0 endpoint (subset)
        └── server.go / users.go / groups.go

pkg/
├── config/config.go                    # + MCP_KEYCLOAK_ADMIN_URL, _CLIENT_ID/_SECRET(ref), MCP_PLATFORM_REALM,
│                                        #   MCP_TENANT_RESERVED_SLUGS, MCP_AUDIT_RETENTION_DAYS (default 365), ceiling
└── secrets/                            # reused: privileged Keycloak cred + per-tenant SCIM bearer + IdP secrets

deploy/dev/
├── compose.yaml / seed-keycloak.sh     # seed the PLATFORM realm + the control-plane's privileged service account
└── postgres-init/                       # (RLS role already present; new tables get policies in migration)

tests/ (co-located *_test.go per Go convention)
└── adversarial: cross-tenant isolation, credential containment, suspend/delete kill-switch
```

**Structure Decision**: Extend the **existing control-plane service** (no new service — Principle VII). Two new platform-scoped packages (`tenants`, `idp`) plus org-scoped `invites`, and a `scimbridge` sub-server mounted on the control-plane. The gateway and sandbox-supervisor are untouched. This keeps the data-plane contract stable and concentrates the privileged Keycloak credential in one service.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| **Operator-gated tenant creation** (deviates from Principle III "frictionless self-service" for the *create-tenant* action) | Creating an isolated company provisions a powerful realm + a privileged credential path; unguarded self-service is an abuse/security risk. v1 ships operator-only; **US5 self-service signup** (email/domain verification + rate limits) restores self-service later. | Pure self-service in v1 was rejected because it exposes a privileged provisioning operation to anonymous actors before guardrails (verification, throttling, slug abuse) are built — a hard-constraint (isolation/abuse) risk. |
| **Control-plane SCIM bridge** (new sub-server, added complexity vs. Principle VII) | "All three user-sync mechanisms in v1" makes SCIM a v1 dependency; Keycloak core has no SCIM server. A bridge we own is testable (Principle V), avoids a commercial/maturity dependency, and reuses the Admin API we already call. **Scoped to the subset** real directories use (Users CRUD, Groups, `active=false`). | A Keycloak SCIM *extension* was considered (less code) but adds a deployment + maturity/licensing dependency and is hard to test hermetically; kept as a documented fallback. Skipping SCIM was rejected — the clarified decision requires it in v1. |

*Post-Phase-1 re-check: see the foot of [research.md](./research.md) — design holds the gate; no new violations introduced.*
