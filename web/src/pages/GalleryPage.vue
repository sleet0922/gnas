<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { apiGet, apiPost, getAuthImageUrl, getAuthDownloadUrl, type MediaItem } from '@/composables/useApi'

const items = ref<MediaItem[]>([])
const loading = ref(true)
const previewDialog = ref(false)
const previewItem = ref<MediaItem | null>(null)
const filter = ref<'all' | 'image' | 'video'>('all')

// 批量选择
const selectMode = ref(false)
const selectedPaths = ref<Set<string>>(new Set())
const batchDeleteDialog = ref(false)

const filteredItems = computed(() => {
  if (filter.value === 'all') return items.value
  return items.value.filter(i => i.type === filter.value)
})

const imageCount = computed(() => items.value.filter(i => i.type === 'image').length)
const videoCount = computed(() => items.value.filter(i => i.type === 'video').length)

const searchQuery = ref('')
const isSearching = ref(false)
const aiEnabled = ref(false)

async function checkAISettings() {
  const data = await apiGet<{ ai_enabled: boolean }>('/api/settings')
  if (data) aiEnabled.value = data.ai_enabled
}

async function handleSearch() {
  const query = searchQuery.value?.trim()
  if (!query) {
    clearSearch()
    return
  }
  loading.value = true
  isSearching.value = true
  try {
    const data = await apiGet<MediaItem[]>('/api/search?q=' + encodeURIComponent(query))
    items.value = data || []
  } catch (e: any) {
    alert(e.message || '搜索失败')
  } finally {
    loading.value = false
  }
}

async function clearSearch() {
  searchQuery.value = ''
  isSearching.value = false
  await loadGallery()
}

watch(searchQuery, (newVal) => {
  if (newVal === null || newVal === '') {
    isSearching.value = false
    loadGallery()
  }
})

async function loadGallery() {
  loading.value = true
  const data = await apiGet<MediaItem[]>('/api/gallery')
  items.value = data || []
  loading.value = false
  selectMode.value = false
  selectedPaths.value.clear()
}

function openPreview(item: MediaItem) {
  if (selectMode.value) {
    toggleSelect(item)
    return
  }
  previewItem.value = item
  previewDialog.value = true
}

function closePreview() {
  previewDialog.value = false
  previewItem.value = null
}

function prevItem() {
  if (!previewItem.value) return
  const idx = filteredItems.value.findIndex(i => i.path === previewItem.value!.path)
  if (idx > 0) previewItem.value = filteredItems.value[idx - 1]
}

function nextItem() {
  if (!previewItem.value) return
  const idx = filteredItems.value.findIndex(i => i.path === previewItem.value!.path)
  if (idx < filteredItems.value.length - 1) previewItem.value = filteredItems.value[idx + 1]
}

function onKeydown(e: KeyboardEvent) {
  if (!previewDialog.value) return
  if (e.key === 'ArrowLeft') prevItem()
  else if (e.key === 'ArrowRight') nextItem()
  else if (e.key === 'Escape') closePreview()
}

function thumbUrl(item: MediaItem): string {
  return getAuthImageUrl(item.path)
}

function originalUrl(item: MediaItem): string {
  return getAuthDownloadUrl(item.path, true)
}

// 删除单个
async function deleteItem(item: MediaItem) {
  try {
    await apiPost('/api/files/delete', { path: item.path })
    items.value = items.value.filter(i => i.path !== item.path)
    if (previewItem.value?.path === item.path) closePreview()
  } catch (e: unknown) {
    alert(e instanceof Error ? e.message : '删除失败')
  }
}

// 批量选择
function toggleSelect(item: MediaItem) {
  if (selectedPaths.value.has(item.path)) {
    selectedPaths.value.delete(item.path)
  } else {
    selectedPaths.value.add(item.path)
  }
}

function selectAll() {
  if (selectedPaths.value.size === filteredItems.value.length) {
    selectedPaths.value.clear()
  } else {
    selectedPaths.value = new Set(filteredItems.value.map(i => i.path))
  }
}

function exitSelectMode() {
  selectMode.value = false
  selectedPaths.value.clear()
}

async function batchDelete() {
  if (selectedPaths.value.size === 0) return
  batchDeleteDialog.value = true
}

async function confirmBatchDelete() {
  try {
    await apiPost('/api/files/batch-delete', { paths: Array.from(selectedPaths.value) })
    items.value = items.value.filter(i => !selectedPaths.value.has(i.path))
    selectedPaths.value.clear()
    batchDeleteDialog.value = false
    selectMode.value = false
  } catch (e: unknown) {
    alert(e instanceof Error ? e.message : '批量删除失败')
    batchDeleteDialog.value = false
    loadGallery()
  }
}

onMounted(() => {
  loadGallery()
  checkAISettings()
  window.addEventListener('keydown', onKeydown)
})
</script>

<template>
  <div>
    <div class="d-flex flex-wrap align-center justify-space-between mb-4 ga-2">
      <div class="d-flex align-center ga-4">
        <h1 class="text-h5 font-weight-bold">相册</h1>
        <v-text-field
          v-model="searchQuery"
          :placeholder="aiEnabled ? '智能多模态搜索...' : '智能搜索未开启 (请在“占用”中启用)'"
          :disabled="!aiEnabled"
          prepend-inner-icon="mdi-magnify"
          density="compact"
          variant="outlined"
          hide-details
          clearable
          style="max-width: 300px; min-width: 200px;"
          @keydown.enter="handleSearch"
          @click:clear="clearSearch"
        />
      </div>
      <div class="d-flex ga-2 align-center">
        <v-chip-group v-model="filter" mandatory>
          <v-chip size="small" value="all" filter>全部 ({{ items.length }})</v-chip>
          <v-chip size="small" value="image" filter>图片 ({{ imageCount }})</v-chip>
          <v-chip size="small" value="video" filter>视频 ({{ videoCount }})</v-chip>
        </v-chip-group>
        <template v-if="selectMode">
          <v-btn variant="tonal" size="small" @click="selectAll">
            {{ selectedPaths.size === filteredItems.length ? '取消全选' : '全选' }}
          </v-btn>
          <v-btn variant="tonal" size="small" color="error" prepend-icon="mdi-delete-outline" :disabled="selectedPaths.size === 0" @click="batchDelete">
            删除 ({{ selectedPaths.size }})
          </v-btn>
          <v-btn variant="text" size="small" @click="exitSelectMode">取消</v-btn>
        </template>
        <template v-else>
          <v-btn variant="tonal" size="small" prepend-icon="mdi-check-circle-outline" @click="selectMode = true">批量</v-btn>
          <v-btn variant="tonal" size="small" icon="mdi-refresh" @click="isSearching ? handleSearch() : loadGallery()" />
        </template>
      </div>
    </div>

    <div v-if="loading" class="d-flex justify-center pa-8">
      <v-progress-circular indeterminate color="primary" />
    </div>

    <div v-else-if="filteredItems.length === 0" class="text-center text-medium-emphasis pa-8">
      暂无媒体文件
    </div>

    <!-- 网格布局 -->
    <div v-else class="gallery-grid">
      <div
        v-for="item in filteredItems"
        :key="item.path"
        class="gallery-item"
        :class="{ 'gallery-selected': selectMode && selectedPaths.has(item.path) }"
        @click="openPreview(item)"
      >
        <!-- 选择模式下显示勾选框 -->
        <div v-if="selectMode" class="gallery-check">
          <v-checkbox-btn :model-value="selectedPaths.has(item.path)" />
        </div>
        <!-- 图片缩略图 -->
        <img
          v-if="item.type === 'image'"
          :src="thumbUrl(item)"
          :alt="item.name"
          loading="lazy"
          class="gallery-thumb"
        />
        <!-- 视频缩略图 -->
        <div v-else class="gallery-thumb gallery-video-thumb">
          <img
            :src="thumbUrl(item)"
            :alt="item.name"
            loading="lazy"
            class="gallery-thumb"
            @error="($event.target as HTMLImageElement).style.display='none'"
          />
          <div class="video-overlay">
            <v-icon size="36" color="white">mdi-play-circle</v-icon>
          </div>
        </div>
        <div class="gallery-label">
          <span class="gallery-name">{{ item.name }}</span>
          <v-btn v-if="!selectMode" icon size="x-small" variant="text" color="error" @click.stop="deleteItem(item)" class="gallery-del-btn">
            <v-icon size="14">mdi-delete-outline</v-icon>
          </v-btn>
        </div>
      </div>
    </div>

    <!-- 预览弹窗 -->
    <v-dialog v-model="previewDialog" fullscreen scrollable>
      <v-card color="black">
        <v-toolbar color="transparent" density="compact">
          <v-btn icon variant="text" @click="prevItem" :disabled="!previewItem || filteredItems.findIndex(i => i.path === previewItem?.path) <= 0">
            <v-icon color="white">mdi-chevron-left</v-icon>
          </v-btn>
          <v-btn icon variant="text" @click="nextItem" :disabled="!previewItem || filteredItems.findIndex(i => i.path === previewItem?.path) >= filteredItems.length - 1">
            <v-icon color="white">mdi-chevron-right</v-icon>
          </v-btn>
          <v-spacer />
          <span class="text-white text-body-2">{{ previewItem?.name }}</span>
          <v-spacer />
          <v-btn icon variant="text" @click="previewItem && deleteItem(previewItem)" v-if="previewItem">
            <v-icon color="white">mdi-delete-outline</v-icon>
          </v-btn>
          <v-btn icon variant="text" :href="previewItem ? getAuthDownloadUrl(previewItem.path) : undefined" download>
            <v-icon color="white">mdi-download</v-icon>
          </v-btn>
          <v-btn icon variant="text" @click="closePreview">
            <v-icon color="white">mdi-close</v-icon>
          </v-btn>
        </v-toolbar>
        <v-card-text class="d-flex justify-center align-center" style="height: calc(100vh - 48px);">
          <img
            v-if="previewItem?.type === 'image'"
            :src="originalUrl(previewItem)"
            style="max-width: 100%; max-height: 100%; object-fit: contain;"
          />
          <video
            v-else-if="previewItem?.type === 'video'"
            :src="originalUrl(previewItem)"
            controls
            autoplay
            style="max-width: 100%; max-height: 100%;"
          />
        </v-card-text>
      </v-card>
    </v-dialog>

    <!-- 批量删除确认对话框 -->
    <v-dialog v-model="batchDeleteDialog" max-width="400">
      <v-card class="pa-6">
        <h3 class="text-h6 mb-4">确认删除</h3>
        <p class="text-body-2 mb-2">确定要删除选中的 <strong>{{ selectedPaths.size }}</strong> 个文件吗？此操作不可恢复。</p>
        <div class="d-flex justify-end ga-2 mt-4">
          <v-btn variant="text" @click="batchDeleteDialog = false">取消</v-btn>
          <v-btn color="error" @click="confirmBatchDelete">确认删除</v-btn>
        </div>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
.gallery-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
}

.gallery-item {
  cursor: pointer;
  border-radius: 8px;
  overflow: hidden;
  background: rgb(var(--v-theme-surface-variant));
  transition: transform 0.15s, box-shadow 0.15s;
  position: relative;
}

.gallery-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
}

.gallery-selected {
  outline: 3px solid rgb(var(--v-theme-primary));
  outline-offset: -3px;
}

.gallery-check {
  position: absolute;
  top: 4px;
  left: 4px;
  z-index: 2;
}

.gallery-thumb {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  display: block;
}

.gallery-video-thumb {
  position: relative;
  background: linear-gradient(135deg, #424242, #212121);
}

.video-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.25);
}

.gallery-label {
  padding: 4px 8px 6px;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: rgb(var(--v-theme-on-surface-variant));
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.gallery-name {
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.gallery-del-btn {
  opacity: 0;
  transition: opacity 0.15s;
  flex-shrink: 0;
}

.gallery-item:hover .gallery-del-btn {
  opacity: 1;
}
</style>
