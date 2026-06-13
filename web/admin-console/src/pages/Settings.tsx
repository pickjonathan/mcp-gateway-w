import { useQuotas } from '../api/quotas'
import { resolveOrg } from '../auth/org'
import { Tile } from '../design-system/components/core/Tile'
import { Button } from '../design-system/components/core/Button'
import { Loading, ErrorState } from '../app/feedback/states'
import { useNotify } from '../app/feedback/notifications'

function connectionEndpoint(): string {
  const base = import.meta.env.VITE_BASE_DOMAIN ?? 'mcp.example.com'
  return `https://${resolveOrg()}.${base}/mcp`
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: 'flex', gap: 'var(--spacing-05)', padding: 'var(--spacing-03) 0' }}>
      <div style={{ width: 200, color: 'var(--text-secondary)' }}>{label}</div>
      <div>{value}</div>
    </div>
  )
}

export function Settings() {
  const notify = useNotify()
  const { data: quotas, isLoading, error } = useQuotas()
  const endpoint = connectionEndpoint()

  const copy = async () => {
    try {
      await navigator.clipboard?.writeText(endpoint)
      notify('success', 'Endpoint copied')
    } catch {
      notify('warning', 'Copy unavailable — select the text manually')
    }
  }

  const limit = (n?: number) => (n === undefined ? '—' : n === 0 ? 'unlimited' : `${n} / min`)

  return (
    <section style={{ maxWidth: 640 }}>
      <h1 style={{ fontSize: 'var(--type-scale-06)', marginBottom: 'var(--spacing-06)' }}>Settings</h1>

      <h2 style={{ fontSize: 'var(--type-scale-04)', marginBottom: 'var(--spacing-04)' }}>Connection endpoint</h2>
      <Tile>
        <p style={{ marginTop: 0, color: 'var(--text-secondary)' }}>
          Share this with your users' MCP clients.
        </p>
        <div style={{ display: 'flex', gap: 'var(--spacing-04)', alignItems: 'center' }}>
          <code
            style={{
              flex: 1,
              padding: 'var(--spacing-04)',
              background: 'var(--layer-02)',
              border: '1px solid var(--border-subtle-00)',
              fontFamily: 'var(--font-mono)',
            }}
          >
            {endpoint}
          </code>
          <Button kind="secondary" size="sm" onClick={() => void copy()}>
            Copy
          </Button>
        </div>
      </Tile>

      <h2 style={{ fontSize: 'var(--type-scale-04)', margin: 'var(--spacing-07) 0 var(--spacing-04)' }}>Rate limits</h2>
      {isLoading ? (
        <Loading label="Loading limits…" />
      ) : error ? (
        <ErrorState message={error instanceof Error ? error.message : 'Failed to load limits'} />
      ) : (
        <Tile>
          <p style={{ marginTop: 0, color: 'var(--text-secondary)' }}>
            Configured per-minute request limits (read-only; editing is out of scope for v1).
          </p>
          <Row label="Per organization" value={limit(quotas?.org_per_min)} />
          <Row label="Per user" value={limit(quotas?.user_per_min)} />
        </Tile>
      )}
    </section>
  )
}
