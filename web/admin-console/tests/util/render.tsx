import { type ReactElement } from 'react'
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { NotificationsProvider } from '../../src/app/feedback/notifications'

// Render a component with the providers most screens need (query cache + router +
// notifications), without the real AuthProvider — tests set the API org/token via
// configureApi or mock useSession directly.
export function renderWithProviders(ui: ReactElement, opts: { route?: string } = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[opts.route ?? '/']}>
        <NotificationsProvider>{ui}</NotificationsProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}
