# Contract: Sandbox Egress Allowlist (the one production change)

The only change to shipping code. **Additive, default-off**, and it implements Constitution II's
"default-deny **with an explicit allowlist**."

## Config

| Key | Type | Default | Behavior |
|---|---|---|---|
| `MCP_SANDBOX_EGRESS_NETWORK` | string | `""` | `""` → sandbox runs `--network none` (today's behavior, unchanged). Non-empty → sandbox runs `--network <value>`. |

`pkg/config/config.go`: load into `Config.SandboxEgressNetwork` (alongside `SandboxRuntime`, `SandboxImage`).

## Wiring

- `services/gateway/internal/sandbox/exec.go` `Select(name, image, log)` → also take/read the egress
  network and set `ContainerRuntime{ Image: image, OCIRuntime: ..., Network: egressNetwork }`.
- `services/gateway/cmd/gateway/main.go` → pass `cfg.SandboxEgressNetwork` into `Select(...)`.
- `services/gateway/internal/sandbox/container.go` `buildArgs` — **unchanged**: already does
  `network := r.Network; if network == "" { network = "none" }` → `--network <network>`.

## Invariants (MUST)

1. **Default unchanged**: with the key unset/empty, the emitted `docker run` contains `--network none`
   (byte-for-byte identical to today). No existing deployment changes behavior.
2. **Allowlist = exactly the named network**: when set, the sandbox can reach only members of that Docker
   network. For the proof the network is `mcp-sandbox-egress` (`internal: true`, member: `ministack`).
3. **No metadata / no internet**: an `internal: true` Docker network has no external route, so
   `169.254.169.254` (cloud metadata), the control plane, Vault, Postgres, and the public internet are
   **unreachable** from the sandbox.
4. **All other hardening preserved**: `--read-only`, `--tmpfs /tmp`, `--cap-drop ALL`,
   `--security-opt no-new-privileges`, `--pids-limit`, `--memory` unchanged.

## Docker network (proof infra; `deploy/dev/compose.yaml`)

```yaml
networks:
  mcp-sandbox-egress:
    internal: true            # no external connectivity (no metadata, no internet)
services:
  ministack:
    image: ministackorg/ministack
    environment: { GATEWAY_PORT: "4566", SERVICES: "s3,iam,sts" }
    ports: ["4566:4566"]      # published for harness/control-plane; sandboxes use the internal network
    networks: [ mcp-runtime-dev, mcp-sandbox-egress ]
```

> Deployment note (D4): when sandboxes run in the **Lima/gVisor Docker daemon**, `ministack` and
> `mcp-sandbox-egress` must exist **in that same daemon** so `--network mcp-sandbox-egress` resolves for
> the sandbox container. Quickstart documents this.

## Adversarial tests (Principle V — written first, must fail before the `Select()` change)

`services/gateway/internal/sandbox/egress_test.go`:
- **Unit (hermetic)**: `Select` with empty egress → `ContainerRuntime.Network == ""` → `buildArgs`
  contains `--network none`. `Select` with `"mcp-sandbox-egress"` → `--network mcp-sandbox-egress`.
- **Live (gated `MCP_TEST_SANDBOX_DOCKER`)**: launch a probe command in the sandbox on the egress network
  and assert: `curl http://ministack:4566` (or TCP connect) **succeeds**; connect to control-plane host,
  to `169.254.169.254:80`, and to a public host **all fail/timeout**. Mirrors SC-010 / matrix V5.
