import { Button } from '../../design-system/components/core/Button'
import { Tile } from '../../design-system/components/core/Tile'

// Lightweight confirmation for destructive/kill-switch actions (the handoff has no
// Modal component, so this is composed from Carbon tokens + Tile + Button).
export function ConfirmDialog({
  title,
  message,
  confirmLabel = 'Confirm',
  danger = false,
  busy = false,
  onConfirm,
  onCancel,
}: {
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}) {
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={title}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'var(--overlay)',
        display: 'grid',
        placeItems: 'center',
        zIndex: 9500,
      }}
    >
      <div style={{ maxWidth: 440, width: '90%' }}>
        <Tile>
          <h3 style={{ marginTop: 0 }}>{title}</h3>
          <p style={{ color: 'var(--text-secondary)' }}>{message}</p>
          <div style={{ display: 'flex', gap: 'var(--spacing-04)', justifyContent: 'flex-end' }}>
            <Button kind="secondary" size="sm" onClick={onCancel} disabled={busy}>
              Cancel
            </Button>
            <Button kind={danger ? 'danger' : 'primary'} size="sm" onClick={onConfirm} disabled={busy}>
              {confirmLabel}
            </Button>
          </div>
        </Tile>
      </div>
    </div>
  )
}
