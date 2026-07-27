import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import { loadConfig } from './config'
import { setTokenProvider } from './api/client'
import { useAuthStore } from './stores/auth'
import './style.css'

async function bootstrap(): Promise<void> {
  await loadConfig()

  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  // Wire the API bearer token to the auth store, then restore any session.
  const auth = useAuthStore(pinia)
  setTokenProvider(() => auth.token)
  await auth.restore()

  app.use(router)
  app.mount('#app')
}

bootstrap().catch((e) => {
  document.getElementById('app')!.innerHTML =
    `<pre style="color:#b00020;padding:2rem">Failed to start: ${String(e)}</pre>`
})
