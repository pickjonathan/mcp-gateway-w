// Resolve the organization the console is operating in. In production the org is
// the first label of the host (`{org}.{base-domain}`), exactly like the data
// plane — non-overridable, the single source of org context (tenant isolation).
//
// In dev (served from localhost, where there is no `{org}.{base}` subdomain) the
// org comes from an explicit developer selection (the DevOrgPicker), falling back
// to VITE_DEV_ORG. The picker is a convenience only: it just chooses which realm
// the console authenticates against. Isolation is still enforced server-side — the
// issued token is realm-bound and the control-plane validates the issuer per
// request, so "selecting" an org you have no credentials in gets you nothing.
const DEV_ORG_KEY = 'mcp.devOrg'

export function resolveOrg(host: string = window.location.hostname): string {
  if (!isDevHost(host)) {
    const base = import.meta.env.VITE_BASE_DOMAIN as string
    const sub = host.slice(0, host.length - base.length - 1)
    return sub.split('.')[0]
  }
  return selectedDevOrg() ?? import.meta.env.VITE_DEV_ORG ?? 'acme'
}

// isDevHost reports whether the console is NOT served from a real
// `{org}.{base-domain}` host — i.e. the dev picker applies. Production hosts
// always derive the org from the subdomain and ignore the picker entirely.
export function isDevHost(host: string = window.location.hostname): boolean {
  const base = import.meta.env.VITE_BASE_DOMAIN
  return !(base && host.endsWith('.' + base))
}

// devOrgs lists the selectable orgs in dev, from VITE_DEV_ORGS (CSV), falling
// back to VITE_DEV_ORG (or 'acme'). Trimmed + de-duplicated.
export function devOrgs(): string[] {
  const csv = import.meta.env.VITE_DEV_ORGS ?? import.meta.env.VITE_DEV_ORG ?? 'acme'
  return Array.from(
    new Set(
      csv
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
    ),
  )
}

// selectedDevOrg returns the stored dev selection, if any. It is NOT validated
// against the static VITE_DEV_ORGS list — the picker offers realms discovered live
// from Keycloak (which need not be in env), and the realm is validated server-side
// on the OIDC flow. A bogus value simply fails auth (recoverable via "Switch org").
export function selectedDevOrg(): string | null {
  try {
    return window.localStorage.getItem(DEV_ORG_KEY) || null
  } catch {
    return null
  }
}

export function setDevOrg(org: string): void {
  try {
    window.localStorage.setItem(DEV_ORG_KEY, org)
  } catch {
    /* ignore (private mode) */
  }
}

export function clearDevOrg(): void {
  try {
    window.localStorage.removeItem(DEV_ORG_KEY)
  } catch {
    /* ignore */
  }
}

// needsDevOrgChoice is true when the picker should run: a dev host with nothing
// chosen yet. The picker fetches the live realm list and auto-selects when there
// is only one, so this stays a cheap synchronous gate.
export function needsDevOrgChoice(): boolean {
  return isDevHost() && selectedDevOrg() === null
}

// fetchDevOrgs queries the control-plane's dev endpoint for the realms that exist
// in Keycloak, so realms created by hand appear without editing env. Returns [] on
// any failure — the picker then falls back to VITE_DEV_ORGS. Dev-only: the
// endpoint 404s in prod, and prod never resolves the org via the picker anyway.
export async function fetchDevOrgs(): Promise<string[]> {
  try {
    const base = import.meta.env.VITE_API_BASE ?? ''
    const res = await fetch(`${base}/v1/dev/orgs`, { headers: { accept: 'application/json' } })
    if (!res.ok) return []
    const body = (await res.json()) as { orgs?: string[] }
    return Array.isArray(body.orgs) ? body.orgs : []
  } catch {
    return []
  }
}
