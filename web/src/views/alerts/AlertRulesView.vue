<script setup lang="ts">
/**
 * M5 告警规则管理（等保三级：admin 可读写，规则变更即时生效）
 */
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { audit } from '@/utils/audit'
import { listAlertRules, createAlertRule, updateAlertRule, deleteAlertRule, testAlertRule } from '@/api/alerts'
import type { AlertRule, AlertQueryFilter } from '@/types'

const loading = ref(false)
const rules = ref<AlertRule[]>([])
const dialogVisible = ref(false)
const testing = ref(false)
const form = ref<AlertRule>(blankRule())

function blankRule(): AlertRule {
  return {
    name: '',
    description: '',
    rule_type: 'frequency',
    enabled: true,
    window_seconds: 300,
    query_filter: {},
    threshold: 5,
    baseline_seconds: 3600,
    group_by: 'host',
    realert_seconds: 3600,
    severity: 'warning',
    actions: [],
    run_interval_seconds: 60
  }
}

const typeOptions = [
  { label: 'frequency · 窗口内计数达阈值', value: 'frequency' },
  { label: 'spike · 突增（相对基线倍数）', value: 'spike' },
  { label: 'flatline · 静默（窗口内无日志）', value: 'flatline' },
  { label: 'any · 命中即告警', value: 'any' }
]
const sevOptions = [
  { label: '严重 critical', value: 'critical' },
  { label: '高危 high', value: 'high' },
  { label: '中危 medium', value: 'medium' },
  { label: '低危 low', value: 'low' },
  { label: '警告 warning', value: 'warning' }
]
const groupOptions = [
  { label: '不分组', value: '' },
  { label: '按主机 host', value: 'host' },
  { label: '按源IP source_ip', value: 'source_ip' },
  { label: '按事件类型 event_type', value: 'event_type' },
  { label: '按用户 user_name', value: 'user_name' }
]
const eventTypeOptions = [
  { label: 'SSH暴力破解', value: 'ssh_bruteforce' },
  { label: '登录失败', value: 'login_failed' },
  { label: '配置变更', value: 'config_change' },
  { label: '攻击事件', value: 'attack' },
  { label: '登录成功', value: 'auth_success' },
  { label: '进程行为', value: 'process' }
]

const severityTag = computed(() => (sev: string) => {
  const map: Record<string, string> = { critical: 'danger', high: 'danger', medium: 'warning', low: 'info', warning: 'warning' }
  return map[sev] || 'info'
})

/** Webhook URL 双向绑定（v-model 需为可写成员表达式） */
const webhookUrl = computed<string>({
  get: () => {
    const a = (form.value.actions || [])[0]
    return a && a.type === 'webhook' ? a.url || '' : ''
  },
  set: (v: string) => {
    if (!form.value.actions) form.value.actions = []
    const a = form.value.actions[0]
    if (!a) {
      form.value.actions.push({ type: 'webhook', url: v })
    } else if (a.type === 'webhook') {
      a.url = v
    } else {
      form.value.actions = [{ type: 'webhook', url: v }, ...form.value.actions]
    }
  }
})

async function doLoad(): Promise<void> {
  loading.value = true
  try {
    const resp = await listAlertRules()
    rules.value = resp.data.items
  } catch {
    ElMessage.error('规则加载失败，请确认后端服务可用')
  } finally {
    loading.value = false
  }
}

onMounted(doLoad)

function onOpenCreate(): void {
  form.value = blankRule()
  dialogVisible.value = true
}

function onOpenEdit(row: AlertRule): void {
  form.value = JSON.parse(JSON.stringify(row))
  if (!form.value.actions) form.value.actions = []
  dialogVisible.value = true
}

async function onSave(): Promise<void> {
  if (!form.value.name.trim()) {
    ElMessage.warning('规则名称不能为空')
    return
  }
  const f = form.value.query_filter as AlertQueryFilter
  const cleanFilter: AlertQueryFilter = {}
  if (f.host) cleanFilter.host = f.host
  if (f.source_ip) cleanFilter.source_ip = f.source_ip
  if (f.event_type) cleanFilter.event_type = f.event_type
  if (f.keyword) cleanFilter.keyword = f.keyword
  if (f.min_severity) cleanFilter.min_severity = f.min_severity
  form.value.query_filter = cleanFilter

  try {
    if (form.value.id) {
      await updateAlertRule(form.value.id, form.value)
      audit('update_alert_rule', form.value.name, '修改告警规则')
      ElMessage.success('规则已更新（调度器即时生效）')
    } else {
      await createAlertRule(form.value)
      audit('create_alert_rule', form.value.name, '新增告警规则')
      ElMessage.success('规则已创建')
    }
    dialogVisible.value = false
    void doLoad()
  } catch {
    ElMessage.error('保存失败')
  }
}

async function onToggle(row: AlertRule): Promise<void> {
  try {
    const copy = JSON.parse(JSON.stringify(row))
    copy.enabled = !copy.enabled
    await updateAlertRule(row.id!, copy)
    audit('update_alert_rule', row.name, copy.enabled ? '启用规则' : '停用规则')
    ElMessage.success(copy.enabled ? '规则已启用' : '规则已停用')
    void doLoad()
  } catch {
    ElMessage.error('状态切换失败')
  }
}

async function onDelete(row: AlertRule): Promise<void> {
  try {
    await ElMessageBox.confirm(`确认删除规则「${row.name}」？历史告警事件保留。`, '删除确认', { type: 'warning' })
    await deleteAlertRule(row.id!)
    audit('delete_alert_rule', row.name, '删除告警规则')
    ElMessage.success('已删除')
    void doLoad()
  } catch {
    /* 取消或失败 */
  }
}

async function onTest(row: AlertRule): Promise<void> {
  testing.value = true
  try {
    const resp = await testAlertRule(row.id!)
    const m = resp.data.matched
    const lines = Object.entries(m)
      .map(([k, v]) => `${k}: ${v} 条`)
      .join('\n')
    await ElMessageBox.alert(
      `规则「${row.name}」试运行结果（窗口 ${resp.data.window_seconds} 秒，不触发动作）：\n${lines || '无命中'}`,
      '规则试运行',
      { confirmButtonText: '知道了' }
    )
  } catch {
    ElMessage.error('试运行失败')
  } finally {
    testing.value = false
  }
}
</script>

<template>
  <div class="panel">
    <div class="panel-head">
      <div>
        <h2>告警规则</h2>
        <p class="sub">自研告警引擎（等价 ElastAlert2）· 调度周期 10 秒 · 规则变更即时生效</p>
      </div>
      <el-button type="primary" v-perm="'admin'" @click="onOpenCreate">新增规则</el-button>
    </div>

    <el-table v-loading="loading" :data="rules" stripe size="small">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="name" label="规则名称" min-width="140" />
      <el-table-column label="类型" width="180">
        <template #default="{ row }">
          <el-tag size="small" :type="row.rule_type === 'flatline' ? 'info' : 'warning'">{{ row.rule_type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="匹配条件" min-width="220">
        <template #default="{ row }">
          <span class="cond-text">
            {{ row.query_filter.host || '任意设备' }}
            {{ row.query_filter.source_ip ? ' · ' + row.query_filter.source_ip : '' }}
            {{ row.query_filter.event_type ? ' · ' + row.query_filter.event_type : '' }}
            {{ row.query_filter.keyword ? ' · "' + row.query_filter.keyword + '"' : '' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="窗口/阈值" width="120">
        <template #default="{ row }">
          {{ row.window_seconds }}s / &ge;{{ row.threshold }}
        </template>
      </el-table-column>
      <el-table-column label="聚合" width="90">
        <template #default="{ row }">{{ row.group_by || '—' }}</template>
      </el-table-column>
      <el-table-column label="级别" width="90">
        <template #default="{ row }">
          <el-tag size="small" :type="severityTag(row.severity)">{{ row.severity }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="alert_count" label="已触发" width="80" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-switch :model-value="row.enabled" @change="() => onToggle(row)" />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="170" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="onOpenEdit(row)">编辑</el-button>
          <el-button link type="primary" size="small" :loading="testing" @click="onTest(row)">试运行</el-button>
          <el-button link type="danger" size="small" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑规则' : '新增规则'" width="640px" destroy-on-close>
      <el-form label-width="110px" size="small">
        <el-form-item label="规则名称" required>
          <el-input v-model="form.name" maxlength="64" placeholder="如：SSH暴力破解高频告警" />
        </el-form-item>
        <el-form-item label="规则类型">
          <el-select v-model="form.rule_type" style="width: 100%">
            <el-option v-for="t in typeOptions" :key="t.value" :label="t.label" :value="t.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="查询窗口(秒)">
          <el-input-number v-model="form.window_seconds" :min="10" :step="60" style="width: 160px" />
          <span class="tip">统计窗口：近 N 秒内的日志</span>
        </el-form-item>
        <el-form-item label="匹配条件">
          <div class="filter-grid">
            <el-input v-model="(form.query_filter as AlertQueryFilter).host" placeholder="设备 host" clearable />
            <el-input v-model="(form.query_filter as AlertQueryFilter).source_ip" placeholder="源IP" clearable />
            <el-select v-model="(form.query_filter as AlertQueryFilter).event_type" clearable placeholder="事件类型">
              <el-option v-for="e in eventTypeOptions" :key="e.value" :label="e.label" :value="e.value" />
            </el-select>
            <el-input v-model="(form.query_filter as AlertQueryFilter).keyword" placeholder="关键词(全文)" clearable />
          </div>
        </el-form-item>
        <el-form-item label="阈值">
          <el-input-number v-model="form.threshold" :min="1" style="width: 140px" />
          <span class="tip">{{ form.rule_type === 'spike' ? '基线倍数(≥2)' : form.rule_type === 'flatline' ? '低于此数即告警' : '窗口内命中条数下限' }}</span>
        </el-form-item>
        <template v-if="form.rule_type === 'spike'">
          <el-form-item label="基线窗口(秒)">
            <el-input-number v-model="form.baseline_seconds" :min="60" :step="300" style="width: 160px" />
            <span class="tip">对比当前窗口之前 N 秒的基线</span>
          </el-form-item>
        </template>
        <template v-if="form.rule_type !== 'flatline'">
          <el-form-item label="聚合维度">
            <el-select v-model="form.group_by" style="width: 200px">
              <el-option v-for="g in groupOptions" :key="g.value" :label="g.label" :value="g.value" />
            </el-select>
            <span class="tip">按维度分组计数，每维独立判定与去重</span>
          </el-form-item>
        </template>
        <el-form-item label="去重窗口(秒)">
          <el-input-number v-model="form.realert_seconds" :min="0" :step="300" style="width: 160px" />
          <span class="tip">realert：同一规则+维度窗口内只告警一次，0 表示不去重</span>
        </el-form-item>
        <el-form-item label="告警级别">
          <el-select v-model="form.severity" style="width: 200px">
            <el-option v-for="s in sevOptions" :key="s.value" :label="s.label" :value="s.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="执行间隔(秒)">
          <el-input-number v-model="form.run_interval_seconds" :min="10" :step="30" style="width: 140px" />
        </el-form-item>
        <el-form-item label="Webhook">
          <el-input v-model="webhookUrl" placeholder="https://hook.example.com/alert（留空仅平台内记录）" clearable />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.description" type="textarea" :rows="2" maxlength="200" placeholder="规则用途说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="onSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.panel { padding: 16px; }
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 12px; }
.panel-head h2 { margin: 0; font-size: 18px; }
.sub { margin: 4px 0 0; color: #6B7280; font-size: 12px; }
.cond-text { color: #374151; font-size: 12px; }
.tip { margin-left: 10px; color: #9CA3AF; font-size: 12px; }
.filter-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 8px; width: 100%; }
</style>
