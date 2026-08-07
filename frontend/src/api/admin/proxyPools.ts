import { apiClient } from '../client'
import type {
  ProxyPool,
  ProxyPoolWithStats,
  ProxyPoolProxy,
  ProxyPoolGroup,
  ProxyPoolRebindLog,
  ProxyPoolBindResult,
  ProxyPoolGroupBindResult,
  ProxyPoolGroupUnbindResult
} from '@/types'

export type ProxyPoolQualityPolicyInput = Partial<{
  quality_mode: ProxyPool['quality_mode']
  active_interval_seconds: number
  passive_window_seconds: number
  quarantine_seconds: number
  soft_tps: number
  hard_tps: number
  consecutive_soft: number
  consecutive_errors: number
  min_healthy_proxies: number
  min_generation_ms: number
  min_output_tokens: number
  quality_model: string
  disable_account_on_hard: boolean
  thinking_guard: boolean
  consecutive_missing_thinking: number
  thinking_cross_verify: boolean
  soft_cross_verify: boolean
  max_output_tokens_probe: number
}>

export async function list(): Promise<ProxyPoolWithStats[]> {
  const { data } = await apiClient.get<ProxyPoolWithStats[]>('/admin/proxy-pools')
  return data
}

export async function get(id: number): Promise<ProxyPool> {
  const { data } = await apiClient.get<ProxyPool>(`/admin/proxy-pools/${id}`)
  return data
}

export async function create(input: {
  name: string
  description?: string | null
  status?: 'active' | 'disabled'
  health_interval_seconds?: number
  failure_threshold?: number
  auto_rebind?: boolean
  quality_policy?: ProxyPoolQualityPolicyInput
}): Promise<ProxyPool> {
  const { data } = await apiClient.post<ProxyPool>('/admin/proxy-pools', input)
  return data
}

export async function update(id: number, input: Partial<{
  name: string
  description: string | null
  status: 'active' | 'disabled'
  health_interval_seconds: number
  failure_threshold: number
  auto_rebind: boolean
  quality_policy: ProxyPoolQualityPolicyInput
}>): Promise<ProxyPool> {
  const { data } = await apiClient.put<ProxyPool>(`/admin/proxy-pools/${id}`, input)
  return data
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/proxy-pools/${id}`)
}

export async function listProxies(id: number): Promise<ProxyPoolProxy[]> {
  const { data } = await apiClient.get<ProxyPoolProxy[]>(`/admin/proxy-pools/${id}/proxies`)
  return data
}

export async function listGroups(id: number): Promise<ProxyPoolGroup[]> {
  const { data } = await apiClient.get<ProxyPoolGroup[]>(`/admin/proxy-pools/${id}/groups`)
  return data
}

export async function listGroupOptions(id: number): Promise<ProxyPoolGroup[]> {
  const { data } = await apiClient.get<ProxyPoolGroup[]>(`/admin/proxy-pools/${id}/group-options`)
  return data
}

export async function bindGroups(id: number, groupIds: number[]): Promise<ProxyPoolGroupBindResult> {
  const { data } = await apiClient.post<ProxyPoolGroupBindResult>(`/admin/proxy-pools/${id}/groups`, { group_ids: groupIds })
  return data
}

export async function unbindGroups(id: number, groupIds: number[]): Promise<ProxyPoolGroupUnbindResult> {
  const { data } = await apiClient.delete<ProxyPoolGroupUnbindResult>(`/admin/proxy-pools/${id}/groups`, { data: { group_ids: groupIds } })
  return data
}

export async function assignProxies(id: number, proxyIds: number[]): Promise<number> {
  const { data } = await apiClient.post<{ assigned: number }>(`/admin/proxy-pools/${id}/proxies`, { proxy_ids: proxyIds })
  return data.assigned ?? 0
}

export async function removeProxies(id: number, proxyIds: number[]): Promise<number> {
  const { data } = await apiClient.delete<{ removed: number }>(`/admin/proxy-pools/${id}/proxies`, { data: { proxy_ids: proxyIds } })
  return data.removed ?? 0
}

export async function bindAccounts(id: number, accountIds: number[]): Promise<ProxyPoolBindResult> {
  const { data } = await apiClient.post<ProxyPoolBindResult>(`/admin/proxy-pools/${id}/accounts`, { account_ids: accountIds })
  return data
}

export async function unbindAccounts(id: number, accountIds: number[]): Promise<number> {
  const { data } = await apiClient.delete<{ unbound: number }>(`/admin/proxy-pools/${id}/accounts`, { data: { account_ids: accountIds } })
  return data.unbound ?? 0
}

export async function rebind(id: number): Promise<{ started: boolean; already_running: boolean }> {
  const { data } = await apiClient.post<{
    rebound_accounts: number
    started?: boolean
    already_running?: boolean
  }>(`/admin/proxy-pools/${id}/rebind`)
  return {
    started: data.started !== false,
    already_running: data.already_running === true
  }
}

export async function rebindLogs(id: number, limit = 50): Promise<ProxyPoolRebindLog[]> {
  const { data } = await apiClient.get<ProxyPoolRebindLog[]>(`/admin/proxy-pools/${id}/rebind-logs`, { params: { limit } })
  return data
}

export default {
  list,
  get,
  create,
  update,
  remove,
  listProxies,
  listGroups,
  listGroupOptions,
  bindGroups,
  unbindGroups,
  assignProxies,
  removeProxies,
  bindAccounts,
  unbindAccounts,
  rebind,
  rebindLogs
}
