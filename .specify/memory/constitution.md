<!--
Sync Impact Report
- Version change: (unset template) → 1.0.0
- Type: initial ratification (template placeholders replaced with concrete principles)
- Principles defined:
  I.   Tenant Isolation Is Inviolable
  II.  Secure Execution of Any MCP
  III. Frictionless Self-Service
  IV.  Hard Constraints Outrank Soft Goals
  V.   Test-First, Adversarially Verified (NON-NEGOTIABLE)
  VI.  Observable & Auditable by Default
  VII. Simplicity with Justified Complexity
- Added sections: Core Principles (I–VII); Security & Compliance Standards;
  Development Workflow & Quality Gates; Governance
- Removed sections: none (template placeholders filled)
- Template alignment:
  ✅ .specify/templates/plan-template.md — Constitution Check gate present and already
     exercised by specs/001-mcp-server-runtime/plan.md
  ✅ .specify/templates/spec-template.md — no new mandatory spec section introduced; compatible
  ⚠ .specify/templates/tasks-template.md — template marks tests OPTIONAL, but Principle V
     requires contract + adversarial tests for any isolation/authz/sandbox/secrets work.
     /speckit-tasks MUST emit those test tasks for this project.
  ✅ CLAUDE.md — agent guidance references the active plan; no conflict introduced
- Deferred TODOs: none (RATIFICATION_DATE set to 2026-06-13; amend if an earlier adoption date applies)
-->

# Willow MCP Gateway Constitution

## Core Principles

### I. Tenant Isolation Is Inviolable

Organization isolation is the platform's primary, non-negotiable guarantee. No code path, query,
cache entry, token, sandbox, or log MAY allow one organization to enumerate, reach, or read
another's data, servers, sessions, or secrets. Every tenant-owned record MUST carry `org_id` and be
scoped at both the application layer and the datastore (e.g., row-level security); cross-tenant
access MUST fail closed (deny and audit), never fall through. Access tokens MUST be audience-bound to
a single organization's endpoint.

Rationale: a cross-org breach is the catastrophic failure mode of a multi-tenant gateway and would
destroy the product's core trust.

### II. Secure Execution of Any MCP

The platform MUST run any MCP server — including arbitrary, unvetted, potentially hostile code
(e.g., `npx -y <pkg>`) — without endangering the platform or other tenants. Untrusted execution MUST
occur inside a hardware-isolated sandbox (microVM) with default-deny network egress; enforced
CPU/memory/PID/disk limits; an ephemeral filesystem; and no path to platform secrets, the control
plane, internal networks, or cloud metadata. A faulty or malicious server MUST be contained to its
own boundary and MUST NOT destabilize the gateway or other tenants.

Rationale: "support any MCP without concerns" is a product promise; it is only safe if hostile code
is assumed and contained by design.

### III. Frictionless Self-Service

Administrators MUST be able to add, govern, and remove MCP servers themselves — no engineering
involvement, no redeploy — with changes taking effect within seconds. Common-path features MUST NOT
introduce manual provisioning, ticket queues, or restarts. Client onboarding MUST support
self-service (dynamic client registration plus PKCE) rather than manual client provisioning.

Rationale: adoption depends on admins moving at their own speed; friction here defeats the product.

### IV. Hard Constraints Outrank Soft Goals

The hard constraints — tenant isolation, frictionless self-service, and secure execution of any MCP
— MUST NOT be weakened to improve a soft goal (user-level isolation, scalability, performance, or
cost). When a hard and a soft goal conflict, the hard constraint wins and the trade-off MUST be
documented in the plan's Complexity Tracking. Spending more (cost) or accepting latency
(performance) to uphold a hard constraint is explicitly permitted.

Rationale: an explicit ranking keeps architecture decisions principled and prevents silent erosion
of the guarantees that matter most.

### V. Test-First, Adversarially Verified (NON-NEGOTIABLE)

Contract tests MUST exist for every external interface. For any change touching isolation,
authorization, sandboxing, or secrets, an adversarial/negative test MUST be written and MUST fail
before the implementation lands. Isolation and security guarantees MUST be proven by automated
tests, never merely asserted in review.

Rationale: isolation cannot be retrofitted or trusted to manual inspection; the only durable proof
is a failing-then-passing adversarial test.

### VI. Observable & Auditable by Default

Every configuration change and security-relevant event MUST produce a tamper-evident audit record
retained for at least one year. All services MUST emit structured logs, metrics, and traces;
per-organization and per-server health MUST be visible to admins; and a misbehaving server MUST be
disableable within seconds. Secrets MUST NEVER appear in logs, traces, or responses.

Rationale: operability and the SOC 2 baseline both require that what happened can be seen,
attributed, and proven after the fact.

### VII. Simplicity with Justified Complexity

Designs MUST start simple and apply YAGNI. Any added complexity — an extra service, a heavier
runtime, a new datastore — MUST be justified against a specific hard constraint and recorded in the
plan's Complexity Tracking; unjustified complexity MUST be removed.

Rationale: complexity is a cost paid forever, acceptable only where a hard constraint genuinely
demands it (for example, microVMs for Principle II).

## Security & Compliance Standards

- Compliance baseline: SOC 2 (Type II). GDPR and HIPAA are out of scope until the platform serves
  EU data subjects or healthcare PHI, respectively; no hard data-residency requirement until then.
- Encryption: all data in transit and at rest MUST be encrypted. Downstream credentials MUST be
  stored in a secrets manager (envelope-encrypted), injected only at runtime, and isolated per
  sandbox.
- Network: sandboxes run default-deny egress with an explicit allowlist; link-local metadata,
  internal ranges, and the control plane MUST be unreachable from any sandbox.
- Identity: a central identity broker (per-organization realm) is the authorization server; the
  gateway is an OAuth 2.0 protected resource server that validates audience-bound tokens on every
  request.
- Audit: configuration and security events are append-only, tamper-evident, and retained ≥ 1 year.

## Development Workflow & Quality Gates

- Every implementation plan MUST pass the Constitution Check gate before and after design;
  violations MUST be justified in Complexity Tracking or the design MUST change.
- Code review MUST explicitly verify the isolation and authorization invariants for any change that
  could affect them; a reviewer MUST be able to point to the test that proves each invariant.
- No change MAY merge if it regresses a hard constraint or removes/weakens an adversarial test
  without an approved, documented justification.
- The adversarial security suite and contract tests MUST pass in CI; the `quickstart.md` isolation
  walkthrough is the acceptance gate for releases.
- Changes MUST be committed in small, reviewable increments with clear messages.

## Governance

This constitution supersedes other practices where they conflict.

Amendments: proposed via pull request describing the change, its rationale, and impact; require
maintainer approval. On merge, the version and Last Amended date MUST be updated and dependent
templates re-synced.

Versioning policy (semantic): MAJOR for backward-incompatible governance/principle removals or
redefinitions; MINOR for a new principle/section or materially expanded guidance; PATCH for
clarifications and non-semantic refinements.

Compliance review: every plan's Constitution Check and every PR review verify adherence; recurring
violations trigger an amendment or a remediation task. Runtime development guidance for agents lives
in `CLAUDE.md`.

**Version**: 1.0.0 | **Ratified**: 2026-06-13 | **Last Amended**: 2026-06-13
