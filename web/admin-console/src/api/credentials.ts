import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import { serversKey } from './servers'

export type CredentialScope = 'org' | 'me'

function credPath(serverId: string, scope: CredentialScope): string {
  return scope === 'org'
    ? `/servers/${serverId}/credentials`
    : `/servers/${serverId}/credentials/me`
}

// Write-only: values are PUT and never read back. Success invalidates the servers
// query so the credential_set status refreshes (org scope).
export function useSetCredential(serverId: string, scope: CredentialScope) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (values: Record<string, string>) => api.put<void>(credPath(serverId, scope), values),
    onSuccess: () => qc.invalidateQueries({ queryKey: serversKey }),
  })
}

export function useClearCredential(serverId: string, scope: CredentialScope) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.del<void>(credPath(serverId, scope)),
    onSuccess: () => qc.invalidateQueries({ queryKey: serversKey }),
  })
}
