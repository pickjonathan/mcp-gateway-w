# Sandbox Runtime & Isolation Architecture Options

**Date**: 2026-06-13 | **Feature**: [Multi-Tenant MCP Server Runtime](./spec.md) | **Related**: [research.md](./research.md) R1/R2

This compares the candidate ways to execute **untrusted MCP server code** (the HC-3 "support any MCP" problem), scored against the seven product criteria, plus where each can run locally.

> **Two axes, kept separate:**
> 1. **Isolation architecture** (plain container / gVisor / microVM / serverless) — this is what the 7 criteria actually differ on, at production scale.
> 2. **Where you run it** (Docker Desktop / Linux VM+KVM / minikube / remote) — affects local setup & fidelity, not the production scoring.

**Legend**: ✅ strong · ⚠️ moderate / caveat · ❌ weak. Criteria 1–2 (isolation) are scored against the threat *"can a malicious/compromised MCP server break out and violate isolation?"* — the data-plane isolation (RBAC, `org_id`, audience-bound tokens) is identical across all options.

---

## 1. Isolation architectures vs the 7 criteria

| Architecture | Isolation tradeoff | 1· Org isolation | 2· User isolation | 3· Frictionless | 4· Security (any MCP) | 5· Scalability | 6· Performance | 7· Cost |
|---|---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| **Plain containers** (ns + seccomp + cgroups) | Shared host kernel — one escape breaks every tenant; maximum density | ❌ escape → cross-org | ❌ escape → cross-user | ✅ instant start | ❌ unsafe for untrusted code | ✅ highest density | ✅ near-native | ✅ cheapest |
| **gVisor (`runsc`)** user-space kernel | Syscalls intercepted in user space; small shared-kernel surface remains | ⚠️ strong, not absolute | ⚠️ strong, not absolute | ✅ fast start | ⚠️ strong mitigation, not a VM | ✅ lightweight, dense | ⚠️ syscall/I/O overhead | ✅ container-like |
| **microVM — Firecracker/Kata** *(our prod choice)* | Per-workload **guest kernel** (hardware virt) — strongest; pays in density + cold-start | ✅ own kernel | ✅ per-user VM when needed | ✅ slight first-launch delay | ✅ best for arbitrary untrusted code | ⚠️ lower density, warm-pools needed | ⚠️ ~125 ms boot; warm reuse fast | ⚠️ higher mem/host overhead |
| **Serverless containers** (Fargate / Cloud Run gen2 / Fly Machines) | VM-grade on most platforms (Fargate & Fly = Firecracker), but you inherit the platform boundary — least control | ✅ VM-backed | ✅ per-task VM | ✅ fully managed | ✅ platform microVM | ✅ auto-scales, scale-to-zero | ⚠️ cold starts; needs session affinity | ⚠️ pay-per-use premium |

**Reading it**: density/cost/perf improve as you move *up* the table; isolation strength for untrusted code improves as you move *down*. The spec ranks **security (HC-3) as hard and cost/perf (SG-3/4) as soft**, which is exactly why the plan picks **microVM** as the production boundary and keeps **gVisor** as a cheaper defense-in-depth / dev layer.

---

## 2. Where you run it (local dev → production)

| Run environment | Runs which architecture | Real microVM (KVM) here? | Setup | Local fidelity | Notes |
|---|---|:--:|:--:|:--:|---|
| **Docker Desktop + gVisor** | gVisor | ❌ no KVM | Low | Medium | Fastest path to code/test; honest (gVisor is a real layer in our design). arm64 OK with minor quirks. |
| **Docker Desktop, plain container** | plain container | ❌ | Lowest | Low | Wiring/integration tests only — **not** an acceptable boundary for untrusted code. |
| **Linux VM + KVM + Kata/Firecracker** (Lima/UTM) | microVM | ✅ yes — your **M4 Max + macOS 26** supports nested virt | High | High | True prod boundary locally; fiddly nested-virt + Kata setup; VM-in-VM overhead. |
| **minikube/k3s inside that KVM VM + Kata `RuntimeClass`** | microVM (k8s) | ✅ | Highest | **Highest** | Mirrors production topology (k8s + RuntimeClass + microVM). |
| **minikube (docker driver) + gVisor addon** | gVisor (k8s) | ❌ | Medium | Medium | Exercises k8s manifests + RuntimeClass wiring without KVM; arm64 addon maturity caveat. |
| **Remote KVM host / cloud** (`.metal` / nested-virt VM) | microVM | ✅ (remote) | Medium | High | Offloads KVM to a real Linux box — great for CI & fidelity; remote loop + cost. |

> 💡 Your hardware (**Apple M4 Max, macOS 26**) supports **nested virtualization**, so unlike M1/M2 Macs the real Firecracker/Kata path *is* runnable locally inside a KVM-enabled Linux VM — not only via gVisor.

---

## 3. Architecture of each solution

### A. Plain containers
A normal OCI container (Linux namespaces, cgroups, seccomp, dropped caps). All tenants' containers share the **one host kernel**; isolation rests entirely on kernel correctness.

```
 ┌────────────┐ ┌────────────┐   per-tenant containers
 │ MCP svr A  │ │ MCP svr B  │
 ├────────────┴─┴────────────┤
 │      SHARED HOST KERNEL    │ ◄─ single boundary: one escape = all tenants
 └────────────────────────────┘
```
*Use:* dev wiring only. *Reject for untrusted code* (HC-3).

### B. gVisor (`runsc`)
Each container runs atop the **Sentry**, a user-space kernel that intercepts and services syscalls, so the workload rarely touches the real host kernel. Much smaller attack surface than plain containers, far lighter than a VM.

```
 ┌────────────┐
 │  MCP svr   │
 ├────────────┤
 │  Sentry    │ ◄─ user-space kernel (intercepts syscalls)
 ├────────────┤
 │ host kernel│ (minimal, filtered surface)
 └────────────┘
```
*Use:* the **default local/dev backend**, and a defense-in-depth layer in prod.

### C. microVM — Firecracker / Kata  *(production boundary)*
Each MCP server runs inside its own lightweight VM with a **dedicated guest kernel**, started by a minimal VMM (Firecracker) on KVM. Hardware-virtualization boundary; ~125 ms boot. Orchestrated via Kata `RuntimeClass` on Kubernetes; warm pools + scale-to-zero manage cold-start/cost.

```
 ┌────────────┐ ┌────────────┐
 │  MCP svr   │ │  MCP svr   │
 │ GUEST KRNL │ │ GUEST KRNL │ ◄─ own kernel per workload
 ├────────────┤ ├────────────┤
 │ Firecracker│ │ Firecracker│  (VMM on /dev/kvm)
 ├────────────┴─┴────────────┤
 │         HOST KERNEL        │ ◄─ tenants isolated by VM boundary
 └────────────────────────────┘
```
*Use:* production. Strongest practical isolation for arbitrary untrusted code; the cost/perf premium is the deliberate HC-3-over-SG-4 trade.

### D. Serverless containers (Fargate / Cloud Run gen2 / Fly Machines)
A managed platform runs each workload in its own isolation unit (Fargate & Fly use Firecracker microVMs; Cloud Run gen2 uses a microVM). You get VM-grade isolation and auto-scaling without operating the substrate — at the price of control, cold-start latency, and per-unit cost.

```
 ┌────────────┐
 │  MCP svr   │
 ├────────────┤
 │ platform   │ ◄─ provider-managed microVM/gVisor
 │ isolation  │
 └────────────┘  (you don't run the host)
```
*Use:* build-vs-buy alternative; least ops, less control/portability.

---

## Recommendation

Make the sandbox runtime **pluggable** (`SandboxRuntime` interface) with swappable backends:

- **Dev / local / CI → gVisor** (Docker Desktop, or minikube + gVisor addon). No KVM, fast loop, faithful to our defense-in-depth layer.
- **Production → microVM (Firecracker/Kata)** on KVM-capable nodes. The hard-constraint (HC-3) isolation boundary.
- **Fidelity check → run the real microVM path** in a KVM-enabled Linux VM on your M4 (nested virt) or a remote host, and pass the adversarial suite (US4 / SC-002–003) there before any production claim.

This satisfies HC-3 in production while keeping development fast and cheap — consistent with research **R1**.
