import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  retries: 1,
  use: {
    baseURL: process.env.CI ? 'http://localhost:8080' : 'http://localhost:5173',
    headless: true,
    screenshot: 'only-on-failure',
  },
  // In CI, docker compose starts the services; no webServer needed.
  // In local dev, the vite dev server is assumed to be running.
  webServer: process.env.CI
    ? undefined
    : {
        command: 'npm run dev -- --port 5173',
        port: 5173,
        reuseExistingServer: true,
        timeout: 30000,
      },
});
