# Phase 0 Research: Automated Tenant Provisioning

Decisions resolving the Technical Context unknowns. Format per decision:
**Decision · Rationale · Alternatives considered.**

## 1. Realm bootstrap mechanism (Half A)

**Decision**: Call the **Keycloak Admin REST API from the control plane** using
`Nerzal/gocloak`, wrapped behind a small internal interface (`idp.Keycloak`) so it
is mockable in tests. Bootstrap is the programmatic equivalent of
`deploy/dev/seed-keycloak.sh`: create realm → `mcp-admin-console` + `mcp-client`
clients → audience + realm-role protocol mappers → `admin` role → initial admin user.

**Rationale**: We already use Keycloak; the Admin API is the canonical way to create
realms/clients/roles/users. A typed Go client behind an interface keeps the call sites
testable (Principle V) and lets us swap the backing mechanism later without touching
callers. The seed script proves every object/attribute needed, so this is well-bounded.

**Alternatives considered**:
- **`kcadm.sh` shell-out** — what the dev seed uses; brittle to parse, hard to test, ties us to a container exec. Rejected for production.
- **Keycloak Operator `KeycloakRealmImport` CRD (GitOps)** — excellent for declarative k8s, but async, requires the operator, and couples provisioning to the cluster. Kept as a **future option** the `idp.Keycloak` interface preserves; not v1.
- **Raw REST by hand** — reinvents a maintained client. Rejected (Principle VII).

## 2. Idempotency & rollback (no half-tenants — FR-006)

**Decision**: Model each lifecycle action as an ordered **saga** of steps with
**compensations**, tracked in a `provisioning_jobs` row (per-step state in JSONB).
Each create step is **idempotent** (check-then-create, exactly as the seed script does
by `clientId`/role name). On failure: attempt **compensation in reverse** (delete what
was created) and mark the job `failed`; a `provision` job is also **resumable** (re-running
converges). The tenant row stays `provisioning`/`failed` until the job completes.

**Rationale**: Provisioning spans multiple external mutations (realm, clients, mappers,
role, user, DB row). Sagas give a recoverable, observable, all-or-nothing *effect* without
distributed transactions. Idempotent upserts make retries safe and satisfy SC-008.

**Alternatives considered**: a single best-effort sequence (no compensation) — rejected,
leaves ghost realms on partial failure (violates FR-006). Two-phase commit across
Keycloak+Postgres — not available; over-engineered (Principle VII).

## 3. Platform-operator authorization (cross-tenant API)

**Decision**: Introduce a dedicated **platform realm** (`MCP_PLATFORM_REALM`, e.g.
`_platform`) issuing operator tokens carrying a **`platform-admin`** role. The platform
API validates against that realm via a new `requirePlatformAdmin` middleware (mirrors
`requireAdmin`, but realm/audience are fixed, not derived from a path `:org`). For DB
reads it sets the existing RLS escape `app.current_org='*'` so an operator can list all
tenants.

**Rationale**: Tenant creation is inherently cross-tenant, so it cannot use a per-org
realm token. A separate realm cleanly separates "who may operate the platform" from any
single customer, and the `'*'` RLS context already exists in `postgres.go`. A tenant
admin token can never reach the platform API (different realm/role) — preserves HC-1.

**Alternatives considered**: a role in the Keycloak **master** realm — works but overloads
the Keycloak super-admin realm with product authz; weaker separation. A special role inside
some tenant's realm — rejected (couples the platform to a customer; isolation smell).

## 4. The privileged Keycloak credential (how the control plane effects changes)

**Decision**: A **service-account client** (client-credentials grant) in the master (or
platform) realm holding the **minimum** realm-management roles (`create-realm`, plus
`manage-realm`/`manage-clients`/`manage-users` on managed realms). Its secret is stored in
**Vault** (`pkg/secrets`), loaded at startup, **never logged**, and **rotatable**. This is
**separate** from the operator's user token: the operator token authorizes the *endpoint*;
the control plane then uses its *own* privileged credential to do the Keycloak work.

**Rationale**: Separation of duties — operators never hold Keycloak admin secrets; the blast
radius of the privileged credential is confined to the control plane (Principle II/VI). Vault
storage + rotation matches the existing secret-handling model.

**Alternatives considered**: operator's own token carries Keycloak admin rights — rejected
(spreads a powerful credential to humans, no rotation boundary). A static admin password —
rejected (not rotatable, weak).

## 5. Tenant registry & RLS placement

**Decision**: A **platform-scoped** `tenants` table (the org registry) + `provisioning_jobs`,
reachable only with `app.current_org='*'` (platform API). **Org-scoped** child tables —
`invitations`, `idp_links`, `scim_connections` — carry `org_id` and use the **same RLS policy
shape** as `servers` (`org_id = current_setting('app.current_org')`). The **gateway never reads
`tenants`** on the request path (org is still derived from Host + issuer).

**Rationale**: Keeps the request-path contract unchanged (no new gateway dependency) and reuses
the proven RLS pattern for per-tenant data while gating the cross-tenant registry behind platform
authz. Consistent with Principle I and the existing `withOrg` helper.

**Alternatives considered**: putting `tenants` under org RLS — nonsensical (the registry is the
list of orgs). A separate database — rejected (Principle VII; one Postgres with RLS suffices).

## 6. SCIM directory sync (the v1 dependency)

**Decision**: A **control-plane-hosted SCIM 2.0 bridge**, mounted per tenant, that authenticates
the customer's IdP with a **per-tenant bearer token** (Vault, write-only) and translates SCIM
operations → Keycloak Admin API. **Scope the subset** that Okta/Entra/Google actually emit:
`/Users` create/replace/PATCH (`active=false` ⇒ disable user ⇒ gateway access removed by next
token), `/Groups` membership, and group→role mapping. Pagination/filtering limited to what those
IdPs require.

**Rationale**: SCIM is now a v1 must-have, and Keycloak core has no SCIM server. Owning the bridge
makes it **hermetically testable** (Principle V), avoids a commercial/maturity/licensing dependency,
and reuses the Admin API we already integrate. Scoping to the real-world subset keeps it tractable
(Principle VII).

**Alternatives considered**: a **Keycloak SCIM extension** (e.g. community `keycloak-scim`,
commercial *SCIM for Keycloak*) — less code but adds a Keycloak deployment + maturity/licensing
dependency and resists hermetic testing; **kept as a documented fallback** if the bridge subset
proves insufficient. Full RFC 7644 compliance — rejected for v1 (YAGNI; expand if a target IdP needs it).
**Risk flag**: this is the highest-effort/-risk item — `/speckit-tasks` should schedule a SCIM
conformance spike against one real IdP early.

## 7. SSO identity brokering

**Decision**: Configure Keycloak **native Identity Brokering** per realm via the Admin API — add an
OIDC/SAML `IdentityProvider` + identity-provider mappers (assertion/group → realm role), with
**first-login JIT** account creation (native). The brokered IdP's client secret is stored in Vault.

**Rationale**: Brokering is a first-class Keycloak feature; configuring it via the Admin API needs no
new runtime and inherits JIT + token refresh. Mappers give the group→role control the spec requires.

**Alternatives considered**: building our own OIDC/SAML broker — rejected (reinvents Keycloak).

## 8. Subdomain / DNS / TLS

**Decision**: Assume a **wildcard `*.{base-domain}` DNS + wildcard TLS** routing all tenants to the
gateway/console. Provisioning **records the slug**; it does not call a DNS API in v1. Readiness =
realm reachable + slug allocated; the tenant exposes a `subdomain_ready` derived state. Dev uses the
existing `/etc/hosts` entry pattern.

**Rationale**: Wildcard routing makes per-tenant DNS a no-op, the simplest correct design (Principle VII)
and matches how `acme` already works locally.

**Alternatives considered**: per-tenant DNS record creation (Route53/etc.) — deferred (vanity domains
are out of scope; wildcard covers v1).

## 9. Audit retention through deletion (Principle VI)

**Decision**: On **delete**, purge realm/clients/credentials/server-defs immediately, but **retain the
tenant's tamper-evident (WORM) audit ≥ 1 year** (`MCP_AUDIT_RETENTION_DAYS`, default 365), then purge.
The tenant row keeps `audit_retention_until`; an audit purge is a **deferred/scheduled** job, not part
of the synchronous delete.

**Rationale**: Principle VI mandates ≥ 1-year tamper-evident retention; deletion must not violate it. The
hash-chain + S3 WORM already retain — delete only schedules the eventual purge.

**Alternatives considered**: immediate hard-delete of audit (the clarify "hard-delete" option) — rejected:
violates Principle VI. Indefinite retention — unnecessary; a bounded window suffices.

## 10. Kill-switch parity on suspend/delete (gateway stays unchanged)

**Decision**: **Suspend** = disable the realm via Admin API ⇒ login/refresh fail ⇒ the gateway rejects
new tokens automatically (no gateway change); already-issued short-lived tokens (≤ 15 min) expire
naturally (documented default). **Delete** = remove the org's server definitions first ⇒ the **existing
`pkg/serverevents` Redis events** the gateway already consumes terminate running instances and revoke
injected credentials ⇒ then delete the realm.

**Rationale**: Reuses mechanisms that already exist (realm-disable token rejection + server-removed
reconciliation), so the **data plane needs no change** — upholding the spec's assumption and Principle VII.

**Alternatives considered**: a new gateway-side "tenant disabled" event + forced session revocation +
realm key rotation for instant cutoff — heavier, changes the gateway; deferred (the ≤ 15-min natural-expiry
window is the documented, acceptable default).

---

## Post-Phase-1 Constitution re-check

After designing the data model + contracts (Phase 1), the gate still holds:
- **Isolation (I)**: every new org-scoped table carries `org_id` + RLS; the `tenants` registry is
  platform-gated; the platform realm/role keeps operator authority off tenant tokens. Adversarial
  cross-tenant + credential-containment tests are first-class (see quickstart + tasks).
- **Auditable (VI)**: contracts route every mutation through `pkg/audit`; retention window honored on delete.
- **Simplicity (VII)**: no new service; the two tracked complexities (operator-gated create, SCIM bridge)
  remain the only deviations, both justified.
No new violations. **PASS.**
