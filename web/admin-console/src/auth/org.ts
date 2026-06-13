// Resolve the organization the console is operating in. In production the org is
// the first label of the host (`{org}.{base-domain}`), exactly like the data
// plane; in dev we fall back to VITE_DEV_ORG. This is the single source of org
// context — it must never be overridable by user input (tenant isolation).
export function resolveOrg(host: string = window.location.hostname): string {
  const base = import.meta.env.VITE_BASE_DOMAIN
  if (base && host.endsWith('.' + base)) {
    const sub = host.slice(0, host.length - base.length - 1)
    return sub.split('.')[0]
  }
  return import.meta.env.VITE_DEV_ORG ?? 'acme'
}
