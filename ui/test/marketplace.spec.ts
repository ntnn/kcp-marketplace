import { describe, expect, it, vi, beforeEach } from 'vitest'

vi.mock('@/api/client', () => ({ request: vi.fn() }))
import { request } from '@/api/client'
import { listAccessibleWorkspaces, listBindableAPIExports } from '@/api/marketplace'

describe('marketplace api mapping', () => {
  beforeEach(() => vi.clearAllMocks())

  it('maps AccessibleWorkspaceList to domain workspaces', async () => {
    vi.mocked(request).mockResolvedValue({
      items: [{ metadata: { name: 'aaa' }, spec: { path: 'root:team-a', cluster: 'aaa' } }],
    })
    const ws = await listAccessibleWorkspaces()
    expect(request).toHaveBeenCalledWith(
      '/services/marketplace/apis/marketplace.kcp.io/v1alpha1/accessibleworkspaces',
    )
    expect(ws).toEqual([{ path: 'root:team-a', cluster: 'aaa' }])
  })

  it('maps BindableAPIExportList to domain exports', async () => {
    vi.mocked(request).mockResolvedValue({
      items: [
        {
          metadata: { name: 'widgets.example.io' },
          spec: {
            path: 'root',
            cluster: 'root',
            exportName: 'widgets.example.io',
            identityHash: 'deadbeef',
            resources: [{ group: 'example.io', resource: 'widgets' }],
            permissionClaims: [{ group: '', resource: 'configmaps', verbs: ['get'], identityHash: 'h' }],
          },
        },
      ],
    })
    const exps = await listBindableAPIExports()
    expect(exps[0]).toEqual({
      path: 'root',
      cluster: 'root',
      exportName: 'widgets.example.io',
      identityHash: 'deadbeef',
      resources: [{ group: 'example.io', resource: 'widgets' }],
      permissionClaims: [{ group: '', resource: 'configmaps', verbs: ['get'], identityHash: 'h' }],
    })
  })
})
