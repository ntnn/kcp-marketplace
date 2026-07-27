import { test, expect } from './support/fixtures'
import { EXPECT } from './support/mock-backend'
import { gotoWorkspaces, browse, pickResource } from './support/pages'

test('shows core + named resources, lists objects, and views YAML', async ({ page, mock }) => {
  await mock()
  await gotoWorkspaces(page)
  await browse(page, EXPECT.browseWs)

  // Core group (/api/v1) resources are present alongside named groups.
  for (const r of EXPECT.coreResources) {
    await expect(page.locator(`[data-testid=resource-item][data-name="${r}"]`)).toBeVisible()
  }

  // apibindings always has objects (the default bindings); view one as YAML.
  await pickResource(page, 'apibindings')
  const row = page.locator('[data-testid=object-row]').first()
  await expect(row).toBeVisible()
  await row.click()

  await expect(page.getByTestId('yaml-panel')).toBeVisible()
  await expect(page.getByTestId('yaml')).toContainText('kind')
})
