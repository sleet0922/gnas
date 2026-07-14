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
  
  const res = await fetch(BASE + path, { ...opts, headers })
  const data = await res.json()
  if (data.code !== 0 && data.code !== undefined) {
    throw new Error(data.message || '操作失败')
  }
  return data.data as T
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
  const data = await res.json()
  if (data.code !== 0) throw new Error(data.message || '上传失败')
  return null
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

// 系统信息
export interface SystemInfo {
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
