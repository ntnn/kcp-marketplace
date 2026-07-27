import { defineStore } from 'pinia'
import type { BindableAPIExport } from '@/types'
import { listBindableAPIExports } from '@/api/marketplace'

interface State {
  items: BindableAPIExport[]
  loading: boolean
  error: string | null
}

export const useApiExportsStore = defineStore('apiexports', {
  state: (): State => ({ items: [], loading: false, error: null }),
  actions: {
    async fetch(): Promise<void> {
      this.loading = true
      this.error = null
      try {
        this.items = await listBindableAPIExports()
      } catch (e) {
        this.error = e instanceof Error ? e.message : String(e)
      } finally {
        this.loading = false
      }
    },
  },
})
