const BASE = ''
const TOKEN_KEY = 'auth_token'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

async function request<T>(path: string, opts: RequestInit = {}): Promise<T | null> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(opts.headers as Record<string, string> || {}),
  }

  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), 30000)
  try {
    const url = BASE + path
    const res = await fetch(url, { ...opts, headers, signal: controller.signal })
    if (!res.ok) {
      // 尝试解析 JSON 错误,失败则用 HTTP 状态码
      let msg = `HTTP ${res.status}`
      try {
        const data = await res.json()
        if (data.message) msg = data.message
        else if (data.error) msg = data.error
      } catch {
        // 非 JSON 响应,用状态文本
        msg = `${res.status} ${res.statusText}`
      }
      // 401 时清除 token 并跳转登录
      if (res.status === 401) {
        localStorage.removeItem(TOKEN_KEY)
        window.location.href = '/login'
      }
      throw new Error(msg)
    }
    const data = await res.json()
    if (data.code !== 0) {
      throw new Error(data.message || data.error || 'Request failed')
    }
    return data.data as T
  } finally {
    clearTimeout(timeoutId)
  }
}

export function apiGet<T>(path: string): Promise<T | null> {
  return request<T>(path)
}

export function apiPost<T>(path: string, body: unknown): Promise<T | null> {
  return request<T>(path, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

// 获取图片 URL
export function getAuthImageUrl(path: string): string {
  const token = getToken()
  return `/api/files/thumb?path=${encodeURIComponent(path)}${token ? `&token=${encodeURIComponent(token)}` : ''}`
}

// 获取原图/下载 URL
export function getAuthDownloadUrl(path: string, inline = false): string {
  const token = getToken()
  return `/api/files/download?path=${encodeURIComponent(path)}${inline ? '&disposition=inline' : ''}${token ? `&token=${encodeURIComponent(token)}` : ''}`
}

// 使用 fetch 加载图片
export async function loadImage(path: string): Promise<string> {
  const url = getAuthImageUrl(path)
  const res = await fetch(url)
  if (!res.ok) {
    throw new Error('图片加载失败')
  }
  const blob = await res.blob()
  return URL.createObjectURL(blob)
}

export async function apiUpload(path: string, file: File, dir: string): Promise<null> {
  const form = new FormData()
  form.append('file', file)
  const token = getToken()
  const res = await fetch(BASE + path + '?path=' + encodeURIComponent(dir), {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    body: form,
  })
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const data = await res.json()
      if (data.message) msg = data.message
      else if (data.error) msg = data.error
    } catch {
      msg = `${res.status} ${res.statusText}`
    }
    if (res.status === 401) {
      localStorage.removeItem(TOKEN_KEY)
      window.location.href = '/login'
    }
    throw new Error(msg)
  }
  const data = await res.json()
  if (data.code !== 0) throw new Error(data.message || '上传失败')
  return null
}

export function getAuthGalleryExportUrl(): string {
  const token = getToken()
  return `/api/gallery/export${token ? `?token=${encodeURIComponent(token)}` : ''}`
}

// 回收站缩略图 URL
export function getRecycleThumbUrl(id: number): string {
  const token = getToken()
  return `/api/recycle-bin/thumb?id=${id}${token ? `&token=${encodeURIComponent(token)}` : ''}`
}

export async function apiGetRecycleBin(): Promise<RecycleItem[] | null> {
  return apiGet<RecycleItem[]>('/api/recycle-bin')
}

export async function apiRestoreRecycleItems(ids: number[]): Promise<{ restored: number } | null> {
  return apiPost<{ restored: number }>('/api/recycle-bin/restore', { ids })
}

export async function apiDeleteRecycleItems(ids: number[]): Promise<{ deleted: number } | null> {
  return apiPost<{ deleted: number }>('/api/recycle-bin/delete', { ids })
}

export async function apiClearRecycleBin(): Promise<{ cleared: number } | null> {
  return apiPost<{ cleared: number }>('/api/recycle-bin/clear', {})
}

// 无用资源扫描/清理
export interface StaleResourceGroup {
  count: number
  sizeBytes: number
  files: string[]
}

export interface StaleScanResult {
  thumbnails: StaleResourceGroup
  vectorThumbnails: StaleResourceGroup
  vectors: StaleResourceGroup
}

export interface StaleCleanupResult {
  thumbnails: StaleResourceGroup
  vectorThumbnails: StaleResourceGroup
  vectors: StaleResourceGroup
  totalFreedBytes: number
}

export async function apiScanStaleResources(): Promise<StaleScanResult | null> {
  return apiGet<StaleScanResult>('/api/system/stale-scan')
}

export async function apiCleanupStaleResources(): Promise<StaleCleanupResult | null> {
  return apiPost<StaleCleanupResult>('/api/system/stale-cleanup', {})
}

export async function apiImportGallery(file: File): Promise<{ imported: number } | null> {
  const form = new FormData()
  form.append('file', file)
  const token = getToken()
  const res = await fetch(BASE + '/api/gallery/import', {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    body: form,
  })
  const data = await res.json()
  if (data.code !== 0) throw new Error(data.message || '导入失败')
  return data.data as { imported: number } | null
}

// 类型定义
export interface LoginStatus {
  needSetup: boolean
}

export interface SystemStatus {
  version: string
  username: string
}


// 文件管理
export interface FileItem {
  name: string
  path: string
  isDir: boolean
  size: number
  modTime: string
}

// 相册媒体
export interface MediaItem {
  name: string
  path: string
  type: 'image' | 'video'
  size: number
  modTime: string
}

// 回收站文件
export interface RecycleItem {
  id: number
  name: string
  isVideo: boolean
  isDir: boolean
  hasThumb: boolean
  deletedAt: string
  expireAt: string
}

// 系统信息
export interface AIServiceStatus {
  status: 'ready' | 'loading' | 'unavailable' | 'disabled' | string
  message?: string
  version?: string
  device?: string
}

export interface AIStatus {
  enabled: boolean
  model: AIServiceStatus
  qdrant: AIServiceStatus
}

export interface SystemInfo {
  ai: AIStatus
  os: string
  arch: string
  cpuCores: number
  memoryTotal: number
  memoryUsed: number
  memoryFree: number
  diskTotal: number
  diskUsed: number
  diskFree: number
  uptime: number
  procMem: number
  procMemSys: number
  cpuUsage: number
  procCPU: number
  dbSize: number
  dbSizeString: string
}
