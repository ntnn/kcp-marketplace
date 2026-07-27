import { defineStore } from 'pinia'
import type { AccessibleWorkspace } from '@/types'
import { listAccessibleWorkspaces } from '@/api/marketplace'

interface State {
  items: AccessibleWorkspace[]
  loading: boolean
  error: string | null
  selected: AccessibleWorkspace | null
}

export const useWorkspacesStore = defineStore('workspaces', {
  state: (): State => ({ items: [], loading: false, error: null, selected: null }),
  actions: {
    async fetch(): Promise<void> {
      this.loading = true
      this.error = null
      try {
        this.items = await listAccessibleWorkspaces()
      } catch (e) {
        this.error = e instanceof Error ? e.message : String(e)
      } finally {
        this.loading = false
      }
    },
    select(ws: AccessibleWorkspace | null): void {
      this.selected = ws
    },
    byPath(path: string): AccessibleWorkspace | undefined {
      return this.items.find((w) => w.path === path)
    },
  },
})
