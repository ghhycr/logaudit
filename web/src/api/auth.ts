import http from './http'
import type { ApiResponse, LoginResponse, PageResult, AuditLogItem, UserInfo } from '@/types'

/** 登录（等保三级：失败次数与锁定由服务端返回，前端展示策略提示） */
export function login(username: string, password: string): Promise<ApiResponse<LoginResponse>> {
  return http.post('/auth/login', { username, password }).then((r) => r.data)
}

/** 刷新令牌 */
export function refreshToken(): Promise<ApiResponse<{ access_token: string }>> {
  return http.post('/auth/refresh').then((r) => r.data)
}

/** 登出（通知服务端吊销会话） */
export function logout(): Promise<ApiResponse<null>> {
  return http.post('/auth/logout').then((r) => r.data)
}

/** 修改密码（等保三级：密码复杂度服务端复核） */
export function changePassword(oldPwd: string, newPwd: string): Promise<ApiResponse<null>> {
  return http.post('/auth/change-password', { old_password: oldPwd, new_password: newPwd }).then((r) => r.data)
}

/** 当前用户信息 */
export function fetchMe(): Promise<ApiResponse<UserInfo>> {
  return http.get('/auth/me').then((r) => r.data)
}

/** 日志检索 */
export function searchLogs(params: Record<string, unknown>): Promise<ApiResponse<PageResult<AuditLogItem>>> {
  return http.get('/logs/search', { params }).then((r) => r.data)
}
