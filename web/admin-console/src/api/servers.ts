import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from './client'

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

// The control-plane serializes empty slices with `omitempty`, so `allowed_roles`
// is absent (undefined) for a server with no role restriction — the default.
// Normalize at the boundary so every consumer can treat it as an array.
function normalizeServer(s: Server): Server {
  return { ...s, allowed_roles: s.allowed_roles ?? [] }
}

/** List the org's MCP servers (org scoping is enforced by the API client). */
export function useServers() {
  return useQuery({
    queryKey: serversKey,
    queryFn: async () => (await api.get<Server[]>('/servers')).map(normalizeServer),
  })
}

export function useServer(id: string) {
  return useQuery({
    queryKey: ['servers', id],
    queryFn: async () => normalizeServer(await api.get<Server>(`/servers/${id}`)),
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

export interface ImportResult {
  slug: string
  ok: boolean
  error?: string
}

/**
 * Create many servers from a parsed `mcpServers` config. Runs sequentially so a
 * duplicate or invalid entry doesn't abort the rest, returning a per-entry
 * outcome the UI can summarize. The list is refreshed once at the end.
 */
export function useImportServers() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (inputs: CreateServerInput[]): Promise<ImportResult[]> => {
      const results: ImportResult[] = []
      for (const input of inputs) {
        try {
          await api.post<Server>('/servers', input)
          results.push({ slug: input.slug, ok: true })
        } catch (e) {
          const error =
            e instanceof ApiError
              ? e.status === 409
                ? 'already exists'
                : `${e.status}: ${e.message}`
              : e instanceof Error
                ? e.message
                : String(e)
          results.push({ slug: input.slug, ok: false, error })
        }
      }
      return results
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: serversKey }),
  })
}
