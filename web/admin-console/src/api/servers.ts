import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'

export type ServerType = 'remote_http' | 'stdio'
export type Health = 'healthy' | 'unhealthy' | 'unknown'
export type CredentialMode = 'none' | 'org_shared' | 'per_user'

export interface Server {
  id: string
  slug: string
  type: ServerType
  enabled: boolean
  health: Health
  credential_mode: CredentialMode
  credential_set?: boolean
  allowed_roles: string[]
  endpoint_url?: string
  command?: string
  args?: string[]
  env?: Record<string, string>
  health_detail?: string
  created_at: string
}

export interface CreateServerInput {
  slug: string
  type: ServerType
  endpoint_url?: string
  command?: string
  args?: string[]
  env?: Record<string, string>
  credential_mode: CredentialMode
  allowed_roles: string[]
  enabled: boolean
}

export type UpdateServerInput = Partial<CreateServerInput>

export const serversKey = ['servers'] as const

/** List the org's MCP servers (org scoping is enforced by the API client). */
export function useServers() {
  return useQuery({ queryKey: serversKey, queryFn: () => api.get<Server[]>('/servers') })
}

export function useServer(id: string) {
  return useQuery({
    queryKey: ['servers', id],
    queryFn: () => api.get<Server>(`/servers/${id}`),
    enabled: Boolean(id),
  })
}

export function useCreateServer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateServerInput) => api.post<Server>('/servers', input),
    onSuccess: () => qc.invalidateQueries({ queryKey: serversKey }),
  })
}

export function useUpdateServer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateServerInput }) =>
      api.patch<Server>(`/servers/${id}`, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: serversKey }),
  })
}

export function useDeleteServer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.del<void>(`/servers/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: serversKey }),
  })
}
