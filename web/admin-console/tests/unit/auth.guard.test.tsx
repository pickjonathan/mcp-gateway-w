import { type ReactNode } from 'react'
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'

// Control the session the guard sees.
const h = vi.hoisted(() => ({ session: null as null | { isAdmin: boolean }, loading: false }))
vi.mock('../../src/auth/AuthProvider', () => ({
  useSession: () => ({
    session: h.session,
    loading: h.loading,
    signIn: vi.fn(),
    signOut: vi.fn(),
    completeSignIn: vi.fn(),
  }),
  AuthProvider: ({ children }: { children: ReactNode }) => children,
}))

import { RequireAdmin } from '../../src/auth/RequireAdmin'

function Harness() {
  return (
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route
          path="/"
          element={
            <RequireAdmin>
              <div>protected content</div>
            </RequireAdmin>
          }
        />
        <Route path="/signin" element={<div>signin page</div>} />
        <Route path="/forbidden" element={<div>forbidden page</div>} />
      </Routes>
    </MemoryRouter>
  )
}

describe('RequireAdmin', () => {
  it('redirects unauthenticated users to sign-in', () => {
    h.session = null
    h.loading = false
    render(<Harness />)
    expect(screen.getByText('signin page')).toBeInTheDocument()
  })

  it('redirects authenticated non-admins to forbidden', () => {
    h.session = { isAdmin: false }
    h.loading = false
    render(<Harness />)
    expect(screen.getByText('forbidden page')).toBeInTheDocument()
  })

  it('renders children for admins', () => {
    h.session = { isAdmin: true }
    h.loading = false
    render(<Harness />)
    expect(screen.getByText('protected content')).toBeInTheDocument()
  })
})
