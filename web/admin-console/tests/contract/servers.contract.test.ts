import { describe, it, expect, beforeEach } from 'vitest'
import { api, configureApi, ApiError } from '../../src/api/client'
import type { Server } from '../../src/api/servers'

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

describe('control-plane servers contract', () => {
  it('lists the org servers', async () => {
    const list = await api.get<Server[]>('/servers')
    expect(list).toHaveLength(2)
    expect(list[0]).toHaveProperty('slug')
    expect(list[0]).toHaveProperty('type')
  })

  it('gets a server by id', async () => {
    const s = await api.get<Server>('/servers/srv_a1')
    expect(s.slug).toBe('sequential-thinking')
  })

  it('creates a server (201)', async () => {
    const s = await api.post<Server>('/servers', { slug: 'x', type: 'remote_http' })
    expect(s.id).toBeTruthy()
  })

  it('patches a server', async () => {
    const s = await api.patch<Server>('/servers/srv_a1', { enabled: false })
    expect(s.enabled).toBe(false)
  })

  it('deletes a server (204 → undefined)', async () => {
    await expect(api.del('/servers/srv_a1')).resolves.toBeUndefined()
  })

  it('throws ApiError on an unknown id', async () => {
    await expect(api.get('/servers/missing')).rejects.toBeInstanceOf(ApiError)
  })
})
