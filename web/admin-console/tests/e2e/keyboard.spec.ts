import { test, expect } from '@playwright/test'

// Keyboard-only operability with visible focus (FR-022).
test('primary flow is keyboard operable', async ({ page }) => {
  await page.goto('/servers')
  await page.getByRole('button', { name: 'Add server' }).focus()
  await expect(page.locator(':focus')).toBeVisible()
  await page.keyboard.press('Enter')
  await expect(page.getByLabel('Name (slug)')).toBeVisible()
  await page.getByLabel('Name (slug)').fill('kbd-test')
  await expect(page.getByLabel('Name (slug)')).toHaveValue('kbd-test')
})
