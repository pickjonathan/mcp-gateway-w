import { defineConfig, devices } from '@playwright/test'

// E2E + accessibility (axe) of the primary flows. Run against a dev server with a
// mocked or live control-plane. Browsers install via `npx playwright install`.
export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  reporter: 'list',
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
