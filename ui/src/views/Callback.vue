<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    const target = await auth.completeLogin()
    await router.replace(target || '/workspaces')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
})
</script>

<template>
  <section class="callback">
    <p v-if="!error">Signing in…</p>
    <div v-else>
      <p class="error">Sign-in failed: {{ error }}</p>
      <router-link to="/login">Try again</router-link>
    </div>
  </section>
</template>

<style scoped>
.callback {
  margin: 6rem auto;
  text-align: center;
}
.error {
  color: #b00020;
}
</style>
