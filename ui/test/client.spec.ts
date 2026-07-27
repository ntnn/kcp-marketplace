import { beforeEach, describe, expect, it, vi, afterEach } from 'vitest'
import { __setConfig } from '@/config'
import { request, setTokenProvider, setUnauthorizedHandler, ApiError } from '@/api/client'

describe('api client', () => {
  beforeEach(() => {
    __setConfig({ issuer: '', clientId: '', scope: '', apiBase: '' })
    setTokenProvider(() => 'tok-123')
  })
  afterEach(() => {
    vi.restoreAllMocks()
    __setConfig(null)
  })

  it('attaches the bearer token and parses JSON', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) =>
      new Response(JSON.stringify({ ok: true }), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const out = await request<{ ok: boolean }>('/services/x')
    expect(out.ok).toBe(true)
    const init = fetchMock.mock.calls[0][1]
    expect((init!.headers as Headers).get('Authorization')).toBe('Bearer tok-123')
  })

  it('honours apiBase', async () => {
    __setConfig({ issuer: '', clientId: '', scope: '', apiBase: 'http://proxy' })
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) =>
      new Response('{}', { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await request('/clusters/root/apis')
    expect(fetchMock.mock.calls[0][0]).toBe('http://proxy/clusters/root/apis')
  })

  it('throws ApiError with the server message on failure', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        new Response(JSON.stringify({ message: 'forbidden: nope' }), { status: 403 }),
      ),
    )
    await expect(request('/x')).rejects.toMatchObject({
      constructor: ApiError,
      status: 403,
      message: 'forbidden: nope',
    })
  })

  it('invokes the unauthorized handler on 401', async () => {
    const onUnauthorized = vi.fn()
    setUnauthorizedHandler(onUnauthorized)
    vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 401 })))
    await expect(request('/x')).rejects.toMatchObject({ status: 401 })
    expect(onUnauthorized).toHaveBeenCalledOnce()
    setUnauthorizedHandler(() => {})
  })
})
