import { useState } from 'react'
import type { Server } from '../../api/servers'
import { useSetCredential, useClearCredential, type CredentialScope } from '../../api/credentials'
import { Button } from '../../design-system/components/core/Button'
import { Tag } from '../../design-system/components/core/Tag'
import { TextInput } from '../../design-system/components/forms/TextInput'
import { useNotify } from '../../app/feedback/notifications'

interface Row {
  key: string
  value: string
}

// Write-only credential management (FR-013): the console can set / rotate / clear
// but NEVER displays stored values. New values are entered as masked, write-only
// fields and cleared from the DOM immediately after a successful save.
export function CredentialsPanel({ server }: { server: Server }) {
  const notify = useNotify()
  const scope: CredentialScope = server.credential_mode === 'per_user' ? 'me' : 'org'
  const set = useSetCredential(server.id, scope)
  const clear = useClearCredential(server.id, scope)
  const [rows, setRows] = useState<Row[]>([{ key: '', value: '' }])
  const [savedThisSession, setSavedThisSession] = useState(false)

  if (server.credential_mode === 'none') {
    return <p style={{ color: 'var(--text-secondary)' }}>This server requires no credentials.</p>
  }

  const update = (i: number, patch: Partial<Row>) =>
    setRows((rs) => rs.map((r, idx) => (idx === i ? { ...r, ...patch } : r)))
  const addRow = () => setRows((rs) => [...rs, { key: '', value: '' }])

  const save = () => {
    const entries = rows.filter((r) => r.key.trim()).map((r) => [r.key.trim(), r.value] as const)
    if (entries.length === 0) {
      notify('warning', 'Add at least one key and value')
      return
    }
    set.mutate(Object.fromEntries(entries), {
      onSuccess: () => {
        setRows([{ key: '', value: '' }]) // wipe the secret from the DOM
        setSavedThisSession(true)
        notify('success', 'Credential saved')
      },
      onError: (e) => notify('error', 'Save failed', e instanceof Error ? e.message : undefined),
    })
  }

  const doClear = () =>
    clear.mutate(undefined, {
      onSuccess: () => {
        setRows([{ key: '', value: '' }])
        setSavedThisSession(false)
        notify('success', 'Credential cleared')
      },
      onError: (e) => notify('error', 'Clear failed', e instanceof Error ? e.message : undefined),
    })

  const isSet = scope === 'org' && server.credential_set
  const actionLabel = isSet ? 'Rotate' : 'Save'

  return (
    <div>
      <div style={{ display: 'flex', gap: 'var(--spacing-04)', alignItems: 'center', marginBottom: 'var(--spacing-04)' }}>
        <span>
          Mode: <strong>{server.credential_mode}</strong>
        </span>
        {scope === 'org' ? (
          isSet ? <Tag type="green">set</Tag> : <Tag type="gray">not set</Tag>
        ) : (
          <Tag type="blue">{savedThisSession ? 'per-user · saved' : 'per-user'}</Tag>
        )}
      </div>

      <p style={{ color: 'var(--text-secondary)', fontSize: 'var(--type-scale-02)', maxWidth: 560 }}>
        Enter credential key/value pairs (e.g. an <code>Authorization</code> header or an API key).
        Values are write-only — never displayed after saving. {isSet ? 'Rotation applies on next use.' : ''}
      </p>

      {rows.map((r, i) => (
        <div key={i} style={{ display: 'flex', gap: 'var(--spacing-04)', marginBottom: 'var(--spacing-04)' }}>
          <TextInput
            id={`cred-key-${i}`}
            label={i === 0 ? 'Key' : undefined}
            value={r.key}
            placeholder="Authorization"
            onChange={(e) => update(i, { key: e.target.value })}
          />
          <TextInput
            id={`cred-val-${i}`}
            label={i === 0 ? 'Value' : undefined}
            type="password"
            value={r.value}
            placeholder="value (write-only)"
            onChange={(e) => update(i, { value: e.target.value })}
          />
        </div>
      ))}

      <div style={{ display: 'flex', gap: 'var(--spacing-04)', marginTop: 'var(--spacing-04)' }}>
        <Button kind="ghost" size="sm" onClick={addRow}>
          Add field
        </Button>
        <Button kind="primary" size="sm" onClick={save} disabled={set.isPending}>
          {actionLabel}
        </Button>
        {isSet && (
          <Button kind="danger-tertiary" size="sm" onClick={doClear} disabled={clear.isPending}>
            Clear
          </Button>
        )}
      </div>
    </div>
  )
}
