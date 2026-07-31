<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost } from '@/composables/useApi'

const logs = ref<string[]>([])
const loading = ref(true)
const clearing = ref(false)
const snackbar = ref(false)
const snackbarText = ref('')

function showMsg(msg: string) {
  snackbarText.value = msg
  snackbar.value = true
}

function logClass(log: string): string {
  if (log.includes('失败') || log.includes('异常') || log.includes('error')) return 'error'
  if (log.includes('成功')) return 'success'
  return ''
}

onMounted(async () => {
  loading.value = true
  try {
    const data = await apiGet<string[]>('/api/logs')
    logs.value = data || []
  } catch (e: unknown) {
    console.error('加载日志失败:', e)
    showMsg(e instanceof Error ? e.message : '加载日志失败')
  } finally {
    loading.value = false
  }
})

async function clearLogs() {
  clearing.value = true
  try {
    await apiPost('/api/logs/clear', {})
    logs.value = []
    showMsg('日志已清除')
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '清除失败')
  } finally {
    clearing.value = false
  }
}
</script>

<template>
  <div>
    <div class="d-flex align-center justify-space-between mb-6">
      <h1 class="text-h5 font-weight-bold">运行日志</h1>
      <v-btn
        variant="tonal"
        size="small"
        prepend-icon="mdi-delete-sweep-outline"
        :loading="clearing"
        @click="clearLogs"
      >
        清除
      </v-btn>
    </div>

    <v-card rounded="lg" class="overflow-hidden" :border="false" :elevation="1">
      <div class="log-viewer">
        <div v-if="loading" class="d-flex justify-center pa-8">
          <v-progress-circular indeterminate color="primary" size="32" />
        </div>
        <div v-else-if="logs.length === 0" class="text-center text-medium-emphasis pa-8">
          暂无日志
        </div>
        <div v-else>
          <div v-for="(log, i) in logs" :key="i" class="log-line" :class="logClass(log)">
            {{ log }}
          </div>
        </div>
      </div>
    </v-card>

    <v-snackbar v-model="snackbar" :timeout="3000" color="on-surface" rounded="lg">
      {{ snackbarText }}
    </v-snackbar>
  </div>
</template>

<style scoped>
.log-viewer {
  background: #1a1c1e;
  color: #e2e1e6;
  padding: 16px;
  font-family: 'Google Sans Mono', 'JetBrains Mono', monospace;
  font-size: 12px;
  max-height: 520px;
  overflow-y: auto;
  line-height: 1.7;
}
.log-line { padding: 1px 0; }
.log-line.error { color: #f2b8b5; }
.log-line.success { color: #98f7b5; }
</style>
