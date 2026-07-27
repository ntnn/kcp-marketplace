import type { Page, Route } from '@playwright/test'

// Shared constants and the data both projects expect. The mocked backend below
// mirrors what hack/dev-seed.sh produces for admin@example.com, so the same spec
// assertions hold against the live stack.
export const ISSUER = 'https://dex.test'
export const CLIENT = 'kcp-marketplace-ui'
export const USER_EMAIL = 'admin@example.com'

export const EXPECT = {
  workspaces: ['root:demo', 'root:alpha'],
  browseWs: 'root:demo',
  bindExport: 'widgets.example.io',
  boundByDefault: ['tenancy.kcp.io', 'topology.kcp.io'],
  coreResources: ['configmaps', 'secrets'],
}

export interface MockOptions {
  ssarAllowed?: boolean
  workspaces?: string[]
  listError?: boolean
  auth?: boolean
}

function json(route: Route, body: unknown, status = 200): Promise<void> {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

function yaml(route: Route, text: string): Promise<void> {
  return route.fulfill({ status: 200, contentType: 'application/yaml', body: text })
}

export async function installMock(page: Page, opts: MockOptions = {}): Promise<void> {
  const ssarAllowed = opts.ssarAllowed ?? true
  const wsPaths = opts.workspaces ?? EXPECT.workspaces

  // Authenticate by seeding the oidc-client-ts user in localStorage.
  if (opts.auth ?? true) {
    await page.addInitScript(
      ([issuer, client, email]) => {
        const user = {
          id_token: `h.${btoa(JSON.stringify({ sub: 's', email }))}.s`,
          access_token: 'a',
          token_type: 'Bearer',
          profile: { sub: 's', email },
          expires_at: Math.floor(Date.now() / 1000) + 3600,
        }
        localStorage.setItem(`oidc.user:${issuer}:${client}`, JSON.stringify(user))
      },
      [ISSUER, CLIENT, USER_EMAIL] as const,
    )
  }

  const state = { widgetsBound: false }

  await page.route('**/config.json', (r) =>
    json(r, { issuer: ISSUER, clientId: CLIENT, scope: 'openid', apiBase: '' }),
  )

  await page.route('**/services/marketplace/**', (r) => {
    const p = new URL(r.request().url()).pathname
    if (p.endsWith('/accessibleworkspaces')) {
      return json(r, {
        items: wsPaths.map((path) => ({
          metadata: { name: path.replace(/[:]/g, '-') },
          spec: { path, cluster: path.replace(/[:]/g, '-') },
        })),
      })
    }
    if (p.endsWith('/bindableapiexports')) {
      return json(r, { items: exportItems() })
    }
    return json(r, { items: [] })
  })

  await page.route('**/clusters/**', (r) => handleCluster(r, state, { ssarAllowed, listError: !!opts.listError }))
}

function exportItems() {
  const mk = (path: string, name: string, resources: { group: string; resource: string }[] = []) => ({
    metadata: { name },
    spec: { path, cluster: path.replace(/[:]/g, '-'), exportName: name, identityHash: 'hash', resources },
  })
  return [
    mk('root', 'tenancy.kcp.io', [{ group: 'tenancy.kcp.io', resource: 'workspaces' }]),
    mk('root', 'topology.kcp.io', [{ group: 'topology.kcp.io', resource: 'partitions' }]),
    mk('root:alpha', 'widgets.example.io', [{ group: 'example.io', resource: 'widgets' }]),
    mk('root:beta', 'gadgets.example.io', [{ group: 'example.io', resource: 'gadgets' }]),
  ]
}

function bindings(state: { widgetsBound: boolean }) {
  const out = [
    binding('tenancy.kcp.io-abc', 'root', 'tenancy.kcp.io'),
    binding('topology.kcp.io-def', 'root', 'topology.kcp.io'),
  ]
  if (state.widgetsBound) out.push(binding('widgets.example.io', 'root:alpha', 'widgets.example.io'))
  return out
}

function binding(name: string, path: string, exportName: string) {
  return {
    apiVersion: 'apis.kcp.io/v1alpha2',
    kind: 'APIBinding',
    metadata: { name, creationTimestamp: '2024-01-01T00:00:00Z' },
    spec: { reference: { export: { path, name: exportName } } },
    status: { phase: 'Bound' },
  }
}

function groupList(state: { widgetsBound: boolean }) {
  const groups = [
    grp('apis.kcp.io', 'v1alpha2'),
    grp('tenancy.kcp.io', 'v1alpha1'),
    grp('topology.kcp.io', 'v1alpha1'),
  ]
  if (state.widgetsBound) groups.push(grp('example.io', 'v1'))
  return { kind: 'APIGroupList', apiVersion: 'v1', groups }
}

function grp(name: string, version: string) {
  const gv = `${name}/${version}`
  return { name, versions: [{ groupVersion: gv, version }], preferredVersion: { groupVersion: gv, version } }
}

const CORE = {
  groupVersion: 'v1',
  resources: [
    res('configmaps', 'ConfigMap', true),
    res('secrets', 'Secret', true),
    res('namespaces', 'Namespace', false),
  ],
}

function res(name: string, kind: string, namespaced: boolean) {
  return { name, kind, namespaced, verbs: ['list', 'get'] }
}

function resourceListFor(gv: string) {
  if (gv === 'apis.kcp.io/v1alpha2') {
    return { groupVersion: gv, resources: [res('apibindings', 'APIBinding', false)] }
  }
  if (gv === 'tenancy.kcp.io/v1alpha1') {
    return { groupVersion: gv, resources: [res('workspaces', 'Workspace', false)] }
  }
  if (gv === 'topology.kcp.io/v1alpha1') {
    return { groupVersion: gv, resources: [res('partitions', 'Partition', false)] }
  }
  if (gv === 'example.io/v1') {
    return { groupVersion: gv, resources: [res('widgets', 'Widget', true)] }
  }
  return { groupVersion: gv, resources: [] }
}

function objectsFor(resource: string, state: { widgetsBound: boolean }) {
  if (resource === 'apibindings') return bindings(state)
  if (resource === 'configmaps') {
    return [{ metadata: { name: 'kube-root-ca.crt', namespace: 'default', creationTimestamp: '2024-01-01T00:00:00Z' } }]
  }
  if (resource === 'secrets') {
    return [{ metadata: { name: 'default-token', namespace: 'default', creationTimestamp: '2024-01-01T00:00:00Z' } }]
  }
  return []
}

async function handleCluster(
  route: Route,
  state: { widgetsBound: boolean },
  opts: { ssarAllowed: boolean; listError: boolean },
): Promise<void> {
  const req = route.request()
  const method = req.method()
  const url = new URL(req.url())
  const rest = decodeURIComponent(url.pathname.split('/clusters/')[1] ?? '')
  const [, ...tail] = rest.split('/') // drop the workspace segment
  const path = tail.join('/')
  const accept = req.headers()['accept'] ?? ''

  // Discovery.
  if (path === 'apis') return json(route, groupList(state))
  if (path === 'api/v1') return json(route, CORE)

  // SelfSubjectAccessReview gate.
  if (path === 'apis/authorization.k8s.io/v1/selfsubjectaccessreviews' && method === 'POST') {
    return json(route, { status: { allowed: opts.ssarAllowed } })
  }

  const seg = path.split('/')

  // APIBindings collection / item.
  const abBase = 'apis/apis.kcp.io/v1alpha2/apibindings'
  if (path === abBase && method === 'GET') return json(route, { items: bindings(state) })
  if (path === abBase && method === 'POST') {
    state.widgetsBound = true
    return json(route, binding('widgets.example.io', 'root:alpha', 'widgets.example.io'), 201)
  }
  if (path.startsWith(abBase + '/')) {
    const name = seg[seg.length - 1]
    if (method === 'DELETE') {
      if (name === 'widgets.example.io') state.widgetsBound = false
      return json(route, { status: 'Success' })
    }
    if (method === 'GET') {
      // phase poll / yaml
      const b = binding(name, 'root:alpha', name)
      if (accept.includes('yaml')) return yaml(route, `kind: APIBinding\nmetadata:\n  name: ${name}\n`)
      return json(route, b)
    }
  }

  // Resource list at /api/v1/<res> or /apis/<group>/<version>/<res>.
  const isCore = seg[0] === 'api'
  const gvLen = isCore ? 2 : 3
  if (seg.length === gvLen) {
    // groupversion discovery for a single group
    const gv = isCore ? 'v1' : `${seg[1]}/${seg[2]}`
    return json(route, isCore ? CORE : resourceListFor(gv))
  }
  if (seg.length === gvLen + 1) {
    // list <resource>
    if (opts.listError) return json(route, { message: 'boom' }, 500)
    const resource = seg[gvLen]
    return json(route, { items: objectsFor(resource, state) })
  }
  // single object (…/<resource>/<name> or …/namespaces/<ns>/<resource>/<name>)
  const name = seg[seg.length - 1]
  if (accept.includes('yaml')) {
    return yaml(route, `apiVersion: v1\nkind: Object\nmetadata:\n  name: ${name}\n`)
  }
  return json(route, { metadata: { name } })
}
