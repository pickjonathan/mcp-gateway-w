import { describe, it, expect, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../util/render'
import { configureApi } from '../../src/api/client'
import { UsageWidget } from '../../src/features/dashboard/UsageWidget'
import { Settings } from '../../src/pages/Settings'

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

describe('UsageWidget', () => {
  it('renders traffic rates queried from Prometheus', async () => {
    renderWithProviders(<UsageWidget />)
    expect(await screen.findByText('Traffic (last 5 min)')).toBeInTheDocument()
    // The metrics mock returns 0.42 for each query.
    expect(screen.getAllByText('0.42').length).toBeGreaterThan(0)
  })
})

describe('Settings', () => {
  it('shows the connection endpoint and read-only rate limits', async () => {
    renderWithProviders(<Settings />)
    expect(screen.getByText('https://acme.mcp.example.com/mcp')).toBeInTheDocument()
    expect(screen.getByText('Copy')).toBeInTheDocument()
    expect(await screen.findByText('600 / min')).toBeInTheDocument()
    expect(screen.getByText('60 / min')).toBeInTheDocument()
  })
})
