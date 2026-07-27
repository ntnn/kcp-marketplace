<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import {
  bindingExportRef,
  canCreateAPIBinding,
  createAPIBinding,
  deleteAPIBinding,
  discoverResources,
  getObjectYaml,
  listAPIBindings,
  listResource,
  waitForBound,
  waitForGone,
} from '@/api/kube'
import { useApiExportsStore } from '@/stores/apiexports'
import type { APIResource, BindableAPIExport, KubeObject } from '@/types'

const props = defineProps<{ path: string }>()

const resources = ref<APIResource[]>([])
const selected = ref<APIResource | null>(null)
const objects = ref<KubeObject[]>([])
const loadingResources = ref(false)
const loadingObjects = ref(false)
const error = ref<string | null>(null)

// Bind panel.
const exportsStore = useApiExportsStore()
const { items: allExports, loading: exportsLoading } = storeToRefs(exportsStore)
const showBind = ref(false)
const canBind = ref<boolean | null>(null)
const binding = ref<string | null>(null)
const bindMsg = ref<string | null>(null)
const bindError = ref<string | null>(null)
const bindings = ref<KubeObject[]>([])
// An export hosted by this very workspace is already available here.
const bindable = computed(() => allExports.value.filter((e) => e.path !== props.path))

// boundName returns the APIBinding name for an export if it is already bound here.
function boundName(exp: BindableAPIExport): string | null {
  for (const b of bindings.value) {
    const ref = bindingExportRef(b)
    if (ref && ref.path === exp.path && ref.name === exp.exportName) {
      return (b.metadata?.name as string) ?? null
    }
  }
  return null
}

// YAML viewer.
const yaml = ref<{ title: string; text: string } | null>(null)
const yamlLoading = ref(false)

async function loadResources() {
  loadingResources.value = true
  error.value = null
  try {
    resources.value = await discoverResources(props.path)
    // Keep the current selection if it still exists after a refresh.
    if (selected.value) {
      const still = resources.value.find(
        (r) => r.name === selected.value!.name && r.groupVersion === selected.value!.groupVersion,
      )
      selected.value = still ?? null
      if (still) await pick(still)
      else objects.value = []
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loadingResources.value = false
  }
}

async function pick(r: APIResource) {
  selected.value = r
  loadingObjects.value = true
  error.value = null
  try {
    objects.value = await listResource(props.path, r.groupVersion, r.name)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
    objects.value = []
  } finally {
    loadingObjects.value = false
  }
}

async function openBind() {
  showBind.value = true
  bindMsg.value = null
  bindError.value = null
  if (allExports.value.length === 0) await exportsStore.fetch()
  try {
    canBind.value = await canCreateAPIBinding(props.path)
  } catch {
    canBind.value = false
  }
  await refreshBindings()
}

async function refreshBindings() {
  try {
    bindings.value = await listAPIBindings(props.path)
  } catch {
    bindings.value = []
  }
}

async function bind(exp: BindableAPIExport) {
  binding.value = exp.exportName
  bindMsg.value = null
  bindError.value = null
  try {
    const created = await createAPIBinding(props.path, exp)
    const name = (created.metadata?.name as string) || exp.exportName
    bindMsg.value = `Binding ${exp.exportName}…`
    const phase = await waitForBound(props.path, name)
    if (phase === 'Bound') {
      bindMsg.value = `${exp.exportName} is bound; resources refreshed.`
      await loadResources()
    } else {
      bindMsg.value = `${exp.exportName} created; current phase: ${phase || 'unknown'}.`
    }
  } catch (e) {
    bindError.value = e instanceof Error ? e.message : String(e)
  } finally {
    binding.value = null
    await refreshBindings()
  }
}

async function unbind(exp: BindableAPIExport) {
  const name = boundName(exp)
  if (!name) return
  binding.value = exp.exportName
  bindMsg.value = null
  bindError.value = null
  try {
    await deleteAPIBinding(props.path, name)
    // Optimistically drop it so the button flips immediately, then wait for kcp
    // to finish removing it before refreshing the resource list.
    bindings.value = bindings.value.filter((b) => (b.metadata?.name as string) !== name)
    bindMsg.value = `Unbinding ${exp.exportName}…`
    await waitForGone(props.path, name)
    bindMsg.value = `${exp.exportName} unbound; resources refreshed.`
    await loadResources()
  } catch (e) {
    bindError.value = e instanceof Error ? e.message : String(e)
  } finally {
    binding.value = null
    await refreshBindings()
  }
}

async function showYaml(o: KubeObject) {
  if (!selected.value) return
  const name = o.metadata?.name
  if (!name) return
  yamlLoading.value = true
  yaml.value = { title: name, text: '' }
  try {
    yaml.value = {
      title: name,
      text: await getObjectYaml(
        props.path,
        selected.value.groupVersion,
        selected.value.name,
        name,
        selected.value.namespaced ? o.metadata?.namespace : undefined,
      ),
    }
  } catch (e) {
    yaml.value = { title: name, text: e instanceof Error ? e.message : String(e) }
  } finally {
    yamlLoading.value = false
  }
}

onMounted(loadResources)
watch(() => props.path, loadResources)
</script>

<template>
  <section>
    <header class="row">
      <h2>Browse <code>{{ path }}</code></h2>
      <div class="tools">
        <button type="button" data-testid="bind-api" @click="openBind">Bind API</button>
        <router-link :to="{ name: 'workspaces' }">← Workspaces</router-link>
      </div>
    </header>

    <p v-if="error" class="error">{{ error }}</p>

    <div class="cols">
      <aside>
        <h3>Resources</h3>
        <p v-if="loadingResources">Loading…</p>
        <ul v-else>
          <li v-for="r in resources" :key="r.groupVersion + '/' + r.name">
            <button
              type="button"
              data-testid="resource-item"
              :data-name="r.name"
              :class="{ active: selected?.name === r.name && selected?.groupVersion === r.groupVersion }"
              @click="pick(r)"
            >
              {{ r.name }}
              <small>{{ r.groupVersion }}</small>
            </button>
          </li>
        </ul>
      </aside>

      <main>
        <h3 v-if="selected">{{ selected.kind }} <small>{{ selected.groupVersion }}</small></h3>
        <p v-if="!selected">Select a resource to list its objects.</p>
        <p v-else-if="loadingObjects">Loading…</p>
        <p v-else-if="objects.length === 0">No objects.</p>
        <table v-else>
          <thead>
            <tr>
              <th>Name</th>
              <th v-if="selected.namespaced">Namespace</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(o, i) in objects"
              :key="(o.metadata?.name ?? '') + i"
              class="clickable"
              data-testid="object-row"
              @click="showYaml(o)"
            >
              <td>{{ o.metadata?.name }}</td>
              <td v-if="selected.namespaced">{{ o.metadata?.namespace }}</td>
              <td>{{ o.metadata?.creationTimestamp }}</td>
            </tr>
          </tbody>
        </table>
      </main>
    </div>

    <!-- Bind panel -->
    <div v-if="showBind" class="overlay" @click.self="showBind = false">
      <div class="panel" data-testid="bind-panel">
        <header class="row">
          <h3>Bind an API into <code>{{ path }}</code></h3>
          <button type="button" @click="showBind = false">Close</button>
        </header>

        <p v-if="canBind === false" class="warn">
          You do not have permission to create APIBindings here.
        </p>
        <p v-if="bindMsg" class="ok" data-testid="bind-msg">{{ bindMsg }}</p>
        <p v-if="bindError" class="error" data-testid="bind-error">{{ bindError }}</p>

        <p v-if="exportsLoading">Loading exports…</p>
        <p v-else-if="bindable.length === 0">No bindable APIExports available.</p>
        <table v-else>
          <thead>
            <tr><th>Export</th><th>Path</th><th>Resources</th><th></th></tr>
          </thead>
          <tbody>
            <tr
              v-for="exp in bindable"
              :key="exp.path + '/' + exp.exportName"
              data-testid="bind-row"
              :data-export="exp.exportName"
            >
              <td>{{ exp.exportName }}</td>
              <td><code>{{ exp.path }}</code></td>
              <td>
                <span v-for="r in exp.resources" :key="r.group + '/' + r.resource" class="chip">
                  {{ r.resource }}<template v-if="r.group">.{{ r.group }}</template>
                </span>
              </td>
              <td>
                <button
                  v-if="boundName(exp)"
                  type="button"
                  class="unbind"
                  data-testid="unbind-btn"
                  :disabled="canBind === false || binding === exp.exportName"
                  @click="unbind(exp)"
                >
                  {{ binding === exp.exportName ? 'Unbinding…' : 'Unbind' }}
                </button>
                <button
                  v-else
                  type="button"
                  data-testid="bind-btn"
                  :disabled="canBind === false || binding === exp.exportName"
                  @click="bind(exp)"
                >
                  {{ binding === exp.exportName ? 'Binding…' : 'Bind' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- YAML viewer -->
    <div v-if="yaml" class="overlay" @click.self="yaml = null">
      <div class="panel" data-testid="yaml-panel">
        <header class="row">
          <h3><code>{{ yaml.title }}</code></h3>
          <button type="button" @click="yaml = null">Close</button>
        </header>
        <p v-if="yamlLoading">Loading…</p>
        <pre v-else class="yaml" data-testid="yaml">{{ yaml.text }}</pre>
      </div>
    </div>
  </section>
</template>

<style scoped>
.row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.tools {
  display: flex;
  gap: 1rem;
  align-items: center;
}
.cols {
  display: grid;
  grid-template-columns: 16rem 1fr;
  gap: 1rem;
}
aside ul {
  list-style: none;
  padding: 0;
}
aside button {
  width: 100%;
  text-align: left;
  display: flex;
  flex-direction: column;
}
aside button.active {
  font-weight: 600;
}
.error {
  color: #b00020;
}
.warn {
  color: #8a6d3b;
}
.ok {
  color: #1b7a34;
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
  vertical-align: top;
}
tr.clickable {
  cursor: pointer;
}
tr.clickable:hover {
  background: #f6f8ff;
}
small {
  color: #666;
}
.chip {
  display: inline-block;
  background: #eef;
  border-radius: 0.5rem;
  padding: 0.1rem 0.5rem;
  margin: 0 0.2rem 0.2rem 0;
  font-size: 0.85em;
}
.unbind {
  border-color: #e0b4b4;
  background: #fdf0f0;
}
.unbind:hover:not(:disabled) {
  background: #fbe3e3;
}
.overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 3rem 1rem;
  overflow: auto;
}
.panel {
  background: #fff;
  border-radius: 0.5rem;
  padding: 1rem 1.25rem;
  width: min(60rem, 100%);
  max-height: 80vh;
  overflow: auto;
}
.yaml {
  background: #0f1117;
  color: #d6deeb;
  padding: 1rem;
  border-radius: 0.375rem;
  overflow: auto;
  font-size: 0.85em;
  line-height: 1.4;
}
</style>
