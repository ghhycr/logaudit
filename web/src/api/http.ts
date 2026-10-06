/**
 * HTTP 客户端封装（等保三级安全要求）
 *  - JWT：Authorization: Bearer <access_token>（token 保存在内存，不落 localStorage）
 *  - 401：自动尝试 refresh_token 刷新后重放原请求；刷新失败则登出
 *  - CSRF：对写请求附加 X-XSRF-TOKEN 头（配合服务端 cookie 校验）
 *  - 统一错误处理与业务码判断
 */
import axios, { type AxiosError, type AxiosRequestConfig, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth'
import { audit } from '@/utils/audit'
import type { ApiResponse } from '@/types'

const BASE = import.meta.env.VITE_API_BASE || '/api/v1'

const http = axios.create({
  baseURL: BASE,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' }
})

/** 写操作请求附加 CSRF 防护头（骨架：值由服务端下发；无则跳过） */
const WRITE_METHODS = new Set(['post', 'put', 'delete', 'patch'])

http.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const auth = useAuthStore()
  if (auth.accessToken) {
    config.headers.set('Authorization', `Bearer ${auth.accessToken}`)
  }
  if (WRITE_METHODS.has((config.method || 'get').toLowerCase())) {
    const csrf = auth.csrfToken
    if (csrf) {
      config.headers.set('X-XSRF-TOKEN', csrf)
    }
  }
  return config
})

/** 401 刷新重试队列 */
let refreshPromise: Promise<string | null> | null = null

http.interceptors.response.use(
  (resp) => resp,
  async (error: AxiosError<ApiResponse>) => {
    const auth = useAuthStore()
    const status = error.response?.status
    const cfg = error.config as (AxiosRequestConfig & { _retried?: boolean }) | undefined

    // 业务错误：透传给调用方
    if (status && status >= 400 && status !== 401) {
      const msg = error.response?.data?.message || error.message
      audit('search_logs', cfg?.url || '', msg.slice(0, 128))
      return Promise.reject(error)
    }

    // 401：尝试刷新
    if (status === 401 && cfg && !cfg._retried && auth.refreshToken) {
      cfg._retried = true
      refreshPromise = refreshPromise || auth.refresh().catch(() => null)
      const newToken = await refreshPromise
      refreshPromise = null
      if (newToken) {
        return http.request(cfg)          // 重放原请求
      }
    }

    // 刷新失败或无令牌：登出
    auth.logout('登录状态已失效，请重新登录')
    return Promise.reject(error)
  }
)

export default http
