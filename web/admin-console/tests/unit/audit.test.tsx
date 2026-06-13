import { describe, it, expect, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../util/render'
import { configureApi } from '../../src/api/client'
import { Audit } from '../../src/pages/Audit'

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

describe('Audit log', () => {
  it('shows the chain status, events, and filters to denials only', async () => {
    renderWithProviders(<Audit />)

    // Chain-integrity banner.
    expect(await screen.findByText('Integrity verified')).toBeInTheDocument()

    // Events from the fixture (a config action + a denial).
    expect(screen.getByText('server.create')).toBeInTheDocument()
    expect(screen.getByText('auth.denied')).toBeInTheDocument()

    // Filter to denials only → the non-denial action disappears.
    fireEvent.change(screen.getByLabelText('Filter'), { target: { value: 'denials' } })
    expect(screen.queryByText('server.create')).toBeNull()
    expect(screen.getByText('auth.denied')).toBeInTheDocument()
  })

  it('filters by free-text search', async () => {
    renderWithProviders(<Audit />)
    await screen.findByText('server.create')
    fireEvent.change(screen.getByPlaceholderText('Search actor, action, target'), { target: { value: 'credentials' } })
    expect(screen.getByText('credentials.put')).toBeInTheDocument()
    expect(screen.queryByText('server.create')).toBeNull()
  })
})
