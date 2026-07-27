<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { useWorkspacesStore } from '@/stores/workspaces'
import type { AccessibleWorkspace } from '@/types'

const store = useWorkspacesStore()
const { items, loading, error } = storeToRefs(store)
const router = useRouter()

onMounted(() => {
  if (items.value.length === 0) store.fetch()
})

function browse(ws: AccessibleWorkspace) {
  store.select(ws)
  router.push({ name: 'browse', params: { path: ws.path } })
}
</script>

<template>
  <section>
    <header class="row">
      <h2>Accessible workspaces</h2>
      <button type="button" :disabled="loading" @click="store.fetch()">Refresh</button>
    </header>

    <p v-if="loading">Loading…</p>
    <p v-else-if="error" class="error">{{ error }}</p>
    <p v-else-if="items.length === 0">No workspaces you can access.</p>

    <table v-else>
      <thead>
        <tr><th>Path</th><th>Cluster</th><th></th></tr>
      </thead>
      <tbody>
        <tr v-for="ws in items" :key="ws.cluster">
          <td>{{ ws.path }}</td>
          <td><code>{{ ws.cluster }}</code></td>
          <td class="actions">
            <button type="button" @click="browse(ws)">Browse</button>
          </td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<style scoped>
.row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.actions {
  display: flex;
  gap: 0.5rem;
}
.error {
  color: #b00020;
}
table {
  width: 100%;
  border-collapse: collapse;
}
th,
td {
  text-align: left;
  padding: 0.4rem 0.6rem;
  border-bottom: 1px solid #eee;
}
</style>
