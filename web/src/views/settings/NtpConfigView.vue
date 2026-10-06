<script setup lang="ts">
/**
 * NTP 服务器配置（系统设置）：时间同步服务器与同步周期（等保三级：日志时间一致性）
 * 数据源：GET/PUT /api/v1/settings/ntp
 */
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { audit } from '@/utils/audit'
import { getNtpConfig, saveNtpConfig } from '@/api/settings'

const loading = ref(false)
const saving = ref(false)
const form = reactive({
  server1: 'ntp.aliyun.com',
  server2: 'ntp.tencent.com',
  interval_hours: 24,
  enabled: true
})

const serverRef = ref()
const rules = {
  server1: [
    { required: true, message: 'NTP 服务器 1 必填', trigger: 'blur' },
    {
      pattern: /^([a-zA-Z0-9.-]+|\d{1,3}(\.\d{1,3}){3})$/,
      message: '请输入域名或 IP 地址', trigger: 'blur'
    }
  ],
  interval_hours: [{ required: true, message: '同步间隔必填', trigger: 'blur' }]
}

async function doLoad(): Promise<void> {
  loading.value = true
  try {
    const resp = await getNtpConfig()
    const d = resp.data
    if (d) {
      form.server1 = d.server1 || 'ntp.aliyun.com'
      form.server2 = d.server2 || 'ntp.tencent.com'
      form.interval_hours = d.interval_hours || 24
      form.enabled = d.enabled
    }
  } catch {
    ElMessage.error('NTP 配置加载失败')
  } finally {
    loading.value = false
  }
}

async function onSave(): Promise<void> {
  try {
    await serverRef.value?.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    await saveNtpConfig({ ...form })
    audit('system_config', 'ntp', `保存 NTP 配置：${form.server1} / ${form.interval_hours}h`)
    ElMessage.success('NTP 配置已保存')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(doLoad)
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div class="page-title">NTP 服务器配置</div>
      <div class="page-sub">时间同步保障日志时间戳一致性（等保三级：审计记录时间来源可信）</div>
    </div>

    <el-card v-loading="loading" shadow="never" class="ntp-card">
      <el-form ref="serverRef" :model="form" :rules="rules" label-width="150px" style="max-width: 520px">
        <el-form-item label="NTP 服务器 1" prop="server1">
          <el-input v-model="form.server1" placeholder="如 ntp.aliyun.com" />
        </el-form-item>
        <el-form-item label="NTP 服务器 2">
          <el-input v-model="form.server2" placeholder="备用服务器，可留空" />
        </el-form-item>
        <el-form-item label="同步间隔（小时）" prop="interval_hours">
          <el-input-number v-model="form.interval_hours" :min="1" :max="168" style="width: 160px" />
          <span class="hint">建议 24（每日同步），范围 1-168</span>
        </el-form-item>
        <el-form-item label="启用时间同步">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="onSave">保存配置</el-button>
        </el-form-item>
      </el-form>
      <el-alert type="info" :closable="false" show-icon class="tip">
        保存后建议在宿主机执行 <code>timedatectl set-ntp true</code> 使系统时钟与 NTP 服务器保持同步；
        采集端（Vector）与日志检索均以宿主机时间为准。
      </el-alert>
    </el-card>
  </div>
</template>

<style scoped>
.page-head { margin-bottom: 14px; }
.page-title { font-size: 18px; font-weight: 600; color: #1a1b1c; }
.page-sub { font-size: 12px; color: #6b7280; margin-top: 4px; }
.ntp-card { max-width: 680px; border-radius: 12px; }
.hint { font-size: 12px; color: #6b7280; margin-left: 10px; }
.tip { margin-top: 6px; }
code { background: rgba(0,0,0,0.06); padding: 1px 6px; border-radius: 4px; font-size: 12px; }
</style>
