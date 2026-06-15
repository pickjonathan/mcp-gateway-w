#!/usr/bin/env bash
# Open the isolation-proof dashboard (report.html) + Grafana live metrics.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
REPORT="$ROOT/tests/isolation-proof/report.html"
GRAFANA="http://localhost:3000"

if [ ! -f "$REPORT" ]; then
  echo "No report yet at $REPORT"
  echo "Run the proof first:  make prove-isolation"
  exit 1
fi

open_url() {
  case "$(uname -s)" in
    Darwin) open "$1" >/dev/null 2>&1 || true ;;
    Linux)  xdg-open "$1" >/dev/null 2>&1 || true ;;
    *)      echo "Open manually: $1" ;;
  esac
}

echo "Proof dashboard : $REPORT"
echo "Grafana (live)  : $GRAFANA  (dashboard: \"MCP Runtime Overview\")"
open_url "$REPORT"
# Only try Grafana if the dev stack is up.
if curl -fsS -o /dev/null "$GRAFANA/api/health" 2>/dev/null; then
  open_url "$GRAFANA"
else
  echo "(Grafana not reachable — start the stack with 'make dev-up' to see live metrics)"
fi
