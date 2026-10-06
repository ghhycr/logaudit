<script setup lang="ts">
/**
 * 留存与容量：TTL 策略、各日期分区真实占用（ClickHouse system.parts）、过期倒计时
 * 数据源：GET /retention（后端增强：分区行数/磁盘/剩余天数）
 */
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { audit } from '@/utils/audit'
import { getRetentionStatus } from '@/api/devices'
import type { RetentionInfo } from '@/types'

const loading = ref(false)
const info = ref<RetentionInfo | null>(null)

function fmtBytes(b: number): string {
  if (!b) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = b
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 ? 0 : 1)} ${units[i]}`
}

function fmtRows(n: number): string {
  return Number(n || 0).toLocaleString()
}

function daysLeftTag(d: number): string {
  if (d < 0) return 'danger'
  if (d <= 30) return 'warning'
  return 'success'
}

function daysLeftText(d: number): string {
  if (d < 0) return `已过期 ${-d} 天`
  if (d === 0) return '今日到期'
  return `${d} 天`
}

const summary = ref([
  { label: 'TTL 留存天数', value: '-' },
  { label: '当前总占用', value: '-' },
  { label: '日期分区数', value: '-' },
  { label: '压缩 / 存储引擎', value: '-' }
])

async function doLoad(): Promise<void> {
  loading.value = true
  try {
    const resp = await getRetentionStatus()
    info.value = resp.data
    summary.value = [
      { label: 'TTL 留存天数', value: `${resp.data.ttl_days} 天` },
      { label: '当前总占用', value: fmtBytes(resp.data.total_bytes) },
      { label: '日期分区数', value: String((resp.data.partitions || []).length) },
      { label: '压缩 / 存储引擎', value: `${resp.data.compression} / ${resp.data.storage_engine}` }
    ]
    audit('view_retention', '', '查看留存与容量')
  } catch {
    ElMessage.error('留存状态加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(doLoad)
</script>

<template>
  <div v-loading="loading">
    <div class="panel-head">
      <div>
        <h2>留存与容量</h2>
        <p class="sub">ClickHouse TTL 自动清理（{{ info?.partition_field }} 分区 · 等保要求留存 ≥ 6 个月）</p>
      </div>
      <el-button size="small" @click="doLoad">刷新</el-button>
    </div>

    <div class="cards">
      <div v-for="c in summary" :key="c.label" class="card">
        <div class="label">{{ c.label }}</div>
        <div class="value">{{ c.value }}</div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">各日期分区占用与到期倒计时（TTL {{ info?.ttl_days }} 天）</div>
      <el-table :data="info?.partitions || []" stripe size="small">
        <el-table-column prop="date" label="日志日期" width="120" />
        <el-table-column prop="key" label="分区键" width="110" />
        <el-table-column prop="rows" label="行数" width="120" align="right">
          <template #default="{ row }">{{ fmtRows(row.rows) }}</template>
        </el-table-column>
        <el-table-column label="占用" width="110" align="right">
          <template #default="{ row }">{{ fmtBytes(row.bytes) }}</template>
        </el-table-column>
        <el-table-column label="到期清理" width="160">
          <template #default="{ row }">
            <el-tag size="small" :type="daysLeftTag(row.days_left)">{{ daysLeftText(row.days_left) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="days_left" label="剩余天数" width="100" align="right" />
      </el-table>
      <div v-if="!info?.partitions?.length" class="empty">暂无分区数据（采集器写入后自动生成）</div>
      <div class="tips">
        <div>· TTL 规则：<code>ts &lt; now() - INTERVAL {{ info?.ttl_days || 180 }} DAY</code> 由 ClickHouse 后台任务自动删除，无需人工干预</div>
        <div>· 索引：{{ info?.indexes?.join('、') }}</div>
        <div>· 元数据存储：{{ info?.meta_backend }}（设备台账 / 用户 / 操作审计）</div>
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
.value { font-size: 20px; font-weight: 700; color: #1a1b1c; margin-top: 4px; }
.panel { background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); }
.panel-title { font-size: 13px; font-weight: 600; color: #1a1b1c; margin-bottom: 8px; }
.empty { padding: 24px 0; text-align: center; color: #9aa0a6; font-size: 12px; }
.tips { margin-top: 12px; padding: 10px 12px; background: #f8f9fb; border-radius: 8px; font-size: 12px; color: #6b7280; line-height: 1.9; }
.tips code { background: #eef1f6; padding: 1px 6px; border-radius: 4px; font-size: 11.5px; }
</style>
