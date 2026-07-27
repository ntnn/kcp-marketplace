import { defineConfig, devices } from '@playwright/test'

// Two modes selected by the E2E_LIVE env (set by the npm scripts):
//   mocked (default) — runs the built SPA against an in-memory fake backend; the
//     PR-gating suite. No cluster required.
//   live — runs the SPA (VITE_API_PROXY -> front-proxy) against a running dev
//     stack (`make up`), authenticating through real Dex. Not run in CI.
const live = !!process.env.E2E_LIVE

const MOCKED_URL = 'http://localhost:4173'
const LIVE_URL = 'http://localhost:5173'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? [['github'], ['list']] : [['list']],

  webServer: live
    ? {
        command: 'VITE_API_PROXY=https://kcp.127.0.0.1.nip.io:8443 npm run dev',
        url: LIVE_URL,
        reuseExistingServer: true,
        timeout: 120_000,
      }
    : {
        command: 'npm run build && npm run preview -- --port 4173 --strictPort',
        url: MOCKED_URL,
        reuseExistingServer: !process.env.CI,
        timeout: 180_000,
      },

  projects: live
    ? [
        {
          name: 'setup-live',
          testMatch: /live-global\.setup\.ts/,
          use: { ...devices['Desktop Chrome'], baseURL: LIVE_URL, ignoreHTTPSErrors: true },
        },
        {
          name: 'live',
          testMatch: /\.spec\.ts$/,
          dependencies: ['setup-live'],
          use: {
            ...devices['Desktop Chrome'],
            baseURL: LIVE_URL,
            ignoreHTTPSErrors: true,
            storageState: '.playwright/admin.json',
          },
        },
      ]
    : [
        {
          name: 'mocked',
          testMatch: /\.spec\.ts$/,
          use: { ...devices['Desktop Chrome'], baseURL: MOCKED_URL },
        },
      ],
})
