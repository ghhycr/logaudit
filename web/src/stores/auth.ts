/**
 * 认证状态（等保三级）
 *  - access_token / refresh_token 保存在内存（Pinia），不落 localStorage
 *    （刷新页面后由 /auth/me 重新认证，避免 token 长期驻留本地）
 *  - 角色权限：admin / viewer / auditor（最小权限）
 *  - 登录失败锁定：本地计数 + 后端锁定状态（服务端为准）
 */
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { login as apiLogin, refreshToken as apiRefresh, logout as apiLogout, fetchMe } from '@/api/auth'
import { audit } from '@/utils/audit'
import { startSessionMonitor, stopSessionMonitor } from '@/utils/session'
import type { UserInfo, UserRole } from '@/types'

const LOCK_THRESHOLD = Number(import.meta.env.VITE_LOGIN_FAIL_LOCK_THRESHOLD || 5)
const LOCK_MINUTES = Number(import.meta.env.VITE_LOGIN_FAIL_LOCK_MINUTES || 10)

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref<string | null>(null)
  const refreshToken = ref<string | null>(null)
  const csrfToken = ref<string | null>(null)
  const user = ref<UserInfo | null>(null)
  const loginFails = ref(0)
  const lockUntil = ref<number | null>(null)   // 时间戳
  const idleWarning = ref(false)

  const isLoggedIn = computed(() => !!accessToken.value && !!user.value)
  const role = computed<UserRole | null>(() => user.value?.role ?? null)

  /** 角色是否可访问（admin 全量；auditor/viewer 只读） */
  const canWrite = computed(() => role.value === 'admin')

  async function login(username: string, password: string): Promise<void> {
    // 本地锁定检查
    if (lockUntil.value && Date.now() < lockUntil.value) {
      const remain = Math.ceil((lockUntil.value - Date.now()) / 60000)
      throw new Error(`连续登录失败次数过多，账号已锁定，请 ${remain} 分钟后再试`)
    }
    try {
      const resp = await apiLogin(username, password)
      const d = resp.data
      accessToken.value = d.access_token
      refreshToken.value = d.refresh_token
      csrfToken.value = d.csrf_token ?? null
      user.value = d.user
      loginFails.value = 0
      lockUntil.value = null
      idleWarning.value = false
      audit('login', username)
      startSessionMonitor()
    } catch (e) {
      loginFails.value += 1
      if (loginFails.value >= LOCK_THRESHOLD) {
        lockUntil.value = Date.now() + LOCK_MINUTES * 60 * 1000
        loginFails.value = 0
        throw new Error(`连续登录失败 ${LOCK_THRESHOLD} 次，账号已锁定 ${LOCK_MINUTES} 分钟（等保三级安全策略）`)
      }
      throw e
    }
  }

  /** 刷新 access token；成功返回新 token */
  async function refresh(): Promise<string | null> {
    if (!refreshToken.value) return null
    const resp = await apiRefresh()
    accessToken.value = resp.data.access_token
    return accessToken.value
  }

  /** 重新拉取当前用户（刷新页面后恢复会话） */
  async function restore(): Promise<boolean> {
    try {
      const resp = await fetchMe()
      user.value = resp.data
      startSessionMonitor()
      return true
    } catch {
      return false
    }
  }

  function logout(reason = ''): void {
    if (accessToken.value) {
      audit('logout', reason)
      void apiLogout().catch(() => { /* 忽略登出接口失败 */ })
    }
    accessToken.value = null
    refreshToken.value = null
    csrfToken.value = null
    user.value = null
    idleWarning.value = false
    stopSessionMonitor()
  }

  function warnIdle(minutes: number): void {
    idleWarning.value = true
  }

  return {
    accessToken, refreshToken, csrfToken, user, loginFails, lockUntil, idleWarning,
    isLoggedIn, role, canWrite, login, refresh, restore, logout, warnIdle
  }
})
