<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  apiGetRecycleBin,
  apiRestoreRecycleItems,
  apiDeleteRecycleItems,
  apiClearRecycleBin,
  getRecycleThumbUrl,
  type RecycleItem,
} from '@/composables/useApi'

const items = ref<RecycleItem[]>([])
const loading = ref(true)
const errorMsg = ref('')
const actionLoading = ref(false)

// 选中项的 id 集合
const selectedIds = ref<Set<number>>(new Set())

// 确认对话框
type ConfirmKind = 'delete' | 'clear' | null
const confirmDialog = ref(false)
const confirmKind = ref<ConfirmKind>(null)
const confirmIds = ref<number[]>([])

// 恢复结果提示
const snackbar = ref(false)
const snackbarText = ref('')
const snackbarColor = ref<'success' | 'error' | 'info'>('success')

function thumbUrl(item: RecycleItem): string {
  return getRecycleThumbUrl(item.id)
}

function formatDate(s: string): string {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return d.toLocaleString()
}

function formatExpire(s: string): string {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const now = Date.now()
  const diff = d.getTime() - now
  if (diff <= 0) return '已过期'
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  if (days > 0) return `${days}天后清理`
  if (hours > 0) return `${hours}小时后清理`
  const minutes = Math.floor(diff / 60000)
  return `${minutes}分钟后清理`
}

const selectedCount = computed(() => selectedIds.value.size)

function isSelected(item: RecycleItem): boolean {
  return selectedIds.value.has(item.id)
}

function toggleSelect(item: RecycleItem) {
  if (selectedIds.value.has(item.id)) {
    selectedIds.value.delete(item.id)
  } else {
    selectedIds.value.add(item.id)
  }
}

function selectAll() {
  if (selectedIds.value.size === items.value.length) {
    selectedIds.value.clear()
  } else {
    selectedIds.value = new Set(items.value.map(i => i.id))
  }
}

function clearSelection() {
  selectedIds.value.clear()
}

async function loadItems() {
  loading.value = true
  errorMsg.value = ''
  try {
    const data = await apiGetRecycleBin()
    items.value = data || []
    clearSelection()
  } catch (e: unknown) {
    errorMsg.value = e instanceof Error ? e.message : '加载回收站失败'
  } finally {
    loading.value = false
  }
}

function showMsg(text: string, color: 'success' | 'error' | 'info' = 'success') {
  snackbarText.value = text
  snackbarColor.value = color
  snackbar.value = true
}

// 恢复选中
async function restoreSelected() {
  if (selectedIds.value.size === 0) return
  actionLoading.value = true
  try {
    const ids = Array.from(selectedIds.value)
    const result = await apiRestoreRecycleItems(ids)
    const restored = result?.restored ?? 0
    items.value = items.value.filter(i => !selectedIds.value.has(i.id))
    clearSelection()
    showMsg(`已恢复 ${restored} 个文件`)
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '恢复失败', 'error')
  } finally {
    actionLoading.value = false
  }
}

// 彻底删除选中 - 弹确认框
function askDeleteSelected() {
  if (selectedIds.value.size === 0) return
  confirmKind.value = 'delete'
  confirmIds.value = Array.from(selectedIds.value)
  confirmDialog.value = true
}

// 清空回收站 - 弹确认框
function askClearAll() {
  if (items.value.length === 0) return
  confirmKind.value = 'clear'
  confirmIds.value = items.value.map(i => i.id)
  confirmDialog.value = true
}

async function confirmAction() {
  const kind = confirmKind.value
  const ids = confirmIds.value
  confirmDialog.value = false
  if (!kind || ids.length === 0) return
  actionLoading.value = true
  try {
    if (kind === 'delete') {
      const result = await apiDeleteRecycleItems(ids)
      const deleted = result?.deleted ?? 0
      items.value = items.value.filter(i => !ids.includes(i.id))
      selectedIds.value = new Set([...selectedIds.value].filter(id => !ids.includes(id)))
      showMsg(`已彻底删除 ${deleted} 个文件`)
    } else if (kind === 'clear') {
      const result = await apiClearRecycleBin()
      const cleared = result?.cleared ?? 0
      items.value = []
      clearSelection()
      showMsg(`已清空回收站,共删除 ${cleared} 个文件`)
    }
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '操作失败', 'error')
    await loadItems()
  } finally {
    actionLoading.value = false
    confirmKind.value = null
    confirmIds.value = []
  }
}

onMounted(() => {
  loadItems()
})
</script>

<template>
  <div>
    <!-- 顶部工具栏 -->
    <div class="d-flex flex-wrap align-center justify-space-between mb-4 ga-2">
      <div class="d-flex align-center ga-4">
        <h1 class="text-h5 font-weight-bold">回收站</h1>
        <span class="text-body-2 text-medium-emphasis">共 {{ items.length }} 项</span>
      </div>
      <div class="d-flex ga-2 align-center flex-wrap">
        <v-btn
          variant="tonal"
          size="small"
          prepend-icon="mdi-undo-variant"
          color="primary"
          :disabled="selectedCount === 0 || actionLoading"
          @click="restoreSelected"
        >
          恢复选中 ({{ selectedCount }})
        </v-btn>
        <v-btn
          variant="tonal"
          size="small"
          prepend-icon="mdi-delete-forever-outline"
          color="error"
          :disabled="selectedCount === 0 || actionLoading"
          @click="askDeleteSelected"
        >
          彻底删除 ({{ selectedCount }})
        </v-btn>
        <v-btn
          variant="tonal"
          size="small"
          prepend-icon="mdi-trash-can-outline"
          color="error"
          :disabled="items.length === 0 || actionLoading"
          @click="askClearAll"
        >
          清空回收站
        </v-btn>
        <v-btn
          variant="tonal"
          size="small"
          prepend-icon="mdi-select-all"
          :disabled="items.length === 0 || actionLoading"
          @click="selectAll"
        >
          {{ selectedIds.size === items.length && items.length > 0 ? '取消全选' : '全选' }}
        </v-btn>
        <v-btn
          variant="tonal"
          size="small"
          icon="mdi-refresh"
          :loading="loading"
          @click="loadItems"
        />
      </div>
    </div>

    <!-- 加载中 -->
    <div v-if="loading" class="d-flex justify-center pa-8">
      <v-progress-circular indeterminate color="primary" />
    </div>

    <!-- 错误状态 -->
    <v-card v-else-if="errorMsg" class="pa-8 text-center text-error">
      <v-icon size="64" color="error" class="mb-4">mdi-alert-circle-outline</v-icon>
      <h3 class="text-h6 font-weight-bold mb-2">加载失败</h3>
      <p class="text-body-2 mb-4">{{ errorMsg }}</p>
      <v-btn color="primary" variant="tonal" prepend-icon="mdi-refresh" @click="loadItems">重试</v-btn>
    </v-card>

    <!-- 空状态 -->
    <v-card v-else-if="items.length === 0" class="pa-8 text-center text-medium-emphasis">
      <v-icon size="64" color="medium-emphasis" class="mb-4">mdi-trash-can-outline</v-icon>
      <h3 class="text-h6 font-weight-bold mb-2">回收站为空</h3>
      <p class="text-body-2">没有已删除的文件</p>
    </v-card>

    <!-- 网格显示 -->
    <div v-else class="recycle-grid">
      <v-card
        v-for="item in items"
        :key="item.id"
        class="recycle-item"
        :class="{ 'recycle-selected': isSelected(item) }"
        flat
        color="surface-variant"
        @click="toggleSelect(item)"
      >
        <!-- 选择框 -->
        <div class="recycle-check">
          <v-checkbox-btn
            :model-value="isSelected(item)"
            @click.stop="toggleSelect(item)"
          />
        </div>

        <!-- 缩略图区域 -->
        <div class="recycle-thumb-wrapper">
          <!-- 目录 -->
          <div v-if="item.isDir" class="recycle-dir">
            <v-icon size="64" color="primary">mdi-folder-outline</v-icon>
          </div>
          <!-- 图片/视频缩略图 -->
          <img
            v-else-if="item.hasThumb"
            :src="thumbUrl(item)"
            :alt="item.name"
            loading="lazy"
            class="recycle-thumb"
            @error="($event.target as HTMLImageElement).style.display='none'"
          />
          <!-- 无缩略图占位 -->
          <div v-else class="recycle-no-thumb">
            <v-icon size="48" color="medium-emphasis">
              {{ item.isVideo ? 'mdi-file-video-outline' : 'mdi-file-image-outline' }}
            </v-icon>
          </div>
          <!-- 视频播放图标 -->
          <div v-if="item.isVideo && !item.isDir" class="video-overlay">
            <v-icon size="36" color="white">mdi-play-circle</v-icon>
          </div>
        </div>

        <!-- 文件信息 -->
        <div class="recycle-info">
          <div class="recycle-name text-truncate" :title="item.name">{{ item.name }}</div>
          <div class="recycle-meta">
            <v-icon size="12" class="me-1">mdi-clock-outline</v-icon>
            <span>{{ formatExpire(item.expireAt) }}</span>
          </div>
          <div class="recycle-meta text-medium-emphasis">
            <v-icon size="12" class="me-1">mdi-delete-outline</v-icon>
            <span>{{ formatDate(item.deletedAt) }}</span>
          </div>
        </div>
      </v-card>
    </div>

    <!-- 确认对话框 -->
    <v-dialog v-model="confirmDialog" max-width="420">
      <v-card class="pa-6">
        <div class="d-flex align-center ga-3 mb-4">
          <v-icon color="error" size="28">mdi-alert-circle-outline</v-icon>
          <h3 class="text-h6 font-weight-bold">
            {{ confirmKind === 'clear' ? '清空回收站' : '彻底删除' }}
          </h3>
        </div>
        <p class="text-body-2 mb-2">
          <template v-if="confirmKind === 'clear'">
            确定要清空回收站中的全部 <strong>{{ confirmIds.length }}</strong> 个文件吗?
          </template>
          <template v-else>
            确定要彻底删除选中的 <strong>{{ confirmIds.length }}</strong> 个文件吗?
          </template>
          此操作不可恢复,文件将被永久删除。
        </p>
        <div class="d-flex justify-end ga-2 mt-4">
          <v-btn variant="text" :disabled="actionLoading" @click="confirmDialog = false">取消</v-btn>
          <v-btn color="error" :loading="actionLoading" @click="confirmAction">
            确认删除
          </v-btn>
        </div>
      </v-card>
    </v-dialog>

    <!-- 消息提示 -->
    <v-snackbar v-model="snackbar" :color="snackbarColor" timeout="3000">
      {{ snackbarText }}
    </v-snackbar>
  </div>
</template>

<style scoped>
.recycle-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
}

.recycle-item {
  cursor: pointer;
  overflow: hidden;
  position: relative;
  transition: transform 0.15s, box-shadow 0.15s, outline 0.15s;
  outline: 3px solid transparent;
  outline-offset: -3px;
}

.recycle-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.recycle-selected {
  outline-color: rgb(var(--v-theme-primary));
}

.recycle-check {
  position: absolute;
  top: 4px;
  left: 4px;
  z-index: 2;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 4px;
}

.recycle-thumb-wrapper {
  position: relative;
  width: 100%;
  aspect-ratio: 1 / 1;
  background: rgb(var(--v-theme-surface-variant));
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.recycle-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.recycle-dir,
.recycle-no-thumb {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, rgba(66,66,66,0.4), rgba(33,33,33,0.4));
}

.video-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.25);
  pointer-events: none;
}

.recycle-info {
  padding: 8px 10px 10px;
}

.recycle-name {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-bottom: 4px;
}

.recycle-meta {
  font-size: 11px;
  display: flex;
  align-items: center;
  color: rgb(var(--v-theme-on-surface-variant));
  margin-top: 2px;
}
</style>
