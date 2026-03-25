import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  retries: process.env.CI ? 2 : 0,
  timeout: 30000,
  reporter: [['html', { open: 'never' }], ['list']],
  use: {
    trace: 'on-first-retry',
    locale: 'pl-PL',
    extraHTTPHeaders: {
      'Accept-Language': 'pl',
    },
    // SSE (EventSource for printer status) keeps connections open indefinitely,
    // which blocks the default 'load' waitUntil. Use 'domcontentloaded' globally.
    navigationTimeout: 20000,
    actionTimeout: 15000,
  },
  expect: {
    timeout: 10000,
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
