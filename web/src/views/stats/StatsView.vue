<script setup lang="ts">
/**
 * 统计分析：登录失败趋势 / 事件类别分布 / TOP 源 IP / TOP 事件类型
 * 数据源：GET /stats/login-fail（MySQL 操作审计）+ GET /stats/overview（ClickHouse 近 7 天聚合）
 */
import { onMounted, onBeforeUnmount, ref } from 'vue'
import * as echarts from 'echarts'
import { ElMessage } from 'element-plus'
import { audit } from '@/utils/audit'
import { getLoginFailStats, getStatsOverview } from '@/api/logs'
import type { LoginFailItem, StatsOverview } from '@/types'

const loading = ref(false)
const loginFailTotal = ref(0)
const logTotal7d = ref(0)
const cards = ref([
  { label: '近 7 天登录失败', value: '-' },
  { label: '近 7 天日志总量', value: '-' },
  { label: 'TOP 事件类型', value: '-' },
  { label: 'TOP 源 IP 数', value: '-' }
])

const charts: Array<{ id: string; inst: echarts.ECharts | null }> = [
  { id: 'fail-trend', inst: null },
  { id: 'category-pie', inst: null },
  { id: 'top-sources', inst: null }
]

function fmt(v: number): string {
  return Number(v).toLocaleString()
}

function resizeAll(): void {
  charts.forEach((c) => c.inst?.resize())
}

function renderFailTrend(items: LoginFailItem[]): void {
  const c = charts.find((x) => x.id === 'fail-trend')
  if (!c) return
  const el = document.getElementById(c.id)
  if (!el) return
  c.inst = echarts.init(el)
  c.inst.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', confine: true },
    grid: { left: 44, right: 16, top: 24, bottom: 30 },
    xAxis: { type: 'category', data: items.map((i) => i.day.slice(5)), axisLabel: { fontSize: 11, color: '#555' } },
    yAxis: { type: 'value', minInterval: 1, axisLabel: { fontSize: 11, color: '#555' } },
    series: [{
      name: '登录失败', type: 'bar', data: items.map((i) => i.count),
      itemStyle: { color: '#EA6668', borderRadius: [4, 4, 0, 0] },
      label: { show: true, position: 'top', fontSize: 10, color: '#555' }
    }]
  })
}

function renderCategoryPie(cats: Array<{ category: string; count: number }>): void {
  const c = charts.find((x) => x.id === 'category-pie')
  if (!c) return
  const el = document.getElementById(c.id)
  if (!el) return
  c.inst = echarts.init(el)
  c.inst.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', confine: true, formatter: '{b}: {c} ({d}%)' },
    legend: { bottom: 0, textStyle: { fontSize: 11 }, itemWidth: 12, itemHeight: 8 },
    series: [{
      type: 'pie', radius: ['38%', '62%'], center: ['50%', '44%'],
      data: cats.map((x) => ({ name: x.category || '(未分类)', value: x.count })),
      label: { fontSize: 10, formatter: '{b}\n{d}%' },
      itemStyle: { borderColor: '#fff', borderWidth: 1 }
    }]
  })
}

function renderTopSources(srcs: Array<{ source_ip: string; count: number }>): void {
  const c = charts.find((x) => x.id === 'top-sources')
  if (!c) return
  const el = document.getElementById(c.id)
  if (!el) return
  const top = srcs.slice(0, 6).reverse()
  c.inst = echarts.init(el)
  c.inst.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', confine: true },
    grid: { left: 80, right: 34, top: 12, bottom: 26 },
    xAxis: { type: 'value', axisLabel: { fontSize: 11, color: '#555' } },
    yAxis: { type: 'category', data: top.map((s) => s.source_ip), axisLabel: { fontSize: 11, color: '#555' } },
    series: [{
      type: 'bar', data: top.map((s) => s.count),
      itemStyle: { color: '#1d7872', borderRadius: [0, 4, 4, 0] },
      label: { show: true, position: 'right', fontSize: 10, color: '#555' }
    }]
  })
}

const topEvents = ref<Array<{ event_type: string; count: number }>>([])

async function doLoad(): Promise<void> {
  loading.value = true
  try {
    const [failResp, ovResp] = await Promise.all([getLoginFailStats(), getStatsOverview()])
    const fails = failResp.data || []
    const ov: StatsOverview = ovResp.data || ({} as StatsOverview)
    loginFailTotal.value = fails.reduce((a, b) => a + (b.count || 0), 0)
    logTotal7d.value = (ov.last_7d || []).reduce((a, b) => a + (b.count || 0), 0)
    cards.value = [
      { label: '近 7 天登录失败', value: fmt(loginFailTotal.value) },
      { label: '近 7 天日志总量', value: fmt(logTotal7d.value) },
      { label: 'TOP 事件类型', value: String((ov.top_events || []).length) },
      { label: 'TOP 源 IP 数', value: String((ov.top_sources || []).length) }
    ]
    renderFailTrend(fails)
    renderCategoryPie(ov.category_dist || [])
    renderTopSources(ov.top_sources || [])
    topEvents.value = ov.top_events || []
    audit('view_stats', '', '查看统计分析')
  } catch {
    ElMessage.error('统计数据加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void doLoad()
  window.addEventListener('resize', resizeAll)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeAll)
  charts.forEach((c) => c.inst?.dispose())
})
</script>

<template>
  <div v-loading="loading">
    <div class="panel-head">
      <div>
        <h2>统计分析</h2>
        <p class="sub">近 7 天 · 登录失败来自操作审计，事件/来源聚合来自 ClickHouse</p>
      </div>
      <el-button size="small" @click="doLoad">刷新</el-button>
    </div>

    <div class="cards">
      <div v-for="c in cards" :key="c.label" class="card">
        <div class="label">{{ c.label }}</div>
        <div class="value">{{ c.value }}</div>
      </div>
    </div>

    <div class="grid">
      <div class="panel chart-panel">
        <div class="panel-title">近 7 天登录失败趋势</div>
        <div id="fail-trend" class="chart" />
      </div>
      <div class="panel chart-panel">
        <div class="panel-title">事件类别分布</div>
        <div id="category-pie" class="chart" />
      </div>
      <div class="panel chart-panel">
        <div class="panel-title">TOP 源 IP</div>
        <div id="top-sources" class="chart" />
      </div>
      <div class="panel">
        <div class="panel-title">TOP 事件类型（近 7 天）</div>
        <el-table :data="topEvents" stripe size="small">
          <el-table-column type="index" label="#" width="48" />
          <el-table-column prop="event_type" label="事件类型" min-width="160" />
          <el-table-column prop="count" label="次数" width="110" align="right" />
        </el-table>
        <div v-if="!topEvents.length" class="empty">暂无数据</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 12px; }
.panel-head h2 { margin: 0; font-size: 18px; }
.sub { margin: 4px 0 0; color: #6B7280; font-size: 12px; }
.cards { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 12px; }
.card { flex: 1 1 160px; min-width: 0; background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); }
.label { font-size: 12px; color: #6b7280; }
.value { font-size: 22px; font-weight: 700; color: #1a1b1c; margin-top: 4px; }
.grid { display: flex; flex-wrap: wrap; gap: 12px; }
.grid > .panel { flex: 1 1 46%; min-width: 0; }
.panel { background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); box-sizing: border-box; }
.panel-title { font-size: 13px; font-weight: 600; color: #1a1b1c; margin-bottom: 8px; }
.chart { width: 100%; height: 260px; }
.empty { padding: 24px 0; text-align: center; color: #9aa0a6; font-size: 12px; }
</style>
