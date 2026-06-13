import { NavLink, Outlet } from 'react-router-dom'
import { Button } from '../../design-system/components/core/Button'
import { useSession } from '../../auth/AuthProvider'

const NAV = [
  { to: '/', label: 'Dashboard', end: true },
  { to: '/servers', label: 'Servers', end: false },
  { to: '/audit', label: 'Audit', end: false },
  { to: '/settings', label: 'Settings', end: false },
]

// Carbon UI-shell: a dark product header + a left side nav + content. Built from
// design tokens (cloud-console kit as reference) so it adheres to Carbon.
export function AppShell() {
  const { session, signOut } = useSession()
  return (
    <div
      style={{
        minHeight: '100vh',
        background: 'var(--background)',
        color: 'var(--text-primary)',
        fontFamily: 'var(--font-sans)',
      }}
    >
      <header
        style={{
          height: 48,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 var(--spacing-05)',
          background: 'var(--gray-100)',
          color: 'var(--text-on-color)',
        }}
      >
        <strong>
          MCP Admin <span style={{ color: 'var(--text-on-color-disabled)' }}>· {session?.org}</span>
        </strong>
        <span style={{ display: 'flex', alignItems: 'center', gap: 'var(--spacing-04)' }}>
          <span style={{ fontSize: 'var(--type-scale-02)' }}>{session?.displayName}</span>
          <Button kind="ghost" size="sm" onClick={() => void signOut()}>
            Sign out
          </Button>
        </span>
      </header>
      <div style={{ display: 'flex' }}>
        <nav
          style={{
            width: 240,
            minHeight: 'calc(100vh - 48px)',
            background: 'var(--layer-01)',
            borderRight: '1px solid var(--border-subtle-00)',
            padding: 'var(--spacing-03)',
          }}
        >
          {NAV.map((n) => (
            <NavLink
              key={n.to}
              to={n.to}
              end={n.end}
              style={({ isActive }) => ({
                display: 'block',
                padding: 'var(--spacing-04) var(--spacing-05)',
                textDecoration: 'none',
                color: 'var(--text-primary)',
                background: isActive ? 'var(--layer-selected-01)' : 'transparent',
                borderLeft: isActive ? '3px solid var(--border-interactive)' : '3px solid transparent',
              })}
            >
              {n.label}
            </NavLink>
          ))}
        </nav>
        <main style={{ flex: 1, padding: 'var(--spacing-07)' }}>
          <Outlet />
        </main>
      </div>
    </div>
  )
}
