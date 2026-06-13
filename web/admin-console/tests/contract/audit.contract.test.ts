import { describe, it, expect, beforeEach } from 'vitest'
import { api, configureApi } from '../../src/api/client'
import type { AuditResponse } from '../../src/api/audit'

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

describe('control-plane audit contract', () => {
  it('returns events newest-first plus a chain-integrity status', async () => {
    const res = await api.get<AuditResponse>('/audit')
    expect(res.chain.status).toBe('verified')
    expect(res.events.length).toBeGreaterThan(0)
    // Fixture is newest-first (seq 3, 2, 1).
    expect(res.events[0].seq).toBe(3)
    expect(res.events.some((e) => e.action === 'auth.denied')).toBe(true)
  })
})
