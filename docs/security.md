# Security Model

Security is organized around the three hard constraints. The guiding rule:
**support any MCP — including untrusted code — without compromising tenant
isolation.**

## Organization isolation (HC-1) — defense in depth

Three independent layers, none trusted alone:

1. **Identity** — each org has its own Keycloak realm; user tokens are
   **audience-bound** to `https://{org}.{base-domain}/mcp`. The org is derived
   from the request host and must match the token audience, so a token minted for
   org A is structurally invalid at org B.
2. **Application** — the gateway keeps a separate downstream `Registry` per org
   inside a `Catalog`; every catalog/store lookup is keyed by org id. Cross-org
   leak tests guard this path.
3. **Database** — `mcp_servers` has **Row-Level Security** (`ENABLE` + `FORCE`)
   with a policy `org_id = current_setting('app.current_org')`. The app sets that
   GUC per transaction (`set_config(..., true)`); an unset context yields no rows
   (**fail-closed**). Services connect as a **non-superuser** role (superusers
   bypass RLS), so even a forgotten `WHERE org_id` cannot leak across tenants.

## Secure execution of any MCP (HC-3/HC-4)

stdio servers run arbitrary, possibly-hostile code, so they are confined:

- **No network** (`--network none`) by default — cannot reach the cloud metadata
  endpoint, internal services, other tenants, or the internet. An optional explicit
  **egress allowlist** (`MCP_SANDBOX_EGRESS_NETWORK` → a Docker `internal` network)
  lets a sandbox reach *only* a named dependency (e.g. a local emulator) and nothing
  else — the allowlist half of default-deny egress (HC-2).
- **Dropped capabilities** (`--cap-drop ALL`, `no-new-privileges`).
- **Read-only rootfs** + a small writable `tmpfs /tmp`.
- **Resource limits** — CPU, memory, pids.
- **Kernel-level isolation** — gVisor (user-space kernel) locally; microVM
  (Firecracker/Kata) in production via a Kubernetes `RuntimeClass`.
- **Kill-switch** — removing a server terminates its running instance(s).

### Adversarial validation

A live containment suite against the gVisor boundary confirmed:

- separate guest kernel (`4.19.0-gvisor`) vs host;
- read-only rootfs enforced; mount/capability escalation denied;
- no host-filesystem leak;
- egress to **metadata, internal, and internet** all blocked.

The end-to-end [two-tenant AWS-MCP isolation proof](isolation-proof.md) re-verifies this
boundary with a real workload (gVisor + per-tenant AWS credentials/buckets): it asserts a
sandbox on the egress allowlist reaches *only* the emulator (control plane + metadata
blocked), and that no tenant can reach another's server, credentials, or bucket — under
stress, all fail closed and audited.

### Remote endpoint SSRF protection

For `remote_http` servers the gateway makes outbound calls to an admin-supplied
URL. A **dial-time** guard refuses connections to non-public IPs (loopback,
RFC1918, IPv6 ULA, link-local incl. `169.254.169.254`, multicast, unspecified),
so an endpoint can't aim the gateway at internal infrastructure. Checking the
actual connection IP (not just the hostname up front) defeats DNS rebinding.
Enabled by default in production (`MCP_BLOCK_PRIVATE_EGRESS`).

## Secrets

- Stored in **Vault KV v2**, **write-only** from the API's perspective — there is
  no endpoint that echoes a value, and values are never logged.
- Three injection modes: `none`, `org_shared` (one shared downstream), `per_user`
  (a per-`(org,user,server)` instance built lazily with the caller's own secret).
- **Rotation** propagates so the next instance uses the new secret.
- **Redaction**: a log-writer scrubs auth headers, bearer tokens, API keys, and
  `key=value` secrets from every log line regardless of how the field was added —
  defense-in-depth on top of "don't log secrets."

## Audit (tamper-evident + durable)

- Every config/security event is sealed into an append-only **SHA-256 hash
  chain**; any edit/insert/reorder breaks `Verify`.
- Durable **WORM** archive: each record is written once to S3-compatible storage
  under **Object Lock (COMPLIANCE)** with a retention window (default 1 year) —
  tampering is both *detectable* (chain) and *preventable* (object lock).
- **Auth denials** (`auth.denied` on 401, `authz.denied` on RBAC) are audited —
  the cross-tenant-probing signal — with **rate-limiting** so unauthenticated
  floods can't amplify audit writes (drops counted in `mcp_audit_dropped_total`,
  never silent).

## Compliance posture

Targets a **SOC 2** baseline: per-org isolation, least-privilege DB role,
write-only secret handling, tamper-evident + retained audit, and full
observability (logs/metrics/traces) with isolation-denial alerting.

## Threat-model highlights

| Threat | Mitigation |
|---|---|
| Cross-tenant data access | Audience tokens + per-org catalog + Postgres RLS |
| Malicious stdio server escapes | No-network microVM/gVisor, dropped caps, read-only fs, limits |
| SSRF via remote endpoint | Dial-time private-IP block (DNS-rebinding safe) |
| Credential leakage | Vault write-only, log redaction, never echoed |
| Audit tampering | Hash chain + WORM object lock |
| Noisy neighbor / DoS | Per-org & per-user quotas (Redis fleet-wide); audit rate-limit |
| Privilege escalation in DB | Non-superuser app role, `FORCE` RLS |
