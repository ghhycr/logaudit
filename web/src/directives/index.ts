/**
 * 安全指令注册（等保三级）
 *  - v-perm="['admin']"      按钮级权限（最小权限）
 *  - v-audit="'search_logs'" 操作审计埋点（点击即记录）
 *  - v-sanitize="html"       白名单 XSS 清洗后渲染
 */
import type { App, Directive } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { audit } from '@/utils/audit'
import { sanitizeHtml } from '@/utils/security'

const perm: Directive<HTMLElement, string[]> = {
  mounted(el, binding) {
    const auth = useAuthStore()
    const roles = binding.value || []
    const ok = auth.role === 'admin' || (auth.role != null && roles.includes(auth.role))
    if (!ok) {
      el.parentNode?.removeChild(el)
    }
  }
}

const auditDirective: Directive<HTMLElement, string> = {
  mounted(el, binding) {
    const action = binding.value || 'user_action'
    const target = binding.arg || el.getAttribute('data-target') || ''
    el.addEventListener('click', () => {
      audit(action as never, target)
    })
  },
  unmounted(el) {
    el.removeEventListener('click', () => {})
  }
}

const sanitize: Directive<HTMLElement, string> = {
  mounted(el, binding) {
    el.innerHTML = sanitizeHtml(binding.value || '')
  },
  updated(el, binding) {
    el.innerHTML = sanitizeHtml(binding.value || '')
  }
}

export function setupDirectives(app: App): void {
  app.directive('perm', perm)
  app.directive('audit', auditDirective)
  app.directive('sanitize', sanitize)
}
