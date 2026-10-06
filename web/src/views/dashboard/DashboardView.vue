<script setup lang="ts">
/**
 * 总览仪表盘：核心指标 + 宿主机硬件信息（CPU/内存/磁盘）+ 统计分析（近 7 天）
 * 数据源：GET /stats/overview（日志聚合）+ GET /stats/login-fail（登录失败）+ GET /host/overview（主机监控）
 */
import { onBeforeUnmount, onMounted, ref } from 'vue'
import * as echarts from 'echarts'
import { ElMessage } from 'element-plus'
import { audit } from '@/utils/audit'
import { getStatsOverview, getLoginFailStats } from '@/api/logs'
import { getHostOverview, type HostOverview } from '@/api/host'
import type { LoginFailItem, StatsOverview } from '@/types'

const loading = ref(true)
const cards = ref([
  { label: '今日日志量', value: '-' },
  { label: '近 7 天日志量', value: '-' },
  { label: '设备接入数', value: '-' },
  { label: '留存天数', value: '180' }
])
const statCards = ref([
  { label: '近 7 天登录失败', value: '-' },
  { label: '近 7 天日志总量', value: '-' },
  { label: 'TOP 事件类型', value: '-' },
  { label: 'TOP 源 IP 数', value: '-' }
])
const host = ref<HostOverview | null>(null)
const topEvents = ref<Array<{ event_type: string; count: number }>>([])

const charts: Array<{ id: string; inst: echarts.ECharts | null }> = [
  { id: 'trend-chart', inst: null },
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

function disposeAll(): void {
  charts.forEach((c) => c.inst?.dispose())
}

function fmtUptime(sec: number): string {
  if (!sec) return '-'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  return `${d} 天 ${h} 小时 ${m} 分`
}

function usageColor(pct: number): string {
  if (pct >= 85) return '#EA6668'
  if (pct >= 70) return '#FAAD14'
  return '#1d7872'
}

function renderTrend(data: Array<{ day: string; count: number }>): void {
  const c = charts.find((x) => x.id === 'trend-chart')
  if (!c) return
  const el = document.getElementById(c.id)
  if (!el) return
  c.inst = echarts.init(el)
  c.inst.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', confine: true },
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
  const top = (srcs || []).slice(0, 6).reverse()
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

async function doLoad(): Promise<void> {
  loading.value = true
  try {
    const [ovResp, failResp, hostResp] = await Promise.all([
      getStatsOverview(), getLoginFailStats(), getHostOverview()
    ])
    const ov: StatsOverview = ovResp.data || ({} as StatsOverview)
    const fails = failResp.data || []

    cards.value = [
      { label: '今日日志量', value: String(ov.today_total ?? '-') },
      { label: '近 7 天日志量', value: fmt((ov.last_7d || []).reduce((a, b) => a + (b.count || 0), 0)) },
      { label: '设备接入数', value: String((ov.category_dist || []).length) || '-' },
      { label: '留存天数', value: '180' }
    ]
    statCards.value = [
      { label: '近 7 天登录失败', value: fmt(fails.reduce((a: number, b: LoginFailItem) => a + (b.count || 0), 0)) },
      { label: '近 7 天日志总量', value: fmt((ov.last_7d || []).reduce((a, b) => a + (b.count || 0), 0)) },
      { label: 'TOP 事件类型', value: String((ov.top_events || []).length) },
      { label: 'TOP 源 IP 数', value: String((ov.top_sources || []).length) }
    ]
    host.value = hostResp.data || null
    topEvents.value = ov.top_events || []

    renderTrend(ov.last_7d || [])
    renderFailTrend(fails)
    renderCategoryPie(ov.category_dist || [])
    renderTopSources(ov.top_sources || [])
    audit('view_dashboard', '', '查看总览仪表盘')
  } catch {
    ElMessage.error('仪表盘数据加载失败')
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
  disposeAll()
})
</script>

<template>
  <div v-loading="loading" class="page">
    <!-- 核心指标 -->
    <div class="cards">
      <div v-for="c in cards" :key="c.label" class="card">
        <div class="label">{{ c.label }}</div>
        <div class="value">{{ c.value }}</div>
      </div>
    </div>

    <!-- 主机硬件信息 -->
    <div class="cards">
      <div class="card host-card">
        <div class="host-head">
          <div class="label">CPU 使用率</div>
          <div class="host-name">{{ host?.hostname || '宿主机' }} · {{ host?.cpu.cores || '-' }} 核</div>
        </div>
        <div class="host-value" :style="{ color: usageColor(host?.cpu.usage_percent ?? 0) }">
          {{ host ? host.cpu.usage_percent.toFixed(1) + '%' : '-' }}
        </div>
        <el-progress :percentage="Math.round(host?.cpu.usage_percent ?? 0)" :show-text="false"
          :color="usageColor(host?.cpu.usage_percent ?? 0)" :stroke-width="8" />
      </div>
      <div class="card host-card">
        <div class="host-head">
          <div class="label">内存使用情况</div>
          <div class="host-name">{{ host ? host.memory.used_gb.toFixed(1) + ' / ' + host.memory.total_gb.toFixed(1) + ' GB' : '' }}</div>
        </div>
        <div class="host-value" :style="{ color: usageColor(host?.memory.usage_percent ?? 0) }">
          {{ host ? host.memory.usage_percent.toFixed(1) + '%' : '-' }}
        </div>
        <el-progress :percentage="Math.round(host?.memory.usage_percent ?? 0)" :show-text="false"
          :color="usageColor(host?.memory.usage_percent ?? 0)" :stroke-width="8" />
      </div>
      <div class="card host-card">
        <div class="host-head">
          <div class="label">磁盘使用情况</div>
          <div class="host-name">{{ host ? host.disk.used_gb.toFixed(1) + ' / ' + host.disk.total_gb.toFixed(1) + ' GB' : '' }}</div>
        </div>
        <div class="host-value" :style="{ color: usageColor(host?.disk.usage_percent ?? 0) }">
          {{ host ? host.disk.usage_percent.toFixed(1) + '%' : '-' }}
        </div>
        <el-progress :percentage="Math.round(host?.disk.usage_percent ?? 0)" :show-text="false"
          :color="usageColor(host?.disk.usage_percent ?? 0)" :stroke-width="8" />
      </div>
      <div class="card host-card">
        <div class="host-head">
          <div class="label">运行时间 / 采集时间</div>
        </div>
        <div class="host-value" style="font-size:16px">{{ host ? fmtUptime(host.uptime_seconds) : '-' }}</div>
        <div class="host-sub">{{ host?.collected_at || '' }}</div>
      </div>
    </div>

    <!-- 统计分析 -->
    <div class="cards">
      <div v-for="c in statCards" :key="c.label" class="card">
        <div class="label">{{ c.label }}</div>
        <div class="value">{{ c.value }}</div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">近 7 天日志量趋势</div>
      <div id="trend-chart" style="width:100%;height:260px" />
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
.page { min-height: 200px; }
.cards { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 12px; }
.card { flex: 1 1 150px; min-width: 0; background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); box-sizing: border-box; }
.label { font-size: 12px; color: #6b7280; }
.value { font-size: 22px; font-weight: 700; color: #1a1b1c; margin-top: 4px; }
.host-card { flex: 1 1 220px; }
.host-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.host-name { font-size: 11px; color: #9aa0a6; }
.host-value { font-size: 20px; font-weight: 700; margin: 6px 0 8px; }
.host-sub { font-size: 11px; color: #9aa0a6; margin-top: 6px; }
.panel { background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); margin-bottom: 12px; }
.panel-title { font-size: 13px; font-weight: 600; color: #1a1b1c; margin-bottom: 8px; }
.grid { display: flex; flex-wrap: wrap; gap: 12px; }
.grid > .panel { flex: 1 1 46%; min-width: 0; margin-bottom: 0; }
.chart { width: 100%; height: 260px; }
.empty { padding: 24px 0; text-align: center; color: #9aa0a6; font-size: 12px; }
</style>
