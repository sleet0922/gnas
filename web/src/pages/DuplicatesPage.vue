<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost, getAuthImageUrl, getAuthDownloadUrl, type MediaItem } from '@/composables/useApi'

interface DuplicateGroup {
  similarity: number
  items: MediaItem[]
}

const duplicateGroups = ref<DuplicateGroup[]>([])
const loading = ref(true)
const aiEnabled = ref(false)

const previewDialog = ref(false)
const previewItem = ref<MediaItem | null>(null)

async function checkAISettings() {
  const data = await apiGet<{ ai_enabled: boolean }>('/api/settings')
  if (data) {
    aiEnabled.value = data.ai_enabled
    if (aiEnabled.value) {
      await loadDuplicates()
    } else {
      loading.value = false
    }
  } else {
    loading.value = false
  }
}

async function loadDuplicates() {
  loading.value = true
  try {
    const data = await apiGet<DuplicateGroup[]>('/api/gallery/duplicates')
    duplicateGroups.value = data || []
  } catch (e: any) {
    alert(e.message || '获取查重数据失败')
  } finally {
    loading.value = false
  }
}

function thumbUrl(item: MediaItem): string {
  return getAuthImageUrl(item.path)
}

function originalUrl(item: MediaItem): string {
  return getAuthDownloadUrl(item.path, true)
}

function openPreview(item: MediaItem) {
  previewItem.value = item
  previewDialog.value = true
}

function closePreview() {
  previewDialog.value = false
  previewItem.value = null
}

async function deleteItem(item: MediaItem, groupIdx: number) {
  if (!confirm(`确定要删除 ${item.name} 吗？此操作不可撤销。`)) return
  try {
    await apiPost('/api/files/delete', { path: item.path })
    // 从列表中移除
    const group = duplicateGroups.value[groupIdx]
    group.items = group.items.filter(i => i.path !== item.path)
    
    // 如果组内剩下的有效项小于2个，直接移除整个组
    if (group.items.length < 2) {
      duplicateGroups.value.splice(groupIdx, 1)
    }
    
    if (previewItem.value?.path === item.path) closePreview()
  } catch (e: any) {
    alert(e.message || '删除失败')
  }
}

onMounted(() => {
  checkAISettings()
})
</script>

<template>
  <div>
    <div class="d-flex align-center justify-space-between mb-6">
      <h1 class="text-h5 font-weight-bold">图片扫描查重</h1>
      <v-btn
        v-if="aiEnabled"
        variant="tonal"
        size="small"
        icon="mdi-refresh"
        :loading="loading"
        @click="loadDuplicates"
      />
    </div>

    <!-- AI 未启用提示 -->
    <v-card v-if="!aiEnabled && !loading" class="pa-8 text-center text-medium-emphasis">
      <v-icon size="64" color="warning" class="mb-4">mdi-brain-off</v-icon>
      <h3 class="text-h6 font-weight-bold mb-2">AI 查重功能未开启</h3>
      <p class="text-body-2 mb-4">图片智能查重需要基于多模态大模型的向量计算能力。请先前往<b>“占用”</b>页面开启 AI 智能功能。</p>
      <v-btn color="primary" to="/system" prepend-icon="mdi-arrow-right">去开启 AI 功能</v-btn>
    </v-card>

    <!-- 加载中 -->
    <div v-else-if="loading" class="d-flex justify-center pa-8">
      <v-progress-circular indeterminate color="primary" />
    </div>

    <!-- 无重复项 -->
    <v-card v-else-if="duplicateGroups.length === 0" class="pa-8 text-center text-medium-emphasis">
      <v-icon size="64" color="success" class="mb-4">mdi-check-circle-outline</v-icon>
      <h3 class="text-h6 font-weight-bold mb-2">未发现相似图片</h3>
      <p class="text-body-2">干得漂亮！您的相册非常整洁，没有任何重复或高度相似的照片。</p>
    </v-card>

    <!-- 查重列表 -->
    <div v-else>
      <v-card
        v-for="(group, groupIdx) in duplicateGroups"
        :key="groupIdx"
        color="surface-variant"
        class="mb-6 pa-5 rounded-lg"
      >
        <div class="d-flex align-center justify-space-between mb-4 flex-wrap ga-2">
          <div class="d-flex align-center ga-2">
            <v-chip color="primary" variant="flat" size="small">
              相似度 {{ (group.similarity * 100).toFixed(1) }}%
            </v-chip>
            <span class="text-subtitle-2 text-medium-emphasis">发现 {{ group.items.length }} 张高度相似的照片</span>
          </div>
        </div>

        <div class="d-flex flex-wrap ga-4">
          <v-card
            v-for="item in group.items"
            :key="item.path"
            width="160"
            flat
            color="background"
            class="pa-2 rounded-lg position-relative"
          >
            <!-- 快速删除按钮 -->
            <div class="position-absolute" style="top: 10px; right: 10px; z-index: 2;">
              <v-btn
                icon="mdi-delete"
                size="x-small"
                color="error"
                variant="flat"
                @click.stop="deleteItem(item, groupIdx)"
              />
            </div>
            
            <v-img
              :src="thumbUrl(item)"
              height="140"
              cover
              class="rounded cursor-pointer"
              @click="openPreview(item)"
            />
            
            <div class="text-caption text-truncate mt-2 px-1 text-center" :title="item.name">
              {{ item.name }}
            </div>
          </v-card>
        </div>
      </v-card>
    </div>

    <!-- 预览大图对话框 -->
    <v-dialog v-model="previewDialog" max-width="90vw" max-height="90vh" @click:outside="closePreview">
      <v-card class="pa-2 bg-black d-flex align-center justify-center position-relative" style="overflow: hidden;">
        <v-btn
          icon="mdi-close"
          color="white"
          variant="text"
          class="position-absolute"
          style="top: 10px; right: 10px; z-index: 10;"
          @click="closePreview"
        />
        <img
          v-if="previewItem"
          :src="originalUrl(previewItem)"
          style="max-width: 100%; max-height: 80vh; object-fit: contain;"
        />
        <div v-if="previewItem" class="text-white text-body-2 mt-2 w-100 text-center text-truncate px-4">
          {{ previewItem.name }}
        </div>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
.cursor-pointer {
  cursor: pointer;
}
</style>
