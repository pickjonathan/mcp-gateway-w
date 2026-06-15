# Quickstart: Multi-Tenant MCP Server Runtime

**Date**: 2026-06-13 | **Plan**: [plan.md](./plan.md)

Local development bring-up and the acceptance-scenario walkthroughs. Goal: prove the core loop (add a server → connect a client → call a tool) and the isolation guarantee end-to-end on a laptop, using a single-node sandbox runtime.

## Prerequisites

- Docker + a Firecracker/Kata-capable Linux host (or a Linux VM with nested virtualization) for the sandbox node. macOS dev: run the sandbox node in a Linux VM; the rest runs in Docker.
- Go 1.23+, `mcp` inspector or any MCP client (e.g. an IDE MCP client), `jq`.

## 1. Bring up the platform (control + data plane)

```bash
# from repo root
docker compose -f deploy/dev/compose.yaml up -d   # keycloak, postgres, redis, vault, envoy, gateway, control-plane
make migrate                                       # apply Postgres migrations
make seed-org SLUG=acme                            # creates org 'acme' + Keycloak realm 'acme' + an admin user
```

Bring up the sandbox plane on the Linux node:

```bash
make sandbox-node            # builds base rootfs (Node/npx + Python/uv + registry cache), starts sandbox-supervisor
```

Edit `/etc/hosts` (dev): `127.0.0.1 acme.withwillow.ai api.withwillow.ai`.

## 2. US2 — add a remote HTTP server (P1, HC-2)

```bash
curl -sX POST https://api.withwillow.ai/v1/orgs/acme/servers \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'content-type: application/json' \
  -d '{"slug":"echo","type":"remote_http","endpoint_url":"https://example.com/mcp"}' | jq
# → 201, health=unknown → (poll) → healthy
```

**Expect**: usable by a permitted user within seconds, no redeploy (SC-005). Unreachable URL → `health=unreachable`, isolated (FR-019).

## 3. US3 — add a stdio server, arbitrary command (P1, HC-3)

```bash
curl -sX POST https://api.withwillow.ai/v1/orgs/acme/servers \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'content-type: application/json' \
  -d '{"slug":"think","type":"stdio","command":"npx",
       "args":["-y","@modelcontextprotocol/server-sequential-thinking"],
       "credential_mode":"none"}' | jq
```

**Expect**: supervisor launches a microVM, runs `npx …` behind default-deny egress (registry mirror allowed), completes the MCP handshake, `health=healthy` (US3 scenario 1). Idle → reclaimed; next use → transparent restart (FR-018).

## 4. US1 — connect a client and call a tool (P1)

1. Point an MCP client at `https://acme.withwillow.ai/mcp`.
2. Client gets `401` → discovers AS (`/.well-known/oauth-protected-resource` → realm metadata) → Authorization Code + **PKCE** → token (`aud=https://acme.withwillow.ai/mcp`).
3. `tools/list` → shows `echo__*` and `think__*` (namespaced, RBAC-filtered).
4. Call `think__sequentialthinking` → result streamed back through the gateway.

**Expect**: tools appear < 2 s (SC-010); proxy overhead < 150 ms (SC-011); cold stdio < 5 s, warm < 500 ms (SC-012).

## 5. RBAC + credentials (US5/US6)

```bash
# Scope 'think' to role 'engineers'
curl -sX PUT …/servers/$THINK_ID/permissions -d '[{"principal_type":"role","principal_id":"engineers","allowed_tools":"*"}]'
# Store an org-level secret for a server that needs one (write-only)
curl -sX PUT …/servers/$ID/credentials -d '{"API_KEY":"…"}'
```

**Expect**: only `engineers` see `think`; revoke → disappears on next list/call (FR-022). Secret never appears in logs/responses (FR-015).

## 6. US4 — isolation acceptance (HARD, must pass)

Run the adversarial suite:

```bash
make test-security    # provisions org-A/org-B + 2 users each; runs hostile MCP servers
```

**Must hold (SC-001/002/003)**:
- Org A user cannot enumerate/reach org B servers, secrets, or sessions → all blocked, `404`/`403`, audited.
- A hostile sandbox attempting egress to `169.254.169.254`, internal IPs, the control plane, or other tenants' data → all blocked, contained to its microVM.
- A token with `aud` for org A replayed against org B's endpoint → rejected.
- Disable a misbehaving server → stops serving within 5 s, other servers unaffected (SC-004).

> **Deeper, real-workload gate**: the [Two-Tenant AWS-MCP Isolation Proof](../../docs/isolation-proof.md)
> (`make prove-isolation`, spec `004-aws-mcp-isolation-proof`) exercises this same boundary end-to-end
> with two tenants each running a credentialed stdio AWS MCP server under gVisor against a local
> `ministack`, including a stress run and sandbox egress-containment checks. Run locally; CI wiring deferred.

## 7. Validation matrix

| Spec item | Step |
|---|---|
| FR-001/002, US1 | §4 |
| FR-005, US2 | §2 |
| FR-006/013/014, US3, HC-3 | §3, §6 |
| FR-009/022, US5 | §5 |
| FR-015/016, US6 | §5 |
| FR-011/012, US4, SC-001/002/003 | §6 |
| SC-010/011/012 (perf) | §4 under load (`make load`) |
