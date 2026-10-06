<script setup lang="ts">
import { computed } from 'vue'
import { checkPassword } from '@/utils/password'

const props = defineProps<{ password: string; username?: string }>()

const result = computed(() => checkPassword(props.password, props.username || ''))

const levelMeta = computed(() => {
  const l = result.value.level
  return l === 'strong'
    ? { label: '强', color: '#52c41a' }
    : l === 'medium'
      ? { label: '中', color: '#faad14' }
      : { label: '弱', color: '#ea6668' }
})

const barWidth = computed(() => `${result.value.score}%`)
</script>

<template>
  <div class="pwd-strength">
    <div class="bar"><div class="fill" :style="{ width: barWidth, background: levelMeta.color }" /></div>
    <div class="meta">
      <span :style="{ color: levelMeta.color, fontWeight: 600 }">{{ levelMeta.label }}（{{ result.score }} 分）</span>
      <span v-if="!result.ok" class="errors">{{ result.errors.join('；') }}</span>
      <span v-else class="ok">符合等保三级密码策略</span>
    </div>
  </div>
</template>

<style scoped>
.pwd-strength { margin-top: 6px; }
.bar { height: 6px; border-radius: 3px; background: rgba(0,0,0,0.08); overflow: hidden; }
.fill { height: 100%; transition: width .3s; }
.meta { display: flex; align-items: center; gap: 8px; margin-top: 4px; font-size: 12px; flex-wrap: wrap; }
.errors { color: #ea6668; }
.ok { color: #52c41a; }
</style>
