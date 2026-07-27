import { request } from './client'
import type {
  AccessibleWorkspace,
  BindableAPIExport,
  BindableResource,
  KubeList,
} from '@/types'

const GROUP = 'marketplace.kcp.io/v1alpha1'
const BASE = `/services/marketplace/apis/${GROUP}`

interface RawWorkspace {
  metadata?: { name?: string }
  spec?: { path?: string; cluster?: string }
}

interface RawExport {
  metadata?: { name?: string }
  spec?: {
    path?: string
    cluster?: string
    exportName?: string
    identityHash?: string
    resources?: BindableResource[]
  }
}

export async function listAccessibleWorkspaces(): Promise<AccessibleWorkspace[]> {
  const list = await request<KubeList<RawWorkspace>>(`${BASE}/accessibleworkspaces`)
  return (list.items ?? []).map((w) => ({
    cluster: w.spec?.cluster ?? w.metadata?.name ?? '',
    path: w.spec?.path ?? '',
  }))
}

export async function listBindableAPIExports(): Promise<BindableAPIExport[]> {
  const list = await request<KubeList<RawExport>>(`${BASE}/bindableapiexports`)
  return (list.items ?? []).map((e) => ({
    cluster: e.spec?.cluster ?? '',
    path: e.spec?.path ?? '',
    exportName: e.spec?.exportName ?? e.metadata?.name ?? '',
    identityHash: e.spec?.identityHash ?? '',
    resources: e.spec?.resources ?? [],
  }))
}
