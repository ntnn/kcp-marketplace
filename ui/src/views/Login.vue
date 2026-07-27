<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getConfig } from '@/config'

const auth = useAuthStore()
const route = useRoute()
const issuer = getConfig().issuer
const certIssue = ref(false)
const error = ref<string | null>(null)

async function signIn() {
  error.value = null
  certIssue.value = false
  try {
    const redirect = (route.query.redirect as string) || '/workspaces'
    await auth.login(redirect)
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    // A failed fetch to the issuer is almost always the untrusted dev TLS cert:
    // the browser hard-fails cross-origin fetch() to a self-signed endpoint.
    if (/failed to fetch|networkerror|load failed/i.test(msg)) {
      certIssue.value = true
    } else {
      error.value = msg
    }
  }
}
</script>

<template>
  <section class="login">
    <h1>kcp marketplace</h1>
    <p>Sign in to browse the workspaces you can access and bind APIExports.</p>
    <button type="button" data-testid="signin" @click="signIn">Sign in with Dex</button>

    <p v-if="error" class="error">{{ error }}</p>

    <div v-if="certIssue" class="cert" data-testid="cert-hint">
      <p class="error">Could not reach the identity provider.</p>
      <p>
        In local dev the OIDC provider uses a self-signed certificate. Open
        <a :href="issuer" target="_blank" rel="noopener">{{ issuer }}</a>
        once, accept the certificate warning, then come back and sign in again.
      </p>
    </div>
  </section>
</template>

<style scoped>
.login {
  max-width: 28rem;
  margin: 6rem auto;
  text-align: center;
}
.error {
  color: #b00020;
}
.cert {
  margin-top: 1rem;
  text-align: left;
  background: #fff8e1;
  border: 1px solid #ffe082;
  border-radius: 0.5rem;
  padding: 0.75rem 1rem;
}
</style>
