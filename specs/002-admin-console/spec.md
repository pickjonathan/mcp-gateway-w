# Feature Specification: Admin Console UI (Carbon Design System)

**Feature Branch**: `002-admin-console`
**Created**: 2026-06-13
**Status**: Draft
**Input**: User description: "I would like to create an admin user interface based on the design system located at the project root (Carbon Design System handoff). The admin should have all features created in the first spec."

A web-based administration console that gives an organization's administrators a
visual, on-brand way to perform **every admin-facing capability** of the
Multi-Tenant MCP Server Runtime (feature `001-mcp-server-runtime`): managing MCP
servers, credentials, role-based access, audit review, and quotas/health — all
presented with the **Carbon Design System** supplied at the project root.

> This feature adds a **presentation layer only**. It is a client of the existing
> control-plane admin API and identity (Keycloak, per-org realm, admin role); it
> introduces no new backend capability and must never weaken the runtime's hard
> constraints (organization isolation, secret confidentiality).

## Scope: features carried over from `001-mcp-server-runtime`

Every admin-facing capability of the first spec is surfaced in the console:

| `001` capability | Console surface |
|---|---|
| Add/list/edit/enable/disable/delete remote & stdio servers (US2/US3, FR-001–006) | Servers catalog + add/edit forms + row actions |
| Health status of each server | Status tags + server detail |
| Credentials: modes none/org_shared/per_user, write-only, rotation (US6) | Credentials panel (set/rotate/clear; never displays values) |
| RBAC: `allowed_roles` per server (US5) | Access panel (role assignment + visibility) |
| Audit trail incl. auth/authz denials, tamper-evident chain (US7) | Audit log table + chain-status indicator |
| Quotas / rate limits (per-org, per-user) (US7) | Settings → limits |
| Org-scoped OAuth (per-org realm, admin role, audience-bound) | Sign-in + org context in the shell |
| Kill-switch (disable/remove terminates instances) | Disable toggle / delete with confirmation |

The data plane (gateway, sandbox execution) is **out of scope** — it is unchanged.

## Clarifications

### Session 2026-06-13

- Q: How should the console handle quotas, given `001` configures rate limits via
  env (no quota API) and FR-023 forbids new mutating backend capability? →
  A: **Read-only display** — show the configured org/user limits via a small
  read-only endpoint; editing limits is deferred (out of scope for v1).
- Q: What source should the v1 dashboard use for request/denial/error rate trends,
  since `/metrics` is Prometheus exposition (not a query API)? →
  A: **Query Prometheus** — the console queries the metrics system's query API for
  rate charts (reachable via same-origin proxy/edge or CORS); server health,
  counts, and denial indicators come from the servers + audit APIs.
- Q: How is the console registered as an OAuth client in each org's Keycloak
  realm? → A: **Pre-registered public client** — a known `client_id`
  (e.g. `mcp-admin-console`) provisioned per realm, using Authorization Code +
  PKCE (not dynamic registration).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sign in and see the organization dashboard (Priority: P1)

An administrator opens the console for their organization, signs in with their
existing credentials, and lands on a dashboard that summarizes the org's MCP
footprint: how many servers exist, their health, recent activity, and any
attention items (e.g., elevated denials). Non-admins are refused.

**Why this priority**: It is the secure entry point and the at-a-glance value;
nothing else is reachable without it. A read-only dashboard over the existing API
is already useful on its own.

**Independent Test**: Sign in as an org admin → the shell and dashboard load with
live counts/health for that org only; sign in as a non-admin → access is refused;
sign in scoped to org A → no org B data is ever shown.

**Acceptance Scenarios**:

1. **Given** a valid admin for org A, **When** they sign in, **Then** the console
   shows org A's dashboard (server count, health summary, recent activity).
2. **Given** an authenticated user without the admin role, **When** they open the
   console, **Then** they are shown an access-denied state and no org data.
3. **Given** an expired session, **When** the admin performs an action, **Then**
   they are prompted to re-authenticate and no stale data is shown.

---

### User Story 2 - Manage MCP servers (Priority: P1)

An administrator views the catalog of their org's MCP servers in a table, adds a
new one (remote HTTP by endpoint URL, or stdio by command/args/env), edits an
existing one, enables/disables it, and deletes it — each with clear validation
and confirmation. This is the core, frictionless admin job.

**Why this priority**: This is the primary reason the console exists and directly
serves the runtime's "frictionless admin" hard constraint.

**Independent Test**: Add a remote and a stdio server through the forms → both
appear in the catalog with correct type/status; disable one → its status reflects
the kill-switch; delete one → it is removed after confirmation.

**Acceptance Scenarios**:

1. **Given** the servers catalog, **When** the admin adds a remote server with a
   valid endpoint URL, **Then** it appears in the list with type "remote" and a
   health status.
2. **Given** the add form, **When** the admin adds a stdio server with a command
   and arguments, **Then** it appears with type "stdio".
3. **Given** a slug already used in the org, **When** the admin submits it again,
   **Then** the form shows an inline error and does not create a duplicate.
4. **Given** a server, **When** the admin disables it and confirms, **Then** its
   status shows disabled and end users can no longer use it.
5. **Given** a server, **When** the admin deletes it and confirms, **Then** it is
   removed from the catalog.
6. **Given** invalid input (e.g., missing endpoint), **When** the admin submits,
   **Then** field-level errors are shown and nothing is saved.

---

### User Story 3 - Manage credentials safely (Priority: P2)

For a server, an administrator chooses how credentials are provided (none,
organization-shared, or per-user) and sets, rotates, or clears the secret values
through a form that **never displays stored secrets** — it shows only whether a
credential is set and when it was last updated.

**Why this priority**: Credentials unlock most real MCP servers, but mishandling
them is the biggest risk; safe handling must be built in, not bolted on.

**Independent Test**: Set an org-shared credential → the panel shows "set" with a
timestamp and never reveals the value; rotate it → next use takes the new value;
clear it → status returns to "not set".

**Acceptance Scenarios**:

1. **Given** a server set to organization-shared, **When** the admin saves a
   credential, **Then** the panel shows "set" + last-updated and the value is
   never shown again anywhere in the UI.
2. **Given** a stored credential, **When** the admin rotates it, **Then** the
   panel confirms the update without displaying either value.
3. **Given** per-user mode, **When** the signed-in admin saves their own
   credential, **Then** it applies to their sessions only.
4. **Given** a save failure, **When** the admin submits, **Then** an error
   notification appears and no partial state is shown as success.

---

### User Story 4 - Control access with roles (Priority: P2)

An administrator restricts which roles may use each server and can see at a glance
which servers are open to all members versus role-restricted.

**Why this priority**: Access control is required for real orgs but builds on the
server-management story.

**Independent Test**: Restrict a server to a role → the catalog marks it
restricted and lists the roles; clear restrictions → it shows as open to all.

**Acceptance Scenarios**:

1. **Given** a server, **When** the admin assigns one or more roles, **Then** the
   catalog and detail view show it as restricted to those roles.
2. **Given** a restricted server, **When** the admin removes all roles, **Then**
   it is shown as available to all org members.

---

### User Story 5 - Review the audit trail (Priority: P2)

An administrator reviews a chronological, filterable record of configuration and
security events for their org — including access denials — and sees whether the
record's tamper-evident integrity is intact.

**Why this priority**: Required for trust and compliance (SOC 2 posture); read-only
and independent of the management stories.

**Independent Test**: Perform a few admin actions → they appear in the audit table
newest-first; filter by action/actor → the list narrows; the chain-integrity
indicator shows "verified".

**Acceptance Scenarios**:

1. **Given** recent admin activity, **When** the admin opens the audit log,
   **Then** events appear newest-first with time, actor, action, and target.
2. **Given** the audit log, **When** the admin filters by action or actor,
   **Then** only matching events are shown.
3. **Given** access-denial events exist, **When** viewing the log, **Then**
   `auth.denied`/`authz.denied` events are visible and distinguishable.
4. **Given** the audit record, **When** the admin views integrity status, **Then**
   it clearly shows "verified" or "tampered".

---

### User Story 6 - Monitor health & usage and view quotas (Priority: P3)

An administrator monitors server health and request/denial/error trends, views the
organization's configured per-organization and per-user rate limits (read-only),
and copies the end-user connection endpoint to share with their users.

**Why this priority**: Operational awareness and onboarding aid; valuable but not
required for the core management loop.

**Independent Test**: View the dashboard usage widgets → they reflect current
activity; open settings → the configured rate limits are shown; copy the
connection endpoint → the correct per-org URL is copied.

**Acceptance Scenarios**:

1. **Given** activity in the org, **When** the admin views the dashboard, **Then**
   request/denial/error and health indicators reflect recent activity.
2. **Given** the settings area, **When** the admin opens it, **Then** the
   configured per-user and per-org rate limits are displayed read-only (editing is
   out of scope for v1).
3. **Given** the org context, **When** the admin copies the connection endpoint,
   **Then** the correct `{org}` MCP URL is provided.

---

### User Story 7 - Consistent, accessible, on-brand experience (Priority: P3)

Every screen adheres to the supplied Carbon Design System — its tokens,
components, typography (IBM Plex), iconography, and 16-column grid — and meets
accessibility standards across desktop and tablet.

**Why this priority**: A cross-cutting quality bar that makes the console
trustworthy and usable; it constrains all other stories rather than standing
alone.

**Independent Test**: Run the design-system adherence checks → they pass; run an
accessibility audit + keyboard-only walkthrough of each primary flow → they pass;
view at desktop and tablet widths → layouts adapt without loss of function.

**Acceptance Scenarios**:

1. **Given** any screen, **When** it is checked against the design system, **Then**
   it uses only approved tokens/components and passes the adherence check.
2. **Given** keyboard-only navigation, **When** the admin performs a primary task,
   **Then** all controls are reachable and operable with visible focus.
3. **Given** a tablet-width viewport, **When** the admin uses the console, **Then**
   navigation and tables remain usable.

### Edge Cases

- **Empty states**: no servers yet, no audit events, no credentials set — each
  shows a helpful empty state with a clear next action.
- **Large catalogs / logs**: tables paginate, search, and sort without degrading.
- **Duplicate or invalid input**: inline, field-level errors; nothing saved.
- **Downstream/health probe failing**: server shows an "unhealthy/unknown" status
  with detail rather than blocking the page.
- **Destructive actions** (disable, delete, rotate, clear): require explicit
  confirmation and clearly state the effect (e.g., "active sessions will stop").
- **Save/network errors**: surfaced as non-blocking notifications; no false
  success; safe retry.
- **Session expiry / token refresh** mid-task: re-authenticate without losing
  unsaved form context where feasible.
- **Non-admin or wrong-org token**: access denied; never render another org's data.
- **Secret confidentiality**: stored secret values never appear in any view, copy
  action, export, or error message.

## Requirements *(mandatory)*

### Functional Requirements

**Access & navigation**

- **FR-001**: The console MUST authenticate administrators using the existing
  organization identity (per-org realm OAuth) via a **pre-registered public client
  with Authorization Code + PKCE** (a known `client_id` provisioned per realm, not
  dynamic registration), and require the admin role; the organization context MUST
  be derived from the org the admin signs in to.
- **FR-002**: The console MUST refuse access to non-admins and MUST never display
  data belonging to a different organization than the signed-in admin's.
- **FR-003**: The console MUST provide a persistent navigation shell (org/brand
  header, primary side navigation, and profile/sign-out) consistent across screens.
- **FR-004**: The console MUST handle session expiry by prompting re-authentication
  and MUST provide an explicit sign-out.

**Dashboard**

- **FR-005**: The dashboard MUST summarize the org's server count, health
  breakdown, recent activity, and attention indicators (e.g., elevated denials).
  Counts, health, recent activity, and the denial indicator are derived from the
  servers + audit APIs; request/denial/error **rate trends** are sourced from the
  metrics system's query API (Prometheus) — see FR-019.

**Server management**

- **FR-006**: The console MUST list the org's MCP servers with name, type
  (remote/stdio), enabled state, health, and access scope, with search, sort, and
  pagination.
- **FR-007**: The console MUST let admins add a remote server (endpoint URL) and a
  stdio server (command, arguments, environment) via guided, validated forms.
- **FR-008**: The console MUST let admins edit an existing server's configuration.
- **FR-009**: The console MUST let admins enable/disable a server and delete it,
  each behind an explicit confirmation that states the effect (kill-switch).
- **FR-010**: The console MUST validate input with field-level errors and MUST not
  create duplicates within an org (e.g., duplicate slug).

**Credentials**

- **FR-011**: The console MUST let admins choose a credential mode per server:
  none, organization-shared, or per-user.
- **FR-012**: The console MUST let admins set, rotate, and clear credential values,
  and MUST communicate that rotation applies on next use.
- **FR-013**: The console MUST be write-only for secrets: stored values are NEVER
  displayed, copied, exported, or logged; only set/not-set status and last-updated
  time are shown.

**Access control (RBAC)**

- **FR-014**: The console MUST let admins assign the roles permitted to use each
  server and MUST visually distinguish open vs. role-restricted servers.

**Audit**

- **FR-015**: The console MUST present org-scoped audit events newest-first with
  time, actor, action, and target, with filtering and pagination.
- **FR-016**: The console MUST surface access-denial events (`auth.denied`,
  `authz.denied`) and MUST display the audit record's tamper-evident integrity
  status (verified vs. tampered).

**Quotas & operations**

- **FR-017**: The console MUST display the configured per-organization and per-user
  rate limits **read-only** (via a read-only quotas endpoint); editing limits is out
  of scope for v1.
- **FR-018**: The console MUST display the end-user connection endpoint for the org
  with a copy action.
- **FR-019**: The console MUST display request/denial/error **rate trends** sourced
  from the metrics system's **query API** (Prometheus), refreshed periodically;
  server health and counts are sourced from the servers + audit APIs.

**Feedback & design adherence**

- **FR-020**: The console MUST give immediate, non-blocking feedback for every
  action (success/error notifications) and MUST show loading/empty/skeleton states.
- **FR-021**: The console MUST adhere to the supplied Carbon Design System — its
  design tokens (color, type, spacing, motion), components, IBM Plex typography,
  iconography, and 16-column grid — and MUST pass the design system's adherence
  check.
- **FR-022**: The console MUST meet WCAG 2.1 AA, including full keyboard
  operability and visible focus, and MUST be usable at desktop and tablet widths.
- **FR-023**: The console MUST operate through the existing control-plane admin API
  and identity and MUST NOT introduce new **mutating** server-side capabilities or
  bypass authorization. Two narrow **read-only / observability** accommodations are
  permitted (per Clarifications): a read-only quotas endpoint (FR-017) and console
  reachability to the metrics query API (FR-019). Neither mutates state nor changes
  authorization.

### Key Entities *(include if feature involves data)*

- **Administrator (session)**: the signed-in principal — organization, user
  identity, roles; drives what is visible and permitted. Not stored by the console.
- **MCP Server (catalog item)**: name/slug, type (remote/stdio), endpoint or
  command/args/env, enabled state, health status, credential mode, permitted roles.
- **Credential (status only)**: mode and whether set + last-updated — never the
  value.
- **Audit Event**: time, actor, action, target, outcome, and the record's chain
  integrity status.
- **Rate Limit**: per-organization and per-user request allowances.
- **Organization context**: the org identity and its end-user connection endpoint.
- **Role**: an org role usable to scope server access.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new administrator can add a working MCP server (remote or stdio)
  on their first session in under 3 minutes without external documentation.
- **SC-002**: 90% of administrators complete each core task (add server, set
  credential, restrict roles, find an audit event) successfully on the first try.
- **SC-003**: Stored secret values never appear in any screen, copy action, export,
  or error — verified to be 100% absent from what the interface presents.
- **SC-004**: An admin signed in to one organization can reach **zero** data from
  any other organization.
- **SC-005**: Every screen passes the design system's adherence check (approved
  tokens/components only) — 100% of screens.
- **SC-006**: The console meets WCAG 2.1 AA on all primary flows (automated checks
  with no critical violations, plus a successful keyboard-only walkthrough).
- **SC-007**: 95% of administrator actions produce visible feedback within 1 second.
- **SC-008**: The dashboard and server health reflect the current state within 30
  seconds of a change.
- **SC-009**: The console is fully usable at desktop and tablet widths with no loss
  of primary function.
- **SC-010**: Every admin-facing capability of `001-mcp-server-runtime` (per the
  scope table above) is reachable in the console.

## Assumptions

- **Design system**: The Carbon Design System provided at the project root
  (handoff package) is the single source of visual truth — its tokens, components
  (Button, Tag, Tile, InlineNotification, ProgressBar, Checkbox, Search, Select,
  TextInput, Toggle, Tabs), icon set, IBM Plex typography, 16-column grid, and the
  included cloud-console UI-kit (shell + dashboard + resource table + catalog) as
  the layout reference. Its adherence checks are authoritative.
- **Backend reuse**: The console is a client of the existing control-plane admin
  API and identity from `001` (Keycloak per-org realm OAuth, admin role,
  audience-bound). No new **mutating** backend or schema is introduced; two narrow
  read-only/observability accommodations are accepted (per Clarifications): a
  read-only quotas endpoint and reachability to the metrics query API.
- **Identity/client**: The console is a **pre-registered public OAuth client**
  (a known `client_id` such as `mcp-admin-console`, Authorization Code + PKCE)
  provisioned in each org's realm — not dynamically registered.
- **Quotas**: View-only in v1 — the configured per-org/per-user limits are
  displayed; editing is deferred (no per-org quota store exists in `001`).
- **Metrics**: Request/denial/error rate trends come from the metrics system's
  query API (Prometheus), reachable from the console via the edge same-origin proxy
  or CORS; health/counts/denials come from servers + audit.
- **Audience**: Administrators only. End users connect through MCP clients, not
  this console; broad end-user self-service is out of scope for v1 (a signed-in
  admin may still manage their own per-user credential).
- **Org context**: Resolved the same way as the data plane (the organization the
  admin authenticates to), preserving organization isolation.
- **Secret handling**: Write-only, consistent with `001` (FR-015) — the console can
  set/rotate/clear but never read secret values.
- **Real-time**: Health and usage are refreshed periodically (polling); live
  push/streaming is not required for v1.
- **Devices**: Desktop and tablet are supported; mobile phone is best-effort.
- **Hard constraints inherited from `001`**: The console MUST NOT weaken
  organization isolation or secret confidentiality; these outrank any convenience
  or aesthetic consideration.
