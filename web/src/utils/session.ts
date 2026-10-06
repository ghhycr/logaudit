/**
 * 等保三级 · 会话管理：空闲超时自动登出 + 会话心跳
 *
 * 策略：
 *  - 用户无操作达到 VITE_SESSION_IDLE_TIMEOUT_MIN 分钟 → 自动登出
 *  - 会话倒计时临近时给用户提示
 *  - 登出前将操作审计队列 flush（安全审计不丢失）
 */
import { useAuthStore } from '@/stores/auth'
import { flushAuditQueue } from '@/utils/audit'

const IDLE_TIMEOUT_MIN = Number(import.meta.env.VITE_SESSION_IDLE_TIMEOUT_MIN || 15)
const WARN_BEFORE_MIN = 1            // 剩余 1 分钟时提示
const IDLE_MS = IDLE_TIMEOUT_MIN * 60 * 1000

let timer: ReturnType<typeof setTimeout> | null = null
let warnTimer: ReturnType<typeof setTimeout> | null = null
let lastActive = Date.now()
let started = false

const EVENTS: Array<keyof WindowEventMap> = ['mousemove', 'mousedown', 'keydown', 'scroll', 'touchstart']

function onUserActive(): void {
  lastActive = Date.now()
}

function scheduleTimers(): void {
  if (timer) clearTimeout(timer)
  if (warnTimer) clearTimeout(warnTimer)

  const remain = lastActive + IDLE_MS - Date.now()
  const warnAt = Math.max(0, remain - WARN_BEFORE_MIN * 60 * 1000)

  timer = setTimeout(() => {
    void logoutDueToIdle()
  }, Math.max(0, remain))

  warnTimer = setTimeout(() => {
    const auth = useAuthStore()
    if (auth.isLoggedIn) {
      auth.warnIdle(WARN_BEFORE_MIN)
      scheduleTimers()            // 重置，等待最终超时
    }
  }, warnAt)
}

async function logoutDueToIdle(): Promise<void> {
  const auth = useAuthStore()
  if (!auth.isLoggedIn) return
  await flushAuditQueue()          // 先落审计，再登出
  auth.logout('会话超时，请重新登录')
}

/** 启动会话超时监控（登录成功后调用） */
export function startSessionMonitor(): void {
  if (started) return
  started = true
  lastActive = Date.now()
  for (const ev of EVENTS) {
    window.addEventListener(ev, onUserActive, { passive: true })
  }
  scheduleTimers()
}

/** 停止监控（登出时调用） */
export function stopSessionMonitor(): void {
  if (!started) return
  started = false
  for (const ev of EVENTS) {
    window.removeEventListener(ev, onUserActive)
  }
  if (timer) clearTimeout(timer)
  if (warnTimer) clearTimeout(warnTimer)
  timer = null
  warnTimer = null
}

/** 用户主动操作时刷新空闲基准（调用方可在关键交互后调用） */
export function touchSession(): void {
  lastActive = Date.now()
}
