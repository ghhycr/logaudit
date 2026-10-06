<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessageBox, ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const menus = computed(() => {
  const all = [
    { path: '/dashboard', title: '总览仪表盘', icon: 'Odometer' },
    { path: '/logs', title: '日志检索', icon: 'Document' },
    { path: '/stats', title: '统计分析', icon: 'DataAnalysis' },
    { path: '/devices', title: '设备台账', icon: 'Monitor' },
    { path: '/alerts/events', title: '告警事件', icon: 'Bell' },
    { path: '/retention', title: '留存与容量', icon: 'Coin' }
  ]
  if (auth.role === 'admin') {
    all.splice(5, 0, { path: '/alerts/rules', title: '告警规则', icon: 'SetUp' })
    all.push({ path: '/settings', title: '系统设置', icon: 'Setting' })
  }
  return all
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
        router
        background-color="transparent"
        text-color="#1a1b1c"
        active-text-color="#1d7872"
        class="menu"
      >
        <el-menu-item v-for="m in menus" :key="m.path" :index="m.path">
          <el-icon><component :is="m.icon" /></el-icon>
          <span>{{ m.title }}</span>
        </el-menu-item>
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
