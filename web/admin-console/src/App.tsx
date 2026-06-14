import { AppProviders } from './app/providers'
import { AppRoutes } from './app/routes'
import { DevOrgPicker } from './auth/DevOrgPicker'
import { needsDevOrgChoice } from './auth/org'

// Composition root: providers (router, query cache, auth, notifications) wrap the
// route tree. Feature screens are filled in per user-story phase.
export function App() {
  // Dev-only: on localhost with multiple selectable orgs and none chosen, pick the
  // org (Keycloak realm) before the OIDC client is created. No-op in production.
  if (needsDevOrgChoice()) return <DevOrgPicker />
  return (
    <AppProviders>
      <AppRoutes />
    </AppProviders>
  )
}
