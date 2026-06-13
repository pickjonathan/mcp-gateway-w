import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useSession } from '../auth/AuthProvider'
import { Loading, ErrorState } from '../app/feedback/states'

export function Callback() {
  const { completeSignIn } = useSession()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    completeSignIn()
      .then(() => navigate('/', { replace: true }))
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)))
  }, [completeSignIn, navigate])

  return (
    <main style={{ maxWidth: 480, margin: '12vh auto', fontFamily: 'var(--font-sans)' }}>
      {error ? <ErrorState message={`Sign-in failed: ${error}`} /> : <Loading label="Completing sign-in…" />}
    </main>
  )
}
