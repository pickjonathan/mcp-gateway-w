import { describe, it, expect } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { server } from '../mocks/server'
import { renderWithProviders } from '../util/render'
import { configureApi } from '../../src/api/client'
import { Dashboard } from '../../src/pages/Dashboard'

// Constitution I — Tenant Isolation Is Inviolable. The console must be
// structurally incapable of reading another org's data: every control-plane
// request is scoped to the session's org, and no UI path requests another.
describe('cross-org isolation (adversarial)', () => {
  it('an org-acme session only ever requests org "acme" and never renders another org', async () => {
    const seenOrgs: string[] = []
    server.use(
      http.get('/v1/orgs/:org/servers', ({ params }) => {
        const org = String(params.org)
        seenOrgs.push(org)
        // A different org would expose a "secret" server — it must never be hit.
        const slug = org === 'acme' ? 'acme-server' : 'evil-secret-server'
        return HttpResponse.json([
          { id: 'x', slug, type: 'remote_http', enabled: true, health: 'healthy', credential_mode: 'none', allowed_roles: [], created_at: '' },
        ])
      }),
      http.get('/v1/orgs/:org/audit', ({ params }) => {
        seenOrgs.push(String(params.org))
        return HttpResponse.json({
          events: [{ seq: 1, time: '', actor: 'admin', action: 'server.create', target: 'acme-server' }],
          chain: { status: 'verified' },
        })
      }),
    )

    configureApi({ token: 'test', org: 'acme' })
    renderWithProviders(<Dashboard />)

    await screen.findByText('server.create')
    await waitFor(() => expect(seenOrgs.length).toBeGreaterThan(0))

    // Every request was scoped to "acme"; no other org was ever requested.
    expect([...new Set(seenOrgs)]).toEqual(['acme'])
    // The other org's data is never rendered.
    expect(screen.queryByText('evil-secret-server')).toBeNull()
  })
})
