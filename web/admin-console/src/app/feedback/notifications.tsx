import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import { InlineNotification } from '../../design-system/components/feedback/InlineNotification'

type Kind = 'error' | 'success' | 'warning' | 'info'
interface Toast {
  id: number
  kind: Kind
  title: string
  subtitle?: string
}

type Notify = (kind: Kind, title: string, subtitle?: string) => void
const NotificationsContext = createContext<Notify | null>(null)

let seq = 0

export function NotificationsProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const remove = useCallback((id: number) => setToasts((t) => t.filter((x) => x.id !== id)), [])
  const notify = useCallback<Notify>(
    (kind, title, subtitle) => {
      const id = ++seq
      setToasts((t) => [...t, { id, kind, title, subtitle }])
      window.setTimeout(() => remove(id), 6000)
    },
    [remove],
  )
  return (
    <NotificationsContext.Provider value={notify}>
      {children}
      <div
        aria-live="polite"
        style={{
          position: 'fixed',
          top: 'var(--spacing-05)',
          right: 'var(--spacing-05)',
          zIndex: 9000,
          display: 'grid',
          gap: 'var(--spacing-03)',
          maxWidth: 360,
        }}
      >
        {toasts.map((t) => (
          <InlineNotification
            key={t.id}
            kind={t.kind}
            title={t.title}
            subtitle={t.subtitle}
            onClose={() => remove(t.id)}
          />
        ))}
      </div>
    </NotificationsContext.Provider>
  )
}

export function useNotify(): Notify {
  const ctx = useContext(NotificationsContext)
  if (!ctx) throw new Error('useNotify must be used within NotificationsProvider')
  return ctx
}
