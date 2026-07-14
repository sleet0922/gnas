<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { apiGet, type SystemInfo } from '@/composables/useApi'

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

async function loadInfo() {
  const data = await apiGet<SystemInfo>('/api/system')
  if (data) info.value = data
}

onMounted(() => {
  loadInfo()
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
