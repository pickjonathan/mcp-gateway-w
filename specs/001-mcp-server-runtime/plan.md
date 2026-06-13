# Implementation Plan: Multi-Tenant MCP Server Runtime

**Branch**: `001-mcp-server-runtime` | **Date**: 2026-06-13 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-mcp-server-runtime/spec.md`

## Summary

Build the runtime that lets each organization expose one OAuth-authorized MCP endpoint (`{org}.withwillow.ai/mcp`) aggregating the MCP servers an admin registers — both **remote HTTP** and **stdio** (arbitrary commands such as `npx -y <pkg>`). The design is governed by three **hard constraints** — organization isolation (HC-1), frictionless admin add (HC-2), and secure execution of *any* MCP including untrusted code (HC-3) — with user isolation, scale, performance, and cost as best-effort goals that may be traded off to uphold them.

Technical approach: a **stateless MCP gateway** (OAuth 2.0 protected resource server, per the MCP authorization spec, with **Keycloak** as the per-org authorization server) terminates client connections, resolves org/user/roles from audience-bound tokens, and aggregates/namespaces the permitted servers. Remote HTTP servers are proxied directly under egress control. stdio servers run in **microVM sandboxes** (Firecracker/Kata) with default-deny egress, resource caps, and ephemeral filesystems — the hardware-virtualization boundary is the primary mechanism for "support any MCP without concerns." Sandboxes are sized per-`(org, server)` by default and per-`(org, user, server)` when per-user credentials are configured, with warm pools and scale-to-zero to control cold-start and cost.

## Technical Context

Open questions were resolved in [research.md](./research.md); the decisions below reflect those outcomes (no unresolved NEEDS CLARIFICATION remain).

**Language/Version**: Go 1.23+ for gateway, control-plane, and sandbox-supervisor (high-concurrency streaming proxy, low latency, k8s-native). Sandbox base images bundle Node 20+ (`npx`) and Python 3.12+ with `uv`/`uvx` so the common stdio servers start quickly.
**Primary Dependencies**: Keycloak (authorization server / identity broker); Envoy (edge / subdomain routing); Firecracker via Kata Containers `RuntimeClass` (sandbox runtime); PostgreSQL (config/RBAC/audit); Redis (session routing, discovery cache, rate-limit/quota counters); HashiCorp Vault (secrets); OpenTelemetry (observability); an MCP protocol library (Go).
**Storage**: PostgreSQL (organizations, users↔roles, server definitions, audit metadata, usage) with per-org row scoping; Vault (org- and user-level downstream credentials, envelope-encrypted); Redis (ephemeral session/routing/cache); S3-compatible object store with Object Lock (tamper-evident audit archive, ≥1-year retention).
**Testing**: Go `testing` + `testcontainers`; contract tests for the MCP endpoint and admin API; an **adversarial security suite** that runs deliberately hostile MCP servers against the isolation boundary; load tests (k6) at target concurrency.
**Target Platform**: Linux on Kubernetes. Sandbox execution runs on Firecracker-capable nodes (bare-metal / AWS Nitro). Reference cloud AWS (EKS), but cloud-portable.
**Project Type**: Multi-service backend (web service). The existing admin panel/frontend and org/subdomain provisioning are reused and extended, not rebuilt.
**Performance Goals**: tool discovery < 2 s p95; gateway proxy overhead < 150 ms p95; cold stdio start < 5 s p95, warm reuse < 500 ms p95; org endpoint availability 99.9%/month. *(SG-3 — targets, not gates.)*
**Constraints**: HARD — HC-1 org isolation, HC-2 frictionless add, HC-3 secure execution of any MCP. SOFT — SG-1 user isolation, SG-2 scale, SG-3 performance, SG-4 cost. Compliance baseline: SOC 2 (Type II); GDPR/HIPAA out of scope for v1; no hard data-residency requirement for v1.
**Scale/Scope**: 100,000 active users, ~10,000 concurrent MCP sessions (assumed peak); thousands of organizations; tens of servers per org.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Status**: PASS. The project constitution was ratified at **v1.0.0** (2026-06-13). This design complies with all seven principles — Principles I–III correspond to the hard constraints HC-1 (org isolation), HC-2 (frictionless self-service), and HC-3 (secure execution of any MCP); Principle IV is the spec's hard/soft constraint ranking; and Principles V–VII map to the gates below (test-first/adversarial verification, observability/auditability, simplicity/justified complexity).

| Default gate | Assessment |
|---|---|
| **Simplicity / YAGNI** | PASS — one gateway, one control plane, one sandbox runtime. The one real complexity (microVMs) is justified by HC-3 (see Complexity Tracking). |
| **Test-First** | PASS (planned) — contract + adversarial security tests are defined before implementation in `/contracts` and the test plan. |
| **Integration & contract testing** | PASS — MCP endpoint, admin API, and sandbox-supervisor contracts are explicit and independently testable. |
| **Observability** | PASS — OpenTelemetry traces/metrics/logs and per-org/per-server health are first-class (FR-008, US7). |
| **Security-by-default** | PASS — default-deny egress, microVM isolation, audience-bound tokens, encrypted runtime-injected secrets (HC-3, FR-013/14/15). |
| **Versioning** | PASS — MCP auth revisions and protocol versions tracked; admin API versioned. |

**Post-Design re-check (after Phase 1)**: PASS — the data model and contracts keep org isolation as the primary boundary and introduce no new gate violations. The single notable complexity (hardware-virtualized sandboxing) remains justified below.

## Project Structure

### Documentation (this feature)

```text
specs/001-mcp-server-runtime/
├── plan.md              # This file (/speckit-plan output)
├── spec.md              # Feature specification
├── research.md          # Phase 0 — decisions, rationale, alternatives
├── data-model.md        # Phase 1 — entities, relationships, state
├── quickstart.md        # Phase 1 — local bring-up + acceptance walkthrough
├── contracts/           # Phase 1 — interface contracts
│   ├── mcp-endpoint.md       # Client-facing MCP + OAuth discovery
│   ├── admin-api.md          # Control-plane REST API (servers/RBAC/secrets/audit)
│   └── sandbox-supervisor.md # Internal gateway↔supervisor↔agent protocol
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
services/
├── gateway/            # MCP proxy + OAuth 2.0 protected resource server (Go)
│   ├── cmd/
│   ├── internal/auth/        # token validation, audience binding, org/user/role resolution
│   ├── internal/aggregate/   # multi-server merge + tool/resource/prompt namespacing
│   ├── internal/proxy/       # MCP streaming proxy (client ↔ downstream)
│   ├── internal/remotehttp/  # remote-HTTP downstream client (egress-controlled)
│   └── internal/session/     # session routing, revocation enforcement
├── control-plane/      # admin/config/RBAC/secrets/audit API (Go)
│   ├── cmd/
│   ├── internal/servers/     # server definition CRUD + health checks
│   ├── internal/rbac/        # role/permission bindings
│   ├── internal/secrets/     # Vault integration (org + per-user credentials)
│   └── internal/audit/       # audit write + query
├── sandbox-supervisor/ # sandbox lifecycle, warm pools, stdio↔control bridge (Go)
│   └── internal/{pool,vm,bridge,egress}/
└── sandbox-agent/      # in-VM init/shim that launches the stdio command (Go)

pkg/                    # shared libraries: mcp/, authz/, rbac/, telemetry/, audit/
deploy/
├── k8s/                # control + data plane manifests, autoscaling, RuntimeClass
├── sandbox-images/     # base rootfs (Node/npx, Python/uv), package cache
├── keycloak/           # per-org realm template, client scopes, registration policies
└── edge/               # Envoy config, wildcard TLS *.withwillow.ai
migrations/             # PostgreSQL schema migrations
tests/
├── contract/           # MCP endpoint + admin API contract tests
├── integration/        # end-to-end flows (add server → connect → call tool)
├── security/           # adversarial / hostile-MCP isolation tests
└── load/               # k6 scenarios at target concurrency
```

**Structure Decision**: Multi-service backend monorepo. Three independently deployable services map to three trust zones — **gateway** (per-request data plane, untrusted input), **control-plane** (admin/config), and **sandbox-supervisor** (untrusted-code execution) — so the highest-risk component (running arbitrary MCP code) is isolated as its own service on dedicated Firecracker-capable nodes. The existing admin frontend consumes the control-plane API and is out of scope here.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Hardware-virtualized microVM sandboxing (Firecracker/Kata) instead of plain containers | HC-3 requires safely running **arbitrary untrusted code** ("support any MCP without concerns"); a shared host kernel is an unacceptable escape surface for hostile tenants | Plain containers (namespaces/cgroups) share the host kernel — a single escape breaks org isolation (HC-1). gVisor reduces but does not eliminate shared-kernel risk; it is retained as defense-in-depth, not the sole boundary. |
| Separate sandbox-supervisor service + node pool | Keeps untrusted execution physically off the control/data plane and lets it scale independently | Co-locating execution in the gateway would put hostile code next to tokens/secrets and the multi-tenant proxy — directly conflicts with HC-1/HC-3. |
| Per-org Keycloak realms (vs one shared realm) | Strong per-org identity isolation (HC-1) and clean per-org federation/SSO | A single shared realm complicates per-org IdP federation and widens the blast radius of a realm misconfiguration across all tenants. |

## Phase 0 — Outline & Research

See [research.md](./research.md). Resolved: the untrusted-execution sandbox technology (microVM); sandbox granularity / isolation model; Keycloak-as-MCP-authorization-server mapping (audience-scoped tokens, dynamic registration, PKCE, RFC 8707 workaround); gateway language and aggregation/namespacing strategy; multi-tenant data isolation; secrets handling; cold-start/cost strategy; edge routing; audit/retention; remote-HTTP handling; orchestration/cloud.

## Phase 1 — Design & Contracts

- [data-model.md](./data-model.md) — entities, relationships, lifecycle/state, validation rules derived from the FRs.
- [contracts/](./contracts/) — MCP endpoint + OAuth discovery, admin API, internal sandbox-supervisor protocol.
- [quickstart.md](./quickstart.md) — local bring-up and the acceptance-scenario walkthroughs (US1/US2/US3) plus the adversarial isolation test (US4).
- Agent context (`CLAUDE.md`) updated to reference this plan.

## Progress Tracking

- [x] Setup & context loaded (spec, constitution, template)
- [x] Technical Context filled (decisions resolved in research.md)
- [x] Constitution Check (no ratified gates; default gates pass)
- [x] Phase 0 — research.md
- [x] Phase 1 — data-model.md, contracts/, quickstart.md
- [x] Agent context (CLAUDE.md) updated
- [x] Post-design Constitution re-check (pass)
- [ ] Next: `/speckit-tasks` to generate the task breakdown
