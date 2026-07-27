// Domain types mirroring the marketplace.kcp.io API and the kcp resources the
// SPA touches.

export interface AccessibleWorkspace {
  // Logical cluster name (metadata.name).
  cluster: string
  // Human-readable workspace path, e.g. root:team-a.
  path: string
}

export interface BindableResource {
  group: string
  resource: string
}

export interface PermissionClaim {
  group: string
  resource: string
  verbs: string[]
  identityHash: string
}

export interface BindableAPIExport {
  cluster: string
  path: string
  exportName: string
  identityHash: string
  resources: BindableResource[]
  permissionClaims: PermissionClaim[]
}

// A generic Kubernetes-like object as returned by list endpoints.
export interface KubeObject {
  apiVersion?: string
  kind?: string
  metadata?: { name?: string; namespace?: string; creationTimestamp?: string }
  [key: string]: unknown
}

export interface KubeList<T = KubeObject> {
  apiVersion?: string
  kind?: string
  items: T[]
}

// A discovered API resource offered by a workspace.
export interface APIResource {
  groupVersion: string
  kind: string
  name: string
  namespaced: boolean
  verbs: string[]
}
