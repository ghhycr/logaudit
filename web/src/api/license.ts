import http from './http'
import type { ApiResponse } from '@/types'

/** 授权管理模块接口（Java 授权服务，Nginx 反代 /api/v1/license/ → license:8091）
 * 授权码统一由本地离线工具 LicenseTool.exe 签发（签名密钥与服务端 LICENSE_SECRET 一致），
 * 平台侧只提供：硬件指纹、校验、导入激活、导出授权文件与留痕查询。
 */

export interface HardwareInfo {
  hardware_id: string
  details: Record<string, string>
}

export interface LicenseStatus {
  activated: boolean
  status: 'none' | 'active' | 'permanent' | 'expired' | 'invalid' | 'mismatch'
  status_text: string
  hardware_id: string
  hardware_bound?: string
  bound_matched?: boolean
  product?: string
  permanent?: boolean
  expire_date?: string
  days_left?: number
  issued_at?: string
  license_code?: string
  license_code_masked?: string
  activated_by?: string
  activated_at?: string
}

export interface VerifyResult {
  valid: boolean
  matched: boolean
  message: string
  hardware_id: string
  local_hardware_id: string
  permanent: boolean
  expire: string
  days_left: number
  issued_at: string
  code: string
}

/** 授权导入记录（等保三级留痕） */
export interface LicenseRecord {
  id: number
  hardware_id: string
  license_code_masked: string
  permanent: number
  expire_date: string
  days: number
  customer: string
  note: string
  operator: string
  issued_at: string
}

/** 服务器硬件指纹（授权绑定依据） */
export function getLicenseHardware(): Promise<ApiResponse<HardwareInfo>> {
  return http.get('/license/hardware').then((r) => r.data)
}

/** 当前授权状态 */
export function getLicenseStatus(): Promise<ApiResponse<LicenseStatus>> {
  return http.get('/license/status').then((r) => r.data)
}

/** 校验授权码 / 授权文件内容（不落库） */
export function verifyLicense(payload: { code?: string; file_content?: string }): Promise<ApiResponse<VerifyResult>> {
  return http.post('/license/verify', payload).then((r) => r.data)
}

/** 导入并激活授权（校验硬件绑定；授权码由 LicenseTool.exe 离线签发） */
export function activateLicense(payload: {
  code?: string
  file_content?: string
  customer?: string
  note?: string
}): Promise<ApiResponse<LicenseStatus>> {
  return http.post('/license/activate', payload).then((r) => r.data)
}

/** 解除当前授权 */
export function deactivateLicense(): Promise<ApiResponse<LicenseStatus>> {
  return http.post('/license/deactivate').then((r) => r.data)
}

/** 授权导入记录（等保三级留痕）：平台不签发，仅记录导入激活历史 */
export function listLicenseRecords(limit = 50): Promise<ApiResponse<{ total: number; list: LicenseRecord[] }>> {
  return http.get('/license/records', { params: { limit } }).then((r) => r.data)
}

/** 导出授权文件（.lic）：返回 Blob */
export async function exportLicenseFile(kind: 'active' | number): Promise<{ blob: Blob; filename: string }> {
  const url = kind === 'active' ? '/license/export-active' : `/license/export/${kind}`
  const resp = await http.get(url, { responseType: 'blob' })
  const disposition: string = (resp.headers?.['content-disposition'] as string) || ''
  const m = /filename="([^"]+)"/.exec(disposition)
  return { blob: resp.data as Blob, filename: m ? m[1] : 'logaudit.lic' }
}
