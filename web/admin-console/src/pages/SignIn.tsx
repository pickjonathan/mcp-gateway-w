import { Navigate } from 'react-router-dom'
import { useSession } from '../auth/AuthProvider'
import { Button } from '../design-system/components/core/Button'
import { Tile } from '../design-system/components/core/Tile'
import { clearDevOrg, isDevHost, resolveOrg } from '../auth/org'

export function SignIn() {
  const { signIn, session, loading } = useSession()
  if (!loading && session) return <Navigate to="/" replace />
  const dev = isDevHost()
  const switchOrg = () => {
    clearDevOrg()
    window.location.reload()
  }
  return (
    <main
      style={{
        maxWidth: 380,
        margin: '12vh auto',
        fontFamily: 'var(--font-sans)',
        color: 'var(--text-primary)',
      }}
    >
      <h1 style={{ fontSize: 'var(--type-scale-06)', marginBottom: 'var(--spacing-05)' }}>
        MCP Admin Console
      </h1>
      <Tile>
        <p style={{ marginTop: 0 }}>
          Sign in with your organization administrator account
          {dev ? (
            <>
              {' '}
              for <strong>{resolveOrg()}</strong>
            </>
          ) : null}
          .
        </p>
        <Button kind="primary" onClick={() => void signIn()}>
          Sign in
        </Button>
        {dev && (
          <Button kind="ghost" size="sm" onClick={switchOrg}>
            Choose a different organization
          </Button>
        )}
      </Tile>
    </main>
  )
}
