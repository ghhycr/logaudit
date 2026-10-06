<script setup lang="ts">
/**
 * 基础设置（系统设置 · 基础设置，仅 admin）
 * 登录失败处理 / 会话超时 / 密码复杂度 / 密码有效期 / 系统访问白名单
 */
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { audit } from '@/utils/audit'
import { getBaseSettings, saveBaseSettings, type BaseSettings } from '@/api/settings'

const loading = ref(false)
const saving = ref(false)
const form = reactive<BaseSettings>({
  max_fail_count: 5,
  lock_minutes: 10,
  session_timeout_minutes: 15,
  pwd_min_length: 8,
  pwd_require_upper: true,
  pwd_require_lower: true,
  pwd_require_digit: true,
  pwd_require_special: true,
  pwd_expire_days: 90,
  whitelist_ips: ''
})

async function doLoad(): Promise<void> {
  loading.value = true
  try {
    const resp = await getBaseSettings()
    Object.assign(form, resp.data)
  } catch {
    ElMessage.error('基础设置加载失败')
  } finally {
    loading.value = false
  }
}

async function doSave(): Promise<void> {
  if (form.max_fail_count < 1 || form.max_fail_count > 10) {
    ElMessage.warning('连续失败次数须在 1-10 之间')
    return
  }
  if (form.lock_minutes < 1 || form.lock_minutes > 120) {
    ElMessage.warning('锁定时长须在 1-120 分钟之间')
    return
  }
  if (form.session_timeout_minutes < 5 || form.session_timeout_minutes > 240) {
    ElMessage.warning('会话超时须在 5-240 分钟之间')
    return
  }
  if (form.pwd_min_length < 8 || form.pwd_min_length > 32) {
    ElMessage.warning('密码最小长度须在 8-32 之间')
    return
  }
  if (form.pwd_expire_days < 0 || form.pwd_expire_days > 365) {
    ElMessage.warning('密码有效期须在 0-365 天之间（0 为不限期）')
    return
  }
  if (!form.pwd_require_upper && !form.pwd_require_lower && !form.pwd_require_digit && !form.pwd_require_special) {
    ElMessage.warning('密码复杂度至少须启用一类字符要求')
    return
  }
  saving.value = true
  try {
    const resp = await saveBaseSettings({ ...form })
    Object.assign(form, resp.data)
    ElMessage.success('基础设置已保存，登录/会话/口令策略即时生效')
    audit('system_config', 'base', '更新基础设置')
  } catch {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => void doLoad())
</script>

<template>
  <div v-loading="loading" class="page">
    <div class="panel">
      <div class="panel-title">登录失败处理（等保三级 · 连续失败锁定）</div>
      <div class="form-grid">
        <el-form-item label="连续失败次数">
          <el-input-number v-model="form.max_fail_count" :min="1" :max="10" style="width: 160px" />
          <span class="hint">次（1-10）</span>
        </el-form-item>
        <el-form-item label="锁定账号时长">
          <el-input-number v-model="form.lock_minutes" :min="1" :max="120" style="width: 160px" />
          <span class="hint">分钟（1-120）</span>
        </el-form-item>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">会话超时</div>
      <div class="form-grid">
        <el-form-item label="空闲自动登出时长">
          <el-input-number v-model="form.session_timeout_minutes" :min="5" :max="240" style="width: 160px" />
          <span class="hint">分钟（5-240，无操作自动登出）</span>
        </el-form-item>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">密码复杂度设置（创建用户 / 重置密码 / 修改密码时强制校验）</div>
      <div class="form-grid">
        <el-form-item label="最小长度">
          <el-input-number v-model="form.pwd_min_length" :min="8" :max="32" style="width: 160px" />
          <span class="hint">位（8-32）</span>
        </el-form-item>
        <el-form-item label="字符要求">
          <el-checkbox v-model="form.pwd_require_upper">大写字母</el-checkbox>
          <el-checkbox v-model="form.pwd_require_lower">小写字母</el-checkbox>
          <el-checkbox v-model="form.pwd_require_digit">数字</el-checkbox>
          <el-checkbox v-model="form.pwd_require_special">特殊字符</el-checkbox>
        </el-form-item>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">密码有效期（等保三级 · 口令定期更换）</div>
      <div class="form-grid">
        <el-form-item label="有效期">
          <el-input-number v-model="form.pwd_expire_days" :min="0" :max="365" style="width: 160px" />
          <span class="hint">天（0 = 不限期；到期后登录时提示修改密码）</span>
        </el-form-item>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">系统访问白名单</div>
      <div class="form-grid">
        <el-form-item label="允许登录来源" label-width="120px">
          <el-input
            v-model="form.whitelist_ips"
            type="textarea"
            :rows="4"
            placeholder="留空 = 不限制。每行一个 IP 或网段，例如：&#10;192.168.143.0/24&#10;10.10.1.5&#10;（登录来源不在白名单内将被拒绝）"
            style="width: 100%"
          />
        </el-form-item>
      </div>
    </div>

    <div class="actions">
      <el-button type="primary" :loading="saving" @click="doSave">保存配置</el-button>
      <el-button @click="doLoad">重置</el-button>
      <span class="tip">保存后立即生效，无需重启服务</span>
    </div>
  </div>
</template>

<style scoped>
.page { min-height: 200px; }
.panel { background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); margin-bottom: 12px; }
.panel-title { font-size: 13px; font-weight: 600; color: #1a1b1c; margin-bottom: 12px; }
.form-grid { display: flex; flex-direction: column; gap: 4px; }
.form-grid .el-form-item { margin-bottom: 8px; }
.hint { font-size: 12px; color: #9aa0a6; margin-left: 8px; }
.actions { display: flex; align-items: center; gap: 8px; }
.tip { font-size: 12px; color: #9aa0a6; margin-left: 8px; }
</style>
