import { UserManager, WebStorageStateStore, InMemoryWebStorage } from 'oidc-client-ts'
import { resolveOrg } from './org'

// A pre-registered public client using Authorization Code + PKCE against the
// org's Keycloak realm (Clarifications 2026-06-13). Tokens live in memory
// (InMemoryWebStorage) to limit XSS exposure; the short-lived PKCE flow state is
// kept in sessionStorage so it survives the redirect round-trip.
export function createUserManager(): UserManager {
  const org = resolveOrg()
  const issuerTemplate = import.meta.env.VITE_OIDC_ISSUER_TEMPLATE
  if (!issuerTemplate) throw new Error('VITE_OIDC_ISSUER_TEMPLATE is not configured')
  const authority = issuerTemplate.replace('%s', org)

  return new UserManager({
    authority,
    client_id: import.meta.env.VITE_OIDC_CLIENT_ID ?? 'mcp-admin-console',
    redirect_uri: `${window.location.origin}/callback`,
    post_logout_redirect_uri: `${window.location.origin}/signin`,
    response_type: 'code',
    scope: import.meta.env.VITE_OIDC_SCOPE ?? 'openid profile',
    automaticSilentRenew: true,
    userStore: new WebStorageStateStore({ store: new InMemoryWebStorage() }),
    stateStore: new WebStorageStateStore({ store: window.sessionStorage }),
  })
}
