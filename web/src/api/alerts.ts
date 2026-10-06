/** M5 告警规则与事件接口 */
import http from './http'
import type {
  AlertAction,
  AlertEvent,
  AlertQueryFilter,
  AlertRule,
  AlertStats,
  ApiResponse,
  PageResult
} from '@/types'

/** 规则列表 */
export function listAlertRules(): Promise<ApiResponse<{ items: AlertRule[]; total: number }>> {
  return http.get('/alerts/rules').then((r) => r.data)
}

/** 新增规则（admin） */
export function createAlertRule(rule: AlertRule): Promise<ApiResponse<AlertRule>> {
  return http.post('/alerts/rules', rule).then((r) => r.data)
}

/** 更新规则（admin） */
export function updateAlertRule(id: number, rule: AlertRule): Promise<ApiResponse<AlertRule>> {
  return http.put(`/alerts/rules/${id}`, rule).then((r) => r.data)
}

/** 删除规则（admin） */
export function deleteAlertRule(id: number): Promise<ApiResponse<null>> {
  return http.delete(`/alerts/rules/${id}`).then((r) => r.data)
}

/** 试运行规则（admin，dry-run 不触发动作） */
export function testAlertRule(id: number): Promise<ApiResponse<{ matched: Record<string, number>; dry_run: boolean }>> {
  return http.post(`/alerts/rules/${id}/test`).then((r) => r.data)
}

/** 告警事件列表 */
export function listAlertEvents(params: {
  status?: string
  rule_id?: number
  page?: number
  size?: number
}): Promise<ApiResponse<PageResult<AlertEvent>>> {
  return http.get('/alerts/events', { params }).then((r) => r.data)
}

/** 确认告警事件 */
export function ackAlertEvent(id: number): Promise<ApiResponse<null>> {
  return http.post(`/alerts/events/${id}/ack`).then((r) => r.data)
}

/** 告警统计 */
export function getAlertStats(): Promise<ApiResponse<AlertStats>> {
  return http.get('/alerts/events/stats').then((r) => r.data)
}

export type { AlertRule, AlertQueryFilter, AlertAction, AlertEvent }
