import { type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useSession } from './AuthProvider'
import { Loading } from '../app/feedback/states'

// Route guard: unauthenticated → /signin; authenticated non-admin → /forbidden
// (Constitution I — never expose org data to a non-admin).
export function RequireAdmin({ children }: { children: ReactNode }) {
  const { session, loading } = useSession()
  if (loading) return <Loading label="Checking access…" />
  if (!session) return <Navigate to="/signin" replace />
  if (!session.isAdmin) return <Navigate to="/forbidden" replace />
  return <>{children}</>
}
