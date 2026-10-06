<script setup lang="ts">
import { onMounted, ref } from 'vue'
import * as echarts from 'echarts'
import { getStatsOverview } from '@/api/logs'
import { audit } from '@/utils/audit'

const loading = ref(true)
const cards = ref([
  { label: '今日日志量', value: '-' },
  { label: '近 7 天日志量', value: '-' },
  { label: '设备接入数', value: '-' },
  { label: '留存天数', value: '180' }
])

onMounted(async () => {
  try {
    const resp = await getStatsOverview()
    const d = resp.data
    cards.value = [
      { label: '今日日志量', value: String(d.today_total ?? '-') },
      { label: '近 7 天日志量', value: String((d.last_7d || []).reduce((a, b) => a + (b.count || 0), 0)) },
      { label: '设备接入数', value: String((d.category_dist || []).length) || '-' },
      { label: '留存天数', value: '180' }
    ]
    renderTrend(d.last_7d || [])
  } catch {
    loading.value = false
  }
})

function renderTrend(data: Array<{ day: string; count: number }>): void {
  const el = document.getElementById('trend-chart')
  if (!el) return
  const chart = echarts.init(el)
  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    grid: { left: 48, right: 16, top: 20, bottom: 28 },
    xAxis: { type: 'category', data: data.map((d) => d.day), axisLabel: { fontSize: 11, color: '#555' } },
    yAxis: { type: 'value', name: '条', axisLabel: { fontSize: 11, color: '#555' } },
    series: [{
      type: 'line', smooth: true, data: data.map((d) => d.count),
      lineStyle: { color: '#1d7872', width: 2 },
      itemStyle: { color: '#1d7872' },
      areaStyle: { color: 'rgba(29,120,114,0.12)' }
    }]
  })
  window.addEventListener('resize', () => chart.resize())
}
</script>

<template>
  <div>
    <div class="cards">
      <div v-for="c in cards" :key="c.label" class="card">
        <div class="label">{{ c.label }}</div>
        <div class="value">{{ c.value }}</div>
      </div>
    </div>
    <div class="panel">
      <div class="panel-title">近 7 天日志量趋势</div>
      <div id="trend-chart" style="width:100%;height:280px" />
    </div>
  </div>
</template>

<style scoped>
.cards { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 12px; }
.card { flex: 1 1 150px; min-width: 0; background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); }
.label { font-size: 12px; color: #6b7280; }
.value { font-size: 22px; font-weight: 700; color: #1a1b1c; margin-top: 4px; }
.panel { background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); }
.panel-title { font-size: 13px; font-weight: 600; color: #1a1b1c; margin-bottom: 8px; }
.tip { margin-top: 12px; font-size: 12px; color: #9aa0a6; }
</style>
