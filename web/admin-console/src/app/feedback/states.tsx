import { type ReactNode } from 'react'
import { InlineNotification } from '../../design-system/components/feedback/InlineNotification'

export function Loading({ label = 'Loading…' }: { label?: string }) {
  return (
    <p role="status" style={{ color: 'var(--text-secondary)', padding: 'var(--spacing-05)' }}>
      {label}
    </p>
  )
}

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div style={{ padding: 'var(--spacing-09)', textAlign: 'center', color: 'var(--text-secondary)' }}>
      <h3 style={{ marginBottom: 'var(--spacing-03)' }}>{title}</h3>
      {children}
    </div>
  )
}

export function ErrorState({ message }: { message: string }) {
  return <InlineNotification kind="error" title="Something went wrong" subtitle={message} hideClose />
}
