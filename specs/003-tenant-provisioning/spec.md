# Feature Specification: Automated Tenant Provisioning

**Feature Branch**: `003-tenant-provisioning`
**Created**: 2026-06-14
**Status**: Draft
**Input**: User description: "Can we use SCIM/Keycloak to auto-create realms as a tenant in the control plane, therefore having new isolated companies under the gateway? Make both halves — realm/tenant bootstrap and user provisioning — in the same spec."

A **control-plane capability to onboard and offboard a company as a fully isolated
tenant** of the Multi-Tenant MCP Gateway. Today a tenant exists only after someone
hand-runs the dev seed script against Keycloak; this feature makes tenant lifecycle
a first-class, automated, audited operation.

"Onboarding a company" has **two halves**, and this spec covers **both**:

| Half | What it does | Mechanism (domain terms) |
|---|---|---|
| **A — Tenant bootstrap** | Stand up the isolation boundary so the gateway recognizes a new org | Create the org's **identity realm** + its OAuth clients, audience/role mappers, the `admin` role, the first org-admin, and allocate its `{org}.{base-domain}` subdomain |
| **B — User provisioning** | Populate that realm with the tenant's people, under role guardrails | **Admin invitations**, **enterprise SSO brokering** (OIDC/SAML from the customer's IdP), and **directory sync (SCIM)** so a corporate directory auto-creates/deactivates users |

> **Why SCIM alone can't do this.** SCIM is a *user/group* provisioning protocol — it
> has no operation to create a realm/tenant. Realm creation is an administrative
> (Admin API) operation. So tenant bootstrap (Half A) is Admin-API-driven, and SCIM
> is **one of the mechanisms in Half B**, used *after* the realm exists. This spec
> keeps the two concerns distinct but delivers them together.

> **Builds on** `001-mcp-server-runtime` — realm-per-org tenancy, org derived from the
> request Host (`{org}.{base-domain}`) bound to the token issuer (`…/realms/{org}`),
> Postgres row-level-security per org. **Operated through** `002-admin-console`. The
> **data plane (gateway) needs no change**: a tenant becomes reachable the moment its
> realm exists and its subdomain resolves, because the gateway derives the org from
> Host + issuer and never reads a central tenant table. This feature MUST NOT weaken
> the inherited hard constraints — **organization isolation** and **secret
> confidentiality** (Constitution).

## Clarifications

### Session 2026-06-14

- Q: One realm per tenant, or one shared realm with an in-realm "organization"
  construct? → A: **Realm-per-tenant.** It is the strongest isolation boundary
  (separate issuer, keys, sessions) and is consistent with `001`'s realm-per-org
  model and HC-1. The single-realm "organizations" alternative is recorded as a
  scaling fallback in Assumptions, not adopted for v1. **Target scale for v1: tens
  to low hundreds of tenants**; revisit the single-realm Organizations model only
  if tenant counts approach the thousands (a deliberate migration, out of scope here).
- Q: Who is allowed to create a tenant? → A: **Platform operators only** for v1
  (US1) — a privileged role above any single org. **Self-service signup** (US5)
  is **deferred to a later release** (out of scope for v1).
- Q: How are users provisioned into a tenant? → A: **All three mechanisms ship in
  v1** — **admin email invitations** (US2) and **enterprise SSO brokering + SCIM
  directory sync** (US4), both P2. A tenant may use any combination. This makes a
  **SCIM-capable identity platform a v1 dependency** (see Assumptions).
- Q: What does "deprovision" mean? → A: **Two-stage.** *Suspend* disables the
  realm (reversible; immediately stops new tokens validating). *Delete* **purges**
  the realm, identity assets, stored credentials and server definitions
  immediately, but **retains the tamper-evident (WORM) audit trail for a fixed
  retention window**, after which it too is purged.
- Q: Is the gateway changed? → A: **No.** Provisioning only creates identity
  assets, DNS, and a control-plane tenant record; the gateway already resolves
  orgs from Host + issuer.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Operator provisions a new isolated tenant (Priority: P1)

A platform operator onboards a new company (e.g. `globex`). They supply the
company name, a desired subdomain slug, and the first administrator's email. The
system provisions everything the company needs to be a tenant: its own isolated
identity realm, the console and MCP OAuth clients (with the correct audiences and
role mappers), the `admin` role, and an initial org-admin account; it allocates
`globex.{base-domain}`. When provisioning completes, the company is immediately
usable — the new admin can sign in to the console for `globex`, and a `globex`
token is accepted by the gateway — with **zero** visibility into any other tenant.

**Why this priority**: This is the core of the feature and the MVP. Without
automated bootstrap there is no repeatable, safe way to add a company; everything
else (invites, SSO, signup) presupposes a tenant exists.

**Independent Test**: Provision `globex` end-to-end; confirm (a) the new admin can
authenticate and load the `globex` console, (b) a `globex`-issued token is accepted
at the gateway while an `acme` token is rejected for `globex` and vice-versa, and
(c) `globex` sees none of `acme`'s servers, users, audit, or credentials.

**Acceptance Scenarios**:

1. **Given** an operator and an unused slug `globex`, **When** they submit a
   provisioning request, **Then** the tenant is created with its realm, both OAuth
   clients, the audience + realm-role mappers, the `admin` role, an initial admin
   account, and the `globex` subdomain — and the request is recorded in the audit
   trail.
2. **Given** `globex` has just been provisioned, **When** its admin signs in to
   `globex.{base-domain}` console, **Then** they authenticate against the `globex`
   realm and see an empty-but-functional org (no servers yet).
3. **Given** both `acme` and `globex` exist, **When** an `acme` token is presented
   for a `globex` host (or vice-versa), **Then** the gateway rejects it (issuer/audience
   mismatch) — isolation holds.
4. **Given** a provisioning request for a slug that already exists or is reserved,
   **When** it is submitted, **Then** it is rejected before any identity asset is
   created, with a clear reason.

---

### User Story 2 - Org admin invites users with role guardrails (Priority: P2)

An existing tenant's admin invites teammates by email and assigns each a role
(e.g. `member`, `admin`, or a tool-scoping role such as `aws-users`). The invitee
receives an invitation, accepts it, sets up credentials, and gets an account **in
that tenant's realm only**, carrying exactly the assigned roles. Their gateway
tool access immediately reflects those roles (RBAC from `001`).

**Why this priority**: Invitations are the simplest, no-dependency way to populate
a tenant and make it useful; they require no customer-side IdP. Needed for any
real use of a freshly provisioned tenant.

**Independent Test**: As the `globex` admin, invite `dana@globex.example` as
`member`; accept the invite; confirm Dana can sign in to `globex`, appears only in
`globex`, and sees only the tools her roles permit — and does not exist in `acme`.

**Acceptance Scenarios**:

1. **Given** a tenant admin, **When** they invite an email with role `member`,
   **Then** a pending invitation is created and the recipient is notified.
2. **Given** a valid pending invitation, **When** the recipient accepts and
   completes setup, **Then** a user is created in that tenant's realm with the
   assigned role(s) and can sign in.
3. **Given** an admin assigns a role, **When** the user obtains a token, **Then**
   the gateway grants/denies tools per that role (consistent with `001` RBAC).
4. **Given** an invitation, **When** it is not accepted within its validity window
   or is revoked by an admin, **Then** it can no longer be used.
5. **Given** the same email is invited into two different tenants, **When** both
   are accepted, **Then** they are **separate accounts** in separate realms with no
   shared state.

---

### User Story 3 - Suspend and deprovision a tenant (Priority: P2)

An operator can **suspend** a tenant (e.g. non-payment, security incident,
offboarding) and later **resume** it, or **delete** it permanently. Suspending
immediately stops the tenant's users from obtaining or renewing access; deleting
removes the realm and disposes of the org's data per the retention policy. Other
tenants are never affected.

**Why this priority**: Lifecycle and a kill-switch are operational and security
necessities — the inverse of provisioning. Suspension is the safe, reversible
control; deletion is the terminal one.

**Independent Test**: Suspend `globex`; confirm new/renewed `globex` tokens are
rejected at the gateway and the console denies access, while `acme` is unaffected;
resume `globex` and confirm access returns; then delete a throwaway tenant and
confirm its realm, clients, servers and credentials are gone and its subdomain is
freed.

**Acceptance Scenarios**:

1. **Given** an active tenant, **When** an operator suspends it, **Then** its realm
   is disabled, new and refreshed tokens fail validation at the gateway, and the
   action is audited — with no effect on other tenants.
2. **Given** a suspended tenant, **When** an operator resumes it, **Then** access
   is restored without data loss.
3. **Given** a tenant marked for deletion, **When** deletion runs, **Then** the
   realm and OAuth clients are removed, the org's server definitions and stored
   credentials are destroyed, the **WORM audit trail is retained for the retention
   window then purged**, and the subdomain slug becomes available again.
4. **Given** any deprovision action, **When** it completes or fails, **Then** the
   outcome is recorded and a partially-deleted tenant is never left in an
   ambiguous state.

---

### User Story 4 - Enterprise SSO and directory sync (brokering + SCIM) (Priority: P2)

A tenant connects its corporate identity provider. Two complementary capabilities:
**(a) SSO brokering** — employees log in to the tenant using their corporate
OIDC/SAML identity (just-in-time account creation on first login); and **(b) SCIM
directory sync** — the corporate directory pushes user create/update/deactivate
events and group memberships into the tenant's realm, mapping groups to roles, so
membership stays in sync without manual invitations.

**Why this priority**: Required for v1 — enterprise customers onboard via their own
IdP (brokered login) and keep membership in sync via SCIM without manual invites. It
depends only on a working tenant (US1) and ships alongside invitations (US2); both
are P2 user-provisioning paths.

**Independent Test**: For `globex`, configure brokering to a test IdP and confirm a
corporate user can log in and is JIT-provisioned into `globex` with mapped roles;
separately, push a SCIM create and a SCIM deactivate and confirm the user appears
and then loses gateway access on their next token — all scoped to `globex`.

**Acceptance Scenarios**:

1. **Given** a tenant with brokering configured to an external IdP, **When** a
   corporate user logs in for the first time, **Then** an account is created in the
   tenant's realm with roles mapped from the IdP assertion, and they can use the
   gateway per RBAC.
2. **Given** a tenant with directory sync enabled, **When** the directory creates a
   user, **Then** a corresponding user appears in the tenant's realm with mapped
   roles; **When** the directory deactivates a user, **Then** that user can no
   longer obtain tenant access.
3. **Given** a group-to-role mapping, **When** a user's group membership changes
   upstream, **Then** their tenant roles (and therefore tool access) change
   accordingly on their next token.
4. **Given** any brokering/sync configuration, **When** it is applied, **Then** it
   affects only the configuring tenant's realm and never another tenant.

---

### User Story 5 - Self-service signup creates a tenant (Priority: P3 — deferred, not in v1 scope)

A prospective customer signs up directly (no operator in the loop). After
verifying their email (and, where required, domain ownership), the system
provisions a new isolated tenant and makes the signer its first org admin — the
same bootstrap as US1, but initiated by the end user behind guardrails (slug
availability/format, reserved names, and abuse rate-limiting).

**Why this priority**: Self-service is the growth path for a SaaS, but it carries
the most abuse and security surface (end users triggering a privileged operation).
**Deferred to a later release** — v1 ships operator-initiated provisioning only;
this story is retained for future scope.

**Independent Test**: Complete a signup for `newco` from scratch, verify the email,
and confirm the signer lands in `newco`'s console as admin with a fully isolated
tenant; then confirm a second signup cannot claim an existing or reserved slug and
that repeated rapid signups are throttled.

**Acceptance Scenarios**:

1. **Given** a visitor on the signup flow, **When** they submit a company name,
   available slug, and email and verify that email, **Then** a tenant is provisioned
   (identical end-state to US1) and they become its admin.
2. **Given** signup, **When** the requested slug is taken, reserved, or malformed,
   **Then** it is rejected with guidance before any identity asset is created.
3. **Given** signup is open, **When** a single actor attempts many signups quickly,
   **Then** the system throttles/limits them to resist abuse.

---

### Edge Cases

- **Reserved / colliding slugs**: existing tenant slugs and reserved labels
  (`www`, `api`, `admin`, `auth`, the apex, etc.) MUST be rejected. Slugs MUST be a
  valid DNS label and a valid realm name.
- **Partial provisioning failure**: if any step fails after the realm is created
  (client, mapper, role, admin user, subdomain, tenant record), the operation MUST
  either fully roll back or be safely resumable — never leave a usable-but-broken
  tenant or a "ghost" realm.
- **Identity store unavailable**: if the identity platform or the privileged
  provisioning credential is unavailable, provisioning fails cleanly and is
  retryable, with no side effects committed.
- **Subdomain not yet routable**: provisioning must state readiness honestly — a
  tenant whose DNS/TLS is not yet live is reported as pending, not "ready".
- **Same human in two tenants**: one email invited/synced into two tenants yields
  two independent accounts; nothing is shared across realms.
- **Suspend with live tokens**: existing access tokens remain valid until they
  expire; the policy for whether suspension must force-revoke active sessions MUST
  be stated (default: deny new/refresh immediately; existing short-lived tokens
  expire naturally).
- **Deprovision with active downstream instances**: deleting/suspending a tenant
  must also terminate that org's running MCP server instances and revoke its
  injected credentials (kill-switch parity with `001`).
- **Concurrent provisioning** of two tenants, or two signups racing for the same
  slug, MUST not corrupt either or double-allocate a slug.
- **Directory deprovision lag**: a SCIM/brokering deactivation must remove gateway
  access by the user's next token at the latest; the maximum lag MUST be bounded.
- **Realm-count scaling**: the number of tenants (realms) the platform can hold has
  a practical ceiling; the system MUST surface tenant count and warn before limits
  that degrade identity-platform performance.

## Requirements *(mandatory)*

### Functional Requirements

**Tenant bootstrap (Half A)**

- **FR-001**: The control plane MUST provide an operation to provision a new tenant
  from a company name, a subdomain slug, and an initial admin identifier (email).
- **FR-002**: Provisioning MUST create a dedicated, isolated identity realm for the
  tenant whose issuer is bound to the tenant slug (`…/realms/{slug}`).
- **FR-003**: Provisioning MUST create the tenant's OAuth clients — the admin
  console client and the MCP data-plane client — each with the correct redirect/web
  origins and the **audience** and **realm-role** mappers required by the console
  (admin-API audience) and the gateway (MCP resource audience `{slug}.{base}/mcp`).
- **FR-004**: Provisioning MUST create the baseline authorization roles (at minimum
  the `admin` role governing the console/control-plane) and the initial org-admin
  account bound to the supplied admin email.
- **FR-005**: Provisioning MUST allocate and record the tenant's
  `{slug}.{base-domain}` and report when the subdomain is actually routable
  (DNS/TLS ready) versus pending.
- **FR-006**: Provisioning MUST be **idempotent** (re-running for the same slug
  converges without duplicates) and MUST **roll back or be resumable** on partial
  failure, never leaving a half-provisioned tenant.
- **FR-007**: The control plane MUST record a **tenant** entity (slug, display name,
  status, timestamps, admin contact) as the system-of-record for lifecycle and
  display, without the gateway needing to read it on the request path.
- **FR-008**: Slugs MUST be validated for format (DNS label + realm-name rules),
  uniqueness, and against a reserved list before any identity asset is created.

**Isolation & security (inherited hard constraints)**

- **FR-009**: A provisioned tenant MUST be fully isolated: a token from one tenant
  MUST NOT be accepted for another, and no tenant MUST be able to see or affect
  another tenant's realm, users, servers, credentials, or audit (HC-1).
- **FR-010**: The privileged credential used to create realms MUST be held only by
  the control plane, stored in the secret store (never in the per-request path or
  the data plane), scoped to the minimum identity-management permissions, and never
  logged or displayed (secret confidentiality).
- **FR-011**: Tenant creation MUST be authorized to **platform operators** only;
  no tenant admin or member may provision or modify another tenant.
- **FR-012**: Every provisioning, user-provisioning, and lifecycle action MUST be
  recorded in the audit trail with actor, target tenant, action, and outcome
  (consistent with `001`'s audit), without recording secret values.

**User provisioning (Half B)**

- **FR-013**: A tenant admin MUST be able to invite a user by email and assign one
  or more roles at invitation time (role guardrails), creating a pending invitation.
- **FR-014**: Accepting a valid invitation MUST create a user **in that tenant's
  realm only**, carrying exactly the assigned roles; the user's gateway tool access
  MUST follow `001` RBAC for those roles.
- **FR-015**: Invitations MUST expire after a validity window and MUST be revocable
  by an admin before acceptance.
- **FR-016**: A tenant MUST be able to enable **SSO brokering** to an external
  OIDC/SAML IdP, with first-login just-in-time account creation and configurable
  mapping of IdP assertions/groups to tenant roles.
- **FR-017**: A tenant MUST be able to enable **directory sync (SCIM)** so an
  external directory can create, update, and deactivate users and map groups to
  roles; a deactivation upstream MUST remove the user's gateway access within a
  bounded time (by their next token at the latest).
- **FR-018**: All three user-provisioning mechanisms — invitations, SSO brokering,
  and SCIM directory sync — are **in scope for v1**, MUST be combinable within a
  tenant, and MUST each affect only that tenant's realm.

**Lifecycle / deprovision**

- **FR-019**: An operator MUST be able to **suspend** a tenant, which immediately
  prevents its users from obtaining or refreshing access (the gateway rejects new
  tokens), and to **resume** it later without data loss.
- **FR-020**: An operator MUST be able to **delete** a tenant: remove its realm and
  OAuth clients, destroy its stored credentials, terminate its running MCP server
  instances, dispose of its server definitions, and free its slug. The tenant's
  **tamper-evident (WORM) audit trail MUST be retained for a fixed, configurable
  retention window** (e.g. matching the platform's compliance retention) and purged
  only after that window elapses.
- **FR-021**: Suspend/delete MUST achieve **kill-switch parity** with `001`: a
  disabled or deleted tenant's downstream server instances are terminated and its
  injected credentials revoked.
- **FR-022**: Deprovision actions MUST be atomic in effect (no ambiguous
  half-deleted state) and fully audited.

**Self-service signup** *(deferred — not in v1 scope; retained for a later release)*

- **FR-023**: The system MAY offer self-service signup that provisions a tenant
  (same end-state as FR-001–008) initiated by an end user, gated by **email (and
  where required domain) verification**.
- **FR-024**: Self-service signup MUST enforce slug guardrails (FR-008) and MUST
  rate-limit/throttle to resist mass-creation abuse.

**Reliability & observability**

- **FR-025**: Provisioning and deprovisioning MUST emit status the operator/admin
  can observe (pending → ready/failed), with actionable failure reasons.
- **FR-026**: The system MUST surface the current tenant (realm) count and warn the
  operator as it approaches a configured ceiling beyond which identity-platform
  performance degrades.

### Key Entities *(include if feature involves data)*

- **Tenant (Organization)**: a customer company; the unit of isolation. Attributes:
  slug (= subdomain = realm name), display name, status (`provisioning`,
  `active`, `suspended`, `deleting`, `failed`), admin contact, created/updated
  timestamps. System-of-record in the control plane.
- **Identity Realm**: the per-tenant identity boundary (issuer, signing keys,
  sessions). One-to-one with a Tenant. Not stored by the control plane beyond the
  binding to the slug.
- **OAuth Client**: the console client and the MCP client created per tenant, each
  with audience and role mappers. Two per Tenant.
- **Role**: tenant-scoped authorization role (e.g. `admin`, `member`,
  tool-scoping roles like `aws-users`) consumed by `001` RBAC.
- **Tenant Admin / Member (User)**: an account within a Tenant's realm; never
  shared across tenants.
- **Invitation**: a pending grant to join a Tenant with assigned role(s); has a
  validity window and a revoked/accepted/expired state.
- **Identity Provider Link (Brokering)**: a tenant's configured external OIDC/SAML
  IdP with assertion/group→role mappings.
- **Directory Sync Connection (SCIM)**: a tenant's configured directory source with
  group→role mappings and user lifecycle (create/update/deactivate).
- **Subdomain Allocation**: the binding of a slug to `{slug}.{base-domain}` and its
  routability/readiness state.
- **Provisioning Record / Job**: an auditable record of a provision/suspend/
  resume/delete operation, its steps, and its outcome (for idempotency, rollback,
  and observability).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can take a company from request to a usable, isolated
  tenant (admin can sign in; gateway accepts the tenant's tokens) in **under 5
  minutes**, with no manual identity-platform steps.
- **SC-002**: **100%** of cross-tenant access attempts are denied — a token, user,
  or admin of one tenant can never read or affect another tenant's resources
  (verified by an isolation test between two provisioned tenants).
- **SC-003**: **Zero** partially-provisioned tenants persist after a failure: every
  failed provision either rolls back completely or is resumable to a correct state
  (verified by fault injection on each provisioning step).
- **SC-004**: A tenant admin can invite a user and have them signed in with the
  correct tool access in **under 3 minutes**, without operator involvement.
- **SC-005**: An upstream directory **deactivation** (SCIM) or brokered-account
  disablement removes the user's gateway access within **one token lifetime** (≤15
  minutes with the default access-token TTL).
- **SC-006**: **Suspending** a tenant stops new/refreshed token acceptance at the
  gateway within **1 minute** and provably does not affect any other tenant.
- **SC-007**: **Deleting** a tenant leaves no residual identity assets, credentials,
  running instances, or routable subdomain, and frees the slug for reuse; its WORM
  audit trail remains retrievable until the retention window elapses, then is
  provably purged (verified by post-deletion checks).
- **SC-008**: Re-running a provisioning request for an existing tenant produces **no
  duplicate** realms, clients, roles, or users (idempotency verified).
- **SC-009**: The platform can operate **tens to low hundreds of tenants** (the v1
  target) with provisioning latency and identity-platform responsiveness within
  agreed limits, and warns the operator before the configured realm-count ceiling.

## Assumptions

- **Realm-per-tenant** is the chosen isolation model (consistent with
  `001-mcp-server-runtime`). The alternative — a single shared realm with an
  in-realm "organizations" construct — is a **scaling fallback** if tenant counts
  exceed what realm-per-tenant supports comfortably; it is **out of scope** for v1.
- The identity platform is the existing per-org **Keycloak** deployment; tenant
  bootstrap uses its **administrative (Admin) API**, and brokering uses its built-in
  OIDC/SAML identity brokering. Because **SCIM directory sync is in v1 scope**, the
  identity platform MUST provide a **SCIM capability in v1** (native or a supported
  extension) — a v1 dependency, not optional.
- A **wildcard subdomain and TLS** (`*.{base-domain}`) route all tenants to the
  gateway/console; per-tenant DNS records are not created by hand. In dev, a hosts
  entry stands in for wildcard DNS.
- A single **base domain** is used; multi-base-domain / vanity domains are out of
  scope for v1.
- The **admin console (`002-admin-console`)** provides the operator and tenant-admin
  UI surfaces for these operations; this spec defines the capability, not new visual
  design.
- The **gateway (data plane) is unchanged** — it already derives the org from Host +
  token issuer and enforces RBAC; provisioning only creates identity assets, DNS,
  and the control-plane tenant record.
- **Secrets remain write-only** and the privileged provisioning credential lives in
  the control plane's secret store (Vault); nothing here displays or logs secrets.
- A **platform-operator** role exists (or is introduced) above any single org to
  authorize tenant creation and lifecycle; it is distinct from a tenant's `admin`.
- **Self-service signup (US5)**, billing/metering, custom vanity domains, and
  migrating an existing tenant between isolation models are **out of scope for v1**
  (self-service is retained for a later release).
