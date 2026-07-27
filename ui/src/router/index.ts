import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  { path: '/', name: 'home', redirect: { name: 'workspaces' } },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/Login.vue'),
    meta: { public: true },
  },
  {
    path: '/callback',
    name: 'callback',
    component: () => import('@/views/Callback.vue'),
    meta: { public: true },
  },
  {
    path: '/workspaces',
    name: 'workspaces',
    component: () => import('@/views/WorkspacePicker.vue'),
  },
  {
    path: '/workspaces/:path/browse',
    name: 'browse',
    component: () => import('@/views/WorkspaceBrowser.vue'),
    props: true,
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true
  const auth = useAuthStore()
  if (!auth.isAuthenticated) {
    await auth.restore()
  }
  if (!auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})
