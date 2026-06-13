import { useSession } from '../auth/AuthProvider'
import { Button } from '../design-system/components/core/Button'
import { Tile } from '../design-system/components/core/Tile'

export function Forbidden() {
  const { signOut } = useSession()
  return (
    <main style={{ maxWidth: 480, margin: '12vh auto', fontFamily: 'var(--font-sans)', color: 'var(--text-primary)' }}>
      <h1 style={{ fontSize: 'var(--type-scale-06)', marginBottom: 'var(--spacing-05)' }}>Access denied</h1>
      <Tile>
        <p style={{ marginTop: 0 }}>
          Your account does not have administrator access to this organization.
        </p>
        <Button kind="secondary" onClick={() => void signOut()}>
          Sign out
        </Button>
      </Tile>
    </main>
  )
}
