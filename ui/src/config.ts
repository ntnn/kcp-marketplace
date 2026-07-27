// Runtime configuration loaded from /config.json so the same build can target
// different Dex issuers / front-proxy origins without rebuilding.
export interface RuntimeConfig {
  // OIDC issuer URL (Dex).
  issuer: string
  // OIDC public client id for the SPA.
  clientId: string
  // OIDC scopes; must include openid and groups for kcp authz.
  scope: string
  // Base URL for API calls. Empty means same-origin (the front-proxy), which is
  // the production shape; in dev Vite proxies to the front-proxy.
  apiBase: string
}

let cached: RuntimeConfig | null = null

export async function loadConfig(): Promise<RuntimeConfig> {
  if (cached) return cached
  const res = await fetch('/config.json', { cache: 'no-store' })
  if (!res.ok) throw new Error(`failed to load /config.json: ${res.status}`)
  cached = (await res.json()) as RuntimeConfig
  return cached
}

// getConfig returns the already-loaded config, or throws if loadConfig has not run.
export function getConfig(): RuntimeConfig {
  if (!cached) throw new Error('config not loaded; call loadConfig() first')
  return cached
}

// Test seam.
export function __setConfig(c: RuntimeConfig | null): void {
  cached = c
}
