import http from './http'
import type { ApiResponse } from '@/types'

/** NTP 服务器配置（系统设置 · NTP，仅 admin） */
export interface NtpConfig {
  server1: string
  server2: string
  interval_hours: number
  enabled: boolean
}

export function getNtpConfig(): Promise<ApiResponse<NtpConfig>> {
  return http.get('/settings/ntp').then((r) => r.data)
}

export function saveNtpConfig(data: NtpConfig): Promise<ApiResponse<NtpConfig>> {
  return http.put('/settings/ntp', data).then((r) => r.data)
}

/** 基础设置（系统设置 · 基础设置，仅 admin）：登录失败处理 / 会话超时 / 密码策略 / 访问白名单 */
export interface BaseSettings {
  max_fail_count: number
  lock_minutes: number
  session_timeout_minutes: number
  pwd_min_length: number
  pwd_require_upper: boolean
  pwd_require_lower: boolean
  pwd_require_digit: boolean
  pwd_require_special: boolean
  pwd_expire_days: number
  whitelist_ips: string
}

export function getBaseSettings(): Promise<ApiResponse<BaseSettings>> {
  return http.get('/settings/base').then((r) => r.data)
}

export function saveBaseSettings(data: BaseSettings): Promise<ApiResponse<BaseSettings>> {
  return http.put('/settings/base', data).then((r) => r.data)
}
