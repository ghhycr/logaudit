<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { searchLogs } from '@/api/logs'
import { audit } from '@/utils/audit'
import { escapeHtml, isValidIPv4, clampKeyword } from '@/utils/security'
import { ElMessage } from 'element-plus'
import type { AuditLogItem, PageResult } from '@/types'

const filters = reactive({
  from: '',
  to: '',
  host: '',
  sourceIp: '',
  eventType: '',
  keyword: '',
  page: 1,
  size: 20
})

const loading = ref(false)
const result = ref<PageResult<AuditLogItem>>({ total: 0, page: 1, size: 20, items: [] })
const detail = ref<AuditLogItem | null>(null)
// v-model 需为可写成员表达式（el-drawer 可见性由 detail 驱动）
const drawerVisible = computed({
  get: () => detail.value != null,
  set: (v: boolean) => {
    if (!v) detail.value = null
  }
})

async function doSearch(): Promise<void> {
  loading.value = true
  audit('search_logs', 'audit_logs', `keyword=${clampKeyword(filters.keyword)}`)
  try {
    const resp = await searchLogs({
      from: filters.from || undefined,
      to: filters.to || undefined,
      host: filters.host || undefined,
      source_ip: filters.sourceIp || undefined,
      event_type: filters.eventType || undefined,
      q: clampKeyword(filters.keyword),
      page: filters.page,
      size: filters.size
    })
    result.value = resp.data
  } catch {
    ElMessage.error('检索失败，请确认后端服务可用')
  } finally {
    loading.value = false
  }
}

function onPageChange(p: number): void {
  filters.page = p
  void doSearch()
}

function onView(row: AuditLogItem): void {
  detail.value = row
  audit('view_log_detail', row.host, row.event_type)
}

const severities = ['0', '1', '2', '3', '4', '5', '6', '7']
</script>

<template>
  <div class="panel">
    <el-form inline :model="filters" @submit.prevent="doSearch">
      <el-form-item label="时间从">
        <el-date-picker v-model="filters.from" type="datetime" placeholder="开始时间" style="width:190px" />
      </el-form-item>
      <el-form-item label="至">
        <el-date-picker v-model="filters.to" type="datetime" placeholder="结束时间" style="width:190px" />
      </el-form-item>
      <el-form-item label="设备">
        <el-input v-model="filters.host" placeholder="主机名" style="width:130px" />
      </el-form-item>
      <el-form-item label="源IP">
        <el-input v-model="filters.sourceIp" placeholder="如 10.1.1.2" style="width:140px" />
      </el-form-item>
      <el-form-item label="事件类型">
        <el-select v-model="filters.eventType" clearable placeholder="全部" style="width:150px">
          <el-option label="SSH暴力破解" value="ssh_bruteforce" />
          <el-option label="登录失败" value="login_failed" />
          <el-option label="配置变更" value="config_change" />
          <el-option label="攻击事件" value="attack" />
        </el-select>
      </el-form-item>
      <el-form-item label="关键词">
        <el-input v-model="filters.keyword" placeholder="全文关键词" style="width:160px" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="loading" v-audit="'search_logs'" @click="doSearch">检索</el-button>
      </el-form-item>
    </el-form>

    <el-table v-loading="loading" :data="result.items" stripe size="small" @row-click="onView">
      <el-table-column prop="ts" label="时间" width="170" />
      <el-table-column prop="host" label="设备" width="120" />
      <el-table-column prop="program" label="程序" width="100" />
      <el-table-column label="级别" width="70">
        <template #default="{ row }">
          <el-tag size="small" :type="row.severity <= 3 ? 'danger' : 'info'">{{ severities[row.severity] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="source_ip" label="源IP" width="120" />
      <el-table-column prop="user_name" label="账号" width="100" />
      <el-table-column prop="event_type" label="事件类型" width="120" />
      <el-table-column prop="message" label="消息" min-width="220" show-overflow-tooltip />
    </el-table>

    <div class="pager">
      <span class="total">共 {{ result.total }} 条</span>
      <el-pagination
        layout="prev, pager, next, sizes"
        :total="result.total"
        :page-size="filters.size"
        :current-page="filters.page"
        :page-sizes="[20, 50, 100]"
        @current-change="onPageChange"
        @size-change="(s: number) => { filters.size = s; filters.page = 1; void doSearch() }"
      />
    </div>

    <el-drawer v-model="drawerVisible" title="日志详情" size="480px">
      <template v-if="detail">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="时间">{{ detail.ts }}</el-descriptions-item>
          <el-descriptions-item label="设备">{{ detail.host }}</el-descriptions-item>
          <el-descriptions-item label="程序">{{ detail.program }}</el-descriptions-item>
          <el-descriptions-item label="源IP">{{ detail.source_ip }}</el-descriptions-item>
          <el-descriptions-item label="账号">{{ detail.user_name }}</el-descriptions-item>
          <el-descriptions-item label="事件类型">{{ detail.event_type }}</el-descriptions-item>
          <el-descriptions-item label="结果">{{ detail.outcome }}</el-descriptions-item>
        </el-descriptions>
        <!-- XSS 防护：消息经白名单清洗后展示（v-sanitize） -->
        <div class="raw" v-sanitize="escapeHtml(detail.message)" />
      </template>
    </el-drawer>
  </div>
</template>

<style scoped>
.panel { background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); }
.pager { display: flex; align-items: center; justify-content: space-between; margin-top: 12px; flex-wrap: wrap; gap: 8px; }
.total { font-size: 12px; color: #6b7280; }
.raw { margin-top: 12px; background: #f6f7f5; border-radius: 8px; padding: 10px 12px; font-size: 12.5px; font-family: Consolas, Menlo, monospace; white-space: pre-wrap; word-break: break-all; }
</style>
