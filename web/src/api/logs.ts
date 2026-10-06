import http from './http'
import type { ApiResponse, PageResult, AuditLogItem, StatsOverview, LoginFailItem } from '@/types'

/** 日志检索（时间/字段/关键词分页） */
export function searchLogs(params: Record<string, unknown>): Promise<ApiResponse<PageResult<AuditLogItem>>> {
  return http.get('/logs/search', { params }).then((r) => r.data)
}

/** 日志详情 */
export function getLogDetail(id: string): Promise<ApiResponse<AuditLogItem>> {
  return http.get(`/logs/${id}`).then((r) => r.data)
}

/** CSV 导出（触发异步任务） */
export function exportLogs(params: Record<string, unknown>): Promise<ApiResponse<{ task_id: string }>> {
  return http.post('/logs/export', params).then((r) => r.data)
}

/** 统计概览 */
export function getStatsOverview(): Promise<ApiResponse<StatsOverview>> {
  return http.get('/stats/overview').then((r) => r.data)
}

/** 登录失败统计 */
export function getLoginFailStats(): Promise<ApiResponse<LoginFailItem[]>> {
  return http.get('/stats/login-fail').then((r) => r.data)
}
