import { Tile } from '../design-system/components/core/Tile'
import { devOrgs, setDevOrg } from './org'

// DEV ONLY. On localhost (where the org can't be derived from a subdomain) this
// lets a developer choose which organization's Keycloak realm the console
// authenticates against. It only selects the login target — isolation is enforced
// server-side (the token is realm-bound; the control-plane validates the issuer).
// In production the org always comes from the host subdomain and this never shows.
export function DevOrgPicker() {
  const orgs = devOrgs()
  const choose = (org: string) => {
    setDevOrg(org)
    window.location.reload()
  }
  return (
    <main
      style={{
        maxWidth: 420,
        margin: '12vh auto',
        fontFamily: 'var(--font-sans)',
        color: 'var(--text-primary)',
      }}
    >
      <h1 style={{ fontSize: 'var(--type-scale-06)', marginBottom: 'var(--spacing-03)' }}>
        Choose organization
      </h1>
      <p style={{ marginTop: 0, marginBottom: 'var(--spacing-05)', color: 'var(--text-secondary)' }}>
        Developer mode — pick the organization (Keycloak realm) to sign in to.
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
