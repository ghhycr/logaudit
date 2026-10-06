/**
 * 等保三级 · 身份鉴别：密码复杂度校验
 * 策略（可配 .env）：长度 ≥8，须含大写/小写/数字/特殊字符，且满足附加规则
 */

export interface PasswordPolicy {
  minLength: number
  requireUpper: boolean
  requireLower: boolean
  requireDigit: boolean
  requireSpecial: boolean
  /** 附加：不得与用户名相同或包含用户名 */
  forbidUsername: boolean
  /** 附加：不得包含连续 3 位以上相同字符 */
  forbidRepeated: boolean
  /** 附加：不得为常见弱口令 */
  forbidCommon: boolean
}

export const DEFAULT_POLICY: PasswordPolicy = {
  minLength: Number(import.meta.env.VITE_PASSWORD_MIN_LENGTH || 8),
  requireUpper: Number(import.meta.env.VITE_PASSWORD_REQUIRE_UPPER || 1) === 1,
  requireLower: Number(import.meta.env.VITE_PASSWORD_REQUIRE_LOWER || 1) === 1,
  requireDigit: Number(import.meta.env.VITE_PASSWORD_REQUIRE_DIGIT || 1) === 1,
  requireSpecial: Number(import.meta.env.VITE_PASSWORD_REQUIRE_SPECIAL || 1) === 1,
  forbidUsername: true,
  forbidRepeated: true,
  forbidCommon: true
}

/** 常见弱口令（示例集合，可按需扩充） */
const COMMON_PASSWORDS = new Set([
  '123456', '12345678', '123456789', 'password', 'Password1',
  'qwerty', 'abc123', '111111', 'admin123', '123123',
  '000000', '654321', '666666', '888888', 'admin', 'root'
])

export interface PasswordCheckResult {
  ok: boolean
  score: number                 // 0-100 强度分
  level: 'weak' | 'medium' | 'strong'
  errors: string[]
}

/**
 * 校验密码是否符合等保三级策略
 * @param pwd 明文密码（仅在内存中使用，绝不写入日志）
 * @param username 用户名（用于"不得包含用户名"规则）
 */
export function checkPassword(pwd: string, username = '', policy: PasswordPolicy = DEFAULT_POLICY): PasswordCheckResult {
  const errors: string[] = []

  if (pwd.length < policy.minLength) {
    errors.push(`密码长度不得少于 ${policy.minLength} 位`)
  }
  if (policy.requireUpper && !/[A-Z]/.test(pwd)) {
    errors.push('必须包含大写字母')
  }
  if (policy.requireLower && !/[a-z]/.test(pwd)) {
    errors.push('必须包含小写字母')
  }
  if (policy.requireDigit && !/\d/.test(pwd)) {
    errors.push('必须包含数字')
  }
  if (policy.requireSpecial && !/[^A-Za-z0-9]/.test(pwd)) {
    errors.push('必须包含特殊字符（如 !@#$%^&*）')
  }
  if (policy.forbidUsername && username && pwd.toLowerCase().includes(username.toLowerCase())) {
    errors.push('密码不得包含用户名')
  }
  if (policy.forbidRepeated && /(.)\1{2,}/.test(pwd)) {
    errors.push('不得包含 3 个及以上连续相同字符')
  }
  if (policy.forbidCommon && COMMON_PASSWORDS.has(pwd.toLowerCase())) {
    errors.push('密码为常见弱口令，请更换')
  }

  // 强度评分
  let score = 0
  score += Math.min(pwd.length, 16) * 3
  if (/[A-Z]/.test(pwd)) score += 10
  if (/[a-z]/.test(pwd)) score += 10
  if (/\d/.test(pwd)) score += 10
  if (/[^A-Za-z0-9]/.test(pwd)) score += 15
  if (pwd.length >= 12) score += 10
  if (errors.length > 0) score = Math.min(score, 55)
  score = Math.min(100, score)

  const level: PasswordCheckResult['level'] = score >= 80 ? 'strong' : score >= 50 ? 'medium' : 'weak'

  return { ok: errors.length === 0, score, level, errors }
}

/** 隐藏敏感串（用于日志脱敏） */
export function maskSecret(v: string): string {
  if (!v) return ''
  if (v.length <= 4) return '****'
  return v.slice(0, 2) + '****' + v.slice(-2)
}
