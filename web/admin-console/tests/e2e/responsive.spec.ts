import { test, expect } from '@playwright/test'

// Usable at desktop + tablet widths (SC-009).
for (const vp of [
  { name: 'desktop', width: 1280, height: 800 },
  { name: 'tablet', width: 834, height: 1112 },
]) {
  test(`navigation and catalog are usable at ${vp.name}`, async ({ page }) => {
    await page.setViewportSize({ width: vp.width, height: vp.height })
    await page.goto('/servers')
    await expect(page.getByRole('button', { name: 'Add server' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Dashboard' })).toBeVisible()
  })
}
