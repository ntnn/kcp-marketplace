import { test as base, expect } from '@playwright/test'
import { installMock, type MockOptions } from './mock-backend'

// LIVE selects the real dev stack (no mocking; auth via saved storageState).
export const LIVE = !!process.env.E2E_LIVE

interface Fixtures {
  // mock installs the fake backend for the mocked project; a no-op in live.
  mock: (opts?: MockOptions) => Promise<void>
}

export const test = base.extend<Fixtures>({
  mock: async ({ page }, use) => {
    await use(async (opts?: MockOptions) => {
      if (!LIVE) await installMock(page, opts)
    })
  },
})

export { expect }
