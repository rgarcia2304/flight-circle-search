import { defineConfig, devices } from '@playwright/test';
import { AUTH_STATE_PATH } from './tests/e2e/live-auth';

// Separate from playwright.config.ts on purpose: these tests need a real
// backend running locally (E2E_TEST_MODE=true go run ./cmd/api) and hit
// live third-party fare data, so they're never run in CI — only manually,
// via `npm run test:e2e:live`. See tests/e2e/live-global-setup.ts.
export default defineConfig({
  testDir: './tests/e2e',
  testMatch: /live-.*\.spec\.ts$/,
  globalSetup: './tests/e2e/live-global-setup.ts',
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  timeout: 60_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: 'http://localhost:5173',
    storageState: AUTH_STATE_PATH,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: true,
    timeout: 120_000,
  },
});
