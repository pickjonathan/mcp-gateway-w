import { useQuery } from '@tanstack/react-query'

// Prometheus query API (reachable via the /metrics-api proxy — edge same-origin in
// prod, Vite proxy in dev). Sourced per Clarifications 2026-06-13 ("Query Prometheus").
const METRICS_BASE = '/metrics-api'

interface PromVectorResponse {
  data?: { result?: Array<{ value?: [number, string] }> }
}

async function promQuery(expr: string): Promise<number> {
  const res = await fetch(`${METRICS_BASE}/api/v1/query?query=${encodeURIComponent(expr)}`)
  if (!res.ok) throw new Error(`metrics query failed: ${res.status}`)
  const body = (await res.json()) as PromVectorResponse
  const v = body.data?.result?.[0]?.value?.[1]
  return v ? Number(v) : 0
}

export interface Usage {
  requestRate: number
  denialRate: number
  errorRate: number
  toolErrorRate: number
}

// Request / denial / error rate trends over the last 5 minutes (per second).
export function useUsage() {
  return useQuery({
    queryKey: ['usage'],
    queryFn: async (): Promise<Usage> => {
      const [requestRate, denialRate, errorRate, toolErrorRate] = await Promise.all([
        promQuery('sum(rate(mcp_requests_total[5m]))'),
        promQuery('sum(rate(mcp_requests_total{code=~"401|403"}[5m]))'),
        promQuery('sum(rate(mcp_requests_total{code=~"5.."}[5m]))'),
        promQuery('sum(rate(mcp_tool_calls_total{outcome="error"}[5m]))'),
      ])
      return { requestRate, denialRate, errorRate, toolErrorRate }
    },
    refetchInterval: 15_000,
  })
}
