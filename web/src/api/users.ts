import http from './http'
import type { ApiResponse } from '@/types'

/** 用户管理（系统设置 · 用户管理，仅 admin） */
export interface UserItem {
  id: number
  username: string
  display_name: string
  role: 'admin' | 'viewer' | 'auditor'
  status: number
  fail_count: number
  last_login_at: string
  locked: boolean
  created_at: string
}

export const ROLE_OPTIONS = [
  { value: 'admin', label: '管理员' },
  { value: 'viewer', label: '只读审计员' },
  { value: 'auditor', label: '审计操作员' }
]

export function roleLabel(role: string): string {
  return ROLE_OPTIONS.find((r) => r.value === role)?.label || role
}

export function listUsers(): Promise<ApiResponse<UserItem[]>> {
  return http.get('/users').then((r) => r.data)
}

export function createUser(data: {
  username: string
  password: string
  display_name: string
  role: string
}): Promise<ApiResponse<{ id: number; username: string }>> {
  return http.post('/users', data).then((r) => r.data)
}

export function updateUser(
  id: number,
  data: { display_name?: string; role?: string; status?: number }
): Promise<ApiResponse<null>> {
  return http.put(`/users/${id}`, data).then((r) => r.data)
}

export function deleteUser(id: number): Promise<ApiResponse<null>> {
  return http.delete(`/users/${id}`).then((r) => r.data)
}

export function resetPassword(
  id: number,
  password?: string
): Promise<ApiResponse<{ username: string; password: string }>> {
  return http.post(`/users/${id}/reset-password`, password ? { password } : {}).then((r) => r.data)
}
