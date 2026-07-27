<script setup lang="ts">
import { onMounted } from 'vue'
import { completeSilent } from '@/auth/oidc'

// Rendered inside the hidden silent-renew iframe; hands the code back to
// oidc-client-ts, which resolves the parent window's renewal promise.
onMounted(() => {
  completeSilent().catch(() => {
    /* the parent handles renew errors via events */
  })
})
</script>

<template>
  <p>Renewing session…</p>
</template>
