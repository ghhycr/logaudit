<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { changePassword } from '@/api/auth'
import PasswordStrength from '@/components/security/PasswordStrength.vue'
import { checkPassword } from '@/utils/password'
import { audit } from '@/utils/audit'
import { auditQueueLength, flushAuditQueue } from '@/utils/audit'

const formRef = ref<FormInstance>()
const queueLen = ref(auditQueueLength())
const form = reactive({ oldPwd: '', newPwd: '', confirm: '' })

const rules: FormRules = {
  oldPwd: [{ required: true, message: '请输入原密码', trigger: 'blur' }],
  newPwd: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    {
      validator: (_r, v: string, cb) => {
        const c = checkPassword(v, '')
        cb(c.ok ? undefined : new Error(c.errors[0]))
      },
      trigger: 'blur'
    }
  ],
  confirm: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (_r, v: string, cb) => cb(v === form.newPwd ? undefined : new Error('两次输入不一致')),
      trigger: 'blur'
    }
  ]
}

async function onSave(): Promise<void> {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  try {
    await changePassword(form.oldPwd, form.newPwd)
    audit('change_password', '', '修改密码成功')
    ElMessage.success('密码修改成功，请重新登录')
    form.oldPwd = form.newPwd = form.confirm = ''
  } catch {
    ElMessage.error('密码修改失败（原密码错误或服务端策略拦截）')
  }
}

async function onFlushAudit(): Promise<void> {
  await flushAuditQueue()
  queueLen.value = auditQueueLength()
  ElMessage.success(queueLen.value === 0 ? '操作审计已全部上报' : '仍有未上报记录')
}
</script>

<template>
  <div class="panel">
    <div class="panel-title">系统设置</div>

    <el-form ref="formRef" :model="form" :rules="rules" label-width="90px" style="max-width:420px">
      <el-form-item label="原密码">
        <el-input v-model="form.oldPwd" type="password" show-password autocomplete="current-password" />
      </el-form-item>
      <el-form-item label="新密码">
        <el-input v-model="form.newPwd" type="password" show-password autocomplete="new-password" />
        <PasswordStrength :password="form.newPwd" />
      </el-form-item>
      <el-form-item label="确认密码">
        <el-input v-model="form.confirm" type="password" show-password autocomplete="new-password" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" v-audit="'change_password'" @click="onSave">修改密码</el-button>
      </el-form-item>
    </el-form>

    <el-divider />

    <div class="sec-title">安全审计</div>
    <div class="row">
      <span>待上报操作审计：{{ queueLen }} 条（等保三级：前端操作留痕）</span>
      <el-button size="small" v-audit="'system_config'" @click="onFlushAudit">立即上报</el-button>
    </div>
  </div>
</template>

<style scoped>
.panel { background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); }
.panel-title { font-size: 13px; font-weight: 600; color: #1a1b1c; margin-bottom: 14px; }
.sec-title { font-size: 13px; font-weight: 600; color: #1a1b1c; margin-bottom: 8px; }
.row { display: flex; align-items: center; gap: 12px; font-size: 12.5px; color: #6b7280; }
</style>
