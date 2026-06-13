import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

// Full WCAG 2.1 AA scan (incl. color-contrast, which jsdom can't compute) on each
// primary screen. Requires a mock-backed dev server + an admin session.
for (const path of ['/', '/servers', '/audit', '/settings']) {
  test(`no WCAG 2.1 AA violations on ${path}`, async ({ page }) => {
    await page.goto(path)
    const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
    expect(results.violations).toEqual([])
  })
}
