# Phase 0 Research: Multi-Tenant MCP Server Runtime

**Date**: 2026-06-13 | **Plan**: [plan.md](./plan.md)

Decisions are evaluated against the spec's constraint ranking: **HARD** = HC-1 org isolation, HC-2 frictionless add, HC-3 secure execution of any MCP; **SOFT** = SG-1 user isolation, SG-2 scale, SG-3 performance, SG-4 cost. Where a hard and soft goal conflict, the hard goal wins.

---

## R1. Execution sandbox for untrusted stdio servers *(the central decision)*

**Decision**: Run each stdio MCP server inside a **hardware-virtualized microVM** (Firecracker, orchestrated via Kata Containers `RuntimeClass` on Kubernetes), with seccomp, dropped capabilities, read-only base rootfs, a writable ephemeral overlay, cgroup/VM CPU-mem-pid-disk caps, and a default-deny egress network namespace. gVisor + seccomp are layered as defense-in-depth, not the sole boundary.

**Rationale**: HC-3 ("support any MCP without concerns", incl. `npx -y <arbitrary-pkg>`) means hostile code is expected. The microVM gives each workload its **own guest kernel**, so a kernel-level exploit does not reach the host or other tenants — directly upholding HC-1. Firecracker boots in ~125 ms and has a deliberately tiny device model (small attack surface), so the security win is affordable against SG-3/SG-4. Cost (SG-4) is explicitly allowed to be higher to satisfy HC-3.

**Alternatives considered**:
- **Plain containers (namespaces + seccomp + cgroups)** — highest density/lowest cost, but a shared host kernel; one container-escape CVE breaks HC-1. Rejected as the isolation boundary for untrusted code.
- **gVisor (runsc) only** — intercepts syscalls in a user-space kernel; strong mitigation, lighter than a VM, but still a shared host kernel and known compatibility/perf gaps for some runtimes. Rejected as *sole* boundary for hostile code; kept as an added layer.
- **Per-call serverless (FaaS)** — MCP stdio servers are long-lived, stateful sessions; stateless short-lived functions fit poorly and cold-start per call would blow SG-3. Rejected.
- **Managed serverless containers (Fargate / Cloud Run gen2 / Fly Machines)** — viable and reduce ops; Cloud Run gen2 uses gVisor, Fargate and Fly give VM-grade isolation. Recorded as a credible build-vs-buy option for teams wanting less infrastructure; the self-managed Firecracker path is the reference for cost control at 10k concurrent and cloud portability.

---

## R2. Sandbox granularity / isolation model

**Decision**: The sandbox boundary is **always at least per-organization**. Default to one instance per **`(org, server)`** shared across that org's users for stateless / org-credential servers; escalate to per **`(org, user, server)`** when the server is configured for per-user credentials (FR-016) or holds user-specific state. Never share a sandbox across organizations.

**Rationale**: Per-`(org, server)` keeps HC-1 absolute while amortizing one warm process across an org's users — good for SG-4/SG-3. Because user isolation is SOFT (SG-1), full per-user instances are reserved for the case where it actually matters (per-user credentials), where they also become a correctness requirement, not just isolation. This makes the cost of stronger isolation opt-in per server.

**Alternatives considered**:
- **Always per-user** — strongest SG-1 but multiplies instance count and cost; unjustified when SG-1 is soft and many servers (e.g. sequential-thinking) are stateless.
- **Shared per-server across orgs with logical filtering** — violates HC-1. Rejected outright.

---

## R3. Authorization: Keycloak as MCP authorization server

**Decision**: **Keycloak**, one **realm per organization**, is the OAuth 2.0/OIDC authorization server. The gateway is an OAuth 2.0 **protected resource server**. Implement per Keycloak's *MCP authorization server* guidance:
- **OAuth 2.1 Authorization Code + PKCE** for public MCP clients.
- **Dynamic Client Registration (RFC 7591)** so clients self-register (frictionless, HC-2); gate it with Keycloak client-registration policies (trusted hosts / allowed scopes / web origins).
- **Authorization Server Metadata (RFC 8414)** + **Protected Resource Metadata (RFC 9728)** for discovery; the gateway returns `401` with a `WWW-Authenticate` pointer to its resource-metadata document.
- **Audience binding**: since Keycloak does not natively honor **RFC 8707** `resource` indicators, bind audience via **audience-mapped client scopes** (`mcp:tools`, `mcp:resources`, `mcp:prompts`) whose Audience mapper sets `aud` to the org's MCP URL. The gateway rejects any token whose `aud` ≠ its own org endpoint.
- Target MCP auth revision **2025-03-26** as baseline (no special Keycloak setup); enable the scope/audience mappings for **2025-06-18 / 2025-11-25** clients; optionally enable **Client-ID Metadata Document (`cimd`)** for 2025-11-25.

**Rationale**: Realm-per-org isolates identities, tokens, and federation config per tenant (HC-1) and gives clean per-org SSO. Audience binding is what stops a token minted for org A reaching org B's endpoint (HC-1). Dynamic registration + PKCE are what make "connect your client and go" frictionless (HC-2).

**Alternatives considered**:
- **Single shared realm with an `org` claim** — fewer realms to manage, but one misconfiguration leaks across all tenants and per-org IdP federation gets awkward. Rejected for HC-1.
- **Custom OAuth server** — reinvents a hardened, certified product. Rejected.
- **Per-org realm scaling concern**: thousands of realms is a known Keycloak operating cost; mitigated with realm templates/automation and horizontal Keycloak scaling. Acceptable given HC-1 priority.

---

## R4. Gateway runtime & MCP aggregation/namespacing

**Decision**: Implement the gateway in **Go**. It terminates client MCP over **Streamable HTTP**, fans out to the permitted servers, and merges their capabilities. **Namespace** every tool/resource/prompt by a stable server slug (e.g. `serverSlug__toolName`) to guarantee no collisions (FR-003). Maintain a per-session routing table (server slug → downstream transport) in memory with Redis-backed lookup for horizontal scale.

**Rationale**: Go's goroutine model handles many concurrent long-lived streaming connections cheaply (SG-2/SG-3). Deterministic namespacing satisfies FR-003's "no silent overwrite". A stateless gateway with externalized session/routing state scales horizontally behind the edge.

**Alternatives considered**: Rust (max performance, steeper velocity) — viable for the hot path, deferred; Node/TS (matches MCP reference SDKs but weaker for high-concurrency proxying) — rejected for the gateway core.

---

## R5. Multi-tenant data isolation (config/RBAC/audit)

**Decision**: **PostgreSQL** with an `org_id` on every tenant-owned row, enforced by (a) application-layer scoping on every query and (b) Postgres **Row-Level Security** policies keyed on a session `org_id` GUC as defense-in-depth. Partition/large-table strategy by `org_id` for scale.

**Rationale**: Belt-and-suspenders org scoping makes accidental cross-tenant reads structurally hard (HC-1). RLS provides a backstop if an application query forgets a filter.

**Alternatives considered**: DB-per-org (strongest isolation, heavy ops at thousands of orgs) — rejected for v1, revisit for enterprise tiers; schema-per-org — middle ground, also heavy. Shared schema + RLS chosen for scale + adequate isolation.

---

## R6. Secrets / downstream credentials

**Decision**: **HashiCorp Vault** (or cloud KMS + envelope encryption) stores org-level (default) and per-user (optional) downstream credentials. The supervisor fetches them just-in-time and injects them into the sandbox as process env/files over the control channel; values are never written to logs, never exposed to the gateway response path, and never shared across sandboxes. Rotation applies on next sandbox start.

**Rationale**: Satisfies FR-015/FR-016 and HC-3 (secrets isolated from hostile code in other sandboxes). Vault gives audit, leasing, and rotation out of the box.

**Alternatives considered**: app-level encryption in Postgres (simpler, weaker key management/rotation) — rejected as primary; acceptable fallback for envelope-encrypted blobs.

---

## R7. Network egress control for sandboxes

**Decision**: Each sandbox runs in its own network namespace with **default-deny egress**. Outbound traffic flows through a per-tenant **egress proxy** that enforces an allowlist (e.g. the npm/PyPI registry mirror for package install, plus admin-approved destinations for the specific server). **Block** link-local cloud metadata (169.254.169.254), RFC-1918 internal ranges, and the platform control plane.

**Rationale**: The classic untrusted-code exfiltration/SSRF and metadata-credential-theft paths are closed (HC-3, FR-014). Allowlisting the registry keeps `npx`/`uvx` working (HC-2) without opening the internet.

**Alternatives considered**: open egress with monitoring — far weaker, rejected; no package fetch (pre-baked only) — hurts "support any MCP". Mitigated instead with a cached registry mirror.

---

## R8. Cold-start & cost strategy

**Decision**: Keep a small **warm pool** of generic microVMs per node; assign on demand and specialize at launch. **Bake Node/npx + Python/uv and a warm package cache** into the base image so common servers start fast. **Reuse** a warm sandbox for an active `(org[,user],server)` session; **scale to zero** after an idle timeout (FR-018) and transparently restart on next use.

**Rationale**: Directly targets SC-012 (cold < 5 s, warm < 500 ms) and SC-014 (idle ≈ zero cost) without weakening isolation. Cost tracks concurrent usage, not registered users/servers.

**Alternatives considered**: always-on per-server instances (predictable latency, high idle cost — fails SC-014); pure on-demand with no warm pool (cheapest, worst cold-start) — rejected for SG-3.

---

## R9. Edge / per-org endpoint routing

**Decision**: Wildcard TLS for `*.withwillow.ai` terminated at an **Envoy** edge that routes by subdomain to the stateless gateway fleet. The gateway derives `org` from the host and cross-checks it against the token's realm/audience.

**Rationale**: One wildcard cert + host-based routing scales to unlimited orgs with no per-org infra at the edge (HC-2, SG-2). Host↔token cross-check is an extra HC-1 guard.

**Alternatives considered**: path-based (`/{org}/mcp`) — breaks the clean per-org-origin model and complicates cookies/CORS; per-org certs — unnecessary operational load.

---

## R10. Audit, observability & retention

**Decision**: **OpenTelemetry** traces/metrics/logs across services, with per-org and per-server health/usage surfaced to admins (US7, FR-008). Audit events (config changes + security-relevant events, FR-010) are written append-only to Postgres for queryable recent history and archived to **object storage with Object Lock** for tamper-evident **≥1-year** retention (SOC 2 baseline).

**Rationale**: Meets FR-010 + the SOC 2 clarification; Object Lock gives tamper-evidence auditors expect.

**Alternatives considered**: logs-only (not queryable/attestable) — insufficient for SOC 2; indefinite hot retention — unnecessary cost.

---

## R11. Remote HTTP downstream servers

**Decision**: The gateway acts as an MCP **client over Streamable HTTP** to remote servers, carrying admin-configured auth, and routes that traffic through the **egress-controlled** path. Health-check on add and continuously (FR-008); isolate failures so one server's outage never breaks the session or other servers (FR-019).

**Rationale**: Remote HTTP is first-class (per the latest clarification) and is the lighter path — no sandbox needed, but still egress-bounded so a malicious URL can't pivot internally.

**Alternatives considered**: routing remote calls outside egress control — rejected (SSRF risk); running remote servers in sandboxes too — unnecessary (no local code execution).

---

## R12. Orchestration & cloud

**Decision**: **Kubernetes** for control + data planes; a dedicated **Firecracker-capable node pool** (bare-metal / AWS Nitro) for sandboxes via Kata `RuntimeClass`. Cloud-portable; AWS (EKS) as the reference.

**Rationale**: K8s gives autoscaling, rollout, and isolation primitives; a separate node pool keeps untrusted execution off control/data-plane hosts (HC-1/HC-3) and lets it scale independently (SG-2).

**Alternatives considered**: fully managed sandbox SaaS (E2B/Fly) — faster start, less control/portability (build-vs-buy, see R1); single shared node pool — rejected (untrusted code adjacent to control plane).

---

## Open items carried to planning/business (non-blocking)

- **Cost target** (SC-015): per-active-user monthly ceiling is a business input; needed to tune warm-pool sizing and instance density. SC-014 (usage-proportional, idle≈0) holds regardless.
- **Observability signal depth**: exact metric/trace catalog and alert thresholds are an implementation detail for `/speckit-tasks`.
- **Realm-count operations**: validate Keycloak at the planned org scale (thousands of realms) during load testing; consider realm-sharding if needed.
