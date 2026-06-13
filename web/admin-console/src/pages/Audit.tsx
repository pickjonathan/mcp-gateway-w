import { useMemo, useState } from 'react'
import { useAudit } from '../api/audit'
import { Search } from '../design-system/components/forms/Search'
import { Select } from '../design-system/components/forms/Select'
import { Tag } from '../design-system/components/core/Tag'
import { Button } from '../design-system/components/core/Button'
import { InlineNotification } from '../design-system/components/feedback/InlineNotification'
import { Loading, ErrorState, EmptyState } from '../app/feedback/states'
import { srOnly } from '../app/srOnly'

const DENIALS = new Set(['auth.denied', 'authz.denied'])
const PAGE_SIZE = 20
const cell = { padding: 'var(--spacing-04) var(--spacing-05)', textAlign: 'left', borderBottom: '1px solid var(--border-subtle-00)' } as const

export function Audit() {
  const { data, isLoading, error } = useAudit()
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<'all' | 'denials'>('all')
  const [page, setPage] = useState(0)

  const events = data?.events ?? []
  const chain = data?.chain.status

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return events.filter((e) => {
      if (filter === 'denials' && !DENIALS.has(e.action)) return false
      if (!q) return true
      return [e.actor, e.action, e.target].some((v) => v.toLowerCase().includes(q))
    })
  }, [events, query, filter])

  if (isLoading) return <Loading label="Loading audit log…" />
  if (error) return <ErrorState message={error instanceof Error ? error.message : 'Failed to load audit log'} />

  const pages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const current = Math.min(page, pages - 1)
  const rows = filtered.slice(current * PAGE_SIZE, current * PAGE_SIZE + PAGE_SIZE)

  return (
    <section>
      <h1 style={{ fontSize: 'var(--type-scale-06)', marginBottom: 'var(--spacing-05)' }}>Audit log</h1>

      {chain === 'tampered' ? (
        <InlineNotification
          kind="error"
          title="Integrity check failed"
          subtitle="The audit chain may have been tampered with — investigate immediately."
          hideClose
        />
      ) : (
        <InlineNotification
          kind="success"
          title="Integrity verified"
          subtitle="The audit chain is intact (tamper-evident)."
          hideClose
        />
      )}

      <div style={{ display: 'flex', gap: 'var(--spacing-05)', alignItems: 'flex-end', margin: 'var(--spacing-05) 0' }}>
        <label style={{ flex: 1, maxWidth: 360 }}>
          <span style={srOnly}>Search audit events</span>
          <Search
            placeholder="Search actor, action, target"
            value={query}
            onChange={(e) => { setQuery(e.target.value); setPage(0) }}
            onClear={() => setQuery('')}
          />
        </label>
        <Select id="audit-filter" label="Filter" value={filter} onChange={(e) => { setFilter(e.target.value as 'all' | 'denials'); setPage(0) }}>
          <option value="all">All actions</option>
          <option value="denials">Denials only</option>
        </Select>
      </div>

      {rows.length === 0 ? (
        <EmptyState title="No audit events" />
      ) : (
        <>
          <table style={{ width: '100%', borderCollapse: 'collapse', background: 'var(--layer-01)' }}>
            <thead>
              <tr>
                {['Time', 'Actor', 'Action', 'Target'].map((h) => (
                  <th key={h} style={{ ...cell, color: 'var(--text-secondary)', fontWeight: 600 }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((e) => {
                const denial = DENIALS.has(e.action)
                return (
                  <tr key={e.seq} style={denial ? { background: 'var(--red-10)' } : undefined}>
                    <td style={cell}>{e.time}</td>
                    <td style={cell}>{e.actor}</td>
                    <td style={cell}>
                      <Tag type={denial ? 'red' : 'gray'}>{e.action}</Tag>
                    </td>
                    <td style={cell}>{e.target}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>

          <div style={{ display: 'flex', gap: 'var(--spacing-04)', alignItems: 'center', marginTop: 'var(--spacing-05)' }}>
            <span style={{ color: 'var(--text-secondary)', fontSize: 'var(--type-scale-02)' }}>
              Page {current + 1} of {pages} · {filtered.length} events
            </span>
            <Button kind="ghost" size="sm" onClick={() => setPage((p) => Math.max(0, p - 1))} disabled={current === 0}>
              Prev
            </Button>
            <Button kind="ghost" size="sm" onClick={() => setPage((p) => p + 1)} disabled={current >= pages - 1}>
              Next
            </Button>
          </div>
        </>
      )}
    </section>
  )
}
