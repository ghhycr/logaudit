/**
 * 路由与守卫（等保三级 · 访问控制）
 *  - 未登录访问受保护页面 → 跳登录
 *  - 角色权限：路由 meta.roles 声明可访问角色，admin 全量
 *  - 404 / 403 兜底
 */
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { audit } from '@/utils/audit'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    /** 允许访问的角色；缺省=登录即可 */
    roles?: Array<'admin' | 'viewer' | 'auditor'>
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/login/LoginView.vue'),
    meta: { title: '登录' }
  },
  {
    path: '/',
    component: () => import('@/components/layout/AppLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'dashboard',
        component: () => import('@/views/dashboard/DashboardView.vue'),
        meta: { title: '总览仪表盘' }
      },
      {
        path: 'logs',
        name: 'logs',
        component: () => import('@/views/logs/LogSearchView.vue'),
        meta: { title: '日志检索' }
      },
      {
        path: 'stats',
        name: 'stats',
        component: () => import('@/views/stats/StatsView.vue'),
        meta: { title: '统计分析' }
      },
      {
        path: 'devices',
        name: 'devices',
        component: () => import('@/views/devices/DevicesView.vue'),
        meta: { title: '设备台账' }
      },
      {
        path: 'alerts/events',
        name: 'alert-events',
        component: () => import('@/views/alerts/AlertEventsView.vue'),
        meta: { title: '告警事件' }
      },
      {
        path: 'alerts/rules',
        name: 'alert-rules',
        component: () => import('@/views/alerts/AlertRulesView.vue'),
        meta: { title: '告警规则', roles: ['admin'] }
      },
      {
        path: 'retention',
        name: 'retention',
        component: () => import('@/views/retention/RetentionView.vue'),
        meta: { title: '留存与容量' }
      },
      {
        path: 'settings',
        name: 'settings',
        component: () => import('@/views/settings/SettingsView.vue'),
        meta: { title: '系统设置', roles: ['admin'] }
      }
    ]
  },
  { path: '/403', name: 'forbidden', component: () => import('@/views/error/Error403.vue'), meta: { title: '无权访问' } },
  { path: '/:pathMatch(.*)*', name: 'notfound', component: () => import('@/views/error/Error404.vue'), meta: { title: '页面不存在' } }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // 页面标题
  document.title = to.meta.title ? `${to.meta.title} - 日志审计平台` : '日志审计平台'

  // 登录页：已登录则回首页
  if (to.name === 'login') {
    return auth.isLoggedIn ? { name: 'dashboard' } : true
  }

  // 未登录：记录来源，跳登录
  if (!auth.isLoggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  // 角色权限校验
  const allowed = to.meta.roles
  if (allowed && auth.role && !allowed.includes(auth.role)) {
    audit('system_config', to.fullPath, '403 越权访问拦截')
    return { name: 'forbidden' }
  }

  return true
})

export default router
