import { request, requestRaw } from './client'
import type { APIResource, BindableAPIExport, KubeList, KubeObject } from '@/types'

// clusterBase maps a workspace path (root:team-a) to its front-proxy cluster
// path (/clusters/root:team-a).
function clusterBase(workspacePath: string): string {
  return `/clusters/${workspacePath}`
}

interface APIGroupList {
  groups: { name: string; preferredVersion: { groupVersion: string } }[]
}

interface APIResourceList {
  groupVersion: string
  resources: {
    name: string
    kind: string
    namespaced: boolean
    verbs: string[]
    // Subresources contain a slash; skip them for browsing.
  }[]
}

// discoverResources returns the listable, top-level resources served by a
// workspace: the core group (/api/v1) plus every preferred group version
// under /apis.
export async function discoverResources(workspacePath: string): Promise<APIResource[]> {
  const base = clusterBase(workspacePath)
  const groups = await request<APIGroupList>(`${base}/apis`)
  const groupVersions = groups.groups.map((g) => g.preferredVersion.groupVersion)

  const out: APIResource[] = []
  const lists = await Promise.all([
    // Core group is served at /api/v1, not /apis.
    request<APIResourceList>(`${base}/api/v1`).catch(() => null),
    ...groupVersions.map((gv) =>
      request<APIResourceList>(`${base}/apis/${gv}`).catch(() => null),
    ),
  ])
  for (const list of lists) {
    if (!list) continue
    for (const r of list.resources) {
      if (r.name.includes('/')) continue
      if (!r.verbs.includes('list')) continue
      out.push({
        groupVersion: list.groupVersion,
        kind: r.kind,
        name: r.name,
        namespaced: r.namespaced,
        verbs: r.verbs,
      })
    }
  }
  out.sort((a, b) => a.name.localeCompare(b.name))
  return out
}

// listResource lists objects of a resource in a workspace.
export async function listResource(
  workspacePath: string,
  groupVersion: string,
  resource: string,
): Promise<KubeObject[]> {
  const base = clusterBase(workspacePath)
  const prefix = groupVersion.includes('/') ? 'apis' : 'api'
  const list = await request<KubeList>(`${base}/${prefix}/${groupVersion}/${resource}`)
  return list.items ?? []
}

// getObjectYaml returns the server-rendered YAML of a single object.
export async function getObjectYaml(
  workspacePath: string,
  groupVersion: string,
  resource: string,
  name: string,
  namespace?: string,
): Promise<string> {
  const base = clusterBase(workspacePath)
  const prefix = groupVersion.includes('/') ? 'apis' : 'api'
  const ns = namespace ? `/namespaces/${namespace}` : ''
  return requestRaw(`${base}/${prefix}/${groupVersion}${ns}/${resource}/${name}`)
}

// listAPIBindings lists the APIBindings in a workspace.
export async function listAPIBindings(workspacePath: string): Promise<KubeObject[]> {
  const base = clusterBase(workspacePath)
  const list = await request<KubeList>(`${base}/apis/apis.kcp.io/v1alpha2/apibindings`)
  return list.items ?? []
}

// deleteAPIBinding removes an APIBinding (unbind) from a workspace.
export async function deleteAPIBinding(workspacePath: string, name: string): Promise<void> {
  const base = clusterBase(workspacePath)
  await request<unknown>(`${base}/apis/apis.kcp.io/v1alpha2/apibindings/${name}`, {
    method: 'DELETE',
  })
}

// bindingExportRef returns the {path,name} an APIBinding references, if any.
export function bindingExportRef(o: KubeObject): { path: string; name: string } | null {
  const exp = (o.spec as { reference?: { export?: { path?: string; name?: string } } } | undefined)
    ?.reference?.export
  if (!exp?.name) return null
  return { path: exp.path ?? '', name: exp.name }
}

// apiBindingPhase returns the status.phase of an APIBinding (e.g. "Bound").
export async function apiBindingPhase(workspacePath: string, name: string): Promise<string> {
  const base = clusterBase(workspacePath)
  const obj = await request<{ status?: { phase?: string } }>(
    `${base}/apis/apis.kcp.io/v1alpha2/apibindings/${name}`,
  )
  return obj.status?.phase ?? ''
}

// waitForBound polls an APIBinding until its phase is "Bound" or the timeout
// elapses; returns the final phase.
export async function waitForBound(
  workspacePath: string,
  name: string,
  timeoutMs = 20000,
  intervalMs = 750,
): Promise<string> {
  const deadline = Date.now() + timeoutMs
  let phase = ''
  while (Date.now() < deadline) {
    phase = await apiBindingPhase(workspacePath, name)
    if (phase === 'Bound') return phase
    await new Promise((r) => setTimeout(r, intervalMs))
  }
  return phase
}

// waitForGone polls until an APIBinding no longer appears (deletion completed).
export async function waitForGone(
  workspacePath: string,
  name: string,
  timeoutMs = 20000,
  intervalMs = 750,
): Promise<boolean> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const list = await listAPIBindings(workspacePath)
    if (!list.some((b) => b.metadata?.name === name)) return true
    await new Promise((r) => setTimeout(r, intervalMs))
  }
  return false
}

// canCreateAPIBinding runs a SelfSubjectAccessReview to gate the create button.
export async function canCreateAPIBinding(workspacePath: string): Promise<boolean> {
  const base = clusterBase(workspacePath)
  const body = {
    apiVersion: 'authorization.k8s.io/v1',
    kind: 'SelfSubjectAccessReview',
    spec: {
      resourceAttributes: {
        group: 'apis.kcp.io',
        resource: 'apibindings',
        verb: 'create',
      },
    },
  }
  const res = await request<{ status?: { allowed?: boolean } }>(
    `${base}/apis/authorization.k8s.io/v1/selfsubjectaccessreviews`,
    { method: 'POST', body: JSON.stringify(body) },
  )
  return res.status?.allowed ?? false
}

// createAPIBinding binds an export into the workspace via a permission-claim-free
// APIBinding referencing the export by path+name.
export async function createAPIBinding(
  workspacePath: string,
  exp: BindableAPIExport,
  bindingName?: string,
): Promise<KubeObject> {
  const base = clusterBase(workspacePath)
  const name = bindingName || exp.exportName
  // Accept every permission claim the export requests (matchAll scope).
  const permissionClaims = (exp.permissionClaims ?? []).map((c) => ({
    group: c.group,
    resource: c.resource,
    verbs: c.verbs,
    identityHash: c.identityHash,
    selector: { matchAll: true },
    state: 'Accepted',
  }))
  const body = {
    apiVersion: 'apis.kcp.io/v1alpha2',
    kind: 'APIBinding',
    metadata: { name },
    spec: {
      reference: {
        export: { path: exp.path, name: exp.exportName },
      },
      permissionClaims,
    },
  }
  return request<KubeObject>(`${base}/apis/apis.kcp.io/v1alpha2/apibindings`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}
