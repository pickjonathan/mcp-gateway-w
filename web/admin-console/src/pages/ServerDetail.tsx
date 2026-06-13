import { type ReactNode } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useServer } from '../api/servers'
import { Tabs } from '../design-system/components/navigation/Tabs'
import { Tag } from '../design-system/components/core/Tag'
import { Button } from '../design-system/components/core/Button'
import { Loading, ErrorState } from '../app/feedback/states'
import { CredentialsPanel } from '../features/credentials/CredentialsPanel'
import { AccessPanel } from '../features/rbac/AccessPanel'

function Row({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div style={{ display: 'flex', gap: 'var(--spacing-05)', padding: 'var(--spacing-03) 0' }}>
      <div style={{ width: 160, color: 'var(--text-secondary)' }}>{label}</div>
      <div>{value}</div>
    </div>
  )
}

export function ServerDetail() {
  const { id } = useParams()
  const { data: s, isLoading, error } = useServer(id ?? '')

  if (isLoading) return <Loading label="Loading server…" />
  if (error || !s) return <ErrorState message={error instanceof Error ? error.message : 'Server not found'} />

  return (
    <section style={{ maxWidth: 760 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--spacing-05)' }}>
        <h1 style={{ fontSize: 'var(--type-scale-06)', margin: 0 }}>{s.slug}</h1>
        <Link to={`/servers/${s.id}/edit`}>
          <Button kind="secondary" size="sm">Edit</Button>
        </Link>
      </div>

      <Tabs tabs={['Overview', 'Credentials', 'Access', 'Health']}>
        <div>
          <Row label="Type" value={s.type === 'remote_http' ? 'Remote (HTTP)' : 'Stdio (sandboxed)'} />
          {s.type === 'remote_http' ? (
            <Row label="Endpoint" value={s.endpoint_url} />
          ) : (
            <Row label="Command" value={<code>{[s.command, ...(s.args ?? [])].join(' ')}</code>} />
          )}
          <Row label="Status" value={<Tag type={s.enabled ? 'green' : 'gray'}>{s.enabled ? 'enabled' : 'disabled'}</Tag>} />
          <Row label="Created" value={s.created_at} />
        </div>

        <div>
          <CredentialsPanel server={s} />
        </div>

        <div>
          <AccessPanel server={s} />
        </div>

        <div>
          <Row label="Health" value={<Tag type={s.health === 'healthy' ? 'green' : s.health === 'unhealthy' ? 'red' : 'gray'}>{s.health}</Tag>} />
          {s.health_detail && <Row label="Detail" value={s.health_detail} />}
        </div>
      </Tabs>
    </section>
  )
}
