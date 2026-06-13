#!/usr/bin/env bash
# Install missing prerequisites, build & test the Go services, and bring up the
# dev service stack: Postgres/Redis/Vault/Keycloak + MinIO (audit archive) +
# Prometheus/Grafana/Jaeger (observability). Idempotent.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"
OS="$(uname -s)"
COMPOSE="deploy/dev/compose.yaml"

install_pkg() { # install_pkg <brew-formula> <apt-package>
  if [ "$OS" = "Darwin" ]; then
    command -v brew >/dev/null 2>&1 || { echo "Homebrew required: https://brew.sh" >&2; exit 1; }
    brew install "$1"
  else
    sudo apt-get update -qq && sudo apt-get install -y "$2"
  fi
}

echo "==> Checking / installing prerequisites"
command -v go   >/dev/null 2>&1 || install_pkg go   golang-go
command -v make >/dev/null 2>&1 || install_pkg make make
command -v jq   >/dev/null 2>&1 || install_pkg jq   jq
if ! docker info >/dev/null 2>&1; then
  echo "ERROR: Docker daemon unreachable. Start Docker Desktop (macOS) or install/start docker.io (Linux), then re-run." >&2
  exit 1
fi

echo "==> Building Go services (go build ./...)"
go build ./...

if [ "${SKIP_TESTS:-0}" != "1" ]; then
  echo "==> Running tests (go test ./...; set SKIP_TESTS=1 to skip)"
  go test ./...
fi

echo "==> Bringing up backing services: postgres, redis, vault, keycloak, minio, jaeger"
docker compose -f "$COMPOSE" up -d postgres redis vault keycloak minio jaeger
# Prometheus/Grafana scrape the app targets; --no-deps avoids building the app
# images here (you run them via `make run` / `go run`).
echo "==> Bringing up observability: prometheus, grafana"
docker compose -f "$COMPOSE" up -d --no-deps prometheus grafana

echo "==> Waiting for readiness (up to 120s)"
deadline=$((SECONDS + 120))
infra_ready() {
  docker compose -f "$COMPOSE" exec -T redis    redis-cli ping        >/dev/null 2>&1 &&
  docker compose -f "$COMPOSE" exec -T postgres pg_isready -U mcp     >/dev/null 2>&1
}
until infra_ready; do
  [ "$SECONDS" -lt "$deadline" ] || { echo "  ! redis/postgres not ready in time (continuing)"; break; }
  sleep 3
done
echo "  redis + postgres: ready"
curl -fsS http://localhost:9000/minio/health/live >/dev/null 2>&1 && echo "  minio: ready" || echo "  ! minio still starting (continuing)"
until curl -fsS http://localhost:8081/realms/master >/dev/null 2>&1; do
  [ "$SECONDS" -lt "$deadline" ] || { echo "  ! keycloak still starting (it can take a minute; continuing)"; break; }
  sleep 3
done
curl -fsS http://localhost:8081/realms/master >/dev/null 2>&1 && echo "  keycloak: ready" || true

echo
echo "==> Dev stack:"
docker compose -f "$COMPOSE" ps
cat <<'EOF'

Done. To run the services locally:
  make run                                   # gateway on :8080 (uses exec sandbox = UNSANDBOXED dev)
  go run ./services/control-plane/cmd/control-plane   # control-plane on :8090 (set MCP_HTTP_ADDR=:8090)

Dev UIs:
  Grafana     http://localhost:3000   (dashboard: "MCP Runtime Overview"; anon admin)
  Prometheus  http://localhost:9090   (alerts under Status > Rules)
  Jaeger      http://localhost:16686  (traces; set MCP_OTLP_ENDPOINT=localhost:4318 when running apps)
  MinIO       http://localhost:9001   (console; audit archive bucket "mcp-audit"; user/pass minioadmin)
  Keycloak    http://localhost:8081   (admin/admin)

Note: Postgres RLS is enforced only for the non-superuser `mcp_app` role, created
by deploy/dev/postgres-init on a FRESH data dir. After changing that init, run
`make dev-down && <this script>` to recreate Postgres.

For the real HC-3 gVisor sandbox, run: .claude/skills/dev-setup/scripts/sandbox-up.sh
EOF
