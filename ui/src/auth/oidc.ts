import { UserManager, WebStorageStateStore, type User } from 'oidc-client-ts'
import { getConfig } from '@/config'

let manager: UserManager | null = null

// userManager lazily builds the oidc-client-ts UserManager from runtime config.
// Auth-Code + PKCE against Dex; tokens kept in localStorage so a reload restores
// the session; silent renew keeps the access token fresh.
export function userManager(): UserManager {
  if (manager) return manager
  const cfg = getConfig()
  const origin = window.location.origin
  manager = new UserManager({
    authority: cfg.issuer,
    client_id: cfg.clientId,
    redirect_uri: `${origin}/callback`,
    post_logout_redirect_uri: origin,
    response_type: 'code',
    scope: cfg.scope,
    automaticSilentRenew: true,
    userStore: new WebStorageStateStore({ store: window.localStorage }),
  })
  return manager
}

export async function login(targetPath?: string): Promise<void> {
  await userManager().signinRedirect({ state: { targetPath } })
}

export async function completeLogin(): Promise<User> {
  return userManager().signinRedirectCallback()
}

export async function logout(): Promise<void> {
  await userManager().signoutRedirect()
}

export async function getUser(): Promise<User | null> {
  return userManager().getUser()
}

// Test seam.
export function __setUserManager(m: UserManager | null): void {
  manager = m
}
