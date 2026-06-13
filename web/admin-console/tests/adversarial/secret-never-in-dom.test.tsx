import { describe, it, expect, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../util/render'
import { configureApi } from '../../src/api/client'
import { CredentialsPanel } from '../../src/features/credentials/CredentialsPanel'
import type { Server } from '../../src/api/servers'

const server: Server = {
  id: 'srv_b2',
  slug: 'weather',
  type: 'remote_http',
  enabled: true,
  health: 'healthy',
  credential_mode: 'org_shared',
  credential_set: false,
  allowed_roles: [],
  created_at: '',
}

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

// Constitution VI / FR-013 — secrets are write-only. A stored value must never
// remain anywhere in the rendered DOM after it is saved (or rotated).
function leaked(secret: string): boolean {
  if (document.body.textContent?.includes(secret)) return true
  return Array.from(document.querySelectorAll('input')).some((i) => (i as HTMLInputElement).value.includes(secret))
}

describe('secret confidentiality (adversarial)', () => {
  it('a secret value never remains in the DOM after set or rotate', async () => {
    renderWithProviders(<CredentialsPanel server={server} />)

    // Set a secret.
    fireEvent.change(screen.getByPlaceholderText('Authorization'), { target: { value: 'X-Api-Key' } })
    fireEvent.change(screen.getByPlaceholderText('value (write-only)'), { target: { value: 'SUPER-SECRET-111' } })
    fireEvent.click(screen.getByText('Save'))
    await screen.findByText('Credential saved')
    await waitFor(() => expect(leaked('SUPER-SECRET-111')).toBe(false))

    // Rotate to a new secret.
    fireEvent.change(screen.getByPlaceholderText('Authorization'), { target: { value: 'X-Api-Key' } })
    fireEvent.change(screen.getByPlaceholderText('value (write-only)'), { target: { value: 'ROTATED-SECRET-222' } })
    fireEvent.click(screen.getByText('Save'))
    await waitFor(() => expect(screen.getAllByText('Credential saved').length).toBeGreaterThan(0))
    await waitFor(() => {
      expect(leaked('SUPER-SECRET-111')).toBe(false)
      expect(leaked('ROTATED-SECRET-222')).toBe(false)
    })
  })
})
