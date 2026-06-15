# MCP Sandboxing & Isolation Architecture

How the gateway runs MCP servers, how they are isolated, what is shared, and —
concretely — how each organization's **AWS MCP** gets its own AWS CLI + credentials
so one org can never reach another's account. Companion to
[Local gVisor Sandbox](local-sandbox.md), [Security Model](security.md), and the
[Two-Tenant Isolation Proof](isolation-proof.md).

---

## 1. The runtime stack (Docker / VM / gVisor in boxes)

A stdio MCP server is launched by the gateway as a **container** via the
`ContainerRuntime` (`services/gateway/internal/sandbox/`). The isolation boundary is
the **OCI runtime** chosen by `MCP_SANDBOX_RUNTIME`. On macOS, containers need a Linux
VM; on Linux they run on the host Docker directly.

```
┌──────────────────────────────────────────────────────────────────────────┐
│ HOST (macOS or Linux)                                                      │
│  • gateway process (Go)  • control-plane  • Vault  • Keycloak  • ministack │
│                                                                            │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │ LINUX VM   (macOS only: Lima / Colima;  Linux: not needed)         │    │
│  │                                                                    │    │
│  │  ┌──────────────────────────────────────────────────────────┐     │    │
│  │  │ DOCKER DAEMON                                              │     │    │
│  │  │   runtime = MCP_SANDBOX_RUNTIME                            │     │    │
│  │  │                                                            │     │    │
│  │  │   ┌────────── one of ──────────┬───────────────┐          │     │    │
│  │  │   │ runc      │ runsc (gVisor)  │ kata          │          │     │    │
│  │  │   │ namespaces│ user-space      │ Firecracker   │          │     │    │
│  │  │   │ (dev only)│ kernel ★HC-3    │ microVM       │          │     │    │
│  │  │   └─────┬─────┴────────┬────────┴───────┬───────┘          │     │    │
│  │  │         └──────────────┴────────────────┘                 │     │    │
│  │  │                        ▼                                   │     │    │
│  │  │   ┌────────────────────────────────────────────────┐      │     │    │
│  │  │   │ SANDBOX CONTAINER  (one per MCP server instance) │      │     │    │
│  │  │   │  image: acme/mcp-sandbox:dev (read-only rootfs)  │      │     │    │
│  │  │   │  cmd:  the stdio MCP server (npx/uvx/…)          │      │     │    │
│  │  │   │  --network none | <egress allowlist>            │      │     │    │
│  │  │   │  --read-only  --tmpfs /tmp  --cap-drop ALL       │      │     │    │
│  │  │   │  --pids-limit 256  --memory 512m  no-new-privs   │      │     │    │
│  │  │   └────────────────────────────────────────────────┘      │     │    │
│  │  └──────────────────────────────────────────────────────────┘     │    │
│  └──────────────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────┘
       ★ "exec" runtime = NO container at all (host process, dev only)
```

The exact `docker run` flags are built in `sandbox/container.go`; the runtime is
selected in `sandbox/exec.go` `Select()`.

---

## 2. The four runtimes

| `MCP_SANDBOX_RUNTIME` | What it is | Isolation boundary | Use |
|---|---|---|---|
| `exec` | No container — the server runs as a child process of the gateway | **None** | Dev convenience only |
| `runc` / `container` | Standard Linux container (namespaces + cgroups) | Namespaces — **not** a boundary for hostile code (shared host kernel) | Dev / CI; fine for trusted code + network isolation |
| `gvisor` (`runsc`) | A user-space guest kernel intercepts syscalls | **★ HC-3 boundary** — hostile code never touches the host kernel | **Production default** |
| `kata` | A real lightweight VM (Firecracker) per sandbox | Hardware virtualization (strongest) | Production where nested virt is available |

Default is `gvisor` (`pkg/config` → `MCP_SANDBOX_RUNTIME`, default `gvisor`).

---

## 3. What is running *right now*

| Context | Runtime | Notes |
|---|---|---|
| `make run-gateway` (plain dev) | `exec` | Unsandboxed host process — fast inner loop, **not** an isolation boundary. |
| `make run-gateway-proof` (proof) | `gvisor` | Real HC-3 boundary + egress allowlist; needs `runsc` in the daemon. |
| **The isolation proof on this machine** | **`runc`** on Docker Desktop | gVisor (`runsc`) is **not installed** on the Docker Desktop daemon here, so the proof runs the sandbox under `runc`. The **network egress allowlist is enforced identically** (Docker networking), so the tenant-isolation and egress results are unchanged; gVisor adds the in-kernel syscall boundary on top. Install it with `dev-setup`'s `sandbox-up.sh` for a hardware-isolated run. |

---

## 4. What runs where (Mac vs Docker Linux)

```
                    exec      runc            gvisor (runsc)              kata (microVM)
  macOS (Apple Si)  ✅ any    ✅ Docker        ✅ via Linux VM             ⚠ needs nested virt
                              Desktop / Lima   (Lima/Colima; gVisor's      (M3+ / macOS 15+)
                                               systrap needs NO nested virt)
  Docker on Linux   ✅        ✅ native        ✅ install runsc in daemon   ⚠ needs KVM / nested virt
```

- **gVisor on macOS** works on **any** Apple-Silicon Mac — its `systrap` platform is
  user-space, so it does **not** require nested virtualization. It does require a Linux
  VM (Docker Desktop, **Colima**, or **Lima**) because gVisor is Linux-only. See
  [Local gVisor Sandbox](local-sandbox.md).
- **Kata / Firecracker** needs nested virtualization (KVM) — on macOS that means
  M3+/macOS 15+; on Linux a KVM-capable host.
- **Docker Desktop** does not ship `runsc`; for gVisor on a Mac, Colima/Lima with
  `runsc install` is the supported path (`.claude/skills/dev-setup/scripts/sandbox-up.sh`).

---

## 5. MCP isolation & shared resources

Every stdio MCP server runs in **its own** sandbox container, launched on demand and
reused (one instance per registered server). Org isolation (HC-1) is enforced at the
gateway/identity/data layers, independent of the sandbox runtime.

```
                         ┌─────────────────────────────────────────────┐
   org A user ──token──▶ │  MCP GATEWAY (one process)                    │
                         │  • derives org from Host + token issuer       │
   org B user ──token──▶ │  • per-org catalog (you only see your org)    │
                         │  • RBAC (allowed_roles × your realm roles)    │
                         └───────┬───────────────────────────┬───────────┘
                          org A  │                     org B  │
                                 ▼                            ▼
                   ┌───────────────────────┐    ┌───────────────────────┐
                   │ SANDBOX A             │    │ SANDBOX B             │
                   │  MCP server (org A)   │    │  MCP server (org B)   │
                   │  own PID/mount/net ns │    │  own PID/mount/net ns │
                   │  own /tmp (tmpfs)     │    │  own /tmp (tmpfs)     │
                   │  injected org-A creds │    │  injected org-B creds │
                   └───────────────────────┘    └───────────────────────┘
```

| Isolated **per sandbox** (not shared) | **Shared** infrastructure |
|---|---|
| Process + PID namespace | The gateway process (does the org routing) |
| Mount namespace: **read-only** rootfs + private `/tmp` tmpfs | The Docker daemon / Linux VM |
| Network namespace (`none`, or one egress allowlist network) | The base image `acme/mcp-sandbox:dev` (read-only, pulled once) |
| Dropped Linux capabilities, no-new-privileges | The host kernel — **except** under gVisor (own guest kernel) / Kata (own VM kernel) |
| CPU / memory / PID limits | The egress network *object* (membership = the allowlist; see §7) |
| **Injected credentials** (env at launch, never shared) | Postgres / Redis / Vault (org-scoped via RLS + per-org secret paths) |

**Organization isolation (HC-1)** holds regardless of runtime:
- The token is **audience-bound** to one org's MCP resource and issued by that org's
  realm; a token for org A is rejected at org B's endpoint (issuer/audience mismatch).
- The gateway derives org from `{org}.{base-domain}` Host + issuer and serves a
  **per-org catalog** — a user never sees another org's servers.
- Postgres **row-level security** scopes every record by `org_id`.
- Cross-org access **fails closed** (404/403) and is **audited**.

---

## 6. AWS MCP per-org isolation (own AWS CLI + creds)

Each org registers its **own** AWS MCP server (`awslabs.aws-api-mcp-server`, stdio).
The gateway launches a **separate sandbox container per org**, so each org's AWS MCP
has its **own AWS CLI process** and its **own credentials** — there is no shared AWS
client and no shared session.

```
 org A user ─token(aud=A)─▶ GATEWAY ─route org=A─▶ ┌── SANDBOX A ─────────────┐
                                                   │ aws-api-mcp-server (A)   │
                                                   │ aws cli  +  creds A      │──▶ ministack
                                                   │ AWS_ACCESS_KEY_ID=111…   │    ACCOUNT 111…
                                                   │ AWS_ENDPOINT_URL=ministack│    bucket alpha-data
                                                   └──────────────────────────┘
 org B user ─token(aud=B)─▶ GATEWAY ─route org=B─▶ ┌── SANDBOX B ─────────────┐
                                                   │ aws-api-mcp-server (B)   │
                                                   │ aws cli  +  creds B      │──▶ ministack
                                                   │ AWS_ACCESS_KEY_ID=222…   │    ACCOUNT 222…
                                                   │ AWS_ENDPOINT_URL=ministack│    bucket beta-data
                                                   └──────────────────────────┘

   ✗ org A can NOT reach org B:
     1. token aud=A is rejected at B's endpoint            (identity)
     2. gateway routes A's user only to A's MCP            (org-scoped catalog)
     3. A's MCP holds only A's creds → A's AWS account     (per-org credentials)
     4. account 111… cannot see account 222…'s buckets     (account isolation)
```

Mechanics:
- **Own credentials.** Each org's AWS access key/secret are stored **write-only** in
  Vault (per-org path) and injected as **environment variables at container launch**
  (`credential_mode: org_shared`). They are never returned by any API, never logged,
  and never shared between orgs' sandboxes.
- **Own AWS CLI.** The AWS CLI binary lives in the shared read-only base image, but
  each org's MCP runs it in **its own container** with **its own** env (creds +
  `AWS_ENDPOINT_URL` + region). Different container = different process = different
  client = different credentials.
- **Own AWS account.** A 12-digit `AWS_ACCESS_KEY_ID` is the AWS **account id**; the
  emulator namespaces (and access-controls) all resources per account, so org A's
  bucket lives in account 111… and is invisible/denied to account 222….
- **Proven.** The [Two-Tenant Isolation Proof](isolation-proof.md) validates this
  end-to-end: each realm's user is routed only to its own MCP, and ministack's
  per-account state confirms each MCP wrote only to its own account (server-side),
  with cross-realm tokens rejected (401) and cross-account access denied.

---

## 7. Network egress: default-deny + an explicit allowlist

Sandboxes are **default-deny** on the network (`--network none`): a server cannot reach
the control plane, cloud metadata (`169.254.169.254`), other tenants, or the internet.

When a server legitimately needs **one** dependency (e.g. the AWS emulator), set
**`MCP_SANDBOX_EGRESS_NETWORK`** to a Docker **`internal: true`** network whose only
member is that dependency. The sandbox can then reach **only** that — still no metadata,
no internet, no control plane. This is the "explicit allowlist" half of the
default-deny model.

```
  default (unset):  sandbox ──✗── everything            (--network none)

  allowlist set:    sandbox ──✅─▶ ministack             (internal network: only member)
                    sandbox ──✗── control-plane / 169.254.169.254 / internet
```

`MCP_SANDBOX_EGRESS_NETWORK` is **additive and off by default** — leaving it empty
preserves `--network none` exactly.

---

## 8. Configuration reference

| Env | Default | Meaning |
|---|---|---|
| `MCP_SANDBOX_RUNTIME` | `gvisor` | `exec` \| `runc`/`container` \| `gvisor` \| `kata` |
| `MCP_SANDBOX_IMAGE` | `acme/mcp-sandbox:dev` | Base image stdio servers run in |
| `MCP_SANDBOX_EGRESS_NETWORK` | `""` (→ `--network none`) | Docker network the sandbox may egress to (the allowlist) |

## See also
- [Local gVisor Sandbox](local-sandbox.md) — provisioning `runsc` on macOS (Colima/Lima).
- [Security Model](security.md) — HC-1/HC-3, SSRF, secrets, audit.
- [Two-Tenant Isolation Proof](isolation-proof.md) — the end-to-end proof of the above.
- Code: `services/gateway/internal/sandbox/` (`exec.go` `Select`, `container.go` `buildArgs`).
