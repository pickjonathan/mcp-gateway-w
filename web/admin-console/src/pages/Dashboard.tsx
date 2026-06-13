import { type ReactNode } from 'react'
import { Tile } from '../design-system/components/core/Tile'
import { Tag } from '../design-system/components/core/Tag'
import { useDashboard } from '../features/dashboard/useDashboard'
import { UsageWidget } from '../features/dashboard/UsageWidget'
import { Loading, ErrorState } from '../app/feedback/states'

function Metric({ label, value, children }: { label: string; value: number; children?: ReactNode }) {
  return (
    <Tile>
      <div style={{ color: 'var(--text-secondary)', fontSize: 'var(--type-scale-02)' }}>{label}</div>
      <div style={{ fontSize: 'var(--type-scale-07)', fontWeight: 600, margin: 'var(--spacing-02) 0' }}>{value}</div>
      {children}
    </Tile>
  )
}

export function Dashboard() {
  const { summary, isLoading, error } = useDashboard()
  if (isLoading) return <Loading label="Loading dashboard…" />
  if (error) return <ErrorState message={error instanceof Error ? error.message : 'Failed to load dashboard'} />

  return (
    <section>
      <h1 style={{ fontSize: 'var(--type-scale-06)', marginBottom: 'var(--spacing-06)' }}>Dashboard</h1>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
          gap: 'var(--spacing-05)',
          marginBottom: 'var(--spacing-07)',
        }}
      >
        <Metric label="Servers" value={summary.serverCount} />
        <Metric label="Healthy" value={summary.health.healthy}>
          <Tag type="green">healthy</Tag>
        </Metric>
        <Metric label="Needs attention" value={summary.health.unhealthy}>
          {summary.health.unhealthy > 0 ? <Tag type="red">unhealthy</Tag> : <Tag type="gray">none</Tag>}
        </Metric>
        <Metric label="Auth denials (recent)" value={summary.denials}>
          {summary.denials > 0 ? <Tag type="red">review</Tag> : <Tag type="green">clear</Tag>}
        </Metric>
      </div>

      <div style={{ marginBottom: 'var(--spacing-07)' }}>
        <UsageWidget />
      </div>

      <h2 style={{ fontSize: 'var(--type-scale-04)', marginBottom: 'var(--spacing-04)' }}>Recent activity</h2>
      <Tile>
        {summary.recent.length === 0 ? (
          <p style={{ margin: 0, color: 'var(--text-secondary)' }}>No recent activity.</p>
        ) : (
          <ul style={{ margin: 0, paddingLeft: 'var(--spacing-06)' }}>
            {summary.recent.map((e) => (
              <li key={e.seq} style={{ marginBottom: 'var(--spacing-02)' }}>
                <strong>{e.action}</strong> · {e.target} ·{' '}
                <span style={{ color: 'var(--text-secondary)' }}>{e.actor}</span>
              </li>
            ))}
          </ul>
        )}
      </Tile>
    </section>
  )
}
