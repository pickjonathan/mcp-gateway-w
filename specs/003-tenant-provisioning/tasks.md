---
description: "Task list for Automated Tenant Provisioning (003)"
---

# Tasks: Automated Tenant Provisioning

**Input**: Design documents from `/specs/003-tenant-provisioning/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: **REQUIRED (not optional).** Constitution Principle V is NON-NEGOTIABLE and the
constitution Sync Impact Report states `/speckit-tasks` MUST emit contract + adversarial tests
for any isolation/authz/secrets work — which is this entire feature. Adversarial/negative tests
MUST be written and MUST FAIL before the implementation lands.

> **Implementation progress (2026-06-14)** — `go build/vet/test ./...` green; **US1–US4 implemented +
> tested**, and **validated live** against the dev Keycloak: provision `smokeco` → `active` (realm + 2
> clients + admin role + admin user created); suspend → `enabled=false`, resume → `enabled=true`;
> delete → realm `404` with `audit_retention_until` = +1 year; an `acme` token → `401` on the platform API.
> **Done & tested:** Foundation · **US1** provision saga + compensation + platform API · **US3**
> suspend/resume/delete (+ audit ≥1y) · **US2** invitations + accept · **US4 brokering** + **US4 SCIM
> bridge** (per-tenant bearer, Users create/replace, `PATCH active:false`→disable, group→role,
> discovery) · **T003** seed `PLATFORM=1`. Adversarial: cross-tenant 403/401, no-ghost-realm
> compensation, no-secret-leak, cross-org + SCIM-bearer isolation, retention.
> **Refinement:** Keycloak Admin via in-house `net/http` client behind idp interfaces (not gocloak);
> a 403-retry refreshes the admin token after realm creation (picks up the new realm's mgmt roles).
> T001 superseded; dev-only `MCP_KEYCLOAK_ADMIN_SECRET` direct path (prod uses the Vault ref).
> **Remaining for v1:** T041 kill-switch server-events on delete · live-gated `MCP_TEST_KEYCLOAK_*`
> test *files* (T018/T029/T037/T047 — behaviour validated manually) · T025 metrics · polish (T057–T062). US5 deferred.

**Organization**: by user story (priority order from the clarified spec): US1 (P1) → US2/US3/US4
(P2) → US5 (P3, **deferred from v1**).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no incomplete dependencies)
- **[Story]**: US1–US5; Setup/Foundational/Polish carry no story label
- Exact file paths included.

## Path Conventions

Go monorepo (`github.com/acme-corp/mcp-runtime`). New code lives under
`services/control-plane/internal/{tenants,idp,invites,scimbridge}/` and `pkg/`. Tests are
**co-located** `*_test.go` (table-driven, hermetic by default); live integration tests are gated
behind `MCP_TEST_*` (new: `MCP_TEST_KEYCLOAK_URL`, `MCP_TEST_KEYCLOAK_ADMIN_*`) + `t.Skip`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: dependencies, config, dev-stack seeding, package scaffolding.

- [~] T001 (superseded — in-house net/http client; no gocloak) Add the Keycloak Admin client dependency: `go get github.com/Nerzal/gocloak/v13` and `go mod tidy` (updates `go.mod`/`go.sum`)
- [x] T002 [P] Add tenant-provisioning config fields to `pkg/config/config.go` (`MCP_KEYCLOAK_ADMIN_URL`, `MCP_KEYCLOAK_ADMIN_CLIENT_ID`, `MCP_KEYCLOAK_ADMIN_SECRET_REF`, `MCP_PLATFORM_REALM`, `MCP_PLATFORM_AUDIENCE`, `MCP_TENANT_RESERVED_SLUGS`, `MCP_AUDIT_RETENTION_DAYS` default 365, `MCP_TENANT_CEILING` default 200) loaded via `config.Get()`
- [x] T003 [P] Extend `deploy/dev/seed-keycloak.sh` with a `PLATFORM=1` path that seeds the `_platform` realm, the `platform-admin` role, an `mcp-platform` operator client, and the control-plane privileged **service-account** client (write its secret to Vault) — DEV ONLY, idempotent
- [x] T004 [P] Scaffold new packages with package docs: `services/control-plane/internal/tenants/doc.go`, `idp/doc.go`, `invites/doc.go`, `scimbridge/doc.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: schema, the Keycloak admin client, platform authz, and base wiring that ALL stories need.

**⚠️ CRITICAL**: No user-story work begins until this phase is complete.

- [x] T005 DB migration for platform-scoped tables `tenants` + `provisioning_jobs` in `services/control-plane/internal/tenants/postgres.go` (follow the migration pattern in `services/control-plane/internal/admin/postgres.go`)
- [~] T006 [P] DB migration for org-scoped tables `invitations`, `idp_links`, `scim_connections` with **RLS** (`org_id = current_setting('app.current_org') OR = '*'`, `FORCE ROW LEVEL SECURITY`) in `services/control-plane/internal/{invites,idp,scimbridge}/postgres.go`
- [x] T007 [P] Define the `idp.Keycloak` interface (realm/client/mapper/role/user + IdP + enable/disable realm) in `services/control-plane/internal/idp/keycloak.go`
- [x] T008 (done as in-house net/http RESTClient in `idp/rest.go`) Implement the gocloak-backed `idp.Keycloak` — admin token via client-credentials, secret read from Vault (`pkg/secrets`), never logged — in `services/control-plane/internal/idp/keycloak_gocloak.go` (depends on T001, T002, T007)
- [x] T009 [P] Add `ValidateForRealm(ctx, token, realm, audience)` to `pkg/authz/jwt.go` (fixed-realm validation for non-org-scoped callers; reuse JWKS path)
- [x] T010 [P] Implement `requirePlatformAdmin` middleware (validate platform-realm token + `platform-admin` role; sets principal) in `services/control-plane/internal/tenants/authz.go` (depends on T009)
- [x] T011 [P] Add audit action constants + helper for provisioning/lifecycle/user events (reuse `pkg/audit`) in `services/control-plane/internal/tenants/audit.go`
- [x] T012 [P] Test support: two-tenant fixtures + token-mint helpers (per-realm + platform) in `services/control-plane/internal/tenants/testsupport_test.go` (powers adversarial isolation tests)
- [~] T013 (read routes mounted via api.Mount; mutating handlers land with US1/US3) Base wiring in `services/control-plane/cmd/control-plane/main.go`: construct the idp client + stores, register the (initially empty) `/v1/platform`, tenant-admin, and SCIM-bridge route groups (depends on T005–T010)

**Checkpoint**: foundation ready — user stories can proceed (in parallel if staffed).

---

## Phase 3: User Story 1 - Operator provisions an isolated tenant (Priority: P1) 🎯 MVP

**Goal**: a platform operator provisions a fully isolated tenant (realm + clients + mappers + admin role + admin user + subdomain) that is immediately usable.

**Independent Test**: `POST /v1/platform/tenants {slug:globex}` → tenant `active`; globex admin signs in; a globex token is accepted by the gateway; an acme token is rejected for globex.

### Tests for User Story 1 (write FIRST, must FAIL) ⚠️

- [x] T014 [P] [US1] Contract test for the platform API (POST/GET list/GET detail/GET jobs) with a mocked `idp.Keycloak` in `services/control-plane/internal/tenants/handlers_test.go` (per `contracts/platform-api.md`)
- [x] T015 [P] [US1] **Adversarial**: an org/tenant token → `403` on every `/v1/platform/*` route in `services/control-plane/internal/tenants/authz_test.go`
- [x] T016 [P] [US1] **Adversarial**: duplicate + reserved + malformed slug → `409`/`422` and **no** Keycloak object created (assert against mock) in `services/control-plane/internal/tenants/service_test.go`
- [x] T017 [P] [US1] **Adversarial**: inject a mid-saga failure (e.g. client step) → compensation runs, tenant ends `failed`, **no ghost realm**, re-run is idempotent (SC-003/008) in `services/control-plane/internal/tenants/service_compensation_test.go`
- [~] T018 [P] [US1] Integration (gated `MCP_TEST_KEYCLOAK_*`): provision a throwaway realm end-to-end and assert audience/issuer alignment per `contracts/keycloak-provisioning.md` in `services/control-plane/internal/idp/bootstrap_integration_test.go`

### Implementation for User Story 1

- [x] T019 [P] [US1] `Tenant` + `ProvisioningJob` models + status state machine in `services/control-plane/internal/tenants/model.go` (per data-model.md)
- [x] T020 [US1] Tenant store (CRUD + job tracking, platform-scoped `app.current_org='*'`) in `services/control-plane/internal/tenants/store.go` + `postgres.go` (depends on T005, T019)
- [x] T021 [P] [US1] Slug validation (format regex + reserved list + uniqueness) in `services/control-plane/internal/tenants/slug.go`
- [x] T022 [US1] Idempotent realm bootstrap (steps 1–7: realm, 2 clients, mappers, admin role, admin user) + per-step compensations in `services/control-plane/internal/idp/bootstrap.go` (depends on T008)
- [x] T023 [US1] Provision orchestration (saga: validate → bootstrap → persist; compensation on failure; resumable) in `services/control-plane/internal/tenants/service.go` (depends on T020, T021, T022)
- [x] T024 [US1] Platform API handlers (POST provision, GET list, GET detail, GET job) in `services/control-plane/internal/tenants/handlers.go`, mounted under `requirePlatformAdmin` (depends on T010, T023)
- [~] T025 [US1] Audit + metrics + observable provisioning status (FR-012/FR-025) wired into the service in `services/control-plane/internal/tenants/service.go`
- [x] T026 [US1] Realm-count ceiling warning (`MCP_TENANT_CEILING`, SC-009) in `services/control-plane/internal/tenants/service.go`

**Checkpoint**: US1 fully functional — an operator can stand up an isolated tenant (MVP). Run the quickstart §1–§2 (provision + isolation gate).

---

## Phase 4: User Story 2 - Org admin invites users with role guardrails (Priority: P2)

**Goal**: a tenant admin invites users by email with roles; invitee accepts and gets an account in that realm only.

**Independent Test**: invite `dana@globex` as `member`; accept; Dana signs in to globex with `member`, absent from any other realm.

### Tests for User Story 2 (write FIRST, must FAIL) ⚠️

- [x] T027 [P] [US2] Contract test for invitations (POST/GET/DELETE + public accept) in `services/control-plane/internal/invites/handlers_test.go` (per `contracts/tenant-admin-api.md`)
- [x] T028 [P] [US2] **Adversarial**: an `acme` admin token cannot read/modify `globex` invitations (RLS + path binding); expired/revoked token → `410` in `services/control-plane/internal/invites/isolation_test.go`
- [~] T029 [P] [US2] Integration (gated): accept creates the user in the correct realm only with the assigned roles in `services/control-plane/internal/invites/accept_integration_test.go`

### Implementation for User Story 2

- [x] T030 [P] [US2] `Invitation` model + store (org-scoped, RLS; token hash, status machine) in `services/control-plane/internal/invites/store.go` + `postgres.go` (depends on T006)
- [x] T031 [P] [US2] Single-use accept-token generation + hashing (raw token emailed once, never stored) in `services/control-plane/internal/invites/token.go`
- [x] T032 [US2] Invitation delivery: dev stub (log/audit the link) behind an interface for a real emailer later, in `services/control-plane/internal/invites/notify.go`
- [x] T033 [US2] `idp.CreateUserWithRoles(realm, email, roles)` in `services/control-plane/internal/idp/users.go` (depends on T008)
- [x] T034 [US2] Invitation handlers (POST/GET/DELETE under `requireAdmin`; public `POST /v1/invitations:accept`) in `services/control-plane/internal/invites/handlers.go` + register in `services/control-plane/internal/admin/api.go` (depends on T030–T033)
- [x] T035 [US2] Audit invite/accept/revoke events in `services/control-plane/internal/invites/handlers.go`

**Checkpoint**: US1 + US2 work independently — operators provision; admins populate via invites.

---

## Phase 5: User Story 3 - Suspend and deprovision a tenant (Priority: P2)

**Goal**: operator suspends/resumes (reversible) or deletes (terminal) a tenant; other tenants unaffected; audit retained ≥1y on delete.

**Independent Test**: suspend globex → new globex tokens rejected at the gateway, acme unaffected; resume restores; delete a throwaway tenant → realm/clients/creds gone, slug reusable, audit retained.

### Tests for User Story 3 (write FIRST, must FAIL) ⚠️

- [x] T036 [P] [US3] Contract test for suspend/resume/delete in `services/control-plane/internal/tenants/lifecycle_test.go` (per `contracts/platform-api.md`)
- [~] T037 [P] [US3] **Adversarial** (gated): after suspend, a freshly minted globex token is rejected at the gateway while acme is unaffected (SC-006); after delete, the realm/clients are gone and the slug is reusable in `services/control-plane/internal/tenants/lifecycle_integration_test.go`
- [x] T038 [P] [US3] **Adversarial**: delete sets `audit_retention_until ≥ now+365d` and audit remains retrievable (Principle VI) in `services/control-plane/internal/tenants/retention_test.go`

### Implementation for User Story 3

- [x] T039 [P] [US3] `idp.SetRealmEnabled(realm,bool)` + `idp.DeleteRealm(realm)` in `services/control-plane/internal/idp/lifecycle.go` (depends on T008)
- [x] T040 [US3] Suspend/resume service (realm disable/enable + status transitions + audit) in `services/control-plane/internal/tenants/service.go` (depends on T039)
- [~] T041 [US3] Delete saga: remove org server defs (emit existing `pkg/serverevents` removals → kill-switch parity, FR-021) → delete clients + realm → set `audit_retention_until` → schedule deferred audit purge, in `services/control-plane/internal/tenants/delete.go` (depends on T039, T020)
- [x] T042 [US3] Lifecycle handlers (`:suspend`, `:resume`, `DELETE`) in `services/control-plane/internal/tenants/handlers.go` (depends on T040, T041)

**Checkpoint**: full tenant lifecycle (create → suspend/resume → delete) with isolation + retention guarantees.

---

## Phase 6: User Story 4 - Enterprise SSO and directory sync (brokering + SCIM) (Priority: P2)

**Goal**: a tenant federates with its corporate IdP (OIDC/SAML brokering + JIT) and keeps membership in sync via SCIM (incl. deactivation removing access).

**Independent Test**: configure brokering to a test IdP → corporate user JIT-provisioned with mapped roles; push a SCIM create + `active:false` → user appears then loses gateway access by next token — all scoped to the tenant.

### Tests for User Story 4 (write FIRST, must FAIL) ⚠️

- [x] T043 [US4] **Spike** (research §6 risk): confirm the SCIM operation subset one real IdP (Okta/Entra) emits; record findings in `specs/003-tenant-provisioning/research.md` (gates T049/T050 scope)
- [x] T044 [P] [US4] Contract test for brokering config (`PUT/GET/DELETE identity-providers`) — secret never returned — in `services/control-plane/internal/idp/brokering_test.go`
- [x] T045 [P] [US4] Contract test for `directory-sync` config (bearer shown once) + SCIM `Users`/`Groups`/`ServiceProviderConfig` in `services/control-plane/internal/scimbridge/server_test.go` (per `contracts/scim.md`)
- [x] T046 [P] [US4] **Adversarial**: a SCIM bearer for `acme` used on `globex`'s SCIM URL → `401`/`403`; brokered IdP secret never echoed in `services/control-plane/internal/scimbridge/isolation_test.go`
- [~] T047 [P] [US4] Integration (gated): SCIM `active:false` ⇒ user's next gateway token rejected (SC-005); group→role mapping yields the expected tools (RBAC parity) in `services/control-plane/internal/scimbridge/sync_integration_test.go`

### Implementation for User Story 4

- [x] T048 [P] [US4] `IdentityProviderLink` store + brokering config via Admin API (IdP + mappers, JIT) with the IdP secret in Vault, in `services/control-plane/internal/idp/brokering.go` + `store.go` (depends on T006, T008)
- [x] T049 [P] [US4] `DirectorySyncConnection` store + per-tenant SCIM bearer issuance/rotation (Vault, write-once) in `services/control-plane/internal/scimbridge/store.go` (depends on T006)
- [x] T050 [US4] SCIM 2.0 bridge server — `/Users` (POST/GET/PUT/PATCH incl. `active:false`), `/Groups`, discovery endpoints; Host→org + bearer auth; scoped to the T043 subset — in `services/control-plane/internal/scimbridge/server.go` (depends on T049)
- [x] T051 [US4] `scim_apply` translation to Admin API (user create/replace/disable, group→role) in `services/control-plane/internal/idp/scim_apply.go` (depends on T008, T050)
- [x] T052 [US4] Tenant-admin handlers (`PUT/GET/DELETE identity-providers`, `PUT/GET/:rotate/DELETE directory-sync`) under `requireAdmin` + register in `services/control-plane/internal/admin/api.go`; mount the SCIM bridge in `main.go` (depends on T048–T051)
- [x] T053 [US4] Audit brokering/SCIM config + sync events; record `last_sync_at` in `services/control-plane/internal/scimbridge/server.go`

**Checkpoint**: all three v1 user-provisioning mechanisms (invite + brokering + SCIM) work, each tenant-isolated.

---

## Phase 7: User Story 5 - Self-service signup (Priority: P3) — ⛔ DEFERRED (NOT in v1 scope)

**Deferred per clarification (Session 2026-06-14).** Tasks captured for a later release; do **not**
implement in v1. Reuses the US1 provisioning saga behind end-user guardrails.

- [ ] T054 [US5] *(deferred)* Signup flow with email/domain verification (gates provisioning) in `services/control-plane/internal/tenants/signup.go`
- [ ] T055 [US5] *(deferred)* Abuse rate-limiting/throttling for signup (FR-024)
- [ ] T056 [US5] *(deferred)* Public signup endpoint + slug guardrails reuse (FR-023) + adversarial abuse tests

---

## Phase 8: Polish & Cross-Cutting Concerns

- [ ] T057 [P] Update docs: `docs/multi-tenant-keycloak.md` + `docs/implementation.md` (tenant lifecycle) + README "Run it locally" (provisioning a 2nd tenant)
- [ ] T058 [P] Add `make provision-tenant SLUG=… NAME=… ADMIN_EMAIL=…` convenience target wrapping the platform API
- [~] T059 Run the `quickstart.md` isolation walkthrough end-to-end — the **release acceptance gate** (Constitution) — and check off its acceptance list
- [ ] T060 Security hardening pass: assert the privileged Keycloak credential + SCIM bearers + IdP secrets never appear in any API response, log, or trace (grep + a redaction test)
- [ ] T061 [P] Performance validation: provision <5min (SC-001), suspend propagation <1min (SC-006), SCIM deactivation ≤15min (SC-005)
- [ ] T062 Final `go build ./... && go vet ./... && go test ./...` green; integration suite green against the dev stack with `MCP_TEST_*` set

---

## Dependencies & Execution Order

### Phase dependencies
- **Setup (P1)** → no deps.
- **Foundational (P2)** → after Setup; **blocks all stories**.
- **US1 (P3)** → after Foundational. MVP.
- **US2, US3, US4 (P4–P6)** → after Foundational; each independently testable. US3 & US4 are easiest to demo on top of a US1-provisioned tenant but do not require US2.
- **US5 (P7)** → deferred (post-v1); reuses US1.
- **Polish (P8)** → after the v1 stories (US1–US4).

### Within each story
- Tests first (must fail) → models → stores → idp/service logic → handlers/wiring → audit.
- Models before stores; stores+idp before service; service before handlers.

### Parallel opportunities
- Setup: T002/T003/T004 in parallel.
- Foundational: T006/T007/T009/T010/T011/T012 in parallel (after T005); T008 after T007; T013 last.
- Each story's `[P]` test tasks run together (different files) and MUST fail before impl.
- With staff: after Foundational, US1/US2/US3/US4 proceed in parallel (distinct packages).

---

## Parallel Example: User Story 1 tests (write first)

```text
T014 Contract test — platform API (handlers_test.go)
T015 Adversarial — org token → 403 (authz_test.go)
T016 Adversarial — bad/duplicate slug → no realm (service_test.go)
T017 Adversarial — mid-saga failure → no ghost realm (service_compensation_test.go)
T018 Integration — bootstrap audience/issuer alignment (bootstrap_integration_test.go)
```

---

## Implementation Strategy

### MVP first (US1 only)
Setup → Foundational → US1 → **STOP & validate** with quickstart §1–§2 (provision + isolation gate). This alone delivers "operators can stand up isolated tenants."

### Incremental delivery
US1 (MVP) → US2 (invites) → US3 (lifecycle/kill-switch) → US4 (brokering + SCIM). Each is an independently testable, deployable increment. US5 deferred.

### Test-first, adversarially verified (NON-NEGOTIABLE)
Every story's contract + adversarial tests are written and failing before its implementation. The cross-tenant isolation and credential-containment tests are the gates that prove HC-1 and secret confidentiality.

---

## Notes
- `[P]` = different files, no incomplete deps. `[Story]` = traceability to spec.md.
- Live integration tests gate behind `MCP_TEST_*` (+ new `MCP_TEST_KEYCLOAK_*`) and `t.Skip`; everything else is hermetic.
- The **gateway and sandbox-supervisor are not modified** — suspend = realm-disable; delete reuses existing server-removed events.
- Mark task progress (`[X]`/`[~]`) here as work lands.
- The SCIM bridge (T043, T049–T053) is the highest-risk area — do the T043 spike before committing to the bridge scope.
