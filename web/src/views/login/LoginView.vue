<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { audit } from '@/utils/audit'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

// 等保三级锁定策略参数（模板中不能直接访问 import.meta.env，须在 script 取值）
const lockThreshold = Number(import.meta.env.VITE_LOGIN_FAIL_LOCK_THRESHOLD || 5)
const lockMinutes = Number(import.meta.env.VITE_LOGIN_FAIL_LOCK_MINUTES || 10)

const formRef = ref<FormInstance>()
const loading = ref(false)
const form = reactive({ username: '', password: '' })

const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

async function onSubmit(): Promise<void> {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    await auth.login(form.username, form.password)
    audit('login', form.username)
    ElMessage.success('登录成功')
    const redirect = (route.query.redirect as string) || '/dashboard'
    void router.push(redirect)
  } catch (e) {
    const msg = e instanceof Error ? e.message : '登录失败'
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <div class="title">日志审计平台</div>
      <div class="subtitle">网络设备 · 服务器 · 安全设备 日志集中留存与审计</div>

      <el-form ref="formRef" :model="form" :rules="rules" size="large" @keyup.enter="onSubmit">
        <el-form-item prop="username">
          <el-input v-model="form.username" placeholder="用户名" :prefix-icon="'User'" autocomplete="username" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码"
            :prefix-icon="'Lock'"
            show-password
            autocomplete="current-password"
          />
        </el-form-item>

        <!-- 等保三级：登录失败锁定提示（服务端锁定状态为准，前端展示） -->
        <el-alert
          v-if="auth.lockUntil && Date.now() < auth.lockUntil"
          type="error"
          :closable="false"
          show-icon
          title="账号已锁定"
          :description="`连续登录失败已达上限，请稍后再试`"
          style="margin-bottom: 12px"
        />

        <el-button type="primary" :loading="loading" style="width: 100%" @click="onSubmit">
          登 录
        </el-button>
      </el-form>

      <div class="foot">
        连续失败 {{ lockThreshold }} 次将锁定 {{ lockMinutes }} 分钟 ｜ 会话空闲 15 分钟自动登出
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(160deg, #eef4f3 0%, #e2ecea 60%, #d7e6e4 100%);
}
.login-card {
  width: 380px;
  background: #fff;
  border-radius: 14px;
  padding: 36px 32px 24px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.08);
}
.title { font-size: 20px; font-weight: 700; color: #1a1b1c; text-align: center; }
.subtitle { font-size: 12px; color: #6b7280; text-align: center; margin: 8px 0 24px; }
.foot { margin-top: 18px; font-size: 11.5px; color: #9aa0a6; text-align: center; line-height: 1.7; }
</style>
