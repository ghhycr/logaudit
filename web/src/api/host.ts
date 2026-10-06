import http from './http'
import type { ApiResponse } from '@/types'

/** 宿主机监控（总览仪表盘 · 主机硬件信息） */
export interface HostOverview {
  hostname: string
  uptime_seconds: number
  cpu: { cores: number; usage_percent: number }
  memory: { total_gb: number; used_gb: number; usage_percent: number }
  disk: { total_gb: number; used_gb: number; usage_percent: number; mount: string }
  collected_at: string
}

export function getHostOverview(): Promise<ApiResponse<HostOverview>> {
  return http.get('/host/overview').then((r) => r.data)
}
