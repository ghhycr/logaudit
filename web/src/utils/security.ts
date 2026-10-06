/**
 * 等保三级 · 入侵防范：XSS 清洗 + 输入校验工具
 *
 * - sanitizeHtml：白名单式 HTML 清洗（禁止 script/style/事件属性/危险链接）
 * - escapeHtml：普通转义（默认展示场景）
 * - 输入校验：IP/主机名/时间范围/关键词长度限制
 */

/** 白名单标签（允许保留格式的标签；其余一律剥离） */
const ALLOWED_TAGS = new Set([
  'b', 'strong', 'i', 'em', 'u', 'code', 'pre', 'p', 'br',
  'ul', 'ol', 'li', 'blockquote', 'span', 'div', 'a', 'table', 'thead', 'tbody', 'tr', 'td', 'th'
])

/** 危险属性（事件处理器与协议注入） */
const DANGEROUS_ATTR = /^on/i
const DANGEROUS_PROTO = /^\s*(javascript|vbscript|data):/i

const div = typeof document !== 'undefined' ? document.createElement('div') : null

/**
 * 白名单式 HTML 清洗（v-sanitize 指令底层）
 * 移除 script/style/事件属性/javascript: 链接，保留白名单标签
 */
export function sanitizeHtml(input: string): string {
  if (!input) return ''
  if (!div) return escapeHtml(input)          // SSR/非浏览器环境降级为转义
  div.innerHTML = input
  const walker = (el: Element): void => {
    // 移除不在白名单的标签
    if (!ALLOWED_TAGS.has(el.tagName.toLowerCase())) {
      el.replaceWith(...Array.from(el.childNodes))
      return
    }
    // 清理危险属性
    for (const attr of Array.from(el.attributes)) {
      const name = attr.name.toLowerCase()
      if (DANGEROUS_ATTR.test(name) || DANGEROUS_PROTO.test(attr.value)) {
        el.removeAttribute(attr.name)
      }
    }
    // 限制 <a href> 协议
    if (el.tagName.toLowerCase() === 'a') {
      const href = el.getAttribute('href')
      if (href && DANGEROUS_PROTO.test(href)) {
        el.removeAttribute('href')
      }
    }
    Array.from(el.children).forEach(walker)
  }
  Array.from(div.children).forEach(walker)
  return div.innerHTML
}

/** HTML 转义（默认文本展示） */
export function escapeHtml(input: string): string {
  return String(input ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

/** 是否合法 IPv4 */
export function isValidIPv4(ip: string): boolean {
  const parts = ip.split('.')
  if (parts.length !== 4) return false
  return parts.every((p) => {
    if (!/^\d{1,3}$/.test(p)) return false
    const n = Number(p)
    return n >= 0 && n <= 255
  })
}

/** 是否合法主机名（设备名） */
export function isValidHostname(name: string): boolean {
  return /^[A-Za-z0-9]([A-Za-z0-9._-]{0,62}[A-Za-z0-9])?$/.test(name)
}

/** 关键词长度限制（防超长查询注入/性能攻击） */
export function clampKeyword(kw: string, max = 64): string {
  return (kw || '').slice(0, max)
}
