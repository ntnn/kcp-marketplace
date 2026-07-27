import { describe, expect, it, vi, beforeEach } from 'vitest'

vi.mock('@/api/client', () => ({ request: vi.fn(), requestRaw: vi.fn() }))
import { request, requestRaw } from '@/api/client'
import {
  apiBindingPhase,
  bindingExportRef,
  canCreateAPIBinding,
  createAPIBinding,
  deleteAPIBinding,
  discoverResources,
  getObjectYaml,
  waitForBound,
} from '@/api/kube'
import type { BindableAPIExport, KubeObject } from '@/types'

describe('kube api', () => {
  beforeEach(() => vi.clearAllMocks())

  it('discoverResources includes the core group (/api/v1) and named groups', async () => {
    vi.mocked(request).mockImplementation(async (path: string) => {
      if (path.endsWith('/apis')) {
        return { groups: [{ name: 'apps', preferredVersion: { groupVersion: 'apps/v1' } }] }
      }
      if (path.endsWith('/api/v1')) {
        return {
          groupVersion: 'v1',
          resources: [
            { name: 'configmaps', kind: 'ConfigMap', namespaced: true, verbs: ['list'] },
            { name: 'secrets', kind: 'Secret', namespaced: true, verbs: ['list'] },
          ],
        }
      }
      if (path.endsWith('/apis/apps/v1')) {
        return {
          groupVersion: 'apps/v1',
          resources: [
            { name: 'deployments', kind: 'Deployment', namespaced: true, verbs: ['list'] },
          ],
        }
      }
      return { resources: [] }
    })
    const res = await discoverResources('root:alpha')
    const names = res.map((r) => r.name)
    expect(names).toContain('configmaps')
    expect(names).toContain('secrets')
    expect(names).toContain('deployments')
    expect(res.find((r) => r.name === 'configmaps')?.groupVersion).toBe('v1')
  })

  it('canCreateAPIBinding posts a SelfSubjectAccessReview and reads .status.allowed', async () => {
    vi.mocked(request).mockResolvedValue({ status: { allowed: true } })
    const ok = await canCreateAPIBinding('root:team-a')
    expect(ok).toBe(true)
    const [path, init] = vi.mocked(request).mock.calls[0]
    expect(path).toBe(
      '/clusters/root:team-a/apis/authorization.k8s.io/v1/selfsubjectaccessreviews',
    )
    const body = JSON.parse((init as RequestInit).body as string)
    expect(body.spec.resourceAttributes).toEqual({
      group: 'apis.kcp.io',
      resource: 'apibindings',
      verb: 'create',
    })
  })

  it('createAPIBinding references the export by path+name and accepts all claims', async () => {
    vi.mocked(request).mockResolvedValue({})
    const exp: BindableAPIExport = {
      path: 'root',
      cluster: 'root',
      exportName: 'widgets.example.io',
      identityHash: '',
      resources: [],
      permissionClaims: [
        { group: '', resource: 'configmaps', verbs: ['get', 'list'], identityHash: 'abc' },
      ],
    }
    await createAPIBinding('root:team-a', exp)
    const [path, init] = vi.mocked(request).mock.calls[0]
    expect(path).toBe('/clusters/root:team-a/apis/apis.kcp.io/v1alpha2/apibindings')
    const body = JSON.parse((init as RequestInit).body as string)
    expect(body.spec.reference.export).toEqual({ path: 'root', name: 'widgets.example.io' })
    expect(body.metadata.name).toBe('widgets.example.io')
    expect(body.spec.permissionClaims).toEqual([
      {
        group: '',
        resource: 'configmaps',
        verbs: ['get', 'list'],
        identityHash: 'abc',
        selector: { matchAll: true },
        state: 'Accepted',
      },
    ])
  })

  it('getObjectYaml requests the object with a namespace segment when namespaced', async () => {
    vi.mocked(requestRaw).mockResolvedValue('kind: Widget\n')
    const y = await getObjectYaml('root:alpha', 'example.io/v1', 'widgets', 'w1', 'ns1')
    expect(y).toBe('kind: Widget\n')
    expect(requestRaw).toHaveBeenCalledWith(
      '/clusters/root:alpha/apis/example.io/v1/namespaces/ns1/widgets/w1',
    )
  })

  it('getObjectYaml omits the namespace segment for cluster-scoped objects', async () => {
    vi.mocked(requestRaw).mockResolvedValue('kind: Widget\n')
    await getObjectYaml('root:alpha', 'example.io/v1', 'widgets', 'w1')
    expect(requestRaw).toHaveBeenCalledWith('/clusters/root:alpha/apis/example.io/v1/widgets/w1')
  })

  it('apiBindingPhase reads status.phase', async () => {
    vi.mocked(request).mockResolvedValue({ status: { phase: 'Bound' } })
    expect(await apiBindingPhase('root:alpha', 'widgets.example.io')).toBe('Bound')
  })

  it('waitForBound polls until Bound', async () => {
    vi.mocked(request)
      .mockResolvedValueOnce({ status: { phase: 'Binding' } })
      .mockResolvedValueOnce({ status: { phase: 'Bound' } })
    const phase = await waitForBound('root:alpha', 'widgets.example.io', 5000, 1)
    expect(phase).toBe('Bound')
    expect(request).toHaveBeenCalledTimes(2)
  })

  it('deleteAPIBinding issues a DELETE on the binding', async () => {
    vi.mocked(request).mockResolvedValue({})
    await deleteAPIBinding('root:alpha', 'widgets.example.io')
    const [path, init] = vi.mocked(request).mock.calls[0]
    expect(path).toBe('/clusters/root:alpha/apis/apis.kcp.io/v1alpha2/apibindings/widgets.example.io')
    expect((init as RequestInit).method).toBe('DELETE')
  })

  it('bindingExportRef reads spec.reference.export', () => {
    const o: KubeObject = {
      metadata: { name: 'b1' },
      spec: { reference: { export: { path: 'root:alpha', name: 'widgets.example.io' } } },
    }
    expect(bindingExportRef(o)).toEqual({ path: 'root:alpha', name: 'widgets.example.io' })
    expect(bindingExportRef({ metadata: { name: 'x' } })).toBeNull()
  })
})
