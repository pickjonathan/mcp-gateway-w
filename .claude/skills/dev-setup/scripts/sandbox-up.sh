#!/usr/bin/env bash
# Provision the gVisor stdio-sandbox environment (the HC-3 isolation boundary).
# macOS: a Lima Docker VM + gVisor (runsc). gVisor needs Linux but NOT nested
# virtualization, so this works on any Apple Silicon Mac. Idempotent.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"
OS="$(uname -s)"
VM="${MCP_LIMA_VM:-mcp}"
IMAGE="${MCP_SANDBOX_IMAGE:-acme/mcp-sandbox:dev}"

if [ "$OS" != "Darwin" ]; then
  cat <<'EOF'
This script targets macOS (Lima). On a Linux host, install gVisor directly:
  ARCH=$(uname -m); URL="https://storage.googleapis.com/gvisor/releases/release/latest/${ARCH}"
  sudo wget -qO /usr/local/bin/runsc "${URL}/runsc"
  sudo wget -qO /usr/local/bin/containerd-shim-runsc-v1 "${URL}/containerd-shim-runsc-v1"
  sudo chmod 0755 /usr/local/bin/runsc /usr/local/bin/containerd-shim-runsc-v1
  sudo runsc install && sudo systemctl restart docker
Then set MCP_SANDBOX_RUNTIME=gvisor.
EOF
  exit 0
fi

command -v limactl >/dev/null 2>&1 || { echo "==> installing lima"; brew install lima; }

if ! limactl list -q 2>/dev/null | grep -qx "$VM"; then
  echo "==> Creating Lima Docker VM '$VM' (downloads an image + boots — several minutes)"
  limactl start --name "$VM" template://docker --tty=false
elif [ "$(limactl list "$VM" --format '{{.Status}}' 2>/dev/null)" != "Running" ]; then
  echo "==> Starting Lima VM '$VM'"
  limactl start "$VM"
else
  echo "==> Lima VM '$VM' already running"
fi

echo "==> Installing rootful Docker + gVisor inside the VM (idempotent)"
limactl shell "$VM" -- sudo bash -c '
  set -e
  export DEBIAN_FRONTEND=noninteractive
  command -v docker >/dev/null 2>&1 || { apt-get update -qq && apt-get install -y -qq docker.io wget; }
  systemctl enable --now docker >/dev/null 2>&1 || true
  systemctl restart docker
  sleep 2
  if ! docker info --format "{{json .Runtimes}}" | grep -q runsc; then
    ARCH=$(uname -m)
    URL="https://storage.googleapis.com/gvisor/releases/release/latest/${ARCH}"
    wget -qO /usr/local/bin/runsc "${URL}/runsc"
    wget -qO /usr/local/bin/containerd-shim-runsc-v1 "${URL}/containerd-shim-runsc-v1"
    chmod 0755 /usr/local/bin/runsc /usr/local/bin/containerd-shim-runsc-v1
    /usr/local/bin/runsc install
    systemctl restart docker
    sleep 4
  fi
'

echo "==> Verifying gVisor is active"
limactl shell "$VM" -- sudo docker run --rm --runtime=runsc alpine dmesg 2>/dev/null | grep -i gvisor | head -1 \
  || echo "  (no gVisor dmesg line — check runsc registration)"

echo "==> Building base sandbox image '$IMAGE' in the VM"
if limactl shell "$VM" -- test -d "$REPO_ROOT/deploy/sandbox-images" 2>/dev/null; then
  limactl shell "$VM" -- sudo docker build -t "$IMAGE" "$REPO_ROOT/deploy/sandbox-images" \
    || echo "  ! image build failed (repo may not be mounted in the VM) — build it inside the VM manually"
else
  echo "  ! repo not visible inside the VM; copy deploy/sandbox-images/ in and build there"
fi

cat <<EOF

gVisor sandbox ready in VM '$VM'.
  Run a sandboxed server manually:
    limactl shell $VM -- sudo docker run --rm --runtime=runsc --network none \\
      --read-only --tmpfs /tmp --cap-drop ALL --pids-limit 256 --memory 512m $IMAGE <cmd> <args>

  Note: gVisor lives in the VM's *rootful* docker. To drive it from our gateway,
  run the gateway/sandbox-supervisor inside the VM (MCP_SANDBOX_RUNTIME=gvisor),
  or forward the rootful docker socket. See docs/local-sandbox.md.

  Stop / remove the VM:  limactl stop $VM   ·   limactl delete $VM
EOF
