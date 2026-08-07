import { describe, expect, it } from 'vitest'
import { buildGrokQualityRefreshKey, buildOpenAIUsageRefreshKey } from '../accountUsageRefresh'

describe('buildOpenAIUsageRefreshKey', () => {
  it('会在 codex 快照变化时生成不同 key', () => {
    const base = {
      id: 1,
      platform: 'openai',
      type: 'oauth',
      updated_at: '2026-03-07T10:00:00Z',
      last_used_at: '2026-03-07T09:59:00Z',
      extra: {
        codex_usage_updated_at: '2026-03-07T10:00:00Z',
        codex_5h_used_percent: 0,
        codex_7d_used_percent: 0
      }
    } as any

    const next = {
      ...base,
      extra: {
        ...base.extra,
        codex_usage_updated_at: '2026-03-07T10:01:00Z',
        codex_5h_used_percent: 100
      }
    }

    expect(buildOpenAIUsageRefreshKey(base)).not.toBe(buildOpenAIUsageRefreshKey(next))
  })

  it('会在 last_used_at 变化时生成不同 key', () => {
    const base = {
      id: 3,
      platform: 'openai',
      type: 'oauth',
      updated_at: '2026-03-07T10:00:00Z',
      last_used_at: '2026-03-07T10:00:00Z',
      extra: {
        codex_usage_updated_at: '2026-03-07T10:00:00Z',
        codex_5h_used_percent: 12,
        codex_7d_used_percent: 24
      }
    } as any

    const next = {
      ...base,
      last_used_at: '2026-03-07T10:02:00Z'
    }

    expect(buildOpenAIUsageRefreshKey(base)).not.toBe(buildOpenAIUsageRefreshKey(next))
  })

  it('非 OpenAI OAuth 账号返回空 key', () => {
    expect(buildOpenAIUsageRefreshKey({
      id: 2,
      platform: 'anthropic',
      type: 'oauth',
      updated_at: '2026-03-07T10:00:00Z',
      last_used_at: '2026-03-07T10:00:00Z',
      extra: {}
    } as any)).toBe('')
  })
})

describe('buildGrokQualityRefreshKey', () => {
  it('changes when an account receives a new quality observation', () => {
    const base = {
      platform: 'grok',
      grok_quality: {
        account_id: 7,
        pool_id: 3,
        proxy_id: 11,
        quality_class: 'healthy',
        output_tps: 42.5,
        output_tokens: 64,
        duration_ms: 1800,
        first_token_ms: 220,
        has_thinking: true,
        source: 'passive',
        observed_at: '2026-08-07T05:00:00Z'
      }
    } as any
    const next = {
      ...base,
      grok_quality: {
        ...base.grok_quality,
        output_tps: 118.75,
        observed_at: '2026-08-07T05:10:00Z'
      }
    }

    expect(buildGrokQualityRefreshKey(base)).not.toBe(buildGrokQualityRefreshKey(next))
  })

  it('distinguishes an unobserved account from an observed account', () => {
    const unobserved = { platform: 'grok' } as any
    const observed = {
      platform: 'grok',
      grok_quality: {
        account_id: 7,
        pool_id: 3,
        proxy_id: 11,
        quality_class: 'healthy',
        output_tps: 42.5,
        output_tokens: 64,
        duration_ms: 1800,
        first_token_ms: 220,
        source: 'active',
        observed_at: '2026-08-07T05:00:00Z'
      }
    } as any

    expect(buildGrokQualityRefreshKey(unobserved)).toBe('')
    expect(buildGrokQualityRefreshKey(observed)).not.toBe('')
  })
})
