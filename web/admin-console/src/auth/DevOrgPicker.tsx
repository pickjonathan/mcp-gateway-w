import { useEffect, useState } from 'react'
import { Tile } from '../design-system/components/core/Tile'
import { devOrgs, fetchDevOrgs, setDevOrg } from './org'

// DEV ONLY. Lets a developer choose which org's Keycloak realm the console
// authenticates against on localhost. The list is fetched live from the
// control-plane (so realms created in Keycloak appear automatically), falling
// back to VITE_DEV_ORGS. In production the org comes from the host subdomain and
// this never renders. Selecting only sets the login target — isolation is still
// enforced server-side (the token is realm-bound; the API validates the issuer).
export function DevOrgPicker() {
  const [orgs, setOrgs] = useState<string[] | null>(null)

  useEffect(() => {
    let active = true
    void fetchDevOrgs().then((fetched) => {
      if (!active) return
      const merged = Array.from(new Set([...fetched, ...devOrgs()]))
      if (merged.length <= 1) {
        // Nothing to choose — go straight in.
        setDevOrg(merged[0] ?? 'acme')
        window.location.reload()
        return
      }
      setOrgs(merged)
    })
    return () => {
      active = false
    }
  }, [])

  const choose = (org: string) => {
    setDevOrg(org)
    window.location.reload()
  }

  const wrap = {
    maxWidth: 420,
    margin: '12vh auto',
    fontFamily: 'var(--font-sans)',
    color: 'var(--text-primary)',
  } as const

  if (orgs === null) {
    return <main style={wrap}>Loading organizations…</main>
  }

  return (
    <main style={wrap}>
      <h1 style={{ fontSize: 'var(--type-scale-06)', marginBottom: 'var(--spacing-03)' }}>
        Choose organization
      </h1>
      <p style={{ marginTop: 0, marginBottom: 'var(--spacing-05)', color: 'var(--text-secondary)' }}>
        Developer mode — pick the organization (Keycloak realm) to sign in to. The list is live
        from Keycloak.
      </p>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--spacing-04)' }}>
        {orgs.map((org) => (
          <Tile key={org} variant="clickable" onClick={() => choose(org)}>
            <strong>{org}</strong>
          </Tile>
        ))}
      </div>
    </main>
  )
}
