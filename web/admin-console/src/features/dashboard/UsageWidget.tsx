import { useUsage } from '../../api/metrics'
import { Tile } from '../../design-system/components/core/Tile'
import { ProgressBar } from '../../design-system/components/feedback/ProgressBar'

const fmt = (n: number) => n.toFixed(2)

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <div style={{ color: 'var(--text-secondary)', fontSize: 'var(--type-scale-02)' }}>{label}</div>
      <div style={{ fontSize: 'var(--type-scale-05)', fontWeight: 600 }}>{fmt(value)}</div>
    </div>
  )
}

// Request/denial/error rate trends from Prometheus (US6). Self-contained: if
// metrics are unreachable it degrades gracefully rather than breaking the page.
export function UsageWidget() {
  const { data, isLoading, isError } = useUsage()

  if (isLoading) {
    return (
      <Tile>
        <p style={{ margin: 0, color: 'var(--text-secondary)' }}>Loading traffic…</p>
      </Tile>
    )
  }
  if (isError || !data) {
    return (
      <Tile>
        <p style={{ margin: 0, color: 'var(--text-secondary)' }}>Traffic metrics unavailable.</p>
      </Tile>
    )
  }

  const errorRatio = data.requestRate > 0 ? Math.min(1, data.errorRate / data.requestRate) : 0
  return (
    <Tile>
      <h3 style={{ marginTop: 0, fontSize: 'var(--type-scale-04)' }}>Traffic (last 5 min)</h3>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(130px, 1fr))', gap: 'var(--spacing-05)' }}>
        <Stat label="Requests / s" value={data.requestRate} />
        <Stat label="Denials / s" value={data.denialRate} />
        <Stat label="5xx / s" value={data.errorRate} />
        <Stat label="Tool errors / s" value={data.toolErrorRate} />
      </div>
      <div style={{ marginTop: 'var(--spacing-05)' }}>
        <ProgressBar
          label="Error ratio"
          value={errorRatio}
          max={1}
          status={errorRatio > 0.05 ? 'error' : 'active'}
          helperText={`${(errorRatio * 100).toFixed(1)}% of requests`}
        />
      </div>
    </Tile>
  )
}
