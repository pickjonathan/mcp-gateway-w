import { describe, it, expect, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../util/render'
import { configureApi } from '../../src/api/client'
import { Servers } from '../../src/pages/Servers'

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

describe('Servers catalog', () => {
  it('renders rows with health tags and filters via search', async () => {
    renderWithProviders(<Servers />)

    expect(await screen.findByText('sequential-thinking')).toBeInTheDocument()
    expect(screen.getByText('weather')).toBeInTheDocument()
    expect(screen.getAllByText('healthy').length).toBeGreaterThan(0)

    const search = screen.getByPlaceholderText('Search by name')
    fireEvent.change(search, { target: { value: 'weather' } })

    expect(screen.queryByText('sequential-thinking')).toBeNull()
    expect(screen.getByText('weather')).toBeInTheDocument()
  })
})
