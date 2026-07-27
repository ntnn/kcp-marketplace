import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { User } from 'oidc-client-ts'

vi.mock('@/auth/oidc', () => ({
  getUser: vi.fn(),
  login: vi.fn(),
  signinSilent: vi.fn(),
  completeLogin: vi.fn(),
  logout: vi.fn(),
  events: () => ({
    addUserLoaded: vi.fn(),
    addUserUnloaded: vi.fn(),
    addAccessTokenExpired: vi.fn(),
    addSilentRenewError: vi.fn(),
  }),
}))

import * as oidc from '@/auth/oidc'
import { useAuthStore } from '@/stores/auth'

const fresh = (): User => ({ id_token: 'i', expired: false, profile: { email: 'a@b' } }) as unknown as User

describe('auth store re-authentication', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    // jsdom default location is /; ensure a non-auth path.
    window.history.replaceState({}, '', '/workspaces')
  })

  it('handleUnauthorized renews silently when possible', async () => {
    vi.mocked(oidc.signinSilent).mockResolvedValue(fresh())
    const store = useAuthStore()
    await store.handleUnauthorized()
    expect(oidc.signinSilent).toHaveBeenCalledOnce()
    expect(oidc.login).not.toHaveBeenCalled()
    expect(store.isAuthenticated).toBe(true)
  })

  it('handleUnauthorized re-logins when silent renew fails', async () => {
    vi.mocked(oidc.signinSilent).mockRejectedValue(new Error('no session'))
    const store = useAuthStore()
    await store.handleUnauthorized()
    expect(oidc.login).toHaveBeenCalledWith('/workspaces')
    expect(store.reauthenticating).toBe(true)
  })

  it('forceReauth is deduped', async () => {
    const store = useAuthStore()
    store.reauthenticating = true
    await store.forceReauth()
    expect(oidc.login).not.toHaveBeenCalled()
  })
})
