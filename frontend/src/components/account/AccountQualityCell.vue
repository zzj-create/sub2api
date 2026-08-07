<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import type { Account, GrokAccountQualitySnapshot } from '@/types'

const props = defineProps<{
  account: Account
}>()

const { t } = useI18n()

const snapshot = computed<GrokAccountQualitySnapshot | null>(() => props.account.grok_quality ?? null)

const statusClass = (value: string): string => {
  switch (value) {
    case 'healthy':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300'
    case 'soft':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-900/25 dark:text-amber-300'
    case 'hard':
    case 'error':
      return 'bg-red-50 text-red-700 dark:bg-red-900/25 dark:text-red-300'
    case 'ignored':
      return 'bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-gray-300'
    default:
      return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
  }
}

const statusLabel = (value: string): string => {
  const key = `admin.accounts.grokQuality.status.${value}`
  const translated = t(key)
  return translated === key ? value : translated
}

const sourceLabel = (value: string): string => {
  const key = `admin.accounts.grokQuality.source.${value}`
  const translated = t(key)
  return translated === key ? value : translated
}

const formatTPS = (value: number | null | undefined): string => {
  if (value == null || !Number.isFinite(Number(value)) || Number(value) <= 0) return '-'
  return `${Number(value).toLocaleString(undefined, { maximumFractionDigits: 2 })} t/s`
}

const formatCount = (value: number | null | undefined): string => {
  if (value == null || !Number.isFinite(Number(value)) || Number(value) <= 0) return '-'
  return Number(value).toLocaleString()
}

const formatMilliseconds = (value: number | null | undefined): string => {
  if (value == null || !Number.isFinite(Number(value)) || Number(value) <= 0) return '-'
  return `${Number(value).toLocaleString()} ms`
}
</script>

<template>
  <span v-if="!snapshot" class="text-sm text-gray-400 dark:text-dark-500">
    {{ account.platform === 'grok' ? t('admin.accounts.grokQuality.notObserved') : '-' }}
  </span>
  <HelpTooltip v-else trigger="click" width-class="w-80">
    <template #trigger>
      <button
        type="button"
        class="inline-flex min-w-0 items-center gap-1.5 rounded px-1 py-0.5 text-left hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-primary-500 dark:hover:bg-dark-700"
        :aria-label="t('admin.accounts.grokQuality.openDetails')"
        data-test="account-quality-trigger"
      >
        <span :class="['inline-flex rounded px-1.5 py-0.5 text-[11px] font-medium', statusClass(snapshot.quality_class)]">
          {{ statusLabel(snapshot.quality_class) }}
        </span>
        <span v-if="snapshot.output_tps > 0" class="font-mono text-xs text-gray-700 dark:text-gray-200">
          {{ formatTPS(snapshot.output_tps) }}
        </span>
        <Icon name="infoCircle" size="xs" class="shrink-0 text-gray-400" />
      </button>
    </template>

    <div class="space-y-2 pr-2" data-test="account-quality-details">
      <div class="flex items-center justify-between gap-3 border-b border-white/15 pb-2">
        <span class="font-medium">{{ t('admin.accounts.grokQuality.title') }}</span>
        <span :class="['rounded px-1.5 py-0.5 text-[11px] font-medium', statusClass(snapshot.quality_class)]">
          {{ statusLabel(snapshot.quality_class) }}
        </span>
      </div>
      <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5">
        <dt class="text-gray-300">{{ t('admin.accounts.grokQuality.tps') }}</dt>
        <dd class="text-right font-mono">{{ formatTPS(snapshot.output_tps) }}</dd>
        <dt class="text-gray-300">{{ t('admin.accounts.grokQuality.outputTokens') }}</dt>
        <dd class="text-right">{{ formatCount(snapshot.output_tokens) }}</dd>
        <dt class="text-gray-300">{{ t('admin.accounts.grokQuality.duration') }}</dt>
        <dd class="text-right">{{ formatMilliseconds(snapshot.duration_ms) }}</dd>
        <dt class="text-gray-300">{{ t('admin.accounts.grokQuality.firstToken') }}</dt>
        <dd class="text-right">{{ formatMilliseconds(snapshot.first_token_ms) }}</dd>
        <dt class="text-gray-300">{{ t('admin.accounts.grokQuality.thinking') }}</dt>
        <dd class="text-right">
          {{ snapshot.has_thinking == null ? '-' : snapshot.has_thinking ? t('common.yes') : t('common.no') }}
        </dd>
        <dt class="text-gray-300">{{ t('admin.accounts.grokQuality.sourceLabel') }}</dt>
        <dd class="text-right">{{ sourceLabel(snapshot.source) }}</dd>
        <dt class="text-gray-300">{{ t('admin.accounts.grokQuality.observedAt') }}</dt>
        <dd class="text-right">{{ formatDateTime(snapshot.observed_at) || '-' }}</dd>
        <dt class="text-gray-300">{{ t('admin.accounts.grokQuality.pool') }}</dt>
        <dd class="truncate text-right">{{ snapshot.pool_name || `#${snapshot.pool_id}` }}</dd>
        <dt class="text-gray-300">{{ t('admin.accounts.grokQuality.proxy') }}</dt>
        <dd class="truncate text-right">{{ snapshot.proxy_name || `#${snapshot.proxy_id}` }}</dd>
        <template v-if="snapshot.http_status">
          <dt class="text-gray-300">{{ t('admin.accounts.grokQuality.httpStatus') }}</dt>
          <dd class="text-right">{{ snapshot.http_status }}</dd>
        </template>
      </dl>
      <p v-if="snapshot.reason" class="break-words border-t border-white/15 pt-2 text-gray-300" data-test="account-quality-reason">
        {{ snapshot.reason }}
      </p>
    </div>
  </HelpTooltip>
</template>
