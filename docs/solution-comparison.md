# Solution Comparison

How the chosen design was selected, scored against the seven evaluation verticals.
Ratings: **✓ strong · △ partial / with caveats · ✗ weak**.

## The seven verticals

1. **Org isolation** (HARD) — orgs never access each other's data.
2. User isolation (soft) — users within an org are separated.
3. **Frictionless admin** (HARD) — admins add MCPs with no infra work.
4. **Secure any MCP** (HARD) — run untrusted servers safely.
5. Scalability (soft) — 100k+ users, 10k+ concurrent.
6. Performance (soft) — low latency.
7. Cost (soft) — efficient resource use.

> Hard constraints (1, 3, 4) outrank soft goals; a soft goal may be sacrificed to
> uphold a hard one, never the reverse.

## A. Multi-tenancy & execution architecture

### Options considered

- **A1 — Single shared process.** All MCP servers run in one gateway process
  (in-proc or child processes), no per-tenant boundary.
- **A2 — Container per server (shared cluster).** Each stdio server runs in an
  OCI container on a shared orchestrator; remote servers proxied.
- **A3 — microVM per server / per session.** Each untrusted server runs in a
  Firecracker/Kata microVM; strong kernel-level isolation.
- **A4 — Dedicated stack per tenant.** A separate VM/cluster/namespace per org.
- **★ Chosen — Hybrid.** Stateless shared gateway + **per-org logical isolation**
  (catalog, audience-bound tokens, DB RLS) + **sandboxed execution** (container →
  microVM/gVisor) for stdio + **proxy** for remote HTTP, with a warm pool.

### Scoring

| Approach | Org iso (H) | User iso | Frictionless (H) | Secure any MCP (H) | Scale | Perf | Cost |
|---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| A1 shared process | ✗ | ✗ | ✓ | ✗ | △ | ✓ | ✓ |
| A2 container/server | △ | △ | ✓ | △ (shared kernel) | ✓ | ✓ | ✓ |
| A3 microVM/server | ✓ | ✓ | ✓ | ✓ | △ | △ (cold start) | △ |
| A4 stack/tenant | ✓ | ✓ | ✗ (heavy onboarding) | ✓ | ✗ | ✓ | ✗ |
| **★ Hybrid** | **✓** | **△→✓** | **✓** | **✓** | **✓** | **✓** | **✓** |

**Why the hybrid wins.** A1 fails two hard constraints outright. A2 leaves the
kernel shared — unacceptable for *arbitrary untrusted* code (HC-4). A4 satisfies
isolation but destroys frictionless onboarding and cost. The hybrid takes A2's
density/scale and elevates only the *risky* part (stdio execution) to A3-grade
microVM isolation, while logical isolation (per-org catalog + audience-bound
tokens + **RLS**) gives strong org isolation cheaply. Cold-start (A3's weakness)
is mitigated by a **warm pool with scale-to-zero**.

## B. Sandbox runtime (for stdio execution, HC-4)

| Runtime | Isolation | Perf (start/throughput) | Cost/density | Local-runnable | Verdict |
|---|---|---|---|---|---|
| `exec` (host process) | ✗ none | ✓✓ instant | ✓✓ | ✓ | **Dev only — UNSANDBOXED** |
| Docker `runc` | △ namespaces, shared kernel | ✓ fast | ✓ high | ✓ | Not enough for untrusted code |
| **gVisor (`runsc`)** | ✓ user-space kernel | △ syscall overhead | ✓ high | ✓ (no nested virt) | **Local HC-3 boundary** |
| **Kata / Firecracker** | ✓✓ real microVM | △ ~100ms start | △ medium | ✗ needs virt | **Production HC-3 boundary** |

Selection: a pluggable `SandboxRuntime` — `exec` for dev, **gVisor** for the
locally-runnable strong boundary (separate guest kernel, no nested virtualization
needed), **microVM** in production via a Kubernetes `RuntimeClass`. All sandboxes
run `--network none`, `--cap-drop ALL`, read-only rootfs, with CPU/mem/pid limits.

The gVisor boundary was validated with a live adversarial suite: distinct guest
kernel vs host, read-only rootfs enforced, mount/cap escalation denied, no
host-fs leak, and **metadata / internal / internet egress all blocked**.

## C. Identity & authorization

| Option | Org iso | Frictionless | Notes |
|---|:--:|:--:|---|
| Shared realm, org as a claim | △ | ✓ | One bug in claim checks → cross-org; weaker blast-radius |
| **★ Per-org Keycloak realm** | ✓ | ✓ | Realm-per-org + **audience-bound** tokens (`https://{org}.host/mcp`); a token for org A is structurally invalid at org B |

Aligned to Keycloak's MCP authorization-server guidance: OAuth 2.1 + PKCE,
RFC 9728 protected-resource metadata, RFC 8414 AS metadata, RFC 7591 dynamic
client registration, JWKS validation, audience-mapped scopes.

## D. Control-plane → data-plane propagation

| Option | Freshness | Durability | Complexity |
|---|:--:|:--:|:--:|
| DB polling | △ (interval) | ✓ | ✓ simple |
| Push/gRPC stream | ✓ | △ (reconnect logic) | △ |
| **★ Redis pub/sub + reconcile-on-startup** | ✓ | ✓ | ✓ |

Redis gives near-instant fan-out; **Postgres reconcile on startup** is the
durability backstop for fire-and-forget delivery, so a restart of either service
converges. This is why both a live path and a reconcile path exist.

## E. Org-isolation enforcement layers (defense in depth, HC-1)

| Layer | Mechanism |
|---|---|
| Identity | Audience-bound token; org derived from host |
| Application | Per-org `Catalog`/`Registry`; every lookup org-scoped (+ cross-org tests) |
| Database | Postgres **Row-Level Security** (`FORCE`, non-superuser role); fail-closed when org context unset |

No single layer is trusted alone — an app-level bug still can't cross tenants
because RLS enforces it at the database.

## Decisions summary

| Vertical | Decision |
|---|---|
| Org isolation | Per-org realm + audience tokens + per-org catalog + Postgres RLS |
| User isolation | Per-user credentials → per-user downstream instances; RBAC |
| Frictionless | One admin API call; platform provisions/runs the server |
| Secure any MCP | No-network microVM/gVisor sandbox for stdio; SSRF guard for remote |
| Scalability | Stateless gateway; Redis-shared quotas; sandbox warm pool |
| Performance | Warm pool (cold-start), connection reuse, in-process routing |
| Cost | Scale-to-zero sandboxes; shared infra; microVM density |

The trade-off philosophy throughout: **uphold the three hard constraints
absolutely; optimize the four soft goals as far as that allows.**
