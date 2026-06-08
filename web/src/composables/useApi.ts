const BASE = ''

async function request<T>(path: string, opts: RequestInit = {}): Promise<T | null> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(opts.headers as Record<string, string> || {}),
  }
  const res = await fetch(BASE + path, { ...opts, headers })
  if (res.status === 401) return null
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

// 类型定义
export interface LoginStatus {
  needSetup: boolean
}

export interface LoginResponse {
  token: string
}

export interface SystemStatus {
  version: string
  username: string
}

export interface NetInterface {
  name: string
  address: string[]
}

export interface DnsConfig {
  name: string
  dnsName: string
  dnsId: string
  dnsSecret: string
  dnsExtParam: string
  ttl: string
  ipv4Enable: boolean
  ipv4GetType: string
  ipv4Url: string
  ipv4NetInterface: string
  ipv4Cmd: string
  ipv4Domains: string
  ipv6Enable: boolean
  ipv6GetType: string
  ipv6Url: string
  ipv6NetInterface: string
  ipv6Cmd: string
  ipv6Reg: string
  ipv6Domains: string
  httpInterface: string
}

export interface AppConfig {
  dnsConf: DnsConfig[]
  notAllowWanAccess: boolean
  username: string
  webhookUrl: string
  webhookRequestBody: string
  webhookHeaders: string
  ipv4Interfaces: NetInterface[]
  ipv6Interfaces: NetInterface[]
}

export const DNS_PROVIDERS = [
  { value: 'alidns', label: '阿里云' },
  { value: 'tencentcloud', label: '腾讯云' },
  { value: 'dnspod', label: 'DNSPod' },
  { value: 'cloudflare', label: 'Cloudflare' },
  { value: 'huaweicloud', label: '华为云' },
  { value: 'callback', label: 'Callback' },
] as const
