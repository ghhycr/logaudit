/** 全局类型定义 */

/** 登录响应 */
export interface LoginResponse {
  access_token: string
  refresh_token: string
  expires_in: number          // access token 有效期（秒）
  csrf_token?: string         // CSRF 防护令牌（服务端下发；写请求附带 X-XSRF-TOKEN）
  session_timeout_minutes?: number // 会话空闲超时（分钟，基础设置）
  need_change_password?: boolean   // 密码已到期，需要修改（基础设置·密码有效期）
  user: UserInfo
}

/** 用户信息 */
export interface UserInfo {
  id: number
  username: string
  display_name: string
  role: UserRole               // admin / viewer / auditor
  locked?: boolean
  last_login_at?: string
}

/** 角色（等保三级：最小权限） */
export type UserRole = 'admin' | 'viewer' | 'auditor'

/** 统一 API 响应包装 */
export interface ApiResponse<T = unknown> {
  code: number                 // 0 成功；非 0 业务错误
  message: string
  data: T
}

/** 日志条目 */
export interface AuditLogItem {
  ts: string
  host: string
  program: string
  severity: number
  source_ip: string
  user_name: string
  event_category: string
  event_type: string
  outcome: string
  message: string
}

/** 分页结果 */
export interface PageResult<T> {
  total: number
  page: number
  size: number
  items: T[]
}

/** 操作审计记录（等保三级：安全审计） */
export interface OpAuditRecord {
  id: number
  username: string
  action: string
  target: string
  detail?: string
  ts: string
  ip: string
}

/** 统计概览 */
export interface StatsOverview {
  today_total: number
  last_7d: Array<{ day: string; count: number }>
  category_dist: Array<{ category: string; count: number }>
  top_sources: Array<{ source_ip: string; count: number }>
  top_events: Array<{ event_type: string; count: number }>
  device_count?: number        // 设备接入数（devices 表启用数）
}

/** 登录失败统计（近 7 天，来自 MySQL 操作审计） */
export interface LoginFailItem {
  day: string
  count: number
}

/** 留存与容量状态 */
export interface RetentionInfo {
  ttl_days: number
  storage_engine: string
  compression: string
  partition_field: string
  indexes: string[]
  meta_backend: string
  total_bytes: number
  partitions: Array<{
    key: string
    date: string
    rows: number
    bytes: number
    days_left: number
  }>
}

/** M5 告警规则 */
export interface AlertRule {
  id?: number
  name: string
  description?: string
  rule_type: 'frequency' | 'spike' | 'flatline' | 'any'
  enabled: boolean
  window_seconds: number
  query_filter: AlertQueryFilter
  threshold: number
  baseline_seconds?: number
  group_by?: string            // '' | host | source_ip | event_type | user_name
  realert_seconds: number
  severity: 'critical' | 'high' | 'medium' | 'low' | 'warning'
  actions?: AlertAction[]
  run_interval_seconds?: number
  last_run_at?: string
  last_alert_at?: string
  alert_count?: number
  created_at?: string
}

/** 告警规则匹配条件 */
export interface AlertQueryFilter {
  host?: string
  source_ip?: string
  event_type?: string
  keyword?: string
  min_severity?: number
}

/** 告警动作 */
export interface AlertAction {
  type: 'webhook' | 'smtp'
  url?: string
  headers?: Record<string, string>
  to?: string[]
}

/** 告警事件 */
export interface AlertEvent {
  id: number
  rule_id: number
  rule_name: string
  severity: string
  match_key: string
  match_count: number
  message: string
  status: 'open' | 'acked' | 'closed'
  acked_by?: string
  created_at: string
}

/** 告警统计 */
export interface AlertStats {
  open: number
  today: number
  total: number
}
