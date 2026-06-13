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

let token: string | null = null
let org = ''

/** Set the org + bearer used by all requests (called by AuthProvider). */
export function configureApi(opts: { token: string | null; org: string }): void {
  token = opts.token
  org = opts.org
}

function headers(): Record<string, string> {
  const h: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) h.Authorization = `Bearer ${token}`
  return h
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/v1/orgs/${encodeURIComponent(org)}${path}`, {
    method,
    headers: headers(),
    body: body === undefined ? undefined : JSON.stringify(body),
  })
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
