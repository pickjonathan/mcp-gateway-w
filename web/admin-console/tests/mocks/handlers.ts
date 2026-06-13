import { http, HttpResponse } from 'msw'
import { servers, auditEvents, quotas } from './fixtures'

// MSW handlers mocking the control-plane admin API the console consumes
// (contracts/control-plane-consumed.md). Credential writes are 204, value never
// echoed. Override per-test with server.use(...).
export const handlers = [
  http.get('/v1/orgs/:org/servers', () => HttpResponse.json(servers)),
  http.get('/v1/orgs/:org/servers/:id', ({ params }) => {
    const s = servers.find((x) => x.id === params.id)
    return s ? HttpResponse.json(s) : new HttpResponse(null, { status: 404 })
  }),
  http.post('/v1/orgs/:org/servers', async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>
    return HttpResponse.json({ id: 'srv_new', health: 'unknown', enabled: true, ...body }, { status: 201 })
  }),
  http.patch('/v1/orgs/:org/servers/:id', async ({ request, params }) => {
    const body = (await request.json()) as Record<string, unknown>
    return HttpResponse.json({ id: params.id, ...body })
  }),
  http.delete('/v1/orgs/:org/servers/:id', () => new HttpResponse(null, { status: 204 })),

  http.put('/v1/orgs/:org/servers/:id/credentials', () => new HttpResponse(null, { status: 204 })),
  http.delete('/v1/orgs/:org/servers/:id/credentials', () => new HttpResponse(null, { status: 204 })),
  http.put('/v1/orgs/:org/servers/:id/credentials/me', () => new HttpResponse(null, { status: 204 })),
  http.delete('/v1/orgs/:org/servers/:id/credentials/me', () => new HttpResponse(null, { status: 204 })),

  // The control-plane returns a bare array of hash-chained records (newest first).
  http.get('/v1/orgs/:org/audit', () => HttpResponse.json(auditEvents)),
  http.get('/v1/orgs/:org/quotas', () => HttpResponse.json(quotas)),

  // Prometheus query API (via the /metrics-api proxy) — returns a constant rate.
  http.get('/metrics-api/api/v1/query', () =>
    HttpResponse.json({ status: 'success', data: { resultType: 'vector', result: [{ metric: {}, value: [0, '0.42'] }] } }),
  ),
]
