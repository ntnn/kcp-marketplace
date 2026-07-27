import { getConfig } from '@/config'

// Token provider is injected at app start (from the auth store) to avoid a
// store<->api import cycle.
let tokenProvider: () => string | null = () => null

export function setTokenProvider(fn: () => string | null): void {
  tokenProvider = fn
}

// Unauthorized handler is injected at app start; called on any 401 so the auth
// store can silently renew or re-login.
let unauthorizedHandler: () => void = () => {}

export function setUnauthorizedHandler(fn: () => void): void {
  unauthorizedHandler = fn
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
    this.name = 'ApiError'
  }
}

// request issues an authenticated call against the front-proxy origin. path is
// an absolute API path (e.g. /clusters/root/apis/...); apiBase is prepended so
// the same build can target a proxied dev origin.
export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const base = getConfig().apiBase
  const token = tokenProvider()
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(`${base}${path}`, { ...init, headers })
  const text = await res.text()
  if (!res.ok) {
    if (res.status === 401) unauthorizedHandler()
    throw new ApiError(res.status, extractMessage(text) || `${res.status} ${res.statusText}`)
  }
  return (text ? JSON.parse(text) : undefined) as T
}

// requestRaw returns the response body as text, negotiating the given content
// type (e.g. application/yaml). Used to show an object's server-rendered YAML.
export async function requestRaw(path: string, accept = 'application/yaml'): Promise<string> {
  const base = getConfig().apiBase
  const token = tokenProvider()
  const headers = new Headers({ Accept: accept })
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(`${base}${path}`, { headers })
  const text = await res.text()
  if (!res.ok) {
    if (res.status === 401) unauthorizedHandler()
    throw new ApiError(res.status, extractMessage(text) || `${res.status} ${res.statusText}`)
  }
  return text
}

function extractMessage(body: string): string | null {
  try {
    const obj = JSON.parse(body) as { message?: string }
    return obj.message ?? null
  } catch {
    return null
  }
}
