# Running the real isolation boundary (HC-3) locally on macOS

The `exec` sandbox backend is **unsandboxed** (dev only). To run the actual HC-3
boundary on a Mac, run a **Linux Docker host in a VM** and use the `gvisor`
backend. **gVisor needs Linux but not nested virtualization** (its `systrap`
platform is user-space), so this works on any Apple Silicon Mac.

For full microVM fidelity use **Kata/Firecracker**, which needs **KVM = nested
virtualization** — supported on **M3+/macOS 15+** (e.g. this M4 host).

## Quick start — gVisor via Colima (recommended)

```bash
brew install colima docker
colima start --cpu 4 --memory 6        # Linux VM with a Docker daemon

# Install gVisor (runsc) inside the VM and register it as a Docker runtime:
colima ssh <<'EOF'
ARCH=$(uname -m)
URL="https://storage.googleapis.com/gvisor/releases/release/latest/${ARCH}"
sudo wget -qO /usr/local/bin/runsc "${URL}/runsc"
sudo wget -qO /usr/local/bin/containerd-shim-runsc-v1 "${URL}/containerd-shim-runsc-v1"
sudo chmod 0755 /usr/local/bin/runsc /usr/local/bin/containerd-shim-runsc-v1
sudo runsc install                      # adds the "runsc" runtime to /etc/docker/daemon.json
sudo systemctl restart docker || sudo service docker restart
EOF

# Confirm gVisor is active (dmesg reports the gVisor kernel):
docker run --rm --runtime=runsc alpine dmesg | grep -i gvisor

# Build the base sandbox image:
docker build -t acme/mcp-sandbox:dev deploy/sandbox-images
```

Point the gateway at it:

```bash
export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"   # colima's daemon
MCP_SANDBOX_RUNTIME=gvisor MCP_SANDBOX_IMAGE=acme/mcp-sandbox:dev make run
```

Now a stdio server (e.g. `npx -y @modelcontextprotocol/server-sequential-thinking`)
added via the control plane launches as `docker run --runtime=runsc --network none
--read-only --cap-drop ALL --pids-limit 256 ...` — real gVisor isolation plus
default-deny egress and resource limits.

## Sandbox egress allowlist (`MCP_SANDBOX_EGRESS_NETWORK`)

By default a sandbox runs `--network none` (default-deny egress). To let it reach exactly
one dependency — e.g. the local AWS emulator used by the
[two-tenant isolation proof](isolation-proof.md) — set **`MCP_SANDBOX_EGRESS_NETWORK`** to
a Docker network whose only members are the allowed targets:

```bash
MCP_SANDBOX_RUNTIME=gvisor MCP_SANDBOX_EGRESS_NETWORK=mcp-sandbox-egress make run
```

Make that network `internal: true` (no external route → no cloud metadata, no internet, no
control plane). Empty (the default) keeps `--network none`, byte-for-byte unchanged. This is
the explicit-allowlist half of default-deny egress: the sandbox can reach **only** that
network's members. (`make run-gateway-proof` sets both vars for you.)

## Full microVM fidelity — Kata (nested virt)

On this M4 (nested virt available), install Kata Containers in the Linux VM,
register the `kata` Docker runtime, and set `MCP_SANDBOX_RUNTIME=kata`. Heavier
setup; same `ContainerRuntime` code path.

## minikube alternative

`minikube start && minikube addons enable gvisor` installs the `gvisor`
`RuntimeClass` (k8s deployment model). Our `ContainerRuntime` uses `docker run`,
so Colima/Docker is the direct match; the minikube path fits the k8s
`RuntimeClass` deployment (T047 prod).

## Adversarial containment tests (US4 / T043)

Run the hostile-MCP isolation suite **inside this Linux VM** (it requires a real
sandbox): attempt egress to `169.254.169.254` / RFC-1918 / the control plane,
fork bombs, and secret/host access — all must be contained to the sandbox. These
cannot run on the macOS host.
