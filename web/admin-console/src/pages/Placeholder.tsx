import { Tile } from '../design-system/components/core/Tile'

// Temporary content for routes whose screens land in their user-story phase
// (US1 Dashboard, US2 Servers, US5 Audit, US6 Settings).
export function Placeholder({ title }: { title: string }) {
  return (
    <section>
      <h1 style={{ fontSize: 'var(--type-scale-06)', marginBottom: 'var(--spacing-05)' }}>{title}</h1>
      <Tile>
        <p style={{ margin: 0, color: 'var(--text-secondary)' }}>
          {title} — implemented in its user-story phase.
        </p>
      </Tile>
    </section>
  )
}
