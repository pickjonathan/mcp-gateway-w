import { describe, it, expect, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../util/render'
import { configureApi } from '../../src/api/client'
import { Dashboard } from '../../src/pages/Dashboard'

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

describe('Dashboard', () => {
  it('renders metrics and recent activity from the API', async () => {
    renderWithProviders(<Dashboard />)

    // Recent activity (from the audit fixture) appears after load.
    expect(await screen.findByText('server.create')).toBeInTheDocument()
    expect(screen.getByText('auth.denied')).toBeInTheDocument()

    // Metric cards rendered (counts derived from the servers fixture).
    expect(screen.getByText('Servers')).toBeInTheDocument()
    expect(screen.getByText('Healthy')).toBeInTheDocument()
    expect(screen.getAllByText('2').length).toBeGreaterThan(0) // 2 servers, 2 healthy
  })
})
