<script setup lang="ts">
/**
 * M5 告警事件中心：查看/筛选/确认处置（等保三级：操作留痕）
 */
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { audit } from '@/utils/audit'
import { listAlertEvents, ackAlertEvent, getAlertStats } from '@/api/alerts'
import type { AlertEvent, AlertStats, PageResult } from '@/types'

const loading = ref(false)
const result = ref<PageResult<AlertEvent>>({ total: 0, page: 1, size: 20, items: [] })
const stats = ref<AlertStats>({ open: 0, today: 0, total: 0 })
const filters = ref({ status: '', page: 1, size: 20 })

const sevType = (s: string): string => {
  const map: Record<string, string> = { critical: 'danger', high: 'danger', medium: 'warning', low: 'info', warning: 'warning' }
  return map[s] || 'info'
}

async function doLoad(): Promise<void> {
  loading.value = true
  try {
    const resp = await listAlertEvents({ ...filters.value })
    result.value = resp.data
    const st = await getAlertStats()
    stats.value = st.data
  } catch {
    ElMessage.error('告警事件加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(doLoad)

function onFilter(): void {
  filters.value.page = 1
  void doLoad()
}

async function onAck(row: AlertEvent): Promise<void> {
  try {
    await ackAlertEvent(row.id)
    audit('ack_alert', `#${row.id}`, '确认告警事件')
    ElMessage.success(`已确认告警 #${row.id}`)
    void doLoad()
  } catch {
    ElMessage.error('确认失败')
  }
}
</script>

<template>
  <div class="panel">
    <div class="panel-head">
      <div>
        <h2>告警事件</h2>
        <p class="sub">引擎触发的告警记录 · 未处理 {{ stats.open }} · 今日 {{ stats.today }} · 累计 {{ stats.total }}</p>
      </div>
      <el-button @click="doLoad">刷新</el-button>
    </div>

    <el-form inline size="small" class="filter-bar">
      <el-form-item label="状态">
        <el-select v-model="filters.status" clearable placeholder="全部" style="width: 130px" @change="onFilter">
          <el-option label="未处理 open" value="open" />
          <el-option label="已确认 acked" value="acked" />
        </el-select>
      </el-form-item>
    </el-form>

    <el-table v-loading="loading" :data="result.items" stripe size="small">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="级别" width="90">
        <template #default="{ row }">
          <el-tag size="small" :type="sevType(row.severity)">{{ row.severity }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="rule_name" label="规则" min-width="140" />
      <el-table-column label="匹配维度" width="130">
        <template #default="{ row }">{{ row.match_key || '全部' }}</template>
      </el-table-column>
      <el-table-column prop="match_count" label="命中数" width="80" />
      <el-table-column prop="message" label="内容" min-width="260" show-overflow-tooltip />
      <el-table-column prop="created_at" label="时间" width="160" />
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag size="small" :type="row.status === 'open' ? 'danger' : 'success'">
            {{ row.status === 'open' ? '未处理' : '已确认' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.status === 'open'" link type="primary" size="small" @click="onAck(row)">确认处置</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="filters.page"
      v-model:page-size="filters.size"
      :total="result.total"
      layout="total, prev, pager, next, sizes"
      small
      style="margin-top: 12px; justify-content: flex-end"
      @current-change="doLoad"
      @size-change="onFilter"
    />
  </div>
</template>

<style scoped>
.panel { padding: 16px; }
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 12px; }
.panel-head h2 { margin: 0; font-size: 18px; }
.sub { margin: 4px 0 0; color: #6B7280; font-size: 12px; }
.filter-bar { margin-bottom: 8px; }
</style>
