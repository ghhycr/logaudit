<script setup lang="ts">
/**
 * 设备台账：设备 CRUD / 启停（等保三级：仅 admin 可写，v-perm 最小权限，操作留痕 v-audit）
 * 数据源：GET/POST/PUT/DELETE /api/v1/devices
 */
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { audit } from '@/utils/audit'
import { listDevices, createDevice, updateDevice, deleteDevice } from '@/api/devices'
import type { DeviceItem } from '@/api/devices'

const loading = ref(false)
const devices = ref<DeviceItem[]>([])
const kinds = ['网络设备', '服务器', '安全设备', '数据库', '应用系统', '其他']

const dialogVisible = ref(false)
const editing = ref<DeviceItem | null>(null)
const form = reactive({
  name: '', ip: '', vendor: '', model: '', kind: '服务器', location: ''
})

const rules = {
  name: [{ required: true, message: '设备名称必填', trigger: 'blur' }],
  ip: [
    { required: true, message: 'IP 必填', trigger: 'blur' },
    { pattern: /^(\d{1,3}\.){3}\d{1,3}$/, message: 'IP 格式不正确', trigger: 'blur' }
  ]
}
const formRef = ref()

async function doLoad(): Promise<void> {
  loading.value = true
  try {
    const resp = await listDevices()
    devices.value = resp.data || []
  } catch {
    ElMessage.error('设备列表加载失败')
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editing.value = null
  Object.assign(form, { name: '', ip: '', vendor: '', model: '', kind: '服务器', location: '' })
  dialogVisible.value = true
}

function openEdit(row: DeviceItem): void {
  editing.value = row
  Object.assign(form, {
    name: row.name, ip: row.ip, vendor: row.vendor || '', model: row.model || '',
    kind: row.kind || '服务器', location: row.location || ''
  })
  dialogVisible.value = true
}

async function onSubmit(): Promise<void> {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  try {
    if (editing.value) {
      await updateDevice(editing.value.id, { ...form })
      audit('update_device', form.ip, `修改设备 ${form.name}`)
      ElMessage.success('设备已更新')
    } else {
      await createDevice({ ...form })
      audit('create_device', form.ip, `新增设备 ${form.name}`)
      ElMessage.success('设备已新增')
    }
    dialogVisible.value = false
    void doLoad()
  } catch {
    ElMessage.error('保存失败')
  }
}

async function onToggle(row: DeviceItem): Promise<void> {
  try {
    await ElMessageBox.confirm(
      row.enabled ? `停用设备「${row.name}」？停用后该设备不再出现在列表` : `重新启用设备「${row.name}」？`,
      row.enabled ? '停用确认' : '启用确认',
      { type: row.enabled ? 'warning' : 'info', confirmButtonText: '确定', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  try {
    if (row.enabled) {
      await deleteDevice(row.id) // 软删 enabled=0
      audit('delete_device', String(row.id), `停用设备 ${row.name}`)
      ElMessage.success('设备已停用')
    } else {
      await updateDevice(row.id, { ...row, enabled: true })
      audit('enable_device', row.ip, `启用设备 ${row.name}`)
      ElMessage.success('设备已启用')
    }
    void doLoad()
  } catch {
    ElMessage.error('操作失败')
  }
}

onMounted(doLoad)
</script>

<template>
  <div v-loading="loading">
    <div class="panel-head">
      <div>
        <h2>设备台账</h2>
        <p class="sub">已接入 {{ devices.length }} 台设备 · 新增设备后需在设备侧配置日志转发（rsyslog 514 / Filebeat 5044）</p>
      </div>
      <el-button type="primary" size="small" v-perm="['admin']" v-audit="'create_device'" @click="openCreate">新增设备</el-button>
    </div>

    <div class="panel">
      <el-table :data="devices" stripe size="small">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" label="设备名" min-width="130" />
        <el-table-column prop="ip" label="IP" width="130" />
        <el-table-column prop="vendor" label="厂商" width="110" show-overflow-tooltip />
        <el-table-column prop="model" label="型号" width="130" show-overflow-tooltip />
        <el-table-column prop="kind" label="类型" width="100" />
        <el-table-column prop="location" label="位置" min-width="110" show-overflow-tooltip />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button v-perm="['admin']" link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button v-perm="['admin']" link :type="row.enabled ? 'danger' : 'success'" size="small" @click="onToggle(row)">
              {{ row.enabled ? '停用' : '启用' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!devices.length" class="empty">暂无设备，点击右上角「新增设备」登记</div>
    </div>

    <el-dialog v-model="dialogVisible" :title="editing ? `编辑设备 ${editing.name}` : '新增设备'" width="480px" append-to-body>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="70px" size="small">
        <el-form-item label="设备名称" prop="name">
          <el-input v-model="form.name" placeholder="如：核心交换机-01" />
        </el-form-item>
        <el-form-item label="IP 地址" prop="ip">
          <el-input v-model="form.ip" placeholder="如：192.168.143.131" />
        </el-form-item>
        <el-form-item label="厂商">
          <el-input v-model="form.vendor" placeholder="如：Huawei / Dell / 奇安信" />
        </el-form-item>
        <el-form-item label="型号">
          <el-input v-model="form.model" placeholder="如：S5720 / R740 / NGFW4000" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.kind" style="width: 100%">
            <el-option v-for="k in kinds" :key="k" :label="k" :value="k" />
          </el-select>
        </el-form-item>
        <el-form-item label="位置">
          <el-input v-model="form.location" placeholder="如：机房A-03 机柜" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button size="small" @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" size="small" @click="onSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 12px; }
.panel-head h2 { margin: 0; font-size: 18px; }
.sub { margin: 4px 0 0; color: #6B7280; font-size: 12px; }
.panel { background: #fff; border-radius: 10px; padding: 14px 16px; border: 1px solid rgba(0,0,0,0.06); }
.empty { padding: 24px 0; text-align: center; color: #9aa0a6; font-size: 12px; }
</style>
