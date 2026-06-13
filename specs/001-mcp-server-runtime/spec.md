# Feature Specification: Multi-Tenant MCP Server Runtime

**Feature Branch**: `001-mcp-server-runtime`
**Created**: 2026-06-13
**Status**: Draft
**Input**: User description: "We are running an MCP Gateway with an admin panel (manage MCP servers, RBAC, more). Every end user connects to the org's runtime {org}.withwillow.ai/mcp, authorized via OAuth. Admins must be able to add remote (HTTP) MCP servers or stdio servers, including their own (e.g. {command: 'npx', args: ['-y', '@modelcontextprotocol/server-sequential-thinking']}). Keep in mind: (1) org isolation, (2) user isolation, (3) frictionless admin experience, (4) security — support any MCP without concerns, (5) scalability to 100,000+ active users, (6) performance, (7) cost."

## Overview

The MCP Gateway lets each organization expose a single, OAuth-authorized runtime endpoint (`{org}.withwillow.ai/mcp`) through which its end users reach a curated set of MCP (Model Context Protocol) servers. Administrators register MCP servers of two kinds — **remote HTTP** servers (an external MCP endpoint) and **stdio** servers (a command + arguments the gateway runs on the user's behalf, e.g. `npx -y @modelcontextprotocol/server-sequential-thinking`) — and govern who may use them via RBAC. The gateway aggregates the permitted servers behind the one endpoint and proxies the full MCP surface (tools, resources, prompts, notifications) to the user's client.

This feature defines the runtime that registers, runs, isolates, and proxies those servers. Not all of its goals carry equal weight: **organization isolation, a frictionless admin experience, and the ability to securely run any MCP are hard constraints**, while **user-level isolation, scalability, performance, and cost are best-effort goals** optimized within those constraints (see Constraints & Trade-off Priorities below).

## Constraints & Trade-off Priorities

The seven stated concerns are deliberately ranked. The following are **hard constraints (MUST be met)** — the design may not ship without them, and may sacrifice the soft goals below to uphold them:

- **HC-1 — Organization isolation** (concern 1): No organization can ever enumerate, reach, or read another organization's data, servers, sessions, or secrets.
- **HC-2 — Frictionless admin experience** (concern 3): Admins add MCP servers themselves, with no engineering work and no platform redeploy, and the servers become usable within seconds.
- **HC-3 — Security / support any MCP** (concern 4): The platform safely runs *any* MCP server, including arbitrary untrusted code, without the operator having to vet or worry about it. A faulty or malicious server cannot break isolation, harm the platform, or affect other tenants.

The remaining concerns are **soft goals (best-effort; optimized, but may be traded off to satisfy the hard constraints)**:

- **SG-1 — User-level isolation** (concern 2): Strongly desired and pursued by the same mechanisms, but not a release gate. Organization isolation is the guaranteed boundary.
- **SG-2 — Scalability to 100k+ users** (concern 5): Target capacity the architecture should scale toward; may be approached incrementally.
- **SG-3 — Performance** (concern 6): Latency figures are targets, not guarantees. It is acceptable to trade some latency (e.g., sandbox cold-start) for the hard security constraint.
- **SG-4 — Cost** (concern 7): Optimize cost, but spending more is acceptable where the hard constraints require it (e.g., stronger isolation).

This ranking explicitly authorizes the planning phase to choose, for example, stronger per-execution sandboxing (HC-3) even at higher cost (SG-4) and some added latency (SG-3).

## Clarifications

### Session 2026-06-13

- **Constraint priority**: Hard constraints that MUST be met are organization isolation (1), frictionless admin experience (3), and security/support-any-MCP (4). User isolation (2), scalability (5), performance (6), and cost (7) are best-effort goals that may be traded off to satisfy the hard constraints. (See Constraints & Trade-off Priorities.)
- **Q1 — Untrusted-code execution scope (FR-013)**: Resolved by HC-3. The platform MUST safely run arbitrary, untrusted MCP server code with full hostile-tenant containment; a vetted-catalog-only model is not sufficient on its own.
- **Q2 — Downstream credential model (FR-016)**: Because user isolation is a soft goal and a frictionless admin experience is hard, the default is admin/org-provided (org-level) credentials, with per-user credentials offered as an optional capability for organizations that want user-level downstream isolation.
- Q: Identity & authentication model — federate to org IdPs only, platform-managed only, or a central broker? → A: Use a **central identity broker** — platform-managed accounts by default, with optional per-organization federation/SSO (OIDC/SAML) to the org's own IdP; identities are isolated per organization (e.g., a per-organization realm). The specific broker product is a plan-level decision.
- Q: Which compliance/regulatory regimes must the platform satisfy? → A: **SOC 2 (Type II)** as the baseline (formal audit logging, access controls, change management, encryption in transit and at rest); GDPR and HIPAA are out of scope for the initial release.
- Q: MCP transport compatibility scope — Streamable HTTP only, also legacy HTTP+SSE, or mixed? → A: **Both stdio and remote HTTP are first-class, fully supported server types.** Remote HTTP servers and client connections use the current MCP **Streamable HTTP** transport; **legacy HTTP+SSE** compatibility is deferred (not a priority for the initial release).
- Q: How should authentication/authorization be designed? → A: Align to the **MCP authorization specification** with **Keycloak** as the OAuth 2.0/OIDC authorization server (central identity broker), per Keycloak's *MCP authorization server* guidance (https://www.keycloak.org/securing-apps/mcp-authz-server). The MCP endpoint is an OAuth 2.0 **protected resource server** that accepts only audience-bound tokens issued for it; clients use OAuth 2.1 Authorization Code + **PKCE**, with **Protected Resource Metadata (RFC 9728)**, **Authorization Server Metadata discovery (RFC 8414)**, and **Dynamic Client Registration (RFC 7591)**. Identities are brokered per-org via Keycloak realms. Baseline MCP auth revision 2025-03-26; for 2025-06-18/2025-11-25, audience binding is enforced via Keycloak audience-mapped client scopes (e.g., `mcp:tools`, `mcp:resources`, `mcp:prompts`), since Keycloak does not natively process RFC 8707 resource indicators. Keycloak-specific configuration is a plan-level detail.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - End user reaches and uses their tools through one endpoint (Priority: P1)

An end user points their MCP client at `{org}.withwillow.ai/mcp`, authorizes via OAuth, and immediately sees and can invoke the tools/resources/prompts from every MCP server the admin has enabled for them — without knowing or configuring anything about the underlying servers.

**Why this priority**: This is the product's core value loop and the single endpoint every user depends on. Without it nothing else matters; with it alone the gateway already delivers value for at least one pre-configured server.

**Independent Test**: With one server pre-configured and granted to a user, connect a standard MCP client, complete OAuth, list the aggregated tools, invoke one tool, and confirm a correct result is returned through the gateway.

**Acceptance Scenarios**:

1. **Given** an authorized user with at least one enabled server, **When** their client connects to the org endpoint, **Then** the aggregated list of permitted tools/resources/prompts is returned within the performance target.
2. **Given** a connected user, **When** they invoke a tool, **Then** the request is routed to the correct downstream server and the response (including streamed/partial output) is returned faithfully.
3. **Given** a user who is not authorized (no valid OAuth grant), **When** they attempt to connect, **Then** the connection is rejected and no server surface is exposed.
4. **Given** tools with identical names from two different servers, **When** the user lists tools, **Then** both are presented unambiguously (no collision, no silent overwrite).

---

### User Story 2 - Admin adds a remote HTTP MCP server with no redeploy (Priority: P1)

An admin opens the admin panel, enters a remote MCP server's URL (and any required authentication), saves it, optionally scopes it via RBAC, and the server becomes available to permitted users within seconds — with no engineering work, deploy, or restart of unrelated servers.

**Why this priority**: Frictionless add is a hard constraint (HC-2), and remote HTTP is the simplest, lowest-risk server type, making it the fastest path to a usable multi-server gateway.

**Independent Test**: Add a reachable remote MCP endpoint through the admin panel; confirm a permitted user sees its tools within the target time and can call one, while other servers and users are unaffected.

**Acceptance Scenarios**:

1. **Given** a valid remote MCP endpoint, **When** the admin saves it, **Then** it is health-checked and its status (healthy/unreachable/auth-failed) is shown to the admin.
2. **Given** a newly added healthy server, **When** a permitted user reconnects or refreshes, **Then** its tools appear without any service redeploy.
3. **Given** an unreachable or failing remote server, **When** users connect, **Then** that server's failure is isolated and does not break the user's session or other servers' tools.

---

### User Story 3 - Admin adds a stdio MCP server defined by a command (Priority: P1)

An admin registers a stdio server by providing `{command, args, env}` (e.g. `npx -y @modelcontextprotocol/server-sequential-thinking`). The gateway runs that server in an isolated execution environment on the user's behalf and bridges its stdio transport to the user's connection, exactly as if it were a remote server.

**Why this priority**: stdio and remote HTTP are both first-class server types; stdio is the capability that most fully exercises "support any MCP" (HC-3) by running arbitrary third-party code, and it is the principal driver of the hard security and containment requirements. Remote HTTP (US2) is the simpler slice to deliver, but both are core.

**Independent Test**: Register `npx -y @modelcontextprotocol/server-sequential-thinking`; confirm a permitted user can list and successfully call its tool, and that the spawned process is confined to its isolation boundary.

**Acceptance Scenarios**:

1. **Given** a valid stdio command, **When** the admin saves it, **Then** the gateway can launch it, complete the MCP handshake, and report the server healthy.
2. **Given** a stdio server in use, **When** the user invokes its tools, **Then** stdio messages are bridged bidirectionally (including streaming) with correct ordering.
3. **Given** a stdio command that fails to start or hangs, **When** the startup timeout elapses, **Then** the server is marked unhealthy, the user is informed, and no resources are leaked.
4. **Given** an idle stdio server, **When** it remains unused beyond the idle threshold, **Then** its process/resources are reclaimed and transparently restarted on next use.

---

### User Story 4 - Organization isolation is guaranteed; user isolation is best-effort (Priority: P1)

As the platform operator and as an admin entrusting sensitive data to the gateway, every request, running server, session, and stored secret is confined to its organization so that no tenant can observe or reach another's servers, data, sessions, or credentials. Within an organization, the platform additionally strives to keep one user's data and sessions private from other users.

**Why this priority**: Organization isolation is a hard, non-negotiable constraint (HC-1); a breach is catastrophic and unshippable, so it must hold from the first release. User-level isolation (SG-1) is strongly desired and pursued by the same mechanisms, but it is an optimization goal rather than a release gate.

**Independent Test**: Provision two orgs each with two users; attempt cross-org access (must always fail) and cross-user access (should fail) via the API and via a deliberately hostile server; confirm cross-org attempts are denied with zero leakage and hostile servers are contained to their tenant.

**Acceptance Scenarios**:

1. **(HARD)** **Given** two organizations, **When** a user of org A issues any request, **Then** they can never enumerate, reach, or receive data from org B's servers, configs, secrets, or sessions.
2. **(HARD)** **Given** a downstream server that attempts to read another tenant's data, the platform's secrets, internal network/metadata endpoints, or other servers, **When** it executes, **Then** all such attempts are blocked and contained to its own boundary.
3. **(HARD)** **Given** an admin revokes a user's access to a server, **When** the user next invokes a tool from it, **Then** the request is denied promptly even within an existing session.
4. **(BEST-EFFORT)** **Given** two users in the same org, **When** one user is active, **Then** the platform aims to ensure they cannot observe or hijack another user's session, in-flight requests, results, or per-user credentials.

---

### User Story 5 - Admin governs access with RBAC (Priority: P2)

An admin assigns which roles or users may see and use which servers (and, where supported, which tools), and changes take effect promptly across new and existing sessions.

**Why this priority**: Governance is required for real organizations and underpins both organization isolation (HC-1) and best-effort user isolation, but it builds on the core connect/add loop and can follow the first usable release.

**Independent Test**: Grant a server to one role and not another; confirm only members of the granted role see it; revoke it and confirm it disappears for affected users on next list/call.

**Acceptance Scenarios**:

1. **Given** a server scoped to role X, **When** a user with role X connects, **Then** they see it; **When** a user without role X connects, **Then** they do not.
2. **Given** a permission change, **When** it is saved, **Then** it is enforced for subsequent requests without requiring a redeploy.

---

### User Story 6 - Secrets and downstream credentials are managed securely (Priority: P3)

Servers that need credentials (API keys, tokens, downstream OAuth) receive them at runtime from secure storage. The default model is admin/org-provided credentials; per-user credentials are offered for organizations that want user-level downstream isolation. In all cases secrets are never exposed to unauthorized users, other servers, or logs.

**Why this priority**: Many useful servers need secrets, but the gateway is usable for credential-free servers first; the org-level default keeps the experience frictionless (HC-2) and the per-user option serves best-effort user isolation (SG-1).

**Independent Test**: Store a secret for a server; confirm a permitted user's session can use it, an unpermitted user cannot, the value never appears in logs or to other servers, and rotation takes effect on next start.

**Acceptance Scenarios**:

1. **Given** an org-level stored secret, **When** a server runs for a permitted user, **Then** the secret is injected at runtime and is not readable by other users, other servers, or logs.
2. **Given** a server configured for per-user credentials, **When** a user without their own credential connects, **Then** they are prompted/blocked appropriately rather than silently using another user's credential.
3. **Given** a rotated secret, **When** the server is next started, **Then** it uses the new value with no downtime for unrelated servers.

---

### User Story 7 - Admin observes health and can stop a misbehaving server (Priority: P3)

An admin sees per-server health, usage, and audit information and can instantly disable or restart a server that is failing, abusive, or compromised, without affecting other servers or tenants.

**Why this priority**: Operational visibility and a kill-switch are important for safely running arbitrary MCPs (HC-3) at scale, but are not required to demonstrate the core value loop.

**Independent Test**: View a server's health/usage; trigger a disable; confirm the server becomes unavailable to users immediately while other servers continue working, and the action is audited.

**Acceptance Scenarios**:

1. **Given** a running server, **When** the admin disables it, **Then** it stops being offered to users and in-flight use is terminated cleanly within the target time.
2. **Given** a server consuming excessive resources, **When** it exceeds its quota, **Then** it is throttled or stopped and the admin is alerted, with no impact on other tenants.

---

### Edge Cases

- **Downstream server down/slow/erroring**: failure is isolated to that server; the user's session and other servers keep working; health reflects the state.
- **stdio cold start vs. warm reuse**: first use of a rarely-used server incurs a bounded startup delay; frequently used servers stay warm.
- **Hostile server behavior** (hard constraint HC-3): attempts at network egress to internal/metadata endpoints, reading other tenants' or the platform's secrets, excessive CPU/memory/disk, fork/spawn abuse, or data exfiltration are contained and rate-limited/killed.
- **Supply-chain risk on stdio**: a command that pulls a malicious or unexpected package is constrained by execution sandboxing and egress control (and, where applicable, pinning/allowlisting).
- **Tool/resource name collisions** across servers are disambiguated (namespacing) with no silent overwrite.
- **Large or unbounded streaming output**: subject to backpressure and size/time limits; never exhausts gateway memory.
- **OAuth token expiry mid-session**: handled with renewal/re-auth without losing in-flight context where possible.
- **Mid-session access revocation**: enforced on the next request, not only at connect time.
- **Noisy neighbor** (best-effort): one org or user should not exhaust shared capacity; per-tenant/per-user quotas and rate limits apply.
- **Secret rotation while running**: applied on next start; no cross-server downtime.
- **Org offboarding / subdomain reuse**: all servers, sessions, secrets, and data are reclaimed and purged.
- **Duplicate or conflicting server registrations**: validated and rejected or disambiguated at save time.

## Requirements *(mandatory)*

Each requirement is tagged with the hard constraint (HC-1/2/3) or soft goal (SG-1/2/3/4) it primarily serves. "MUST" denotes a hard requirement; "SHOULD" denotes a best-effort requirement that may be traded off to satisfy a MUST.

### Functional Requirements

**Connectivity & protocol (baseline)**

- **FR-001**: System MUST expose one MCP endpoint per organization at `{org}.withwillow.ai/mcp` that speaks the MCP protocol to standard MCP clients. *(HC-1)*
- **FR-002**: System MUST authorize every connection by validating an OAuth 2.0/OIDC access token before exposing any server surface, acting as an OAuth 2.0 **protected resource server** that delegates authentication to a central identity broker (the authorization server), and MUST resolve each token to a specific organization, user, and roles. The broker issues platform-managed accounts by default and federates to an organization's own IdP (OIDC/SAML SSO) when configured; identities are isolated per organization (e.g., a per-organization realm). *(HC-1)*
- **FR-003**: System MUST aggregate all servers a user is permitted to use behind the single endpoint, presenting their tools, resources, and prompts as one unified surface with collision-safe namespacing. *(baseline)*
- **FR-004**: System MUST proxy the full MCP interaction bidirectionally — including tool calls, resource/prompt access, streaming/partial responses, and notifications — faithfully between client and downstream server. *(baseline)*
- **FR-005**: System MUST support **remote HTTP** MCP servers identified by a URL plus optional authentication, using the current MCP Streamable HTTP transport; legacy HTTP+SSE compatibility is deferred to a future release. *(baseline)*
- **FR-006**: System MUST support **stdio** MCP servers defined by `{command, args, env}`, running the command on the user's behalf and bridging its stdio transport to the user's connection. *(HC-3)*

**Administration & configuration**

- **FR-007**: Admins MUST be able to add, edit, enable/disable, and remove server configurations through the admin panel, with changes taking effect without redeploying the platform or disrupting unrelated servers. *(HC-2)*
- **FR-008**: System MUST validate and health-check a server when it is added or changed, and surface its status (e.g., healthy, unreachable, auth-failed, startup-failed) to the admin. *(HC-2)*
- **FR-009**: System MUST let admins scope each server (and, where supported, its tools) to roles/users via RBAC, and MUST enforce those permissions on every request. *(HC-1)*
- **FR-010**: System MUST record a tamper-evident, access-controlled audit trail of configuration changes and security-relevant events (who added/changed/removed/disabled what, and when), retained per the audit-retention assumption and sufficient to support SOC 2 (Type II) controls. *(HC-3)*

**Isolation & security**

- **FR-011**: System MUST guarantee organization isolation: no user, server, session, or stored datum of one organization can be enumerated, reached, or read by another organization. *(HC-1)*
- **FR-012**: System SHOULD uphold user isolation: one user's session, in-flight requests, results, and per-user state/credentials should not be observable or reachable by another user. *(SG-1 — best-effort)*
- **FR-013**: System MUST safely run arbitrary, untrusted MCP server code (e.g., a command such as `npx -y <package>`) in an isolated execution environment such that a faulty or malicious server cannot access other tenants' data, the platform's secrets or control plane, or other servers, and its blast radius is contained to its own environment. Supporting any MCP — including code the operator has not vetted — is required; a vetted-catalog-only approach is not sufficient as the sole mechanism. *(HC-3)*
- **FR-014**: System MUST restrict each running server's network egress, filesystem, and resource access by default — specifically denying access to internal infrastructure, cloud metadata endpoints, and other tenants' or the platform's resources. *(HC-3)*
- **FR-015**: System MUST store all secrets encrypted, inject them into servers only at runtime, and never expose them to unauthorized users, to other servers, or in logs. *(HC-3)*
- **FR-016**: System MUST allow an admin to provide the credentials a server needs at the organization level (the default, frictionless model), and SHOULD support per-user credentials for organizations that require user-level downstream isolation; in all cases credentials are handled per FR-015. *(HC-2 for the default; SG-1 for the per-user option)*
- **FR-017**: System SHOULD enforce per-organization and per-user resource quotas and rate limits so no tenant or user degrades service for others (noisy-neighbor protection). Note: preventing a single server from crashing the host or escaping its limits is part of FR-013/FR-014 (hard); fair-share quality-of-service is best-effort. *(SG-2/SG-3/SG-4)*

**Lifecycle, performance & reliability**

- **FR-018**: System SHOULD start, stop, recycle, and reclaim server instances on demand, releasing resources from idle servers and transparently restoring them on next use. *(SG-4)*
- **FR-019**: System MUST isolate downstream failures (crash, timeout, error, slow start) so that one server's failure does not break the user's session or other servers' availability. *(HC-3)*
- **FR-020**: System MUST apply startup, idle, and request timeouts and enforce output size/time limits with backpressure to protect gateway stability against any server. *(HC-3)*
- **FR-021**: System SHOULD scale horizontally to sustain the target active-user and concurrent-session load (see Success Criteria) without service degradation. *(SG-2)*
- **FR-022**: System MUST enforce access-control changes (grants/revocations, disables) promptly on existing sessions, not only at connection time. *(HC-1)*

**Authorization (MCP authorization specification)**

- **FR-023**: The MCP endpoint MUST act as an OAuth 2.0 protected resource server and accept only access tokens issued for it, validating each token's audience binding so a token minted for one organization or resource cannot be replayed against another. *(HC-1)*
- **FR-024**: System MUST support MCP authorization discovery — publishing OAuth 2.0 Protected Resource Metadata (RFC 9728) and relying on Authorization Server Metadata discovery (RFC 8414) — so standard MCP clients can locate the authorization server and required scopes without manual configuration. *(HC-2)*
- **FR-025**: System MUST support the OAuth 2.1 Authorization Code flow with PKCE for public MCP clients and Dynamic Client Registration (RFC 7591), so users can connect without an admin pre-registering each client. *(HC-2)*

### Key Entities *(include if feature involves data)*

- **Organization (Tenant)**: The primary, guaranteed isolation boundary; owns a subdomain/endpoint, members, server configurations, secrets, and usage. No data crosses this boundary.
- **User**: A member of an organization with one or more roles; the best-effort secondary isolation boundary. The user's identity is either platform-managed or federated from the organization's IdP through the central identity broker, and is scoped to (isolated within) its organization.
- **Role / Permission Binding**: Maps users to allowed actions over servers (and, where supported, individual tools).
- **MCP Server Definition**: A registered server owned by an organization; type (`remote_http` or `stdio`), connection details (URL/auth or command/args/env), enabled state, RBAC scope, credential model (org-level or per-user), and health/status.
- **Credential / Secret**: Sensitive material associated with a server at the org level (default) and/or per user (optional); stored encrypted and injected only at runtime.
- **Server Instance / Runtime Session**: A running or connected instance of a server bound to an organization (and possibly a specific user), with lifecycle state (starting, healthy, idle, stopped, failed).
- **User Connection / Session**: An authorized MCP client connection carrying the resolved org/user/roles and the aggregated, permitted server surface.
- **Tool / Resource / Prompt**: The MCP capabilities exposed to users, namespaced by their source server.
- **Audit Event**: A record of configuration and security-relevant actions.
- **Usage / Quota Record**: Per-tenant and per-user metering used for rate limiting, noisy-neighbor protection, and cost accounting.

## Success Criteria *(mandatory)*

Criteria are grouped by concern and marked **HARD CONSTRAINT** (must be verified before release) or **TARGET** (best-effort goal; may be traded off to satisfy a hard constraint).

### Isolation — organizations (concern 1) · HARD CONSTRAINT

- **SC-001**: 100% of cross-organization access attempts are blocked; security testing and audits find zero instances of one org's data, configuration, secrets, or sessions being exposed to another.

### Security / support any MCP (concern 4) · HARD CONSTRAINT

- **SC-002**: In adversarial testing, a deliberately hostile server (including arbitrary untrusted code) cannot read other tenants' data, the platform's secrets, internal/metadata endpoints, or other servers; every such attempt is contained to its own boundary with no lateral movement.
- **SC-003**: Any MCP server the admin configures — remote HTTP or arbitrary stdio command — can be run without the operator pre-vetting its code, and a faulty/malicious one cannot destabilize the gateway or other tenants.
- **SC-004**: A misbehaving or abusive server can be disabled by an admin and stops serving users within 5 seconds, with no impact on other servers or tenants.

### Frictionless experience (concern 3) · HARD CONSTRAINT

- **SC-005**: An admin can add a new server and have it usable by a permitted user in under 2 minutes end-to-end, with no engineering involvement and no platform redeploy; median time from save to availability is under 60 seconds.
- **SC-006**: At least 95% of server additions succeed on the first attempt without requiring support intervention, with clear status feedback when they do not.

### Isolation — users (concern 2) · TARGET (best-effort)

- **SC-007**: Cross-user access attempts within an org are blocked in the supported configurations; no user can observe or hijack another user's session, results, or per-user credentials.

### Scalability (concern 5) · TARGET (best-effort)

- **SC-008**: The system scales toward 100,000 active users and at least 10,000 concurrent MCP sessions (assumed peak; see Assumptions) while continuing to meet the performance targets, and scales horizontally beyond this without architectural change.
- **SC-009**: No single organization or user can consume more than its allotted share of capacity; load from one tenant does not degrade latency for others beyond defined limits.

### Performance (concern 6) · TARGET (best-effort)

- **SC-010**: Users see their available tools within 2 seconds of connecting (p95), measured at target peak load.
- **SC-011**: The gateway adds no more than 150 ms of overhead to a tool call (p95) beyond the downstream server's own response time.
- **SC-012**: First use of a cold stdio server becomes available within 5 seconds (p95); a warm/reused server becomes available within 500 ms (p95).
- **SC-013**: The org endpoint is available 99.9% of the time on a monthly basis.

### Cost (concern 7) · TARGET (best-effort)

- **SC-014**: Idle or unused configured servers incur near-zero ongoing runtime cost; total infrastructure cost tracks concurrent usage rather than the number of registered users or configured servers.
- **SC-015**: Per-active-user monthly infrastructure cost stays at or below the business target [target to be confirmed; see Assumptions] and does not grow as more idle servers are configured.

## Assumptions

- **Connection model & authorization standard**: End users connect with standard MCP clients over the current MCP Streamable HTTP transport to `{org}.withwillow.ai/mcp`. Authorization follows the MCP authorization specification: the endpoint is an OAuth 2.0 protected resource server and **Keycloak** is the OAuth 2.0/OIDC authorization server (central identity broker), aligned to Keycloak's *MCP authorization server* guidance (https://www.keycloak.org/securing-apps/mcp-authz-server). Keycloak provides platform-managed accounts by default and federates to an organization's own IdP (OIDC/SAML SSO) when configured, with identities isolated per organization via per-org realms. Keycloak-specific configuration (audience-mapped client scopes, client/registration policies) is a plan-level detail. The existing admin panel, RBAC model, and org/subdomain provisioning already exist and are reused or extended.
- **Who adds servers**: By default only admins may register servers or stdio commands; this is governed by RBAC and may be delegated.
- **Meaning of "their own" servers**: Admins may register servers they operate, exposed either as a reachable remote HTTP URL or as a stdio command the gateway runs. Connecting into private/air-gapped networks via a tunnel or installed agent is out of scope for the initial release.
- **stdio statefulness**: stdio servers are treated as ephemeral processes scoped to a session/use; durable local state across sessions is not guaranteed unless the server itself connects to external storage.
- **Network egress default**: Sandboxed servers run under default-deny egress (internal/metadata blocked) with only required external access permitted — part of the hard security constraint (HC-3).
- **Compliance baseline & audit retention**: The platform targets SOC 2 (Type II) — formal audit logging, access controls, change management, and encryption in transit and at rest. Audit and security-event logs are retained for at least 1 year (assumed; adjustable per auditor guidance). GDPR and HIPAA are out of scope for the initial release and would be added before serving EU data subjects or healthcare PHI respectively. No hard data-residency requirement is assumed for v1 (single-region operation is acceptable; regional pinning is a future enhancement).
- **Transport scope**: Both stdio servers (FR-006) and remote HTTP servers (FR-005) are first-class, fully supported server types. Client connections and remote downstream servers use the current MCP Streamable HTTP transport; legacy HTTP+SSE compatibility is deferred to a future release.
- **Concurrency target**: "100,000 active users" is interpreted as registered active users with an assumed peak concurrency of roughly 10% (~10,000 simultaneous MCP sessions). This is a best-effort target (SG-2); exact peak concurrency should be confirmed.
- **Cost target**: A specific per-active-user cost ceiling is a business input; SC-015 uses it as a placeholder target to be confirmed. Cost is a soft goal (SG-4) and may be exceeded where the hard constraints require it.
- **Trade-off authority**: Per Constraints & Trade-off Priorities, the planning phase may sacrifice user isolation, scalability, performance, or cost where necessary to guarantee organization isolation, frictionless admin experience, and secure execution of any MCP.
- **Solution comparison**: This document specifies *what* the runtime must achieve and the criteria it is judged against. The comparison of candidate architectures (e.g., on-demand isolated sandboxes vs. shared multi-tenant worker pools vs. serverless execution vs. remote-only connectors) is performed in the planning phase against these success criteria and constraint priorities, not in this spec.
