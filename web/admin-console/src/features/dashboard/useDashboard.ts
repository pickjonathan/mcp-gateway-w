import { useServers } from '../../api/servers'
import { useAudit, type AuditEvent } from '../../api/audit'

export interface DashboardSummary {
  serverCount: number
  health: { healthy: number; unhealthy: number; unknown: number }
  recent: AuditEvent[]
  denials: number
}

const DENIAL_ACTIONS = new Set(['auth.denied', 'authz.denied'])

// Composes the dashboard summary from the servers + audit APIs (no new backend).
// Live request/denial/error RATE charts are sourced from Prometheus in US6.
export function useDashboard() {
  const serversQ = useServers()
  const auditQ = useAudit()

  const servers = serversQ.data ?? []
  const events = auditQ.data ?? []
  const health = { healthy: 0, unhealthy: 0, unknown: 0 }
  for (const s of servers) health[s.health] += 1

  const summary: DashboardSummary = {
    serverCount: servers.length,
    health,
    recent: events.slice(0, 5),
    denials: events.filter((e) => DENIAL_ACTIONS.has(e.action)).length,
  }
  return {
    summary,
    isLoading: serversQ.isLoading || auditQ.isLoading,
    error: serversQ.error ?? auditQ.error,
  }
}
