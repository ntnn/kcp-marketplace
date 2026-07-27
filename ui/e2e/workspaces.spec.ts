import { test, expect } from './support/fixtures'
import { EXPECT } from './support/mock-backend'
import { gotoWorkspaces, browse } from './support/pages'

test('lists accessible workspaces and browses one', async ({ page, mock }) => {
  await mock()
  await gotoWorkspaces(page)

  for (const ws of EXPECT.workspaces) {
    await expect(page.locator('[data-testid=ws-row]', { hasText: ws })).toBeVisible()
  }

  await browse(page, EXPECT.browseWs)
})
