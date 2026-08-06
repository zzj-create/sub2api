import { describe, expect, it } from 'vitest'

import {
  getProxyPoolMemberState,
  hasPassedGrokQuality,
  isKnownInvalidProxy
} from '../proxyHealth'

describe('proxy health helpers', () => {
  it('classifies pool members using both connectivity and Grok quality', () => {
    expect(getProxyPoolMemberState({ status: 'active', pool_health: 'healthy', grok_quality_status: 'pass' })).toBe('healthy')
    expect(getProxyPoolMemberState({ status: 'active', pool_health: 'healthy', grok_quality_status: 'unknown' })).toBe('pending')
    expect(getProxyPoolMemberState({ status: 'active', pool_health: 'unknown', grok_quality_status: 'pass' })).toBe('pending')
    expect(getProxyPoolMemberState({ status: 'active', pool_health: 'healthy', grok_quality_status: 'warn' })).toBe('invalid')
    expect(getProxyPoolMemberState({ status: 'active', pool_health: 'unhealthy', grok_quality_status: 'pass' })).toBe('invalid')
    expect(getProxyPoolMemberState({ status: 'inactive', pool_health: 'healthy', grok_quality_status: 'pass' })).toBe('invalid')
  })

  it('selects only known invalid proxies and leaves unchecked proxies pending', () => {
    expect(isKnownInvalidProxy({ status: 'active', latency_status: undefined, quality_status: undefined, grok_quality_status: undefined })).toBe(false)
    expect(isKnownInvalidProxy({ status: 'active', latency_status: 'success', quality_status: 'healthy', grok_quality_status: 'pass' })).toBe(false)
    expect(isKnownInvalidProxy({ status: 'active', latency_status: 'success', quality_status: 'healthy', grok_quality_status: 'challenge' })).toBe(true)
    expect(isKnownInvalidProxy({ status: 'active', latency_status: 'failed', quality_status: 'healthy', grok_quality_status: 'pass' })).toBe(true)
    expect(isKnownInvalidProxy({ status: 'active', latency_status: undefined, quality_status: 'failed', grok_quality_status: undefined })).toBe(true)
    expect(isKnownInvalidProxy({ status: 'active', latency_status: 'success', quality_status: 'failed', grok_quality_status: 'pass' })).toBe(false)
    expect(isKnownInvalidProxy({ status: 'expired', latency_status: 'success', quality_status: 'healthy', grok_quality_status: 'pass' })).toBe(true)
  })

  it('requires an explicit Grok pass from a live quality result', () => {
    expect(hasPassedGrokQuality({ items: [
      { target: 'base_connectivity', status: 'pass' },
      { target: 'grok', status: 'pass' }
    ] })).toBe(true)
    expect(hasPassedGrokQuality({ items: [
      { target: 'base_connectivity', status: 'pass' },
      { target: 'grok', status: 'warn' }
    ] })).toBe(false)
    expect(hasPassedGrokQuality({ items: [{ target: 'base_connectivity', status: 'pass' }] })).toBe(false)
    expect(hasPassedGrokQuality({ items: [
      { target: 'base_connectivity', status: 'fail' },
      { target: 'grok', status: 'pass' }
    ] })).toBe(false)
  })
})
