import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useServers, useUpdateServer, useDeleteServer, type Server, type Health } from '../api/servers'
import { Button } from '../design-system/components/core/Button'
import { Tag } from '../design-system/components/core/Tag'
import { Search } from '../design-system/components/forms/Search'
import { Loading, ErrorState, EmptyState } from '../app/feedback/states'
import { useNotify } from '../app/feedback/notifications'
import { srOnly } from '../app/srOnly'
import { ConfirmDialog } from '../features/servers/ConfirmDialog'

const HEALTH_COLOR: Record<Health, 'green' | 'red' | 'gray'> = {
  healthy: 'green',
  unhealthy: 'red',
  unknown: 'gray',
}

const cell = { padding: 'var(--spacing-04) var(--spacing-05)', textAlign: 'left', borderBottom: '1px solid var(--border-subtle-00)' } as const

export function Servers() {
  const navigate = useNavigate()
  const notify = useNotify()
  const { data, isLoading, error } = useServers()
  const update = useUpdateServer()
  const del = useDeleteServer()
  const [query, setQuery] = useState('')
  const [toDelete, setToDelete] = useState<Server | null>(null)

  const rows = useMemo(() => {
    const all = data ?? []
    const q = query.trim().toLowerCase()
    return (q ? all.filter((s) => s.slug.toLowerCase().includes(q)) : all).slice().sort((a, b) => a.slug.localeCompare(b.slug))
  }, [data, query])

  if (isLoading) return <Loading label="Loading servers…" />
  if (error) return <ErrorState message={error instanceof Error ? error.message : 'Failed to load servers'} />

  const toggleEnabled = (s: Server) =>
    update.mutate(
      { id: s.id, input: { enabled: !s.enabled } },
      {
        onSuccess: () => notify('success', `${s.slug} ${s.enabled ? 'disabled' : 'enabled'}`),
        onError: (e) => notify('error', 'Update failed', e instanceof Error ? e.message : undefined),
      },
    )

  const confirmDelete = () => {
    if (!toDelete) return
    const s = toDelete
    del.mutate(s.id, {
      onSuccess: () => {
        notify('success', `${s.slug} deleted`)
        setToDelete(null)
      },
      onError: (e) => notify('error', 'Delete failed', e instanceof Error ? e.message : undefined),
    })
  }

  return (
    <section>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--spacing-05)' }}>
        <h1 style={{ fontSize: 'var(--type-scale-06)', margin: 0 }}>Servers</h1>
        <Button kind="primary" size="sm" onClick={() => navigate('/servers/new')}>
          Add server
        </Button>
      </div>

      <label style={{ display: 'block', maxWidth: 360, marginBottom: 'var(--spacing-05)' }}>
        <span style={srOnly}>Search servers by name</span>
        <Search placeholder="Search by name" value={query} onChange={(e) => setQuery(e.target.value)} onClear={() => setQuery('')} />
      </label>

      {rows.length === 0 ? (
        <EmptyState title="No servers yet">
          <Button kind="primary" size="sm" onClick={() => navigate('/servers/new')}>
            Add your first server
          </Button>
        </EmptyState>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', background: 'var(--layer-01)' }}>
          <thead>
            <tr>
              {['Name', 'Type', 'Status', 'Health', 'Access', 'Actions'].map((h) => (
                <th key={h} style={{ ...cell, color: 'var(--text-secondary)', fontWeight: 600 }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((s) => (
              <tr key={s.id}>
                <td style={cell}>
                  <Link to={`/servers/${s.id}`} style={{ color: 'var(--link-primary)', textDecoration: 'none' }}>
                    {s.slug}
                  </Link>
                </td>
                <td style={cell}>{s.type === 'remote_http' ? 'remote' : 'stdio'}</td>
                <td style={cell}>
                  <Tag type={s.enabled ? 'green' : 'gray'}>{s.enabled ? 'enabled' : 'disabled'}</Tag>
                </td>
                <td style={cell}>
                  <Tag type={HEALTH_COLOR[s.health]}>{s.health}</Tag>
                </td>
                <td style={cell}>
                  <Tag type={s.allowed_roles.length ? 'purple' : 'gray'}>
                    {s.allowed_roles.length ? 'restricted' : 'open'}
                  </Tag>
                </td>
                <td style={cell}>
                  <span style={{ display: 'flex', gap: 'var(--spacing-03)' }}>
                    <Button kind="ghost" size="sm" onClick={() => navigate(`/servers/${s.id}/edit`)}>
                      Edit
                    </Button>
                    <Button kind="ghost" size="sm" onClick={() => toggleEnabled(s)} disabled={update.isPending}>
                      {s.enabled ? 'Disable' : 'Enable'}
                    </Button>
                    <Button kind="danger-tertiary" size="sm" onClick={() => setToDelete(s)}>
                      Delete
                    </Button>
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {toDelete && (
        <ConfirmDialog
          title={`Delete ${toDelete.slug}?`}
          message="This removes the server and terminates any running instances. This cannot be undone."
          confirmLabel="Delete"
          danger
          busy={del.isPending}
          onConfirm={confirmDelete}
          onCancel={() => setToDelete(null)}
        />
      )}
    </section>
  )
}
