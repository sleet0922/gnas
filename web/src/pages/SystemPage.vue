<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { apiGet, apiPost, type SystemInfo } from '@/composables/useApi'

const info = ref<SystemInfo | null>(null)
let timer: ReturnType<typeof setInterval>

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) { size /= 1024; i++ }
  return size.toFixed(i === 0 ? 0 : 1) + ' ' + units[i]
}

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (d > 0) return `${d}天 ${h}小时 ${m}分钟`
  if (h > 0) return `${h}小时 ${m}分钟 ${s}秒`
  if (m > 0) return `${m}分钟 ${s}秒`
  return `${s}秒`
}

const memPercent = computed(() => {
  if (!info.value || info.value.memoryTotal === 0) return 0
  return Math.round((info.value.memoryUsed / info.value.memoryTotal) * 100)
})

const procMemPercent = computed(() => {
  if (!info.value || info.value.memoryTotal === 0) return 0
  return ((info.value.procMem / info.value.memoryTotal) * 100).toFixed(2)
})

const cpuPercent = computed(() => {
  if (!info.value) return 0
  return info.value.cpuUsage.toFixed(1)
})

const procCPUPercent = computed(() => {
  if (!info.value) return 0
  return info.value.procCPU.toFixed(1)
})

function serviceStatusLabel(status: string): string {
  switch (status) {
    case 'ready': return '已就绪'
    case 'loading': return '加载中'
    case 'disabled': return '已关闭'
    default: return '不可用'
  }
}

function serviceStatusColor(status: string): string {
  switch (status) {
    case 'ready': return 'success'
    case 'loading': return 'warning'
    case 'disabled': return 'grey'
    default: return 'error'
  }
}

const aiEnabled = ref(false)
const updatingSettings = ref(false)
const sslEnabled = ref(false)
const sslCertFile = ref('/ssl/1.pem')
const sslKeyFile = ref('/ssl/1.key')
const updatingSSL = ref(false)

async function loadSettings() {
  const data = await apiGet<{
    ai_enabled: boolean
    ssl_enabled: boolean
    ssl_cert_file: string
    ssl_key_file: string
  }>('/api/settings')
  if (data) {
    aiEnabled.value = data.ai_enabled
    sslEnabled.value = data.ssl_enabled
    sslCertFile.value = data.ssl_cert_file
    sslKeyFile.value = data.ssl_key_file
  }
}

async function toggleAISettings() {
  updatingSettings.value = true
  try {
    await apiPost('/api/settings/update', { ai_enabled: aiEnabled.value })
  } catch (e: any) {
    alert(e.message || '更新设置失败')
    // 失败时回滚状态
    aiEnabled.value = !aiEnabled.value
  } finally {
    updatingSettings.value = false
  }
}

async function saveSSLSettings() {
  updatingSSL.value = true
  try {
    const data = await apiPost<{
      restart_required: boolean
      restart_scheduled: boolean
    }>('/api/settings/update', {
      ssl_enabled: sslEnabled.value,
      ssl_cert_file: sslCertFile.value,
      ssl_key_file: sslKeyFile.value,
    })
    if (data?.restart_required) {
      const protocol = sslEnabled.value ? 'https:' : 'http:'
      if (data.restart_scheduled) {
        alert('SSL 设置已保存，服务正在重启，即将切换访问协议。')
        window.setTimeout(() => {
          const target = new URL(window.location.href)
          target.protocol = protocol
          window.location.assign(target.toString())
        }, 1800)
      } else {
        alert('SSL 设置已保存，请重启 gnas 服务后使用新的访问协议。')
      }
    } else {
      alert('SSL 设置已保存')
    }
  } catch (e: any) {
    alert(e.message || '更新 SSL 设置失败')
  } finally {
    updatingSSL.value = false
  }
}

async function loadInfo() {
  const data = await apiGet<SystemInfo>('/api/system')
  if (data) info.value = data
}

onMounted(() => {
  loadInfo()
  loadSettings()
  timer = setInterval(loadInfo, 3000)
})

onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div v-if="info">
    <h1 class="text-h5 font-weight-bold mb-6">占用</h1>

    <!-- 概览卡片 -->
    <v-row class="mb-4">
      <v-col cols="6" sm="4" md="3">
        <v-card color="surface-variant" class="pa-4 text-center">
          <v-icon color="primary" size="28" class="mb-2">mdi-desktop-classic</v-icon>
          <div class="text-caption text-on-surface-variant">系统</div>
          <div class="text-body-2 font-weight-medium">{{ info.os }} / {{ info.arch }}</div>
        </v-card>
      </v-col>
      <v-col cols="6" sm="4" md="3">
        <v-card color="surface-variant" class="pa-4 text-center">
          <v-icon color="primary" size="28" class="mb-2">mdi-clock-outline</v-icon>
          <div class="text-caption text-on-surface-variant">运行时间</div>
          <div class="text-body-2 font-weight-medium">{{ formatUptime(info.uptime) }}</div>
        </v-card>
      </v-col>
      <v-col cols="6" sm="4" md="3">
        <v-card color="surface-variant" class="pa-4 text-center">
          <v-icon color="primary" size="28" class="mb-2">mdi-database-outline</v-icon>
          <div class="text-caption text-on-surface-variant">数据库</div>
          <div class="text-body-2 font-weight-medium">{{ info.dbSizeString || formatBytes(info.dbSize) }}</div>
        </v-card>
      </v-col>
      <v-col cols="6" sm="4" md="3">
        <v-card color="surface-variant" class="pa-4 text-center">
          <v-icon color="primary" size="28" class="mb-2">mdi-chip</v-icon>
          <div class="text-caption text-on-surface-variant">CPU 核心</div>
          <div class="text-body-2 font-weight-medium">{{ info.cpuCores }} 核</div>
        </v-card>
      </v-col>
    </v-row>

    <!-- AI 设置 -->
    <v-card color="surface-variant" class="pa-5 mb-4">
      <div class="d-flex align-center justify-space-between flex-wrap ga-3">
        <div style="flex: 1; min-width: 250px;">
          <div class="d-flex align-center ga-3 mb-1">
            <v-icon color="primary">mdi-brain</v-icon>
            <span class="text-subtitle-1 font-weight-bold">AI 智能搜图与图片查重</span>
          </div>
          <div class="text-caption text-medium-emphasis">
            启用后系统将自动安装并运行 Qdrant 向量数据库与 Qwen3 多模态大模型，以提供强大的语义搜索和查重能力（首次启用由于下载模型，可能需要几分钟初始化）。关闭后将自动释放内存资源。
          </div>
        </div>
        <v-switch
          v-model="aiEnabled"
          :disabled="updatingSettings"
          :loading="updatingSettings"
          color="primary"
          hide-details
          inset
          @change="toggleAISettings"
        />
      </div>
    </v-card>

    <!-- SSL 设置 -->
    <v-card color="surface-variant" class="pa-5 mb-4">
      <div class="d-flex align-center justify-space-between flex-wrap ga-3 mb-3">
        <div class="d-flex align-center ga-3">
          <v-icon color="primary">mdi-lock-outline</v-icon>
          <span class="text-subtitle-1 font-weight-bold">HTTPS / SSL</span>
        </div>
        <v-switch
          v-model="sslEnabled"
          :disabled="updatingSSL"
          color="primary"
          hide-details
          inset
        />
      </div>
      <div class="text-caption text-medium-emphasis mb-4">
        默认使用 HTTP。开启后服务会自动重启并在 8082 端口使用 HTTPS。
      </div>
      <v-row>
        <v-col cols="12" md="6">
          <v-text-field
            v-model="sslCertFile"
            label="证书文件路径"
            placeholder="/ssl/1.pem"
            prepend-inner-icon="mdi-certificate-outline"
            :disabled="updatingSSL"
            hide-details="auto"
          />
        </v-col>
        <v-col cols="12" md="6">
          <v-text-field
            v-model="sslKeyFile"
            label="私钥文件路径"
            placeholder="/ssl/1.key"
            prepend-inner-icon="mdi-key-outline"
            :disabled="updatingSSL"
            hide-details="auto"
          />
        </v-col>
      </v-row>
      <div class="d-flex justify-end mt-3">
        <v-btn
          color="primary"
          variant="tonal"
          :loading="updatingSSL"
          :disabled="updatingSSL"
          prepend-icon="mdi-content-save-outline"
          @click="saveSSLSettings"
        >
          保存 SSL 设置
        </v-btn>
      </div>
    </v-card>

    <v-card color="surface-variant" class="pa-5 mb-4">
      <div class="d-flex align-center ga-3 mb-4">
        <v-icon color="primary">mdi-server-network</v-icon>
        <span class="text-subtitle-1 font-weight-medium">AI 服务状态</span>
      </div>
      <v-row>
        <v-col cols="12" md="6">
          <div class="d-flex align-center justify-space-between mb-2">
            <div class="d-flex align-center ga-2">
              <v-icon size="20">mdi-brain</v-icon>
              <span class="text-body-2">Qwen3 多模态模型</span>
            </div>
            <v-chip size="small" :color="serviceStatusColor(info.ai.model.status)" variant="tonal">
              {{ serviceStatusLabel(info.ai.model.status) }}
            </v-chip>
          </div>
          <div class="text-caption text-medium-emphasis">
            {{ info.ai.model.message }}<span v-if="info.ai.model.device"> · 设备 {{ info.ai.model.device }}</span>
          </div>
        </v-col>
        <v-col cols="12" md="6">
          <div class="d-flex align-center justify-space-between mb-2">
            <div class="d-flex align-center ga-2">
              <v-icon size="20">mdi-database-search</v-icon>
              <span class="text-body-2">Qdrant 向量数据库</span>
            </div>
            <v-chip size="small" :color="serviceStatusColor(info.ai.qdrant.status)" variant="tonal">
              {{ serviceStatusLabel(info.ai.qdrant.status) }}
            </v-chip>
          </div>
          <div class="text-caption text-medium-emphasis">
            {{ info.ai.qdrant.message }}<span v-if="info.ai.qdrant.version"> · 版本 {{ info.ai.qdrant.version }}</span>
          </div>
        </v-col>
      </v-row>
    </v-card>

    <!-- 进程资源 -->
    <v-card color="surface-variant" class="pa-5 mb-4">
      <div class="d-flex align-center ga-3 mb-4">
        <v-icon color="primary">mdi-application-outline</v-icon>
        <span class="text-subtitle-1 font-weight-medium">本进程</span>
      </div>

      <v-row>
        <v-col cols="12" md="6">
          <div class="d-flex align-center justify-space-between mb-2">
            <span class="text-body-2">内存占用</span>
            <span class="text-body-2 font-weight-medium">
              {{ formatBytes(info.procMem) }} / {{ formatBytes(info.memoryTotal) }}
              <span class="text-caption text-on-surface-variant ml-1">({{ procMemPercent }}%)</span>
            </span>
          </div>
          <v-progress-linear
            :model-value="Number(procMemPercent)"
            color="info"
            height="8"
            rounded
          />
        </v-col>
        <v-col cols="12" md="6">
          <div class="d-flex align-center justify-space-between mb-2">
            <span class="text-body-2">CPU 占用</span>
            <span class="text-body-2 font-weight-medium">{{ procCPUPercent }}%</span>
          </div>
          <v-progress-linear
            :model-value="Number(procCPUPercent)"
            color="warning"
            height="8"
            rounded
          />
        </v-col>
      </v-row>
    </v-card>

    <!-- 系统资源 -->
    <v-card color="surface-variant" class="pa-5 mb-4">
      <div class="d-flex align-center ga-3 mb-4">
        <v-icon color="primary">mdi-server</v-icon>
        <span class="text-subtitle-1 font-weight-medium">系统资源</span>
      </div>

      <v-row>
        <v-col cols="12" md="6">
          <div class="d-flex align-center justify-space-between mb-2">
            <span class="text-body-2">内存使用</span>
            <span class="text-body-2 font-weight-medium">
              {{ formatBytes(info.memoryUsed) }} / {{ formatBytes(info.memoryTotal) }}
              <span class="text-caption text-on-surface-variant ml-1">({{ memPercent }}%)</span>
            </span>
          </div>
          <v-progress-linear
            :model-value="memPercent"
            color="primary"
            height="8"
            rounded
          />
          <v-row class="mt-3">
            <v-col cols="4">
              <div class="text-caption text-on-surface-variant">已用</div>
              <div class="text-body-2">{{ formatBytes(info.memoryUsed) }}</div>
            </v-col>
            <v-col cols="4">
              <div class="text-caption text-on-surface-variant">可用</div>
              <div class="text-body-2">{{ formatBytes(info.memoryFree) }}</div>
            </v-col>
            <v-col cols="4">
              <div class="text-caption text-on-surface-variant">总计</div>
              <div class="text-body-2">{{ formatBytes(info.memoryTotal) }}</div>
            </v-col>
          </v-row>
        </v-col>
        <v-col cols="12" md="6">
          <div class="d-flex align-center justify-space-between mb-2">
            <span class="text-body-2">CPU 使用率</span>
            <span class="text-body-2 font-weight-medium">{{ cpuPercent }}%</span>
          </div>
          <v-progress-linear
            :model-value="Number(cpuPercent)"
            color="error"
            height="8"
            rounded
          />
        </v-col>
      </v-row>
    </v-card>

    <!-- 磁盘 -->
    <v-card color="surface-variant" class="pa-5">
      <div class="d-flex align-center ga-3 mb-4">
        <v-icon color="primary">mdi-harddisk</v-icon>
        <span class="text-subtitle-1 font-weight-medium">存储空间</span>
      </div>
      <div class="d-flex align-center justify-space-between mb-2">
        <span class="text-body-2">已用 / 总计</span>
        <span class="text-body-2 font-weight-medium">
          {{ formatBytes(info.diskUsed) }} / {{ formatBytes(info.diskTotal) }}
        </span>
      </div>
      <v-progress-linear
        :model-value="Math.round((info.diskUsed / info.diskTotal) * 100)"
        color="success"
        height="8"
        rounded
      />
      <v-row class="mt-3">
        <v-col cols="4">
          <div class="text-caption text-on-surface-variant">已用</div>
          <div class="text-body-2">{{ formatBytes(info.diskUsed) }}</div>
        </v-col>
        <v-col cols="4">
          <div class="text-caption text-on-surface-variant">可用</div>
          <div class="text-body-2">{{ formatBytes(info.diskFree) }}</div>
        </v-col>
        <v-col cols="4">
          <div class="text-caption text-on-surface-variant">总计</div>
          <div class="text-body-2">{{ formatBytes(info.diskTotal) }}</div>
        </v-col>
      </v-row>
    </v-card>
  </div>

  <div v-else class="d-flex justify-center pa-8">
    <v-progress-circular indeterminate color="primary" />
  </div>
</template>
