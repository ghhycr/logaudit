<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessageBox, ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const menus = computed(() => {
  const base = [
    { path: '/dashboard', title: '总览仪表盘', icon: 'Odometer' },
    { path: '/logs', title: '日志检索', icon: 'Document' },
    { path: '/stats', title: '统计分析', icon: 'DataAnalysis' },
    { path: '/devices', title: '设备台账', icon: 'Monitor' }
  ]
  const alertGroup = {
    title: '告警配置', icon: 'Bell',
    children: [{ path: '/alerts/events', title: '告警事件' }]
  }
  if (auth.role === 'admin') {
    alertGroup.children.push({ path: '/alerts/rules', title: '告警规则' })
  }
  const settingGroup: { title: string; icon: string; children: { path: string; title: string }[] } = {
    title: '系统设置', icon: 'Setting',
    children: [
      { path: '/settings/users', title: '用户管理' },
      { path: '/settings/roles', title: '权限管理' },
      { path: '/settings/ntp', title: 'NTP服务器配置' },
      { path: '/retention', title: '留存与容量' }
    ]
  }
  return { base, alertGroup, settingGroup, isAdmin: auth.role === 'admin' }
})

async function onLogout(): Promise<void> {
  try {
    await ElMessageBox.confirm('确定退出登录吗？', '提示', { type: 'warning' })
  } catch {
    return
  }
  auth.logout('用户主动登出')
  ElMessage.success('已退出登录')
  void router.push('/login')
}
</script>

<template>
  <el-container class="layout">
    <el-aside width="210px" class="aside">
      <div class="brand">
        <div class="brand-logo">安审</div>
        <div class="brand-name">日志审计平台</div>
      </div>
      <el-menu
        :default-active="route.path"
        :default-openeds="['alerts-group', 'settings-group']"
        router
        background-color="transparent"
        text-color="#1a1b1c"
        active-text-color="#1d7872"
        class="menu"
      >
        <el-menu-item v-for="m in menus.base" :key="m.path" :index="m.path">
          <el-icon><component :is="m.icon" /></el-icon>
          <span>{{ m.title }}</span>
        </el-menu-item>

        <!-- 告警配置：二级子模块（告警事件 / 告警规则） -->
        <el-sub-menu index="alerts-group">
          <template #title>
            <el-icon><component :is="menus.alertGroup.icon" /></el-icon>
            <span>{{ menus.alertGroup.title }}</span>
          </template>
          <el-menu-item v-for="c in menus.alertGroup.children" :key="c.path" :index="c.path">
            {{ c.title }}
          </el-menu-item>
        </el-sub-menu>

        <!-- 系统设置：用户管理 / 权限管理 / NTP 配置 / 留存与容量（仅 admin） -->
        <el-sub-menu v-if="menus.isAdmin" index="settings-group">
          <template #title>
            <el-icon><component :is="menus.settingGroup.icon" /></el-icon>
            <span>{{ menus.settingGroup.title }}</span>
          </template>
          <el-menu-item v-for="c in menus.settingGroup.children" :key="c.path" :index="c.path">
            {{ c.title }}
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="page-title">{{ route.meta.title || '' }}</div>
        <div class="header-right">
          <!-- 会话超时提示（等保三级：会话空闲自动失效） -->
          <el-tag v-if="auth.idleWarning" type="warning" effect="dark" size="small">
            即将因空闲超时自动登出，请操作以保持会话
          </el-tag>
          <el-dropdown @command="(c: string) => c === 'logout' && onLogout()">
            <span class="user">
              <el-icon><User /></el-icon>
              {{ auth.user?.display_name || auth.user?.username }}
              <el-tag size="small" style="margin-left:6px">{{ auth.role }}</el-tag>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.layout { height: 100%; }
.aside { background: #ffffff; border-right: 1px solid rgba(0,0,0,0.08); display: flex; flex-direction: column; }
.brand { display: flex; align-items: center; gap: 8px; padding: 16px 14px; }
.brand-logo { width: 34px; height: 34px; border-radius: 8px; background: linear-gradient(135deg,#1d7872,#94d4d0); color: #fff; display: flex; align-items: center; justify-content: center; font-weight: 700; font-size: 13px; }
.brand-name { font-weight: 600; font-size: 15px; color: #1a1b1c; }
.menu { border-right: none; padding: 0 8px; flex: 1; }
.header { background: #fff; border-bottom: 1px solid rgba(0,0,0,0.08); display: flex; align-items: center; justify-content: space-between; height: 56px; }
.page-title { font-size: 15px; font-weight: 600; color: #1a1b1c; }
.header-right { display: flex; align-items: center; gap: 12px; }
.user { display: inline-flex; align-items: center; gap: 4px; cursor: pointer; color: #1a1b1c; }
.main { padding: 16px; overflow: auto; }
</style>
