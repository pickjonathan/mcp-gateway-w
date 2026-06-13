# Contract: Sandbox Supervisor (internal)

**Scope**: internal protocol between the **gateway** and the **sandbox-supervisor**, and between the supervisor and the in-VM **sandbox-agent**. Not exposed to tenants. Transport: mTLS gRPC on the control network (never reachable from inside a sandbox).

## Gateway ↔ Supervisor

| RPC | Request | Response / behavior |
|---|---|---|
| `EnsureSession` | `{org_id, server_id, user_id?, server_spec, credential_refs, egress_allowlist}` | Returns `{session_id, state}`. Reuses a warm instance for the key `(org_id, server_id[, user_id])` or launches a new microVM from the warm pool (R2, R8). **org_id is always part of the key — never shared across orgs (HC-1).** |
| `OpenStream` | `{session_id}` (bidi stream) | Bridges MCP stdio: client→`stdin`, `stdout`→client, ordered, streaming (US3 scenario 2). |
| `StopSession` | `{session_id, reason}` | Terminates instance, reclaims resources (idle/disable/revoke). |
| `Health` | `{session_id}` | `starting\|healthy\|idle\|failed` + detail (FR-008/FR-019). |

## Supervisor responsibilities (HC-3 enforcement)

1. **Launch** microVM (Firecracker via Kata `RuntimeClass`) from warm pool; attach read-only base rootfs (Node/npx, Python/uv) + writable ephemeral overlay.
2. **Confine**: drop capabilities, seccomp profile, gVisor layer; cgroup/VM CPU·mem·pid·disk caps; startup/idle/request timeouts (FR-013, FR-020).
3. **Network**: per-sandbox netns, **default-deny egress** via egress proxy enforcing `egress_allowlist` + registry mirror; **block** 169.254.169.254, RFC-1918, control plane (FR-014, R7).
4. **Secrets**: fetch `credential_refs` from Vault just-in-time, inject as env/files into the VM over the control channel; never log; never expose to other sandboxes (FR-015).
5. **Lifecycle**: mark `idle` then reclaim after idle timeout (FR-018); transparent restart on next `EnsureSession`; `failed` on crash/startup-timeout with no resource leak (US3 scenario 3).

## Supervisor ↔ Sandbox-agent (in-VM)

| Message | Behavior |
|---|---|
| `Init{command,args,env}` | Agent execs the stdio MCP server, completes MCP handshake, reports healthy (US3 scenario 1). |
| `Stdio{frame}` | Relays stdin/stdout frames to the bridge. |
| `Shutdown` | Graceful stop; agent is the only privileged process and cannot reach the host network/control plane. |

## Invariants (verified by the adversarial security suite — US4, SC-002/SC-003)

- A sandbox cannot reach another tenant's instance, the platform secrets/control plane, internal IPs, or cloud metadata.
- A crash/abuse in one sandbox never affects another sandbox, the gateway, or another tenant.
- No sandbox is ever shared across organizations.
