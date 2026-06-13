# Observability

Three signals, both services, demonstrable locally.

## Logs

- Structured JSON via **zerolog** (`pkg/logging`), configured from the config
  singleton (`MCP_LOG_LEVEL`, `MCP_LOG_FORMAT`).
- A **redacting writer** scrubs secrets (auth headers, bearer tokens, API keys,
  `key=value` env secrets) from every line — so a credential cannot leak even if
  mistakenly passed as a log field.

## Metrics

- OpenTelemetry meter exported in **Prometheus** format at `GET /metrics` on both
  the gateway and control plane (`pkg/telemetry`).
- Instruments:
  - `mcp_requests_total{method,code}` — every HTTP request.
  - `mcp_tool_calls_total{outcome}` — `tools/call` dispatches (`ok`/`error`).
  - `mcp_audit_dropped_total{action}` — audit events suppressed by the
    anti-amplification rate limiter (so suppression is observable, never silent).
- `/metrics` is unauthenticated by design — scrape it from inside the mesh, never
  expose it publicly.

## Traces

- OpenTelemetry tracing with an **OTLP/HTTP** exporter (`MCP_OTLP_ENDPOINT`); when
  unset, spans are still created but not exported (instrumentation always live).
- A **server span per request** on both services records method, route, status,
  and `mcp.org`, and marks errors.
- **W3C trace-context propagation**: the gateway injects `traceparent` into remote
  downstream calls, so a trace flows admin → (Redis) → gateway → downstream.

## Dashboards & alerts (Grafana + Prometheus)

- **Grafana** is provisioned (datasource + dashboard, no manual setup). Dashboard
  **"MCP Runtime Overview"**: request rate by code, auth/isolation denials,
  `tools/call` by outcome, targets-up.
- **Prometheus alert rules** (`deploy/dev/alerts.yml`):
  - `GatewayTargetDown` / `ControlPlaneTargetDown` — scrape target down.
  - `HighServerErrorRate` — >5% 5xx.
  - `HighAuthDenialRate` — sustained 401/403 (isolation-denial / probing signal).
  - `HighToolCallErrorRate` — >20% tool-call failures (downstream or quota).

## Local endpoints

| UI | URL | Notes |
|---|---|---|
| Grafana | http://localhost:3000 | dashboard "MCP Runtime Overview" (anon admin in dev) |
| Prometheus | http://localhost:9090 | alerts under Status → Rules |
| Jaeger | http://localhost:16686 | set `MCP_OTLP_ENDPOINT=localhost:4318` when running apps |

Bring it all up with the `/dev-setup` skill (see **[Local Dev & Runbook](local-dev.md)**).

## Example queries

```promql
sum by (code) (rate(mcp_requests_total[5m]))               # request rate by status
sum(rate(mcp_requests_total{code=~"401|403"}[5m]))         # auth/isolation denials
sum by (outcome) (rate(mcp_tool_calls_total[5m]))          # tool-call success vs error
rate(mcp_audit_dropped_total[5m])                          # audit suppression (should be ~0)
```
