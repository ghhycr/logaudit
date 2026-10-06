/**
 * 等保三级 · 安全审计：前端操作审计埋点
 *
 * 记录用户关键操作（登录/查询/导出/管理动作），本地队列缓存，
 * 定时或登出前 flush 到后端 /audit/ops（后端落 op_audit 表）。
 * 队列上限 200 条，超过即强制上报，避免审计数据丢失。
 */
import { useAuthStore } from '@/stores/auth'

export type AuditAction =
  | 'login'
  | 'logout'
  | 'search_logs'
  | 'export_logs'
  | 'view_log_detail'
  | 'create_device'
  | 'update_device'
  | 'delete_device'
  | 'create_user'
  | 'update_user'
  | 'change_password'
  | 'system_config'

interface AuditEntry {
  action: AuditAction
  target: string
  detail?: string
  ts: string
}

const QUEUE_MAX = 200
let queue: AuditEntry[] = []
let flushing = false

/** 记录一条操作审计 */
export function audit(action: AuditAction, target = '', detail?: string): void {
  queue.push({
    action,
    target: target.slice(0, 128),
    detail: detail ? detail.slice(0, 512) : undefined,
    ts: new Date().toISOString()
  })
  if (queue.length >= QUEUE_MAX) {
    void flushAuditQueue()
  }
}

/** 将队列上报后端（幂等；失败保留队列，下次重试） */
export async function flushAuditQueue(): Promise<void> {
  if (flushing || queue.length === 0) return
  flushing = true
  const auth = useAuthStore()
  const token = auth.accessToken
  const entries = queue
  try {
    if (token) {
      const resp = await fetch('/api/v1/audit/ops', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({ entries })
      })
      if (resp.ok) {
        queue = []          // 仅成功后清空
      }
    } else {
      // 未登录时（如登录尝试）仅保留内存
      queue = []
    }
  } catch {
    // 网络失败：保留队列等待下次 flush
  } finally {
    flushing = false
  }
}

/** 读取当前队列长度（设置页展示审计待上报数） */
export function auditQueueLength(): number {
  return queue.length
}

/** 清理队列（登出时由 session.ts 先 flush） */
export function clearAuditQueue(): void {
  queue = []
}
