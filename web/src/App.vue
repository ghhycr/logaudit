<script setup lang="ts">
import { onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { startSessionMonitor } from '@/utils/session'

const auth = useAuthStore()

onMounted(() => {
  // 会话超时监控（等保三级：会话空闲自动失效）
  if (auth.isLoggedIn) {
    startSessionMonitor()
  }
})
</script>

<template>
  <router-view />
</template>
