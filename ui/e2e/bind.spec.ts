import { test, expect, LIVE } from './support/fixtures'
import { EXPECT } from './support/mock-backend'
import {
  gotoWorkspaces,
  browse,
  openBindPanel,
  bindRow,
  bindButtonLabel,
  resourceItem,
} from './support/pages'

test('binds and unbinds an APIExport, refreshing the resource list', async ({ page, mock }) => {
  await mock()
  await gotoWorkspaces(page)
  await browse(page, EXPECT.browseWs)
  await openBindPanel(page)

  // Already-bound exports (the defaults) offer Unbind, not Bind.
  for (const e of EXPECT.boundByDefault) {
    await expect(bindRow(page, e).locator('button')).toHaveText(/Unbind/)
  }

  const exp = EXPECT.bindExport
  // Normalise to unbound (a live stack may carry leftover state).
  if ((await bindButtonLabel(page, exp)) === 'Unbind') {
    await bindRow(page, exp).getByTestId('unbind-btn').click()
    await expect(page.getByTestId('bind-msg')).toContainText('unbound', { timeout: 30_000 })
  }
  await expect(bindRow(page, exp).locator('button')).toHaveText('Bind')

  // Bind → polls to Bound → button flips and the new resource appears.
  await bindRow(page, exp).getByTestId('bind-btn').click()
  await expect(page.getByTestId('bind-msg')).toContainText('is bound', { timeout: 30_000 })
  await expect(bindRow(page, exp).locator('button')).toHaveText('Unbind')
  await expect(resourceItem(page, 'widgets')).toBeVisible()

  // Unbind → waits for removal → button flips back and the resource disappears.
  await bindRow(page, exp).getByTestId('unbind-btn').click()
  await expect(page.getByTestId('bind-msg')).toContainText('unbound', { timeout: 30_000 })
  await expect(bindRow(page, exp).locator('button')).toHaveText('Bind')
  await expect(resourceItem(page, 'widgets')).toHaveCount(0)
})

test('disables binding when the SelfSubjectAccessReview denies', async ({ page, mock }) => {
  test.skip(LIVE, 'mocked-only')
  await mock({ ssarAllowed: false })
  await gotoWorkspaces(page)
  await browse(page, EXPECT.browseWs)
  await openBindPanel(page)
  await expect(page.getByTestId('bind-btn').first()).toBeDisabled()
})
