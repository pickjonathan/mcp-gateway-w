import { useQuery } from '@tanstack/react-query'
import { api } from './client'

// Read-only (Clarifications 2026-06-13): the configured per-org/per-user limits.
export interface Quotas {
  org_per_min: number
  user_per_min: number
}

export function useQuotas() {
  return useQuery({ queryKey: ['quotas'], queryFn: () => api.get<Quotas>('/quotas') })
}
