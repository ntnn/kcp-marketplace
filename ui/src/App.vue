<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const { user } = storeToRefs(auth)
</script>

<template>
  <div class="app">
    <nav class="topbar">
      <router-link class="brand" :to="{ name: 'workspaces' }">kcp marketplace</router-link>
      <div class="spacer" />
      <template v-if="user">
        <span class="user">{{ auth.username }}</span>
        <button type="button" @click="auth.logout()">Sign out</button>
      </template>
    </nav>
    <main class="content">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.75rem 1.5rem;
  border-bottom: 1px solid #e5e5e5;
}
.brand {
  font-weight: 600;
  text-decoration: none;
  color: inherit;
}
.spacer {
  flex: 1;
}
.user {
  color: #555;
}
.content {
  max-width: 60rem;
  margin: 1.5rem auto;
  padding: 0 1.5rem;
}
</style>
