import type { CSSProperties } from 'react'

// Visually-hidden but screen-reader-accessible (for labels on icon/placeholder-only
// controls). WCAG 2.1 AA.
export const srOnly: CSSProperties = {
  position: 'absolute',
  width: 1,
  height: 1,
  padding: 0,
  margin: -1,
  overflow: 'hidden',
  clip: 'rect(0 0 0 0)',
  whiteSpace: 'nowrap',
  border: 0,
}
