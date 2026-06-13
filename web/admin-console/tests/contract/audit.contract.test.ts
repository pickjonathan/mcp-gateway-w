import { describe, it, expect, beforeEach } from 'vitest'
import { api, configureApi } from '../../src/api/client'
import type { AuditEvent } from '../../src/api/audit'

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

describe('control-plane audit contract', () => {
  it('returns a bare array of events, newest-first', async () => {
    const events = await api.get<AuditEvent[]>('/audit')
    expect(Array.isArray(events)).toBe(true)
    expect(events.length).toBeGreaterThan(0)
    // Fixture is newest-first (seq 3, 2, 1).
    expect(events[0].seq).toBe(3)
    expect(events.some((e) => e.action === 'auth.denied')).toBe(true)
  })
})
