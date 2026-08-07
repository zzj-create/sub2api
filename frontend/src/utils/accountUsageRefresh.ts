import type { Account } from '@/types'

const normalizeUsageRefreshValue = (value: unknown): string => {
  if (value == null) return ''
  return String(value)
}

export const buildOpenAIUsageRefreshKey = (account: Pick<Account, 'id' | 'platform' | 'type' | 'updated_at' | 'last_used_at' | 'rate_limit_reset_at' | 'extra'>): string => {
  if (account.platform !== 'openai' || account.type !== 'oauth') {
    return ''
  }

  const extra = account.extra ?? {}
  return [
    account.id,
    account.updated_at,
    account.last_used_at,
    account.rate_limit_reset_at,
    extra.codex_usage_updated_at,
    extra.codex_5h_used_percent,
    extra.codex_5h_reset_at,
    extra.codex_5h_reset_after_seconds,
    extra.codex_5h_window_minutes,
    extra.codex_7d_used_percent,
    extra.codex_7d_reset_at,
    extra.codex_7d_reset_after_seconds,
    extra.codex_7d_window_minutes
  ].map(normalizeUsageRefreshValue).join('|')
}

export const buildGrokQualityRefreshKey = (account: Pick<Account, 'platform' | 'grok_quality'>): string => {
  if (account.platform !== 'grok' || !account.grok_quality) {
    return ''
  }

  const quality = account.grok_quality
  return [
    quality.account_id,
    quality.pool_id,
    quality.pool_name,
    quality.proxy_id,
    quality.proxy_name,
    quality.quality_class,
    quality.output_tps,
    quality.output_tokens,
    quality.duration_ms,
    quality.first_token_ms,
    quality.has_thinking,
    quality.source,
    quality.reason,
    quality.error_kind,
    quality.http_status,
    quality.observed_at
  ].map(normalizeUsageRefreshValue).join('|')
}
