<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { apiGet, apiPost, apiUpload, getAuthImageUrl, getAuthDownloadUrl, type FileItem } from '@/composables/useApi'

const files = ref<FileItem[]>([])
const currentPath = ref('/')
const loading = ref(true)
const snackbar = ref(false)
const snackbarText = ref('')
const uploadInput = ref<HTMLInputElement>()
const mkdirDialog = ref(false)
const renameDialog = ref(false)
const renameTarget = ref<FileItem | null>(null)
const renameName = ref('')
const newFolderName = ref('')

// 批量选择
const selectMode = ref(false)
const selectedPaths = ref<Set<string>>(new Set())
const batchDeleteDialog = ref(false)

// 预览相关
const previewDialog = ref(false)
const previewUrl = ref('')
const previewType = ref<'image' | 'video' | 'audio' | 'pdf' | 'text' | 'unknown'>('unknown')
const previewName = ref('')

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

async function loadFiles(dir?: string) {
  loading.value = true
  const path = dir ?? currentPath.value
  const data = await apiGet<FileItem[]>('/api/files?path=' + encodeURIComponent(path))
  files.value = data || []
  currentPath.value = path
  loading.value = false
  // 退出选择模式
  selectMode.value = false
  selectedPaths.value.clear()
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
    loadFiles()
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
    loadFiles()
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '批量删除失败')
    batchDeleteDialog.value = false
    loadFiles()
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
    loadFiles()
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
    loadFiles()
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '重命名失败')
  }
}

function downloadFile(item: FileItem) {
  window.open(getAuthDownloadUrl(item.path))
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

onMounted(() => loadFiles())
</script>

<template>
  <div>
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
      <v-list v-else density="comfortable">
        <v-list-item v-if="currentPath !== '/'" @click="goUp" class="text-medium-emphasis">
          <template #prepend><v-icon>mdi-arrow-up</v-icon></template>
          <v-list-item-title>..</v-list-item-title>
        </v-list-item>
        <v-list-item
          v-for="item in files"
          :key="item.path"
          @click="enterDir(item)"
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
            <v-btn icon size="small" variant="text" @click.stop="openRename(item)">
              <v-icon size="18">mdi-pencil-outline</v-icon>
            </v-btn>
            <v-btn icon size="small" variant="text" color="error" @click.stop="deleteItem(item)">
              <v-icon size="18">mdi-delete-outline</v-icon>
            </v-btn>
          </template>
        </v-list-item>
      </v-list>
    </v-card>

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
