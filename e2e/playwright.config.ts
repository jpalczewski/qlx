import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  retries: 0,
  timeout: 30000,
  reporter: [['html', { open: 'never' }], ['list']],
  use: {
    trace: 'on-first-retry',
    locale: 'pl-PL',
    extraHTTPHeaders: {
      'Accept-Language': 'pl',
    },
    // SSE (EventSource for printer status) keeps connections open indefinitely,
    // which can block the default 'load' waitUntil on slow CI runners.
    navigationTimeout: 15000,
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
