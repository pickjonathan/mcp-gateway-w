import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import type { User } from 'oidc-client-ts'
import { createUserManager } from './oidc'
import { resolveOrg } from './org'
import { configureApi } from '../api/client'

export interface Session {
  org: string
  userId: string
  displayName: string
  roles: string[]
  isAdmin: boolean
  accessToken: string
}

interface AuthState {
  session: Session | null
  loading: boolean
  signIn: () => Promise<void>
  signOut: () => Promise<void>
  completeSignIn: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

function toSession(user: User): Session {
  const claims = user.profile as Record<string, unknown>
  const realm = claims.realm_access as { roles?: string[] } | undefined
  const roles = realm?.roles ?? []
  const name =
    (claims.name as string | undefined) ??
    (claims.preferred_username as string | undefined) ??
    user.profile.sub
  return {
    org: resolveOrg(),
    userId: user.profile.sub,
    displayName: name,
    roles,
    isAdmin: roles.includes('admin'),
    accessToken: user.access_token,
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const mgr = useMemo(() => createUserManager(), [])
  const [session, setSession] = useState<Session | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let active = true
    const apply = (user: User | null) => {
      const next = user && !user.expired ? toSession(user) : null
      setSession(next)
      configureApi({ token: next?.accessToken ?? null, org: next?.org ?? resolveOrg() })
    }
    mgr
      .getUser()
      .then((u) => {
        if (active) {
          apply(u)
          setLoading(false)
        }
      })
      .catch(() => active && setLoading(false))

    const onLoaded = (u: User) => apply(u)
    const onUnloaded = () => apply(null)
    mgr.events.addUserLoaded(onLoaded)
    mgr.events.addUserUnloaded(onUnloaded)
    return () => {
      active = false
      mgr.events.removeUserLoaded(onLoaded)
      mgr.events.removeUserUnloaded(onUnloaded)
    }
  }, [mgr])

  const value: AuthState = {
    session,
    loading,
    signIn: () => mgr.signinRedirect(),
    signOut: () => mgr.signoutRedirect(),
    completeSignIn: async () => {
      await mgr.signinRedirectCallback()
    },
  }
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useSession(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useSession must be used within AuthProvider')
  return ctx
}
