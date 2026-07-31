<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, reactive, nextTick } from 'vue'
import { apiGet, apiPost, apiUpload, getAuthImageUrl, getAuthDownloadUrl, type FileItem } from '@/composables/useApi'

const files = ref<FileItem[]>([])
const currentPath = ref('/')
const loading = ref(true)

type FileVirtualRow = { isUp: boolean; item: FileItem | null }
const fileViewport = ref<HTMLElement | null>(null)
const fileScrollTop = ref(0)
const fileViewportHeight = ref(600)
const fileRowHeight = 68
const fileOverscanRows = 5
const fileTotalRows = computed(() => files.value.length + (currentPath.value === '/' ? 0 : 1))
const fileStartRow = computed(() => Math.max(0, Math.floor(fileScrollTop.value / fileRowHeight) - fileOverscanRows))
const fileEndRow = computed(() => Math.min(
  fileTotalRows.value,
  Math.ceil((fileScrollTop.value + fileViewportHeight.value) / fileRowHeight) + fileOverscanRows,
))
const fileVisibleRows = computed<FileVirtualRow[]>(() => {
  const rows: FileVirtualRow[] = []
  const hasUpRow = currentPath.value !== '/'
  for (let row = fileStartRow.value; row < fileEndRow.value; row++) {
    if (hasUpRow && row === 0) {
      rows.push({ isUp: true, item: null })
      continue
    }
    const item = files.value[row - (hasUpRow ? 1 : 0)]
    if (item) rows.push({ isUp: false, item })
  }
  return rows
})
const fileShowUp = computed(() => currentPath.value !== '/' && fileStartRow.value === 0)
const fileVisibleItems = computed(() => fileVisibleRows.value
  .filter(row => !row.isUp)
  .map(row => row.item as FileItem))
const fileTopOffset = computed(() => fileStartRow.value * fileRowHeight)
const fileBottomOffset = computed(() => Math.max(0, (fileTotalRows.value - fileEndRow.value) * fileRowHeight))
let fileResizeObserver: ResizeObserver | null = null

function measureFileViewport() {
  if (!fileViewport.value) return
  fileViewportHeight.value = fileViewport.value.clientHeight
}

function onFileScroll(event: Event) {
  fileScrollTop.value = (event.currentTarget as HTMLElement).scrollTop
}

function resetFileScroll() {
  fileScrollTop.value = 0
  if (fileViewport.value) fileViewport.value.scrollTop = 0
}

function restoreFileScroll(scrollTop: number) {
  if (!fileViewport.value) return
  const maxScrollTop = fileViewport.value.scrollHeight - fileViewport.value.clientHeight
  fileViewport.value.scrollTop = Math.min(scrollTop, Math.max(0, maxScrollTop))
  fileScrollTop.value = fileViewport.value.scrollTop
}
const snackbar = ref(false)
const snackbarText = ref('')
const uploadInput = ref<HTMLInputElement>()
const mkdirDialog = ref(false)
const renameDialog = ref(false)
const renameTarget = ref<FileItem | null>(null)
const renameName = ref('')
const newFolderName = ref('')
const flattenDialog = ref(false)
const flattenTarget = ref<FileItem | null>(null)
const flattening = ref(false)

// 批量选择
const selectMode = ref(false)
const selectedPaths = ref<Set<string>>(new Set())
const batchDeleteDialog = ref(false)

// 预览相关
const previewDialog = ref(false)
const previewUrl = ref('')
const previewType = ref<'image' | 'video' | 'audio' | 'pdf' | 'text' | 'unknown'>('unknown')
const previewName = ref('')

const contextMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  item: null as FileItem | null,
})

function showMsg(msg: string) {
  snackbarText.value = msg
  snackbar.value = true
}

const breadcrumbs = computed(() => {
  if (currentPath.value === '/') return [{ title: '根目录', path: '/' }]
  const parts = currentPath.value.split('/').filter(Boolean)
  const crumbs = [{ title: '根目录', path: '/' }]
  let p = ''
  for (const part of parts) {
    p += '/' + part
    crumbs.push({ title: part, path: p })
  }
  return crumbs
})

// 判断文件是否可预览
function getPreviewType(name: string): typeof previewType.value {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico']
  const videoExts = ['mp4', 'webm', 'ogv', 'mov']
  const audioExts = ['mp3', 'ogg', 'wav', 'flac', 'aac', 'm4a']
  const textExts = ['txt', 'md', 'json', 'html', 'htm', 'css', 'js', 'log', 'go', 'vue', 'ts']
  if (imageExts.includes(ext)) return 'image'
  if (videoExts.includes(ext)) return 'video'
  if (audioExts.includes(ext)) return 'audio'
  if (ext === 'pdf') return 'pdf'
  if (textExts.includes(ext)) return 'text'
  return 'unknown'
}

// 判断是否为图片（用于缩略图显示）
function isImage(name: string): boolean {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp'].includes(ext)
}

function canPreview(item: FileItem): boolean {
  if (item.isDir) return false
  return getPreviewType(item.name) !== 'unknown'
}

function openPreview(item: FileItem) {
  const type = getPreviewType(item.name)
  if (type === 'unknown') return
  previewType.value = type
  previewName.value = item.name
  previewUrl.value = getAuthDownloadUrl(item.path, true)
  previewDialog.value = true
}

async function loadFiles(dir?: string, preserveScroll = false) {
  const previousScrollTop = fileViewport.value?.scrollTop ?? fileScrollTop.value
  loading.value = true
  try {
    const path = dir ?? currentPath.value
    const data = await apiGet<FileItem[]>('/api/files?path=' + encodeURIComponent(path))
    files.value = data || []
    currentPath.value = path
    nextTick(() => preserveScroll ? restoreFileScroll(previousScrollTop) : resetFileScroll())
    // 退出选择模式
    selectMode.value = false
    selectedPaths.value.clear()
  } catch (e: unknown) {
    console.error('加载文件失败:', e)
    showMsg(e instanceof Error ? e.message : '加载文件失败')
  } finally {
    loading.value = false
  }
}

function enterDir(item: FileItem) {
  if (selectMode.value) {
    toggleSelect(item)
    return
  }
  if (item.isDir) loadFiles(item.path)
}

function goPath(path: string) {
  loadFiles(path)
}

function goUp() {
  if (currentPath.value === '/') return
  const parts = currentPath.value.split('/').filter(Boolean)
  parts.pop()
  loadFiles('/' + parts.join('/'))
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) { size /= 1024; i++ }
  return size.toFixed(i === 0 ? 0 : 1) + ' ' + units[i]
}

function formatDate(s: string): string {
  return new Date(s).toLocaleString('zh-CN')
}

async function deleteItem(item: FileItem) {
  try {
    await apiPost('/api/files/delete', { path: item.path })
    showMsg('已删除: ' + item.name)
    loadFiles(undefined, true)
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '删除失败')
  }
}

// 批量选择
function toggleSelect(item: FileItem) {
  if (selectedPaths.value.has(item.path)) {
    selectedPaths.value.delete(item.path)
  } else {
    selectedPaths.value.add(item.path)
  }
}

function selectAll() {
  if (selectedPaths.value.size === files.value.length) {
    selectedPaths.value.clear()
  } else {
    selectedPaths.value = new Set(files.value.map(f => f.path))
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
    showMsg(`已删除 ${selectedPaths.value.size} 个文件`)
    batchDeleteDialog.value = false
    loadFiles(undefined, true)
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '批量删除失败')
    batchDeleteDialog.value = false
    loadFiles(undefined, true)
  }
}

async function createFolder() {
  if (!newFolderName.value.trim()) return
  try {
    const path = currentPath.value === '/'
      ? '/' + newFolderName.value
      : currentPath.value + '/' + newFolderName.value
    await apiPost('/api/files/mkdir', { path })
    mkdirDialog.value = false
    newFolderName.value = ''
    showMsg('文件夹已创建')
    loadFiles(undefined, true)
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '创建失败')
  }
}

function openRename(item: FileItem) {
  renameTarget.value = item
  renameName.value = item.name
  renameDialog.value = true
}

async function doRename() {
  if (!renameTarget.value || !renameName.value.trim()) return
  try {
    await apiPost('/api/files/rename', { oldPath: renameTarget.value.path, newName: renameName.value })
    renameDialog.value = false
    showMsg('重命名成功')
    loadFiles(undefined, true)
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '重命名失败')
  }
}

function openFlatten(item: FileItem) {
  if (!item.isDir) return
  flattenTarget.value = item
  flattenDialog.value = true
}

function closeContextMenu() {
  contextMenu.visible = false
  contextMenu.item = null
}

function openContextMenu(event: MouseEvent, item: FileItem) {
  event.preventDefault()
  event.stopPropagation()
  contextMenu.item = item
  contextMenu.x = Math.min(event.clientX, Math.max(8, window.innerWidth - 224))
  contextMenu.y = Math.min(event.clientY, Math.max(8, window.innerHeight - 340))
  contextMenu.visible = true
}

function onContextKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') closeContextMenu()
}

function runContextAction(action: 'open' | 'preview' | 'download' | 'rename' | 'flatten' | 'delete') {
  const item = contextMenu.item
  closeContextMenu()
  if (!item) return
  if (action === 'open') enterDir(item)
  else if (action === 'preview') openPreview(item)
  else if (action === 'download') downloadFile(item)
  else if (action === 'rename') openRename(item)
  else if (action === 'flatten') openFlatten(item)
  else if (action === 'delete') deleteItem(item)
}

function openFlattenRoot() {
  flattenTarget.value = {
    name: '所有文件夹',
    path: '/',
    isDir: true,
    size: 0,
    modTime: '',
  }
  flattenDialog.value = true
}

async function confirmFlatten() {
  if (!flattenTarget.value) return
  flattening.value = true
  try {
    const result = await apiPost<{ moved: number; removedDirs: number }>('/api/files/flatten', {
      path: flattenTarget.value.path,
    })
    flattenDialog.value = false
    showMsg(`已移动 ${result?.moved ?? 0} 个文件，清理 ${result?.removedDirs ?? 0} 个空文件夹`)
    await loadFiles()
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '拆散文件夹失败')
  } finally {
    flattening.value = false
  }
}

function downloadFile(item: FileItem) {
  window.open(getAuthDownloadUrl(item.path), '_blank', 'noopener')
}

async function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files?.length) return
  try {
    for (const file of input.files) {
      await apiUpload('/api/files/upload', file, currentPath.value)
    }
    showMsg('上传成功')
    loadFiles()
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '上传失败')
  }
  input.value = ''
}

onMounted(() => {
  loadFiles()
  nextTick(() => {
    measureFileViewport()
    if (fileViewport.value) {
      fileResizeObserver = new ResizeObserver(measureFileViewport)
      fileResizeObserver.observe(fileViewport.value)
    }
  })
  window.addEventListener('click', closeContextMenu)
  window.addEventListener('scroll', closeContextMenu, true)
  window.addEventListener('keydown', onContextKeydown)
})

onUnmounted(() => {
  fileResizeObserver?.disconnect()
  fileResizeObserver = null
  window.removeEventListener('click', closeContextMenu)
  window.removeEventListener('scroll', closeContextMenu, true)
  window.removeEventListener('keydown', onContextKeydown)
})
</script>

<template>
  <div @contextmenu.prevent="closeContextMenu">
    <div class="d-flex align-center justify-space-between mb-4">
      <h1 class="text-h5 font-weight-bold">文件管理</h1>
      <div class="d-flex ga-2">
        <template v-if="selectMode">
          <v-btn variant="tonal" size="small" @click="selectAll">
            {{ selectedPaths.size === files.length ? '取消全选' : '全选' }}
          </v-btn>
          <v-btn variant="tonal" size="small" color="error" prepend-icon="mdi-delete-outline" :disabled="selectedPaths.size === 0" @click="batchDelete">
            删除 ({{ selectedPaths.size }})
          </v-btn>
          <v-btn variant="text" size="small" @click="exitSelectMode">取消</v-btn>
        </template>
        <template v-else>
          <v-btn v-if="currentPath === '/'" variant="tonal" size="small" prepend-icon="mdi-folder-move" @click="openFlattenRoot">拆散文件夹</v-btn>
          <v-btn variant="tonal" size="small" prepend-icon="mdi-check-circle-outline" @click="selectMode = true">批量</v-btn>
          <v-btn variant="tonal" size="small" prepend-icon="mdi-folder-plus" @click="mkdirDialog = true">新建文件夹</v-btn>
          <v-btn variant="tonal" size="small" prepend-icon="mdi-upload" @click="uploadInput?.click()">上传</v-btn>
        </template>
        <input ref="uploadInput" type="file" multiple class="d-none" @change="onFileChange" />
      </div>
    </div>

    <!-- 面包屑 -->
    <v-card color="surface-variant" class="pa-3 mb-4 d-flex align-center ga-1 flex-wrap">
      <v-btn v-for="(crumb, i) in breadcrumbs" :key="i" variant="text" size="small" density="compact" @click="goPath(crumb.path)">
        {{ crumb.title }}
      </v-btn>
    </v-card>

    <!-- 文件列表 -->
    <v-card rounded="lg">
      <div v-if="loading" class="d-flex justify-center pa-8">
        <v-progress-circular indeterminate color="primary" />
      </div>
      <div v-else-if="files.length === 0" class="text-center text-medium-emphasis pa-8">
        空文件夹
      </div>
      <div v-else ref="fileViewport" class="file-viewport" @scroll="onFileScroll">
        <div aria-hidden="true" :style="{ height: `${fileTopOffset}px` }" />
        <v-list density="comfortable" class="file-list">
        <v-list-item v-if="fileShowUp" @click="goUp" class="file-list-item text-medium-emphasis">
          <template #prepend><v-icon>mdi-arrow-up</v-icon></template>
          <v-list-item-title>..</v-list-item-title>
        </v-list-item>
        <v-list-item
          v-for="item in fileVisibleItems"
          :key="item.path"
          class="file-list-item"
          @click="enterDir(item)"
          @contextmenu.prevent.stop="openContextMenu($event, item)"
          :class="{ 'bg-primary-lighten-5': selectMode && selectedPaths.has(item.path) }"
        >
          <template #prepend>
            <template v-if="selectMode">
              <v-checkbox-btn
                :model-value="selectedPaths.has(item.path)"
                @click.stop="toggleSelect(item)"
              />
            </template>
            <template v-if="isImage(item.name)">
              <img
                :src="getAuthImageUrl(item.path)"
                style="width: 40px; height: 40px; object-fit: cover; border-radius: 4px; margin-right: 12px;"
                loading="lazy"
              />
            </template>
            <v-icon v-else-if="!selectMode" :color="item.isDir ? 'primary' : 'on-surface-variant'">
              {{ item.isDir ? 'mdi-folder' : 'mdi-file-outline' }}
            </v-icon>
          </template>
          <v-list-item-title>{{ item.name }}</v-list-item-title>
          <v-list-item-subtitle v-if="!item.isDir">{{ formatSize(item.size) }} · {{ formatDate(item.modTime) }}</v-list-item-subtitle>
          <template #append v-if="!selectMode">
            <v-btn v-if="canPreview(item)" icon size="small" variant="text" color="primary" @click.stop="openPreview(item)">
              <v-icon size="18">mdi-eye-outline</v-icon>
            </v-btn>
            <v-btn v-if="!item.isDir" icon size="small" variant="text" @click.stop="downloadFile(item)">
              <v-icon size="18">mdi-download</v-icon>
            </v-btn>
            <v-btn v-if="item.isDir" icon size="small" variant="text" @click.stop="openFlatten(item)" aria-label="拆散到根目录">
              <v-icon size="18">mdi-folder-move</v-icon>
              <v-tooltip activator="parent" location="top">拆散到根目录</v-tooltip>
            </v-btn>
            <v-btn icon size="small" variant="text" @click.stop="openRename(item)">
              <v-icon size="18">mdi-pencil-outline</v-icon>
            </v-btn>
            <v-btn icon size="small" variant="text" color="error" @click.stop="deleteItem(item)">
              <v-icon size="18">mdi-delete-outline</v-icon>
            </v-btn>
          </template>
        </v-list-item>
        </v-list>
        <div aria-hidden="true" :style="{ height: `${fileBottomOffset}px` }" />
      </div>
    </v-card>

    <div
      v-if="contextMenu.visible"
      class="web-context-menu"
      :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
      @click.stop
      @contextmenu.prevent.stop
    >
      <v-list density="compact" min-width="210" class="py-1">
        <v-list-item
          v-if="contextMenu.item?.isDir"
          prepend-icon="mdi-folder-open-outline"
          title="打开文件夹"
          @click="runContextAction('open')"
        />
        <v-list-item
          v-if="contextMenu.item && canPreview(contextMenu.item)"
          prepend-icon="mdi-eye-outline"
          title="预览"
          @click="runContextAction('preview')"
        />
        <v-list-item
          v-if="contextMenu.item && !contextMenu.item.isDir"
          prepend-icon="mdi-download-outline"
          title="下载"
          @click="runContextAction('download')"
        />
        <v-list-item
          v-if="contextMenu.item?.isDir"
          prepend-icon="mdi-folder-move-outline"
          title="拆散到根目录"
          @click="runContextAction('flatten')"
        />
        <v-list-item
          prepend-icon="mdi-pencil-outline"
          title="重命名"
          @click="runContextAction('rename')"
        />
        <v-divider class="my-1" />
        <v-list-item
          prepend-icon="mdi-delete-outline"
          title="删除"
          class="text-error"
          @click="runContextAction('delete')"
        />
      </v-list>
    </div>

    <!-- 预览对话框 -->
    <v-dialog v-model="previewDialog" max-width="900" scrollable>
      <v-card>
        <v-card-title class="d-flex align-center">
          <span class="text-subtitle-1 font-weight-medium">{{ previewName }}</span>
          <v-spacer />
          <v-btn icon size="small" variant="text" @click="previewDialog = false">
            <v-icon>mdi-close</v-icon>
          </v-btn>
        </v-card-title>
        <v-divider />
        <v-card-text class="pa-0 d-flex justify-center align-center" style="min-height: 300px; background: #000;">
          <img v-if="previewType === 'image'" :src="previewUrl" style="max-width: 100%; max-height: 70vh; object-fit: contain;" />
          <video v-else-if="previewType === 'video'" :src="previewUrl" controls style="max-width: 100%; max-height: 70vh;" />
          <audio v-else-if="previewType === 'audio'" :src="previewUrl" controls class="pa-8" />
          <iframe v-else-if="previewType === 'pdf'" :src="previewUrl" style="width: 100%; height: 70vh; border: none;" />
          <iframe v-else-if="previewType === 'text'" :src="previewUrl" style="width: 100%; height: 70vh; border: none; background: #fff;" />
          <div v-else class="text-medium-emphasis pa-8">不支持预览此文件类型</div>
        </v-card-text>
      </v-card>
    </v-dialog>

    <!-- 新建文件夹对话框 -->
    <v-dialog v-model="mkdirDialog" max-width="400">
      <v-card class="pa-6">
        <h3 class="text-h6 mb-4">新建文件夹</h3>
        <v-text-field v-model="newFolderName" label="文件夹名称" @keyup.enter="createFolder" />
        <div class="d-flex justify-end ga-2 mt-4">
          <v-btn variant="text" @click="mkdirDialog = false">取消</v-btn>
          <v-btn color="primary" @click="createFolder">创建</v-btn>
        </div>
      </v-card>
    </v-dialog>

    <!-- 重命名对话框 -->
    <v-dialog v-model="renameDialog" max-width="400">
      <v-card class="pa-6">
        <h3 class="text-h6 mb-4">重命名</h3>
        <v-text-field v-model="renameName" label="新名称" @keyup.enter="doRename" />
        <div class="d-flex justify-end ga-2 mt-4">
          <v-btn variant="text" @click="renameDialog = false">取消</v-btn>
          <v-btn color="primary" @click="doRename">确认</v-btn>
        </div>
      </v-card>
    </v-dialog>

    <!-- 批量删除确认对话框 -->
    <v-dialog v-model="flattenDialog" max-width="440">
      <v-card class="pa-6">
        <h3 class="text-h6 mb-4">拆散文件夹</h3>
        <p class="text-body-2 mb-2">
          将“{{ flattenTarget?.name }}”中的所有文件移动到文件根目录，并删除处理后为空的文件夹。
        </p>
        <p class="text-caption text-medium-emphasis">同名文件会自动添加序号，不会覆盖原文件。</p>
        <div class="d-flex justify-end ga-2 mt-4">
          <v-btn variant="text" :disabled="flattening" @click="flattenDialog = false">取消</v-btn>
          <v-btn color="primary" :loading="flattening" @click="confirmFlatten">确认拆散</v-btn>
        </div>
      </v-card>
    </v-dialog>

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

    <v-snackbar v-model="snackbar" :timeout="3000" color="on-surface" rounded="lg">
      {{ snackbarText }}
    </v-snackbar>
  </div>
</template>

<style scoped>
.file-viewport {
  height: calc(100vh - 300px);
  min-height: 320px;
  overflow: auto;
  contain: strict;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}

.file-list-item {
  height: 68px;
  min-height: 68px;
  contain: layout paint;
}

.web-context-menu {
  position: fixed;
  z-index: 2400;
  overflow: hidden;
  border: 1px solid rgba(var(--v-border-color), 0.16);
  border-radius: 8px;
  background: rgb(var(--v-theme-surface));
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);
}
</style>
