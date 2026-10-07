<script setup lang="ts">
/**
 * 系统设置 · 授权管理（Java 授权服务）
 *  - 授权码统一由本地离线工具 LicenseTool.exe 签发（签名密钥与服务端 LICENSE_SECRET 一致）
 *  - 服务器硬件指纹：授权绑定依据，复制后填入 LicenseTool 签发
 *  - 授权状态：当前生效授权的授权类型 / 到期时间 / 剩余天数 / 硬件绑定校验
 *  - 导入授权：粘贴授权码或上传 .lic 授权文件 → 校验 → 激活（绑定本机硬件）
 *  - 授权记录：授权导入留痕（等保三级），可随时重新导出 .lic 授权文件
 */
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { audit } from '@/utils/audit'
import {
  activateLicense,
  deactivateLicense,
  exportLicenseFile,
  getLicenseHardware,
  getLicenseStatus,
  listLicenseRecords,
  verifyLicense,
  type HardwareInfo,
  type LicenseRecord,
  type LicenseStatus,
  type VerifyResult
} from '@/api/license'

const loading = ref(false)
const hardware = ref<HardwareInfo>({ hardware_id: '', details: {} })
const status = ref<LicenseStatus>({ activated: false, status: 'none', status_text: '未授权', hardware_id: '' })

// ---------- 导入授权 ----------
const importForm = reactive({ code: '', customer: '', note: '' })
const verifyResult = ref<VerifyResult | null>(null)
const importLoading = ref(false)

// ---------- 授权记录 ----------
const records = ref<LicenseRecord[]>([])
const recordsLoading = ref(false)

const statusTagType = (s: string): 'success' | 'warning' | 'danger' | 'info' => {
  if (s === 'permanent' || s === 'active') return 'success'
  if (s === 'mismatch' || s === 'invalid') return 'danger'
  if (s === 'expired') return 'warning'
  return 'info'
}

async function loadAll(): Promise<void> {
  loading.value = true
  try {
    const hw = await getLicenseHardware()
    hardware.value = hw.data
  } catch {
    ElMessage.error('硬件指纹获取失败')
  }
  try {
    const st = await getLicenseStatus()
    status.value = st.data
  } catch {
    ElMessage.error('授权状态获取失败')
  } finally {
    loading.value = false
  }
  loadRecords()
}

async function loadRecords(): Promise<void> {
  recordsLoading.value = true
  try {
    const r = await listLicenseRecords(50)
    records.value = r.data.list || []
  } catch {
    records.value = []
  } finally {
    recordsLoading.value = false
  }
}

async function copyText(text: string, tip = '已复制到剪贴板'): Promise<void> {
  if (!text) {
    ElMessage.warning('暂无可复制内容')
    return
  }
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
  ElMessage.success(tip)
}

async function downloadLic(kind: 'active' | number): Promise<void> {
  try {
    const { blob, filename } = await exportLicenseFile(kind)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    ElMessage.success(`授权文件已导出：${filename}`)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '授权文件导出失败')
  }
}

async function onPickFile(file: any): Promise<void> {
  try {
    const text: string = await file.raw.text()
    importForm.code = text
    ElMessage.success(`已读取授权文件：${file.name}`)
  } catch {
    ElMessage.error('授权文件读取失败')
  }
}

async function onVerify(): Promise<void> {
  if (!importForm.code.trim()) {
    ElMessage.warning('请粘贴授权码或选择 .lic 授权文件')
    return
  }
  importLoading.value = true
  try {
    const r = await verifyLicense({ file_content: importForm.code })
    verifyResult.value = r.data
    ElMessage[r.data.matched ? 'success' : 'warning'](r.data.message)
  } catch (e: any) {
    verifyResult.value = null
    ElMessage.error(e?.response?.data?.message || '授权码校验失败')
  } finally {
    importLoading.value = false
  }
}

async function onActivate(): Promise<void> {
  if (!importForm.code.trim()) {
    ElMessage.warning('请粘贴授权码或选择 .lic 授权文件')
    return
  }
  importLoading.value = true
  try {
    const r = await activateLicense({
      file_content: importForm.code,
      customer: importForm.customer,
      note: importForm.note
    })
    status.value = r.data
    audit('system_config', 'license', `导入并激活授权（${r.data.status_text}）`)
    ElMessage.success('授权导入成功，已在本机激活')
    verifyResult.value = null
    importForm.code = ''
    loadRecords()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '授权激活失败')
  } finally {
    importLoading.value = false
  }
}

async function onDeactivate(): Promise<void> {
  try {
    await ElMessageBox.confirm('确定解除当前授权吗？解除后本机将回到未授权状态。', '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    const r = await deactivateLicense()
    status.value = r.data
    audit('system_config', 'license', '解除本机授权')
    ElMessage.success('已解除授权')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '解除授权失败')
  }
}

const detailLabel: Record<string, string> = {
  hostname: '主机名',
  machine_id: 'Machine ID',
  product_uuid: '主板 UUID',
  product_name: '产品型号',
  board_serial: '主板序列号',
  bios_version: 'BIOS 版本',
  mac_addresses: '网卡 MAC',
  cpu_model: 'CPU 型号',
  cpu_cores: 'CPU 核数'
}

onMounted(loadAll)
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div class="page-title">授权管理</div>
      <div class="page-sub">
        授权码由本地离线工具 LicenseTool.exe 签发（HMAC-SHA256 签名、绑定服务器硬件指纹、防跨机复制）。
        本平台不提供在线签发，仅负责硬件指纹展示、授权码 / .lic 授权文件校验、导入激活与导出留痕。
      </div>
    </div>

    <el-row :gutter="14">
      <!-- 硬件指纹 -->
      <el-col :span="12">
        <el-card v-loading="loading" shadow="never" class="card">
          <template #header>
            <div class="card-head">
              <span>服务器硬件指纹</span>
              <el-button size="small" v-audit="'system_config'" @click="copyText(hardware.hardware_id, '硬件指纹已复制')">
                复制指纹
              </el-button>
            </div>
          </template>
          <div class="fp">{{ hardware.hardware_id || '获取中…' }}</div>
          <el-descriptions :column="1" size="small" border class="desc">
            <el-descriptions-item v-for="(v, k) in hardware.details" :key="k" :label="detailLabel[k] || k">
              <span class="mono">{{ v || '-' }}</span>
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>

      <!-- 授权状态 -->
      <el-col :span="12">
        <el-card v-loading="loading" shadow="never" class="card">
          <template #header>
            <div class="card-head">
              <span>当前授权状态</span>
              <el-tag :type="statusTagType(status.status)" size="small">{{ status.status_text }}</el-tag>
            </div>
          </template>
          <el-descriptions :column="1" size="small" border class="desc">
            <el-descriptions-item label="产品名称">{{ status.product || 'LogAudit 日志审计平台' }}</el-descriptions-item>
            <el-descriptions-item label="授权类型">
              <span v-if="status.activated">{{ status.permanent ? '永久授权' : '限时授权' }}</span>
              <span v-else>-</span>
            </el-descriptions-item>
            <el-descriptions-item label="到期时间">{{ status.expire_date || '-' }}</el-descriptions-item>
            <el-descriptions-item label="剩余天数">
              <span v-if="status.activated && status.days_left === -1">永久有效</span>
              <span v-else-if="status.activated">{{ status.days_left }} 天</span>
              <span v-else>-</span>
            </el-descriptions-item>
            <el-descriptions-item label="绑定硬件">
              <span class="mono">{{ status.hardware_bound || '-' }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="硬件校验">
              <el-tag v-if="status.bound_matched" type="success" size="small">与本机一致</el-tag>
              <el-tag v-else-if="status.activated" type="danger" size="small">与本机不一致</el-tag>
              <span v-else>-</span>
            </el-descriptions-item>
            <el-descriptions-item label="激活人 / 时间">
              {{ status.activated_by ? `${status.activated_by} · ${status.activated_at}` : '-' }}
            </el-descriptions-item>
          </el-descriptions>
          <div class="ops">
            <el-button
              size="small"
              :disabled="!status.activated || !status.license_code"
              @click="copyText(status.license_code || '', '授权码已复制')"
            >
              复制授权码
            </el-button>
            <el-button size="small" type="primary" :disabled="!status.activated" @click="downloadLic('active')">
              导出授权文件
            </el-button>
            <el-button size="small" type="danger" plain :disabled="!status.activated" @click="onDeactivate">
              解除授权
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 离线签发指引 -->
    <el-card shadow="never" class="card mt">
      <template #header><span>授权码签发方式（LicenseTool.exe 离线签发）</span></template>
      <el-steps :active="4" finish-status="success" align-center class="steps">
        <el-step title="复制硬件指纹" description="点击上方「复制指纹」获取本机硬件指纹" />
        <el-step title="打开授权工具" description="运行 LicenseTool.exe，顶部填写签名密钥" />
        <el-step title="生成授权码" description="填入硬件指纹与授权天数（或永久），生成并导出 .lic" />
        <el-step title="回平台导入" description="在下方粘贴授权码或上传 .lic 文件，校验后激活" />
      </el-steps>
      <el-alert type="info" :closable="false" show-icon class="mt">
        <template #title>
          离线工具签名密钥必须与服务端 <span class="mono">LICENSE_SECRET</span> 一致（由
          <span class="mono">deploy/docker/.env</span> 注入，平台不展示密钥明文）
        </template>
        密钥不一致将提示「授权码签名校验失败」。部署时修改服务端 LICENSE_SECRET，LicenseTool.exe 顶部密钥需同步更换；
        请联系平台管理员获取签名密钥，勿将密钥提交至代码仓库或对外披露。
      </el-alert>
    </el-card>

    <!-- 导入授权 -->
    <el-card shadow="never" class="card mt">
      <template #header><span>导入授权码 / 授权文件</span></template>
      <div class="import-row">
        <el-input
          v-model="importForm.code"
          type="textarea"
          :rows="3"
          placeholder="粘贴 LicenseTool 生成的授权码（LGA2. 开头），或上传 .lic 授权文件；内容首行须为授权码"
        />
        <div class="import-ops">
          <el-upload :auto-upload="false" :show-file-list="false" accept=".lic,.txt" :on-change="onPickFile">
            <el-button size="small">选择 .lic 授权文件</el-button>
          </el-upload>
          <el-button size="small" :loading="importLoading" @click="onVerify">校验</el-button>
          <el-button size="small" type="primary" :loading="importLoading" v-audit="'system_config'" @click="onActivate">
            导入并激活
          </el-button>
        </div>
      </div>
      <div class="extra">
        <el-input v-model="importForm.customer" placeholder="客户名称（选填）" style="max-width: 240px" size="small" />
        <el-input v-model="importForm.note" placeholder="备注（选填，用于留痕）" style="max-width: 240px" size="small" />
      </div>
      <el-alert
        v-if="verifyResult"
        :type="verifyResult.matched ? 'success' : 'warning'"
        :closable="false"
        show-icon
        class="mt"
        :title="verifyResult.message"
      >
        <div class="vr">
          绑定硬件：<span class="mono">{{ verifyResult.hardware_id }}</span> ｜
          本机指纹：<span class="mono">{{ verifyResult.local_hardware_id }}</span> ｜
          类型：{{ verifyResult.permanent ? '永久授权' : '限时授权' }} ｜
          到期：{{ verifyResult.expire }} ｜
          剩余：{{ verifyResult.days_left === -1 ? '永久' : verifyResult.days_left + ' 天' }}
        </div>
      </el-alert>
    </el-card>

    <!-- 授权记录 -->
    <el-card shadow="never" class="card mt">
      <template #header>
        <div class="card-head">
          <span>授权导入记录</span>
          <span class="sub">授权导入留痕（等保三级），可重新导出 .lic 授权文件</span>
        </div>
      </template>
      <el-table v-loading="recordsLoading" :data="records" size="small" border>
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="hardware_id" label="绑定硬件" min-width="200" show-overflow-tooltip />
        <el-table-column label="类型" width="90">
          <template #default="{ row }">
            <el-tag :type="row.permanent ? 'success' : 'warning'" size="small">
              {{ row.permanent ? '永久' : row.days + '天' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="expire_date" label="到期时间" width="120" />
        <el-table-column prop="customer" label="客户" width="120" show-overflow-tooltip />
        <el-table-column prop="operator" label="操作人" width="100" />
        <el-table-column prop="issued_at" label="导入时间" width="160" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="downloadLic(row.id)">导出 .lic</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.page-head { margin-bottom: 14px; }
.page-title { font-size: 18px; font-weight: 600; color: #1a1b1c; }
.page-sub { font-size: 12px; color: #6b7280; margin-top: 4px; line-height: 1.6; }
.card { border-radius: 12px; }
.card-head { display: flex; align-items: center; justify-content: space-between; }
.sub { font-size: 12px; color: #9ca3af; font-weight: normal; }
.mt { margin-top: 14px; }
.fp { font-family: Consolas, Menlo, monospace; font-size: 15px; font-weight: 600; color: #1d7872; letter-spacing: 0.5px; margin-bottom: 10px; word-break: break-all; }
.desc { margin-top: 2px; }
.mono { font-family: Consolas, Menlo, monospace; font-size: 12px; }
.ops { margin-top: 12px; display: flex; gap: 8px; }
.steps { margin: 6px 0 2px; }
.import-row { display: flex; gap: 12px; align-items: flex-start; }
.import-ops { display: flex; flex-direction: column; gap: 8px; width: 170px; }
.extra { margin-top: 10px; display: flex; gap: 10px; flex-wrap: wrap; }
.vr { font-size: 12px; line-height: 1.8; word-break: break-all; }
</style>
