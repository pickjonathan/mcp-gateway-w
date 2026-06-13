import { describe, it, expect, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../util/render'
import { configureApi } from '../../src/api/client'
import { AccessPanel } from '../../src/features/rbac/AccessPanel'
import type { Server } from '../../src/api/servers'

const base: Server = {
  id: 'srv_b2',
  slug: 'weather',
  type: 'remote_http',
  enabled: true,
  health: 'healthy',
  credential_mode: 'none',
  allowed_roles: [],
  created_at: '',
}

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

describe('AccessPanel', () => {
  it('shows "open to all members" when there are no roles', () => {
    renderWithProviders(<AccessPanel server={base} />)
    expect(screen.getByText('open to all members')).toBeInTheDocument()
  })

  it('shows restricted and lists the assigned roles', () => {
    renderWithProviders(<AccessPanel server={{ ...base, allowed_roles: ['engineers'] }} />)
    expect(screen.getByText('restricted')).toBeInTheDocument()
    expect(screen.getByText('engineers')).toBeInTheDocument()
  })

  it('adds a role, flipping the server to restricted', () => {
    renderWithProviders(<AccessPanel server={base} />)
    fireEvent.change(screen.getByLabelText('Add role'), { target: { value: 'admins' } })
    fireEvent.click(screen.getByText('Add'))
    expect(screen.getByText('admins')).toBeInTheDocument()
    expect(screen.getByText('restricted')).toBeInTheDocument()
  })
})
