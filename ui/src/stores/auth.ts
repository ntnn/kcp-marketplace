import { defineStore } from 'pinia'
import type { User } from 'oidc-client-ts'
import * as oidc from '@/auth/oidc'

interface AuthState {
  user: User | null
  loading: boolean
  // Guards against firing multiple re-auth redirects at once.
  reauthenticating: boolean
}

// currentTarget returns the path to return to after re-login, excluding the
// auth routes themselves.
function currentTarget(): string {
  const p = window.location.pathname + window.location.search
  return /^\/(login|callback|silent)/.test(window.location.pathname) ? '/workspaces' : p
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({ user: null, loading: false, reauthenticating: false }),
  getters: {
    isAuthenticated: (s): boolean => !!s.user && !s.user.expired,
    // kcp's OIDC authenticator validates the ID token (aud = clientID); send that
    // as the bearer, not the access token.
    token: (s): string | null => s.user?.id_token ?? s.user?.access_token ?? null,
    username: (s): string => {
      const p = s.user?.profile as Record<string, unknown> | undefined
      return (p?.email as string) || (p?.preferred_username as string) || (p?.sub as string) || ''
    },
    groups: (s): string[] => {
      const g = (s.user?.profile as Record<string, unknown> | undefined)?.groups
      return Array.isArray(g) ? (g as string[]) : []
    },
  },
  actions: {
    async restore(): Promise<void> {
      this.user = await oidc.getUser()
    },
    // initEvents subscribes to oidc-client-ts renewal events: keep the store in
    // sync on silent renew, and re-authenticate when the token finally expires.
    initEvents(): void {
      const ev = oidc.events()
      ev.addUserLoaded((u) => {
        this.user = u
      })
      ev.addUserUnloaded(() => {
        this.user = null
      })
      ev.addAccessTokenExpired(() => {
        void this.forceReauth()
      })
      ev.addSilentRenewError(() => {
        void this.handleUnauthorized()
      })
    },
    async login(targetPath?: string): Promise<void> {
      await oidc.login(targetPath)
    },
    async completeLogin(): Promise<string | undefined> {
      const user = await oidc.completeLogin()
      this.user = user
      const state = user.state as { targetPath?: string } | undefined
      return state?.targetPath
    },
    async logout(): Promise<void> {
      this.user = null
      await oidc.logout()
    },
    // handleUnauthorized reacts to a 401: try a silent renew first, and only fall
    // back to a full re-login if that fails.
    async handleUnauthorized(): Promise<void> {
      if (this.reauthenticating) return
      try {
        const u = await oidc.signinSilent()
        if (u && !u.expired) {
          this.user = u
          return
        }
      } catch {
        /* fall through to re-login */
      }
      await this.forceReauth()
    },
    async forceReauth(): Promise<void> {
      if (this.reauthenticating) return
      this.reauthenticating = true
      this.user = null
      await oidc.login(currentTarget())
    },
  },
})
