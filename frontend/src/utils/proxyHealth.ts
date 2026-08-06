import type { Proxy, ProxyPoolProxy, ProxyQualityCheckResult } from '@/types'

export type ProxyHealthFilter = 'all' | 'healthy' | 'invalid' | 'pending'
export type ProxyOperationalState = Exclude<ProxyHealthFilter, 'all'>

const failedGrokStatuses = new Set(['warn', 'fail', 'failed', 'challenge'])

export function getProxyPoolMemberState(
  proxy: Pick<ProxyPoolProxy, 'status' | 'pool_health' | 'grok_quality_status'>
): ProxyOperationalState {
  if (
    proxy.status !== 'active' ||
    proxy.pool_health === 'unhealthy' ||
    failedGrokStatuses.has(proxy.grok_quality_status)
  ) {
    return 'invalid'
  }

  if (proxy.pool_health === 'healthy' && proxy.grok_quality_status === 'pass') {
    return 'healthy'
  }

  return 'pending'
}

export function isKnownInvalidProxy(
  proxy: Pick<Proxy, 'status' | 'latency_status' | 'quality_status' | 'grok_quality_status'>
): boolean {
  if (
    proxy.status !== 'active' ||
    proxy.latency_status === 'failed'
  ) {
    return true
  }

  if (proxy.grok_quality_status && proxy.grok_quality_status !== 'unknown') {
    return failedGrokStatuses.has(proxy.grok_quality_status)
  }

  return proxy.quality_status === 'failed' || proxy.quality_status === 'challenge'
}

export function hasPassedGrokQuality(
  result: Pick<ProxyQualityCheckResult, 'items'>
): boolean {
  const base = result.items.find((item) => item.target === 'base_connectivity')
  const grok = result.items.find((item) => item.target === 'grok')
  return base?.status === 'pass' && grok?.status === 'pass'
}
