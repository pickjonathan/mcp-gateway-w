import { test, expect } from '@playwright/test'

// End-to-end: add a remote and a stdio server and see them in the catalog.
// Requires a dev server backed by the API mock and an authenticated admin
// session (see playwright.config.ts / quickstart). Run with:
//   npx playwright install && npm run test:e2e
test.describe('Servers catalog (e2e)', () => {
  test('add a remote and a stdio server, both appear in the catalog', async ({ page }) => {
    await page.goto('/servers')

    // Add a remote server.
    await page.getByRole('button', { name: 'Add server' }).click()
    await page.getByLabel('Name (slug)').fill('weather')
    await page.getByLabel('Type').selectOption('remote_http')
    await page.getByLabel('Endpoint URL').fill('https://mcp.example.org/weather')
    await page.getByRole('button', { name: 'Create server' }).click()
    await expect(page.getByText('weather')).toBeVisible()

    // Add a stdio server.
    await page.getByRole('button', { name: 'Add server' }).click()
    await page.getByLabel('Name (slug)').fill('thinking')
    await page.getByLabel('Type').selectOption('stdio')
    await page.getByLabel('Command').fill('npx')
    await page.getByLabel('Arguments (space-separated)').fill('-y @modelcontextprotocol/server-sequential-thinking')
    await page.getByRole('button', { name: 'Create server' }).click()
    await expect(page.getByText('thinking')).toBeVisible()
  })
})
