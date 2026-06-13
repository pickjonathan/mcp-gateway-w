import { describe, it, expect, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import axe from 'axe-core'
import { renderWithProviders } from '../util/render'
import { configureApi } from '../../src/api/client'
import { Servers } from '../../src/pages/Servers'
import { Audit } from '../../src/pages/Audit'
import { Settings } from '../../src/pages/Settings'

beforeEach(() => configureApi({ token: 'test', org: 'acme' }))

// Hermetic WCAG 2.1 AA gate (FR-022). jsdom can't compute layout, so
// color-contrast is reported as "incomplete" (covered by the Playwright/axe e2e);
// structural rules (labels, names, roles, headings) run here and must be clean.
async function seriousViolations(container: HTMLElement): Promise<string[]> {
  const results = await axe.run(container, { resultTypes: ['violations'] })
  return results.violations
    .filter((v) => v.impact === 'serious' || v.impact === 'critical')
    .map((v) => v.id)
}

describe('accessibility (axe, structural)', () => {
  it('Servers has no serious/critical violations', async () => {
    const { container } = renderWithProviders(<Servers />)
    await screen.findByText('weather')
    expect(await seriousViolations(container)).toEqual([])
  })

  it('Audit has no serious/critical violations', async () => {
    const { container } = renderWithProviders(<Audit />)
    await screen.findByText('server.create')
    expect(await seriousViolations(container)).toEqual([])
  })

  it('Settings has no serious/critical violations', async () => {
    const { container } = renderWithProviders(<Settings />)
    await screen.findByText('600 / min')
    expect(await seriousViolations(container)).toEqual([])
  })
})
