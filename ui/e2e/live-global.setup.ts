import { test as setup } from '@playwright/test'
import fs from 'node:fs'

// Logs the demo admin in through real Dex and saves the session so the live
// specs reuse it. Requires the dev stack to be up (`make up`).
setup('authenticate admin', async ({ page }) => {
  await page.goto('/')
  await page.getByTestId('signin').click()
  await page.waitForURL(/dex.*\/auth\/local\/login/, { timeout: 20_000 })
  await page.fill('input[name=login]', 'admin@example.com')
  await page.fill('input[name=password]', 'password')
  await page.click('button[type=submit], input[type=submit]')
  await page.waitForURL(/\/workspaces/, { timeout: 20_000 })
  fs.mkdirSync('.playwright', { recursive: true })
  await page.context().storageState({ path: '.playwright/admin.json' })
})
