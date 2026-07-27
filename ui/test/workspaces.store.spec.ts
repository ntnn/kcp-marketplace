import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('@/api/marketplace', () => ({
  listAccessibleWorkspaces: vi.fn(),
  listBindableAPIExports: vi.fn(),
}))

import { listAccessibleWorkspaces } from '@/api/marketplace'
import { useWorkspacesStore } from '@/stores/workspaces'

describe('workspaces store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads workspaces on fetch', async () => {
    vi.mocked(listAccessibleWorkspaces).mockResolvedValue([
      { path: 'root:team-a', cluster: 'aaa' },
      { path: 'root:team-b', cluster: 'bbb' },
    ])
    const store = useWorkspacesStore()
    await store.fetch()
    expect(store.loading).toBe(false)
    expect(store.items).toHaveLength(2)
    expect(store.byPath('root:team-b')?.cluster).toBe('bbb')
    expect(store.error).toBeNull()
  })

  it('captures the error message on failure', async () => {
    vi.mocked(listAccessibleWorkspaces).mockRejectedValue(new Error('boom'))
    const store = useWorkspacesStore()
    await store.fetch()
    expect(store.error).toBe('boom')
    expect(store.items).toHaveLength(0)
  })

  it('tracks the selected workspace', () => {
    const store = useWorkspacesStore()
    store.select({ path: 'root', cluster: 'root' })
    expect(store.selected?.path).toBe('root')
  })
})
