// Typed control-plane API client. Every request is org-scoped (`/v1/orgs/{org}/…`)
// and carries the bearer token. In dev the Vite proxy serves `/v1` same-origin;
// in prod the edge routes it. The org/token are set once by the AuthProvider.

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

/**
 * Resolves the bearer for a request. `force: true` asks the provider to refresh
 * (renew) the token rather than return a cached one — used on a 401 retry.
 */
export type TokenProvider = (opts?: { force?: boolean }) => Promise<string | null>

let getToken: TokenProvider = async () => null
let org = ''

/**
 * Set the org + bearer source used by all requests (called by AuthProvider).
 * Accepts either a static `token` (tests) or an async `getToken` provider that
 * returns a *fresh* token per request (prod — so it can't go stale between the
 * background renew timer firing and an actual API call).
 */
export function configureApi(opts: {
  org: string
  token?: string | null
  getToken?: TokenProvider
}): void {
  org = opts.org
  if (opts.getToken) {
    getToken = opts.getToken
  } else {
    const t = opts.token ?? null
    getToken = async () => t
  }
}

async function headers(force?: boolean): Promise<Record<string, string>> {
  const h: Record<string, string> = { 'Content-Type': 'application/json' }
  const t = await getToken(force ? { force: true } : undefined)
  if (t) h.Authorization = `Bearer ${t}`
  return h
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const url = `/v1/orgs/${encodeURIComponent(org)}${path}`
  const payload = body === undefined ? undefined : JSON.stringify(body)
  const send = async (force?: boolean) =>
    fetch(url, { method, headers: await headers(force), body: payload })

  let res = await send()
  // A 401 means the bearer was rejected — almost always a just-expired token
  // that the background renew hasn't replaced yet. Force a refresh and retry once.
  if (res.status === 401) res = await send(true)

  if (res.status === 204) return undefined as T
  const text = await res.text()
  if (!res.ok) throw new ApiError(res.status, text || res.statusText)
  return text ? (JSON.parse(text) as T) : (undefined as T)
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  patch: <T>(path: string, body?: unknown) => request<T>('PATCH', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  del: <T>(path: string) => request<T>('DELETE', path),
}
