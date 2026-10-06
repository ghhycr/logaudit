<script setup lang="ts">
/**
 * 用户管理（系统设置）：用户列表 / 创建 / 角色与状态 / 重置密码 / 停用
 * 等保三级：仅 admin 可操作（v-perm），操作留痕（v-audit），口令 ≥8 位
 * 数据源：/api/v1/users（GET/POST/PUT/DELETE）+ reset-password
 */
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { audit } from '@/utils/audit'
import {
  listUsers, createUser, updateUser, deleteUser, resetPassword,
  ROLE_OPTIONS, roleLabel, type UserItem
} from '@/api/users'

const loading = ref(false)
const users = ref<UserItem[]>([])

const dialogVisible = ref(false)
const resetVisible = ref(false)
const form = reactive({ username: '', password: '', display_name: '', role: 'viewer' })
const resetForm = reactive({ username: '', password: '' })
const editing = ref<UserItem | null>(null)

const formRef = ref()
const rules = {
  username: [
    { required: true, message: '用户名必填', trigger: 'blur' },
    { min: 3, max: 64, message: '长度 3-64 字符', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '密码必填', trigger: 'blur' },
    { min: 8, message: '口令长度不得少于 8 位（等保三级）', trigger: 'blur' }
  ]
}

async function doLoad(): Promise<void> {
  loading.value = true
  try {
    const resp = await listUsers()
    users.value = resp.data || []
  } catch {
    ElMessage.error('用户列表加载失败')
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editing.value = null
  Object.assign(form, { username: '', password: '', display_name: '', role: 'viewer' })
  dialogVisible.value = true
}

async function onSubmit(): Promise<void> {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  try {
    const resp = await createUser({ ...form })
    audit('create_user', resp.data?.username || form.username, `创建用户 ${form.username}`)
    ElMessage.success('用户已创建')
    dialogVisible.value = false
    void doLoad()
  } catch {
    ElMessage.error('创建失败（用户名可能已存在）')
  }
}

async function onToggle(row: UserItem): Promise<void> {
  const next = row.status === 1 ? 0 : 1
  const action = next === 1 ? '启用' : '停用'
  try {
    await ElMessageBox.confirm(`确定${action}用户 ${row.username} 吗？`, '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await updateUser(row.id, { status: next })
    audit('update_user', row.username, `${action}用户 ${row.username}`)
    ElMessage.success(`已${action}`)
    void doLoad()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败')
  }
}

async function onChangeRole(row: UserItem, role: string): Promise<void> {
  try {
    await updateUser(row.id, { role })
    audit('update_user', row.username, `修改 ${row.username} 角色为 ${role}`)
    ElMessage.success('角色已更新')
    void doLoad()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '角色更新失败')
    void doLoad()
  }
}

function openReset(row: UserItem): void {
  Object.assign(resetForm, { username: row.username, password: '' })
  resetVisible.value = true
}

async function onReset(): Promise<void> {
  const target = users.value.find((u) => u.username === resetForm.username)
  if (!target) return
  try {
    const resp = await resetPassword(target.id, resetForm.password || undefined)
    audit('reset_password', target.username, `重置用户 ${target.username} 的密码`)
    ElMessage.success(`密码已重置：${resp.data?.password}`)
    resetVisible.value = false
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '密码重置失败')
  }
}

async function onDelete(row: UserItem): Promise<void> {
  try {
    await ElMessageBox.confirm(`确定停用并删除用户 ${row.username} 吗？此操作可审计追溯。`, '危险操作', {
      type: 'warning', confirmButtonText: '停用', confirmButtonClass: 'el-button--danger'
    })
  } catch {
    return
  }
  try {
    await deleteUser(row.id)
    audit('delete_user', row.username, `停用用户 ${row.username}`)
    ElMessage.success('用户已停用')
    void doLoad()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败')
  }
}

onMounted(doLoad)
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <div class="page-title">用户管理</div>
        <div class="page-sub">账号、角色与口令管理（等保三级：最小权限、操作留痕）</div>
      </div>
      <el-button type="primary" @click="openCreate" v-perm="'admin'">新建用户</el-button>
    </div>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="users" stripe>
        <el-table-column prop="username" label="用户名" width="150" />
        <el-table-column prop="display_name" label="显示名" min-width="120">
          <template #default="{ row }">{{ row.display_name || '-' }}</template>
        </el-table-column>
        <el-table-column label="角色" width="150">
          <template #default="{ row }">
            <el-select :model-value="row.role" size="small" @change="(v: string) => onChangeRole(row, v)">
              <el-option v-for="r in ROLE_OPTIONS" :key="r.value" :label="r.label" :value="r.value" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
              {{ row.status === 1 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="fail_count" label="失败次数" width="80" />
        <el-table-column label="锁定" width="70">
          <template #default="{ row }">
            <el-tag v-if="row.locked" type="danger" size="small">锁定</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="last_login_at" label="最后登录" width="160">
          <template #default="{ row }">{{ row.last_login_at || '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160" />
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="openReset(row)">重置密码</el-button>
            <el-button link :type="row.status === 1 ? 'warning' : 'success'" size="small" @click="onToggle(row)">
              {{ row.status === 1 ? '停用' : '启用' }}
            </el-button>
            <el-button link type="danger" size="small" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新建用户 -->
    <el-dialog v-model="dialogVisible" title="新建用户" width="480px">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="90px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="3-64 字符，字母/数字/下划线" />
        </el-form-item>
        <el-form-item label="显示名">
          <el-input v-model="form.display_name" placeholder="如：张三 / 日志管理员" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role" style="width:100%">
            <el-option v-for="r in ROLE_OPTIONS" :key="r.value" :label="r.label" :value="r.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="初始密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="至少 8 位（等保三级口令策略）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmit">创建</el-button>
      </template>
    </el-dialog>

    <!-- 重置密码 -->
    <el-dialog v-model="resetVisible" title="重置密码" width="440px">
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:14px">
        用户：<b>{{ resetForm.username }}</b>。留空将由系统生成强口令（大小写+数字+符号）。
      </el-alert>
      <el-form label-width="90px" @submit.prevent>
        <el-form-item label="新密码">
          <el-input v-model="resetForm.password" type="password" show-password placeholder="留空则随机生成" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetVisible = false">取消</el-button>
        <el-button type="primary" @click="onReset">确认重置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
.page-title { font-size: 18px; font-weight: 600; color: #1a1b1c; }
.page-sub { font-size: 12px; color: #6b7280; margin-top: 4px; }
.table-card { border-radius: 12px; }
</style>
