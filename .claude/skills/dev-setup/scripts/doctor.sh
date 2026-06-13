#!/usr/bin/env bash
# Read-only prerequisite & environment check for the multi-tenant MCP runtime.
# Installs nothing; safe to run anytime. Exit code = number of missing required tools.
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT" || exit 1
OS="$(uname -s)"; ARCH="$(uname -m)"
COMPOSE="deploy/dev/compose.yaml"

red=$'\033[31m'; grn=$'\033[32m'; yel=$'\033[33m'; dim=$'\033[2m'; rst=$'\033[0m'
req_missing=0
ok()   { printf "  ${grn}OK${rst}   %-10s ${dim}%s${rst}\n" "$1" "${2:-}"; }
warn() { printf "  ${yel}WARN${rst} %-10s %s\n" "$1" "${2:-}"; }
miss() { printf "  ${red}MISS${rst} %-10s %s\n" "$1" "${2:-}"; req_missing=$((req_missing + 1)); }
ver_ge() { [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" = "$2" ]; }

echo "MCP runtime · environment doctor  (${OS}/${ARCH})"
echo "repo: ${REPO_ROOT}"
echo
echo "Required (build & run):"

if command -v go >/dev/null 2>&1; then
  gv="$(go version | awk '{print $3}' | sed 's/^go//')"
  if ver_ge "$gv" "1.25"; then ok go "$gv"; else warn go "$gv (want >= 1.25)"; fi
else
  miss go "not found  → macOS: brew install go · Linux: apt-get install golang-go"
fi

command -v git  >/dev/null 2>&1 && ok git  "$(git --version | awk '{print $3}')"  || miss git  "not found"
command -v make >/dev/null 2>&1 && ok make "present"                                || miss make "not found"

if command -v docker >/dev/null 2>&1; then
  if docker info >/dev/null 2>&1; then
    ok docker "$(docker version --format '{{.Server.Version}}' 2>/dev/null) (daemon up)"
  else
    miss docker "CLI present but daemon unreachable → start Docker Desktop / dockerd"
  fi
else
  miss docker "not found → Docker Desktop (macOS) / docker.io (Linux)"
fi

echo
echo "Optional helpers:"
for t in jq curl; do
  command -v "$t" >/dev/null 2>&1 && ok "$t" "$(command -v "$t")" || warn "$t" "not found (optional)"
done

echo
echo "Sandbox / HC-3 (only for the gVisor stdio sandbox):"
if [ "$OS" = "Darwin" ]; then
  if command -v limactl >/dev/null 2>&1; then
    ok lima "$(limactl --version 2>/dev/null | awk '{print $3}')"
    vm="${MCP_LIMA_VM:-mcp}"
    if limactl list -q 2>/dev/null | grep -qx "$vm"; then
      ok "vm:$vm" "$(limactl list "$vm" --format '{{.Status}}' 2>/dev/null)"
    else
      warn "vm:$vm" "not created yet (run scripts/sandbox-up.sh)"
    fi
  else
    warn lima "not found → brew install lima (needed only for the sandbox)"
  fi
else
  command -v runsc >/dev/null 2>&1 && ok gvisor "$(runsc --version 2>/dev/null | head -1)" || warn gvisor "runsc not found (needed only for the sandbox)"
fi

echo
echo "Project:"
[ -f go.mod ]     && ok   go.mod  "$(head -1 go.mod)" || miss go.mod  "not found at repo root"
[ -f "$COMPOSE" ] && ok   compose "$COMPOSE"          || miss compose "$COMPOSE missing"

if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1 && [ -f "$COMPOSE" ]; then
  echo
  echo "Dev services:"
  docker compose -f "$COMPOSE" ps 2>/dev/null | sed 's/^/  /' || true
fi

echo
if [ "$req_missing" -eq 0 ]; then
  echo "${grn}All required prerequisites present.${rst}  Next: scripts/setup.sh (services) · scripts/sandbox-up.sh (gVisor)."
else
  echo "${red}${req_missing} required prerequisite(s) missing.${rst}  Run scripts/setup.sh to install."
fi
exit "$req_missing"
