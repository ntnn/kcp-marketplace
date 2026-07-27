import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The SPA talks only to the front-proxy origin. In dev, VITE_API_PROXY lets Vite
// proxy /services and /clusters to a running front-proxy to sidestep CORS.
export default defineConfig(() => {
  const target = process.env.VITE_API_PROXY
  return {
    plugins: [vue()],
    resolve: {
      alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
    },
    server: target
      ? {
          proxy: {
            '/services': { target, changeOrigin: true, secure: false },
            '/clusters': { target, changeOrigin: true, secure: false },
            '/apis': { target, changeOrigin: true, secure: false },
          },
        }
      : undefined,
  }
})
