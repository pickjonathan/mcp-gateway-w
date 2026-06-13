import { type ReactNode } from 'react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from '../api/queryClient'
import { AuthProvider } from '../auth/AuthProvider'
import { NotificationsProvider } from './feedback/notifications'

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <BrowserRouter>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <NotificationsProvider>{children}</NotificationsProvider>
        </AuthProvider>
      </QueryClientProvider>
    </BrowserRouter>
  )
}
