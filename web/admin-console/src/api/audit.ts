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

export type ChainStatus = 'verified' | 'tampered'

export interface AuditResponse {
  events: AuditEvent[]
  chain: { status: ChainStatus }
}

export const auditKey = ['audit'] as const

/** Fetch the org's audit events (newest-first) plus the chain-integrity status. */
export function useAudit() {
  return useQuery({ queryKey: auditKey, queryFn: () => api.get<AuditResponse>('/audit') })
}
