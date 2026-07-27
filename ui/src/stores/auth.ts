import { defineStore } from 'pinia'
import type { User } from 'oidc-client-ts'
import * as oidc from '@/auth/oidc'

interface AuthState {
  user: User | null
  loading: boolean
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({ user: null, loading: false }),
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
  },
})
