import { describe, it, expect, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../util/render'
import { configureApi } from '../../src/api/client'
import { CredentialsPanel } from '../../src/features/credentials/CredentialsPanel'
import type { Server } from '../../src/api/servers'

const base: Server = {
  id: 'srv_b2',
  slug: 'weather',
  type: 'remote_http',
  enabled: true,
  health: 'healthy',
  credential_mode: 'org_shared',
  credential_set: true,
  allowed_roles: [],
  created_at: '',
}
const unset: Server = { ...base, credential_set: false }

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

describe('CredentialsPanel', () => {
  it('shows "set" + Rotate/Clear when a credential exists', () => {
    renderWithProviders(<CredentialsPanel server={base} />)
    expect(screen.getByText('set')).toBeInTheDocument()
    expect(screen.getByText('Rotate')).toBeInTheDocument()
    expect(screen.getByText('Clear')).toBeInTheDocument()
  })

  it('shows "not set" + Save when no credential exists', () => {
    renderWithProviders(<CredentialsPanel server={unset} />)
    expect(screen.getByText('not set')).toBeInTheDocument()
    expect(screen.getByText('Save')).toBeInTheDocument()
  })

  it('saves a credential and clears the inputs', async () => {
    renderWithProviders(<CredentialsPanel server={unset} />)
    fireEvent.change(screen.getByPlaceholderText('Authorization'), { target: { value: 'Authorization' } })
    fireEvent.change(screen.getByPlaceholderText('value (write-only)'), { target: { value: 'tok-123' } })
    fireEvent.click(screen.getByText('Save'))
    expect(await screen.findByText('Credential saved')).toBeInTheDocument()
    expect((screen.getByPlaceholderText('value (write-only)') as HTMLInputElement).value).toBe('')
  })

  it('renders no inputs for "none" mode', () => {
    renderWithProviders(<CredentialsPanel server={{ ...unset, credential_mode: 'none' }} />)
    expect(screen.getByText(/requires no credentials/)).toBeInTheDocument()
  })
})
