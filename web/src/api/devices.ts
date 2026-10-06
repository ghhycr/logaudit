import http from './http'
import type { ApiResponse, RetentionInfo } from '@/types'

/** 设备台账（骨架：类型由后端契约确定） */
export interface DeviceItem {
  id: number
  name: string
  ip: string
  vendor: string
  model: string
  kind: string
  location: string
  enabled: boolean
}

export function listDevices(): Promise<ApiResponse<DeviceItem[]>> {
  return http.get('/devices').then((r) => r.data)
}

export function createDevice(data: Partial<DeviceItem>): Promise<ApiResponse<DeviceItem>> {
  return http.post('/devices', data).then((r) => r.data)
}

export function updateDevice(id: number, data: Partial<DeviceItem>): Promise<ApiResponse<DeviceItem>> {
  return http.put(`/devices/${id}`, data).then((r) => r.data)
}

export function deleteDevice(id: number): Promise<ApiResponse<null>> {
  return http.delete(`/devices/${id}`).then((r) => r.data)
}

/** 留存状态 */
export function getRetentionStatus(): Promise<ApiResponse<RetentionInfo>> {
  return http.get('/retention').then((r) => r.data)
}

/** 用户管理（管理员） */
export function listUsers(): Promise<ApiResponse<unknown>> {
  return http.get('/users').then((r) => r.data)
}
