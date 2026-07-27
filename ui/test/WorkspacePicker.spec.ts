import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/marketplace', () => ({
  listAccessibleWorkspaces: vi.fn(),
  listBindableAPIExports: vi.fn(),
}))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

import { listAccessibleWorkspaces } from '@/api/marketplace'
import WorkspacePicker from '@/views/WorkspacePicker.vue'

const stubs = { 'router-link': true }

describe('WorkspacePicker.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('renders a row per accessible workspace', async () => {
    vi.mocked(listAccessibleWorkspaces).mockResolvedValue([
      { path: 'root:team-a', cluster: 'aaa' },
      { path: 'root:team-b', cluster: 'bbb' },
    ])
    const wrapper = mount(WorkspacePicker, { global: { stubs } })
    await flushPromises()
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(wrapper.text()).toContain('root:team-a')
  })

  it('navigates to browse when Browse is clicked', async () => {
    vi.mocked(listAccessibleWorkspaces).mockResolvedValue([{ path: 'root:team-a', cluster: 'aaa' }])
    const wrapper = mount(WorkspacePicker, { global: { stubs } })
    await flushPromises()
    await wrapper.findAll('tbody button')[0].trigger('click')
    expect(push).toHaveBeenCalledWith({ name: 'browse', params: { path: 'root:team-a' } })
  })

  it('shows the empty state', async () => {
    vi.mocked(listAccessibleWorkspaces).mockResolvedValue([])
    const wrapper = mount(WorkspacePicker, { global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('No workspaces you can access.')
  })
})
