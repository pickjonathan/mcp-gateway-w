import { describe, it, expect, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { server } from '../mocks/server'
import { renderWithProviders } from '../util/render'
import { configureApi } from '../../src/api/client'
import { ServerForm } from '../../src/pages/ServerForm'

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

describe('ServerForm', () => {
  it('shows a validation error when the name is empty', () => {
    renderWithProviders(<ServerForm />, { route: '/servers/new' })
    fireEvent.click(screen.getByText('Create server'))
    expect(screen.getByText('Name is required')).toBeInTheDocument()
  })

  it('requires a command for stdio servers', () => {
    renderWithProviders(<ServerForm />, { route: '/servers/new' })
    fireEvent.change(screen.getByLabelText('Name (slug)'), { target: { value: 'thinking' } })
    fireEvent.change(screen.getByLabelText('Type'), { target: { value: 'stdio' } })
    fireEvent.click(screen.getByText('Create server'))
    expect(screen.getByText('Command is required')).toBeInTheDocument()
  })

  it('surfaces a duplicate-slug error returned by the API', async () => {
    server.use(http.post('/v1/orgs/:org/servers', () => new HttpResponse('slug taken', { status: 409 })))
    renderWithProviders(<ServerForm />, { route: '/servers/new' })
    fireEvent.change(screen.getByLabelText('Name (slug)'), { target: { value: 'dup' } })
    fireEvent.change(screen.getByLabelText('Endpoint URL'), { target: { value: 'https://x.example' } })
    fireEvent.click(screen.getByText('Create server'))
    expect(await screen.findByText('That name is already in use')).toBeInTheDocument()
  })
})
