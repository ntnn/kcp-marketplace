import { test, expect, LIVE } from './support/fixtures'

test.describe('auth', () => {
  test('redirects unauthenticated visitors to login', async ({ page, mock }) => {
    test.skip(LIVE, 'mocked-only')
    await mock({ auth: false })
    await page.goto('/workspaces')
    await expect(page).toHaveURL(/\/login/)
    await expect(page.getByTestId('signin')).toBeVisible()
  })

  test('shows a cert hint when the identity provider is unreachable', async ({ page, mock }) => {
    test.skip(LIVE, 'mocked-only')
    await mock({ auth: false })
    // oidc-client-ts fetches the discovery doc on sign-in; fail it.
    await page.route('**/.well-known/openid-configuration', (r) => r.abort())
    await page.goto('/login')
    await page.getByTestId('signin').click()
    await expect(page.getByTestId('cert-hint')).toBeVisible()
  })

  test('does not show workspaces for an expired session', async ({ page, mock }) => {
    test.skip(LIVE, 'mocked-only')
    // No real IdP to renew against; keep the app from navigating away.
    await page.route(/dex\.test|\.well-known|\/auth\b/, (r) => r.abort())
    await mock({ authExpired: true })
    await page.goto('/workspaces')
    await expect(page.getByTestId('ws-row')).toHaveCount(0)
  })
})
