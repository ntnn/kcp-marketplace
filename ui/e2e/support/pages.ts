import type { Page } from '@playwright/test'
import { expect } from '@playwright/test'

// Page objects shared by the mocked and live specs.

export async function gotoWorkspaces(page: Page): Promise<void> {
  await page.goto('/workspaces')
  await expect(page).toHaveURL(/\/workspaces/)
}

export async function browse(page: Page, wsPath: string): Promise<void> {
  const row = page.locator('[data-testid=ws-row]', { hasText: wsPath })
  await row.getByTestId('ws-browse').click()
  await expect(page).toHaveURL(new RegExp(`${wsPath.replace(/[:]/g, '\\:')}/browse`))
  // Resource sidebar populated.
  await expect(page.locator('[data-testid=resource-item]').first()).toBeVisible()
}

export function resourceItem(page: Page, name: string) {
  return page.locator(`[data-testid=resource-item][data-name="${name}"]`)
}

export async function pickResource(page: Page, name: string): Promise<void> {
  await resourceItem(page, name).click()
}

export async function openBindPanel(page: Page): Promise<void> {
  await page.getByTestId('bind-api').click()
  await expect(page.getByTestId('bind-panel')).toBeVisible()
  // Wait for the export rows to load.
  await expect(page.locator('[data-testid=bind-row]').first()).toBeVisible()
}

export function bindRow(page: Page, exportName: string) {
  return page.locator(`[data-testid=bind-row][data-export="${exportName}"]`)
}

export async function bindButtonLabel(page: Page, exportName: string): Promise<string> {
  return (await bindRow(page, exportName).locator('button').innerText()).trim()
}
