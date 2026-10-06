<script setup lang="ts">
/**
 * 权限管理（系统设置）：角色权限矩阵 + 按角色用户分布（等保三级：RBAC）
 * 数据源：/api/v1/users（角色字段），权限矩阵为平台既定策略说明
 */
import { computed, onMounted, ref } from 'vue'
import { listUsers, roleLabel, type UserItem } from '@/api/users'

const users = ref<UserItem[]>([])

const matrix = [
  { module: '总览仪表盘 / 日志检索 / 统计分析', admin: '✔', viewer: '✔', auditor: '✔' },
  { module: '设备台账（查看）', admin: '✔', viewer: '✔', auditor: '✔' },
  { module: '设备台账（新增/修改/停用）', admin: '✔', viewer: '—', auditor: '—' },
  { module: '告警事件（查看/确认）', admin: '✔', viewer: '✔', auditor: '✔' },
  { module: '告警规则（配置/启停）', admin: '✔', viewer: '—', auditor: '—' },
  { module: '留存与容量（查看）', admin: '✔', viewer: '✔', auditor: '—' },
  { module: '用户管理 / 权限管理 / NTP 配置', admin: '✔', viewer: '—', auditor: '—' },
  { module: '操作审计留痕（v-audit）', admin: '✔', viewer: '✔', auditor: '✔' }
]

const roleCount = computed(() => ({
  admin: users.value.filter((u) => u.role === 'admin').length,
  viewer: users.value.filter((u) => u.role === 'viewer').length,
  auditor: users.value.filter((u) => u.role === 'auditor').length
}))

const roleDesc = [
  { role: 'admin', label: '管理员', desc: '系统全量权限：配置告警规则、管理设备与用户、系统设置、查看与导出日志' },
  { role: 'viewer', label: '只读审计员', desc: '查看日志、统计、告警事件与设备台账，可确认告警，不可进行写操作' },
  { role: 'auditor', label: '审计操作员', desc: '查看类权限 + 留存与容量查看；默认不开放写操作（可依制度调整）' }
]

onMounted(async () => {
  try {
    const resp = await listUsers()
    users.value = resp.data || []
  } catch {
    /* 非 admin 无权限查看时静默 */
  }
})
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div class="page-title">权限管理</div>
      <div class="page-sub">基于角色的访问控制（RBAC）· 等保三级访问控制要求</div>
    </div>

    <div class="role-cards">
      <div v-for="r in roleDesc" :key="r.role" class="role-card">
        <div class="role-name">
          {{ r.label }}
          <el-tag size="small" :type="r.role === 'admin' ? 'danger' : 'info'">{{ r.role }}</el-tag>
        </div>
        <div class="role-count">{{ roleCount[r.role as keyof typeof roleCount] || 0 }} 个账号</div>
        <div class="role-desc">{{ r.desc }}</div>
      </div>
    </div>

    <el-card shadow="never" class="table-card">
      <template #header><b>功能权限矩阵</b></template>
      <el-table :data="matrix" stripe>
        <el-table-column prop="module" label="功能模块" min-width="220" />
        <el-table-column prop="admin" label="管理员 (admin)" width="120" align="center" />
        <el-table-column prop="viewer" label="只读审计员 (viewer)" width="150" align="center" />
        <el-table-column prop="auditor" label="审计操作员 (auditor)" width="160" align="center" />
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.page-head { margin-bottom: 14px; }
.page-title { font-size: 18px; font-weight: 600; color: #1a1b1c; }
.page-sub { font-size: 12px; color: #6b7280; margin-top: 4px; }
.role-cards { display: flex; gap: 14px; flex-wrap: wrap; margin-bottom: 14px; }
.role-card {
  flex: 1 1 240px; padding: 14px; border-radius: 12px;
  background: #fff; border: 1px solid rgba(0,0,0,0.08);
}
.role-name { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 15px; color: #1a1b1c; }
.role-count { font-size: 22px; font-weight: 700; color: #1d7872; margin: 8px 0 6px; }
.role-desc { font-size: 12px; color: #6b7280; line-height: 1.6; }
.table-card { border-radius: 12px; }
</style>
