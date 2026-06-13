import { useState } from 'react'
import type { Server } from '../../api/servers'
import { useUpdateServer } from '../../api/servers'
import { Tag } from '../../design-system/components/core/Tag'
import { TextInput } from '../../design-system/components/forms/TextInput'
import { Button } from '../../design-system/components/core/Button'
import { useNotify } from '../../app/feedback/notifications'

// RBAC (US5/FR-014): assign the roles permitted to use a server. Empty = open to
// all org members. Roles are free-form chips (no roles-catalog endpoint exists in
// 001; a Checkbox group would require one).
export function AccessPanel({ server }: { server: Server }) {
  const notify = useNotify()
  const update = useUpdateServer()
  const [roles, setRoles] = useState<string[]>(server.allowed_roles)
  const [draft, setDraft] = useState('')

  const add = () => {
    const r = draft.trim()
    if (r && !roles.includes(r)) setRoles([...roles, r])
    setDraft('')
  }
  const remove = (r: string) => setRoles((rs) => rs.filter((x) => x !== r))
  const dirty = JSON.stringify(roles) !== JSON.stringify(server.allowed_roles)

  const save = () =>
    update.mutate(
      { id: server.id, input: { allowed_roles: roles } },
      {
        onSuccess: () =>
          notify('success', roles.length ? 'Access restricted to selected roles' : 'Open to all members'),
        onError: (e) => notify('error', 'Update failed', e instanceof Error ? e.message : undefined),
      },
    )

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--spacing-04)', marginBottom: 'var(--spacing-05)' }}>
        <span>Access:</span>
        {roles.length ? <Tag type="purple">restricted</Tag> : <Tag type="gray">open to all members</Tag>}
      </div>

      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--spacing-03)', marginBottom: 'var(--spacing-05)' }}>
        {roles.length === 0 ? (
          <span style={{ color: 'var(--text-secondary)' }}>
            No role restrictions — all organization members can use this server.
          </span>
        ) : (
          roles.map((r) => (
            <Tag key={r} type="purple" filter onClose={() => remove(r)}>
              {r}
            </Tag>
          ))
        )}
      </div>

      <div style={{ display: 'flex', gap: 'var(--spacing-04)', alignItems: 'flex-end' }}>
        <TextInput
          id="role-add"
          label="Add role"
          value={draft}
          placeholder="e.g. engineers"
          onChange={(e) => setDraft(e.target.value)}
        />
        <Button kind="ghost" size="sm" onClick={add}>
          Add
        </Button>
      </div>

      <div style={{ marginTop: 'var(--spacing-05)' }}>
        <Button kind="primary" size="sm" onClick={save} disabled={!dirty || update.isPending}>
          Save access
        </Button>
      </div>
    </div>
  )
}
