import { useQuery } from '@tanstack/react-query'
import { api } from './client'

export interface AuditEvent {
  seq: number
  time: string
  actor: string
  action: string
  target: string
  metadata?: Record<string, string>
}

export const auditKey = ['audit'] as const

/**
 * Fetch the org's audit events (newest-first). The control-plane returns a bare
 * array of hash-chained records (`GET /v1/orgs/{org}/audit`); chain-integrity
 * verification is not exposed as a field, so the client does not synthesize one.
 */
export function useAudit() {
  return useQuery({ queryKey: auditKey, queryFn: () => api.get<AuditEvent[]>('/audit') })
}
