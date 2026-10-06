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
