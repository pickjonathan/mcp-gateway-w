import { AppProviders } from './app/providers'
import { AppRoutes } from './app/routes'

// Composition root: providers (router, query cache, auth, notifications) wrap the
// route tree. Feature screens are filled in per user-story phase.
export function App() {
  return (
    <AppProviders>
      <AppRoutes />
    </AppProviders>
  )
}
