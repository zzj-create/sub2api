<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center justify-end gap-2">
          <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadPools">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button class="btn btn-primary" @click="openCreate">
            <Icon name="plus" size="md" class="mr-2" />
            {{ t('admin.proxyPools.create') }}
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="pools" :loading="loading" row-key="id">
          <template #cell-name="{ row }">
            <button class="text-left font-medium text-gray-900 hover:text-primary-600 dark:text-white dark:hover:text-primary-400" @click="openDetail(row)">
              {{ row.name }}
            </button>
            <div v-if="row.description" class="mt-0.5 max-w-xs truncate text-xs text-gray-500 dark:text-gray-400">{{ row.description }}</div>
          </template>
          <template #cell-status="{ value }">
            <span :class="['badge', value === 'active' ? 'badge-success' : 'badge-gray']">
              {{ value === 'active' ? t('admin.proxyPools.active') : t('admin.proxyPools.disabled') }}
            </span>
          </template>
          <template #cell-health="{ row }">
            <div class="flex items-center gap-2 text-sm">
              <span class="font-medium text-emerald-600 dark:text-emerald-400">{{ row.healthy_proxy_count }}</span>
              <span class="text-gray-300 dark:text-dark-500">/</span>
              <span :class="row.unhealthy_proxy_count ? 'font-medium text-red-600 dark:text-red-400' : 'text-gray-400'">{{ row.unhealthy_proxy_count }}</span>
              <span class="text-xs text-gray-400">{{ t('admin.proxyPools.healthyRatio') }}</span>
            </div>
          </template>
          <template #cell-auto_rebind="{ value }">
            <span :class="['badge', value ? 'badge-primary' : 'badge-gray']">{{ value ? t('common.enabled') : t('common.disabled') }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-1">
              <button class="rounded p-2 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700" :title="t('admin.proxyPools.details')" @click="openDetail(row)">
                <Icon name="server" size="sm" />
              </button>
              <button class="rounded p-2 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700" :title="t('common.edit')" @click="openEdit(row)">
                <Icon name="edit" size="sm" />
              </button>
              <button class="rounded p-2 text-gray-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20" :title="t('common.delete')" @click="deletingPool = row">
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>
          <template #empty>
            <div class="py-12 text-center text-gray-500 dark:text-gray-400">
              <Icon name="server" size="xl" class="mx-auto mb-3" />
              <p>{{ t('admin.proxyPools.noPools') }}</p>
            </div>
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <BaseDialog :show="showForm" :title="editingPool ? t('admin.proxyPools.edit') : t('admin.proxyPools.create')" width="wide" @close="showForm = false">
      <form id="proxy-pool-form" class="space-y-5" @submit.prevent="savePool">
        <div>
          <label class="input-label">{{ t('admin.proxyPools.name') }}</label>
          <input v-model.trim="form.name" class="input" maxlength="100" required />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxyPools.descriptionLabel') }}</label>
          <textarea v-model.trim="form.description" class="input" rows="3"></textarea>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div>
            <label class="input-label">{{ t('admin.proxyPools.healthInterval') }}</label>
            <input v-model.number="form.health_interval_seconds" class="input" type="number" min="30" max="86400" required />
          </div>
          <div>
            <label class="input-label">{{ t('admin.proxyPools.failureThreshold') }}</label>
            <input v-model.number="form.failure_threshold" class="input" type="number" min="1" max="10" required />
          </div>
          <div>
            <label class="input-label">{{ t('admin.proxyPools.status') }}</label>
            <Select v-model="form.status" :options="statusOptions" />
          </div>
        </div>
        <div class="border-t border-gray-200 pt-4 dark:border-dark-600">
          <div class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.proxyPools.qualityGuard') }}</div>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <label class="input-label">{{ t('admin.proxyPools.qualityMode') }}</label>
              <Select v-model="form.quality_mode" :options="qualityModeOptions" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.activeInterval') }}</label>
              <input v-model.number="form.active_interval_seconds" class="input" type="number" min="60" max="604800" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.passiveWindow') }}</label>
              <input v-model.number="form.passive_window_seconds" class="input" type="number" min="1" max="86400" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.quarantineSeconds') }}</label>
              <input v-model.number="form.quarantine_seconds" class="input" type="number" min="30" max="86400" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.softTps') }}</label>
              <input v-model.number="form.soft_tps" class="input" type="number" min="1" max="1000000" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.hardTps') }}</label>
              <input v-model.number="form.hard_tps" class="input" type="number" min="1" max="1000000" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.minHealthyProxies') }}</label>
              <input v-model.number="form.min_healthy_proxies" class="input" type="number" min="1" max="1000" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.consecutiveSoft') }}</label>
              <input v-model.number="form.consecutive_soft" class="input" type="number" min="1" max="50" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.consecutiveErrors') }}</label>
              <input v-model.number="form.consecutive_errors" class="input" type="number" min="1" max="50" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.minGenerationMs') }}</label>
              <input v-model.number="form.min_generation_ms" class="input" type="number" min="100" max="86400000" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.minOutputTokens') }}</label>
              <input v-model.number="form.min_output_tokens" class="input" type="number" min="1" max="1000000" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.maxProbeTokens') }}</label>
              <input v-model.number="form.max_output_tokens_probe" class="input" type="number" min="1" max="4096" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.proxyPools.missingThinkingCount') }}</label>
              <input v-model.number="form.consecutive_missing_thinking" class="input" type="number" min="1" max="50" required :disabled="!form.thinking_guard" />
            </div>
            <div class="sm:col-span-2">
              <label class="input-label">{{ t('admin.proxyPools.qualityModel') }}</label>
              <input v-model.trim="form.quality_model" class="input" maxlength="100" required />
            </div>
          </div>
          <div class="mt-4 flex flex-wrap gap-x-6 gap-y-3">
            <label class="flex items-center gap-3 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="form.thinking_guard" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" type="checkbox" />
              <span>{{ t('admin.proxyPools.thinkingGuard') }}</span>
            </label>
            <label class="flex items-center gap-3 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="form.thinking_cross_verify" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" type="checkbox" :disabled="!form.thinking_guard" />
              <span>{{ t('admin.proxyPools.thinkingCrossVerify') }}</span>
            </label>
            <label class="flex items-center gap-3 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="form.soft_cross_verify" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" type="checkbox" />
              <span>{{ t('admin.proxyPools.softCrossVerify') }}</span>
            </label>
            <label class="flex items-center gap-3 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="form.disable_account_on_hard" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" type="checkbox" />
              <span>{{ t('admin.proxyPools.disableAccountOnHard') }}</span>
            </label>
          </div>
        </div>
        <label class="flex items-center gap-3 border-t border-gray-200 pt-4 dark:border-dark-600">
          <input v-model="form.auto_rebind" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" type="checkbox" />
          <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.proxyPools.autoRebind') }}</span>
        </label>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="showForm = false">{{ t('common.cancel') }}</button>
          <button type="submit" form="proxy-pool-form" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.save') }}</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="!!detailPool" :title="detailPool?.name || ''" width="extra-wide" @close="closeDetail">
      <div v-if="detailPool" class="space-y-6">
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-5">
          <div class="rounded bg-gray-50 px-3 py-2 dark:bg-dark-700">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.proxyPools.proxies') }}</div>
            <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ detailProxies.length }}</div>
          </div>
          <div class="rounded bg-emerald-50 px-3 py-2 dark:bg-emerald-900/20">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.proxyPools.healthy') }}</div>
            <div class="mt-1 text-xl font-semibold text-emerald-600 dark:text-emerald-400">{{ healthyCount }}</div>
          </div>
          <div class="rounded bg-red-50 px-3 py-2 dark:bg-red-900/20">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.proxyPools.unhealthy') }}</div>
            <div class="mt-1 text-xl font-semibold text-red-600 dark:text-red-400">{{ unhealthyCount }}</div>
          </div>
          <div class="rounded bg-primary-50 px-3 py-2 dark:bg-primary-900/20">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.proxyPools.boundAccounts') }}</div>
            <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ boundAccounts }}</div>
          </div>
          <div class="rounded bg-violet-50 px-3 py-2 dark:bg-violet-900/20">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.proxyPools.boundGroups') }}</div>
            <div class="mt-1 text-xl font-semibold text-violet-700 dark:text-violet-300">{{ boundGroups.length }}</div>
          </div>
        </div>

        <div class="border-y border-gray-200 py-3 dark:border-dark-600">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.proxyPools.groups') }}</h3>
            <button class="btn btn-secondary btn-sm" :disabled="bindingGroups" data-test="bind-groups" @click="openBindGroups">
              <Icon name="users" size="sm" class="mr-2" :class="bindingGroups ? 'animate-pulse' : ''" />
              {{ t('admin.proxyPools.bindGroups') }}
            </button>
          </div>
          <div v-if="boundGroups.length" class="mt-3 divide-y divide-gray-100 dark:divide-dark-700">
            <div v-for="group in boundGroups" :key="group.id" class="flex flex-wrap items-center gap-3 py-2">
              <div class="min-w-0 flex-1">
                <div class="truncate font-medium text-gray-900 dark:text-white">{{ group.name }}</div>
                <div class="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                  <span>{{ t(`admin.groups.platforms.${group.platform}`) }}</span>
                  <span>{{ t('admin.proxyPools.groupAccounts', { count: group.account_count }) }}</span>
                </div>
              </div>
              <button class="rounded p-2 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20" :title="t('admin.proxyPools.unbindGroup')" @click="unbindingGroup = group">
                <Icon name="x" size="sm" />
              </button>
            </div>
          </div>
          <div v-else class="mt-3 text-sm text-gray-400">{{ t('admin.proxyPools.noGroups') }}</div>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3 border-y border-gray-200 py-3 dark:border-dark-600">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.proxyPools.members') }}</h3>
          <div class="flex gap-2">
            <button class="btn btn-secondary btn-sm" :disabled="rebinding" @click="runRebind">
              <Icon name="refresh" size="sm" class="mr-2" :class="rebinding ? 'animate-spin' : ''" />
              {{ t('admin.proxyPools.checkNow') }}
            </button>
            <button class="btn btn-primary btn-sm" @click="openAssign">
              <Icon name="plus" size="sm" class="mr-2" />{{ t('admin.proxyPools.addProxies') }}
            </button>
          </div>
        </div>

        <div v-if="detailProxies.length" class="flex flex-wrap items-center gap-2">
          <div class="relative min-w-0 flex-1 sm:max-w-xs">
            <Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model.trim="memberSearch"
              class="input pl-9"
              :placeholder="t('admin.proxyPools.searchMembers')"
              data-test="member-search"
            />
          </div>
          <div class="w-full sm:w-44">
            <Select
              v-model="memberStatusFilter"
              :options="memberFilterOptions"
              data-test="member-status-filter"
            />
          </div>
          <button
            class="btn btn-secondary btn-sm"
            :disabled="invalidMemberProxies.length === 0 || batchRemoving"
            data-test="select-invalid-members"
            @click="selectInvalidMembers"
          >
            <Icon name="filter" size="sm" />
            {{ t('admin.proxyPools.selectInvalid', { count: invalidMemberProxies.length }) }}
          </button>
          <button
            class="btn btn-danger btn-sm"
            :disabled="selectedMemberCount === 0 || batchRemoving"
            data-test="remove-selected-members"
            @click="showBatchRemoveDialog = true"
          >
            <Icon name="x" size="sm" />
            {{ t('admin.proxyPools.removeSelected', { count: selectedMemberCount }) }}
          </button>
        </div>

        <div v-if="filteredDetailProxies.length" class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="border-b border-gray-200 text-left text-xs uppercase text-gray-500 dark:border-dark-600 dark:text-gray-400">
              <tr>
                <th class="w-10 px-3 py-2">
                  <input
                    type="checkbox"
                    class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="allVisibleMembersSelected"
                    :indeterminate="someVisibleMembersSelected"
                    :aria-label="t('common.selectAll')"
                    data-test="select-all-members"
                    @change="toggleAllVisibleMembers(($event.target as HTMLInputElement).checked)"
                  />
                </th>
                <th class="px-3 py-2">{{ t('admin.proxyPools.proxy') }}</th>
                <th class="px-3 py-2">{{ t('admin.proxyPools.health') }}</th>
                <th class="px-3 py-2">{{ t('admin.proxyPools.grokQuality') }}</th>
                <th class="px-3 py-2">{{ t('admin.proxyPools.exit') }}</th>
                <th class="px-3 py-2">{{ t('admin.proxyPools.latency') }}</th>
                <th class="px-3 py-2">{{ t('admin.proxyPools.checkedAt') }}</th>
                <th class="px-3 py-2">{{ t('admin.proxyPools.boundAccounts') }}</th>
                <th class="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
              <tr
                v-for="proxy in filteredDetailProxies"
                :key="proxy.id"
                :class="selectedMemberProxyIds.has(proxy.id) ? 'bg-primary-50/60 dark:bg-primary-900/10' : ''"
                data-test="pool-member-row"
              >
                <td class="px-3 py-3">
                  <input
                    type="checkbox"
                    class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="selectedMemberProxyIds.has(proxy.id)"
                    :aria-label="t('admin.proxyPools.selectProxy', { name: proxy.name })"
                    :data-test="`select-member-${proxy.id}`"
                    @change="toggleMember(proxy.id)"
                  />
                </td>
                <td class="px-3 py-3"><div class="font-medium text-gray-900 dark:text-white">{{ proxy.name }}</div><code class="text-xs text-gray-500">{{ proxy.host }}:{{ proxy.port }}</code></td>
                <td class="px-3 py-3"><span :class="healthClass(proxy.pool_health)">{{ healthLabel(proxy.pool_health) }}</span><div v-if="proxy.pool_failures" class="mt-1 text-xs text-red-500">{{ t('admin.proxyPools.failures', { count: proxy.pool_failures }) }}</div></td>
                <td class="px-3 py-3" :title="proxy.quality_last_reason || proxy.grok_quality_message || undefined">
                  <span :class="grokQualityClass(proxy.grok_quality_status)">{{ grokQualityLabel(proxy.grok_quality_status) }}</span>
                  <div v-if="proxy.grok_quality_http_status" class="mt-1 text-xs text-gray-500">HTTP {{ proxy.grok_quality_http_status }}</div>
                  <div v-if="proxy.quality_class && proxy.quality_class !== 'unknown'" class="mt-1 text-xs text-gray-500">
                    {{ qualityLabel(proxy.quality_class) }}<span v-if="proxy.quality_output_tps"> / {{ Math.round(proxy.quality_output_tps) }} tok/s</span>
                  </div>
                  <div v-if="isActiveQualityQuarantine(proxy)" class="mt-1 text-xs font-medium text-red-600 dark:text-red-400">
                    {{ t('admin.proxyPools.qualityQuarantinedUntil', { time: formatDateTime(proxy.quarantined_until!) }) }}
                  </div>
                  <div v-else-if="isAwaitingQualityRecovery(proxy)" class="mt-1 text-xs font-medium text-amber-600 dark:text-amber-400">
                    {{ t('admin.proxyPools.qualityAwaitingRecovery') }}
                  </div>
                </td>
                <td class="px-3 py-3"><div class="text-gray-700 dark:text-gray-200">{{ proxy.ip_address || '-' }}</div><div v-if="proxy.country" class="mt-0.5 text-xs text-gray-500">{{ proxy.country }}<span v-if="proxy.country_code"> ({{ proxy.country_code }})</span></div></td>
                <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ typeof proxy.latency_ms === 'number' ? `${proxy.latency_ms}ms` : '-' }}</td>
                <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ proxy.pool_checked_at ? formatDateTime(proxy.pool_checked_at) : '-' }}</td>
                <td class="px-3 py-3 text-gray-700 dark:text-gray-200">{{ proxy.account_count }}</td>
                <td class="px-3 py-3 text-right"><button class="rounded p-2 text-gray-400 hover:bg-red-50 hover:text-red-600" :title="t('admin.proxyPools.removeProxy')" @click="removeProxy(proxy.id)"><Icon name="x" size="sm" /></button></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else-if="detailProxies.length" class="border-y border-dashed border-gray-300 py-10 text-center text-sm text-gray-500 dark:border-dark-600">{{ t('admin.proxyPools.noMatchingMembers') }}</div>
        <div v-else class="border-y border-dashed border-gray-300 py-10 text-center text-sm text-gray-500 dark:border-dark-600">{{ t('admin.proxyPools.noMembers') }}</div>

        <div>
          <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.proxyPools.rebindLogs') }}</h3>
          <div v-if="logs.length" class="space-y-2">
            <div v-for="entry in logs" :key="entry.id" class="grid grid-cols-[auto_1fr_auto] items-center gap-3 border-b border-gray-100 py-2 text-sm dark:border-dark-700">
              <span class="text-xs text-gray-400">{{ formatDateTime(entry.created_at) }}</span>
              <span class="flex min-w-0 items-center gap-2 text-gray-700 dark:text-gray-200"><span class="truncate">{{ entry.from_proxy_name || entry.from_proxy_id || '-' }}</span><Icon name="arrowRight" size="xs" class="flex-none text-gray-400" /><span class="truncate">{{ entry.to_proxy_name || entry.to_proxy_id || '-' }}</span></span>
              <span class="badge badge-primary">{{ entry.account_count }}</span>
            </div>
          </div>
          <div v-else class="py-4 text-sm text-gray-400">{{ t('admin.proxyPools.noLogs') }}</div>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="showBindGroups" :title="t('admin.proxyPools.bindGroups')" width="wide" @close="showBindGroups = false">
      <div class="space-y-3">
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.proxyPools.groupBindingHint') }}</p>
        <input v-model.trim="groupSearch" class="input" :placeholder="t('admin.proxyPools.searchGroups')" />
        <div class="max-h-96 overflow-y-auto border-y border-gray-200 py-2 dark:border-dark-600">
          <label v-if="assignableGroups.length" class="sticky top-0 z-10 flex cursor-pointer items-center gap-3 border-b border-gray-200 bg-gray-50 px-2 py-2 text-sm font-medium text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200">
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
              :checked="allVisibleGroupsSelected"
              :indeterminate="someVisibleGroupsSelected"
              :aria-label="t('common.selectAll')"
              data-test="select-all-groups"
              @change="toggleAllVisibleGroups(($event.target as HTMLInputElement).checked)"
            />
            <span>{{ t('common.selectAll') }}</span>
          </label>
          <label
            v-for="group in assignableGroups"
            :key="group.id"
            class="flex items-center gap-3 px-2 py-2"
            :class="group.bound_pool_id && group.bound_pool_id !== detailPool?.id ? 'cursor-not-allowed opacity-60' : 'cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-700'"
          >
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600"
              :checked="selectedGroupIds.has(group.id)"
              :disabled="!!group.bound_pool_id && group.bound_pool_id !== detailPool?.id || group.status !== 'active'"
              :data-test="`bind-group-${group.id}`"
              @change="toggleGroup(group.id)"
            />
            <span class="min-w-0 flex-1 truncate text-sm font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
            <span class="text-xs text-gray-500">{{ t(`admin.groups.platforms.${group.platform}`) }}</span>
            <span class="text-xs text-gray-500">{{ t('admin.proxyPools.groupAccounts', { count: group.account_count }) }}</span>
            <span v-if="group.bound_pool_id && group.bound_pool_id !== detailPool?.id" class="max-w-40 truncate text-xs text-amber-600 dark:text-amber-400" :title="t('admin.proxyPools.groupBoundTo', { name: group.bound_pool_name || group.bound_pool_id })">
              {{ t('admin.proxyPools.groupBoundTo', { name: group.bound_pool_name || group.bound_pool_id }) }}
            </span>
          </label>
          <div v-if="!assignableGroups.length" class="py-8 text-center text-sm text-gray-500">{{ t('admin.proxyPools.noAssignableGroups') }}</div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="showBindGroups = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="!selectedGroupIds.size || bindingGroups" data-test="submit-bind-groups" @click="bindSelectedGroups">
            {{ bindingGroups ? t('common.saving') : t('admin.proxyPools.bindGroups') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showAssign" :title="t('admin.proxyPools.addProxies')" width="wide" @close="showAssign = false">
      <div class="space-y-3">
        <input v-model.trim="proxySearch" class="input" :placeholder="t('admin.proxies.searchProxies')" />
        <div class="max-h-96 overflow-y-auto border-y border-gray-200 py-2 dark:border-dark-600">
          <label v-if="assignableProxies.length" class="sticky top-0 z-10 flex cursor-pointer items-center gap-3 border-b border-gray-200 bg-gray-50 px-2 py-2 text-sm font-medium text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200">
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
              :checked="allVisibleProxiesSelected"
              :indeterminate="someVisibleProxiesSelected"
              :aria-label="t('common.selectAll')"
              data-test="select-all-proxies"
              @change="toggleAllVisibleProxies(($event.target as HTMLInputElement).checked)"
            />
            <span>{{ t('common.selectAll') }}</span>
          </label>
          <label v-for="proxy in assignableProxies" :key="proxy.id" class="flex cursor-pointer items-center gap-3 px-2 py-2 hover:bg-gray-50 dark:hover:bg-dark-700">
            <input type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" :checked="selectedProxyIds.has(proxy.id)" :data-test="`assign-proxy-${proxy.id}`" @change="toggleProxy(proxy.id)" />
            <span class="min-w-0 flex-1 truncate text-sm font-medium text-gray-900 dark:text-white">{{ proxy.name }}</span>
            <code class="text-xs text-gray-500">{{ proxy.host }}:{{ proxy.port }}</code>
          </label>
          <div v-if="!assignableProxies.length" class="py-8 text-center text-sm text-gray-500">{{ t('admin.proxyPools.noAssignableProxies') }}</div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3"><button class="btn btn-secondary" @click="showAssign = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="!selectedProxyIds.size || assigning" @click="assignProxies">{{ assigning ? t('common.saving') : t('admin.proxyPools.addSelected') }}</button></div>
      </template>
    </BaseDialog>

    <ConfirmDialog :show="!!deletingPool" :title="t('admin.proxyPools.delete')" :message="t('admin.proxyPools.deleteConfirm', { name: deletingPool?.name })" :danger="true" @confirm="deletePool" @cancel="deletingPool = null" />
    <ConfirmDialog
      :show="showBatchRemoveDialog"
      :title="t('admin.proxyPools.removeSelectedTitle')"
      :message="t('admin.proxyPools.removeSelectedConfirm', { count: selectedMemberCount })"
      :danger="true"
      @confirm="confirmBatchRemove"
      @cancel="showBatchRemoveDialog = false"
    />
    <ConfirmDialog
      :show="!!unbindingGroup"
      :title="t('admin.proxyPools.unbindGroupsTitle')"
      :message="t('admin.proxyPools.unbindGroupsConfirm', { name: unbindingGroup?.name })"
      :danger="true"
      @confirm="confirmUnbindGroup"
      @cancel="unbindingGroup = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import { getProxyPoolMemberState, type ProxyHealthFilter } from '@/utils/proxyHealth'
import type { Column } from '@/components/common/types'
import type { Proxy, ProxyPoolWithStats, ProxyPoolProxy, ProxyPoolGroup, ProxyPoolRebindLog } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const pools = ref<ProxyPoolWithStats[]>([])
const loading = ref(false)
const saving = ref(false)
const showForm = ref(false)
const editingPool = ref<ProxyPoolWithStats | null>(null)
const deletingPool = ref<ProxyPoolWithStats | null>(null)
const detailPool = ref<ProxyPoolWithStats | null>(null)
const detailProxies = ref<ProxyPoolProxy[]>([])
const boundGroups = ref<ProxyPoolGroup[]>([])
const groupOptions = ref<ProxyPoolGroup[]>([])
const logs = ref<ProxyPoolRebindLog[]>([])
const rebinding = ref(false)
const showAssign = ref(false)
const assigning = ref(false)
const allProxies = ref<Proxy[]>([])
const selectedProxyIds = ref(new Set<number>())
const proxySearch = ref('')
const memberSearch = ref('')
const memberStatusFilter = ref<ProxyHealthFilter>('all')
const selectedMemberProxyIds = ref(new Set<number>())
const showBatchRemoveDialog = ref(false)
const batchRemoving = ref(false)
const showBindGroups = ref(false)
const bindingGroups = ref(false)
const groupSearch = ref('')
const selectedGroupIds = ref(new Set<number>())
const unbindingGroup = ref<ProxyPoolGroup | null>(null)
const unbindingGroups = ref(false)

const columns: Column[] = [
  { key: 'name', label: t('admin.proxyPools.name') },
  { key: 'status', label: t('admin.proxyPools.status') },
  { key: 'health', label: t('admin.proxyPools.health') },
  { key: 'bound_account_count', label: t('admin.proxyPools.boundAccounts') },
  { key: 'bound_group_count', label: t('admin.proxyPools.boundGroups') },
  { key: 'health_interval_seconds', label: t('admin.proxyPools.healthInterval') },
  { key: 'failure_threshold', label: t('admin.proxyPools.failureThreshold') },
  { key: 'auto_rebind', label: t('admin.proxyPools.autoRebind') },
  { key: 'actions', label: '' }
]
const statusOptions = computed(() => [
  { value: 'active', label: t('admin.proxyPools.active') },
  { value: 'disabled', label: t('admin.proxyPools.disabled') }
])
const qualityModeOptions = computed(() => [
  { value: 'hybrid', label: t('admin.proxyPools.qualityHybrid') },
  { value: 'passive', label: t('admin.proxyPools.qualityPassive') },
  { value: 'active', label: t('admin.proxyPools.qualityActive') }
])
const memberFilterOptions = computed(() => [
  { value: 'all', label: t('admin.proxyPools.filterAll') },
  { value: 'healthy', label: t('admin.proxyPools.filterHealthy') },
  { value: 'invalid', label: t('admin.proxyPools.filterInvalid') },
  { value: 'pending', label: t('admin.proxyPools.filterPending') }
])
const form = reactive({
  name: '', description: '', status: 'active', health_interval_seconds: 300, failure_threshold: 2, auto_rebind: true,
  quality_mode: 'hybrid' as 'passive' | 'active' | 'hybrid', active_interval_seconds: 1800, passive_window_seconds: 300,
  quarantine_seconds: 120, soft_tps: 500, hard_tps: 1000, consecutive_soft: 2, consecutive_errors: 2,
  min_healthy_proxies: 1, min_generation_ms: 1000, min_output_tokens: 32, quality_model: 'grok-4.5',
  thinking_guard: true, consecutive_missing_thinking: 1, thinking_cross_verify: true, soft_cross_verify: true,
  max_output_tokens_probe: 384, disable_account_on_hard: false
})
const healthyCount = computed(() => detailProxies.value.filter((proxy) => proxy.pool_health === 'healthy').length)
const unhealthyCount = computed(() => detailProxies.value.filter((proxy) => proxy.pool_health === 'unhealthy').length)
const boundAccounts = computed(() => detailProxies.value.reduce((sum, proxy) => sum + proxy.account_count, 0))
const assignableGroups = computed(() => {
  const currentIds = new Set(boundGroups.value.map((group) => group.id))
  const query = groupSearch.value.toLowerCase()
  return groupOptions.value.filter((group) => {
    if (currentIds.has(group.id)) return false
    if (!query) return true
    return [group.name, group.platform, group.status]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(query))
  })
})
const selectableGroups = computed(() => assignableGroups.value.filter((group) => group.status === 'active' && (!group.bound_pool_id || group.bound_pool_id === detailPool.value?.id)))
const visibleSelectedGroupCount = computed(() => selectableGroups.value.filter((group) => selectedGroupIds.value.has(group.id)).length)
const allVisibleGroupsSelected = computed(() => selectableGroups.value.length > 0 && visibleSelectedGroupCount.value === selectableGroups.value.length)
const someVisibleGroupsSelected = computed(() => visibleSelectedGroupCount.value > 0 && !allVisibleGroupsSelected.value)
const assignableProxies = computed(() => {
  const memberIds = new Set(detailProxies.value.map((proxy) => proxy.id))
  const query = proxySearch.value.toLowerCase()
  return allProxies.value.filter((proxy) => !memberIds.has(proxy.id) && (!query || proxy.name.toLowerCase().includes(query) || proxy.host.toLowerCase().includes(query)))
})
const visibleSelectedProxyCount = computed(() => assignableProxies.value.filter((proxy) => selectedProxyIds.value.has(proxy.id)).length)
const allVisibleProxiesSelected = computed(() => assignableProxies.value.length > 0 && visibleSelectedProxyCount.value === assignableProxies.value.length)
const someVisibleProxiesSelected = computed(() => visibleSelectedProxyCount.value > 0 && !allVisibleProxiesSelected.value)
const invalidMemberProxies = computed(() => detailProxies.value.filter((proxy) => getProxyPoolMemberState(proxy) === 'invalid'))
const filteredDetailProxies = computed(() => {
  const query = memberSearch.value.toLowerCase()
  return detailProxies.value.filter((proxy) => {
    const matchesStatus = memberStatusFilter.value === 'all' || getProxyPoolMemberState(proxy) === memberStatusFilter.value
    if (!matchesStatus) return false
    if (!query) return true
    return [proxy.name, proxy.host, proxy.ip_address, proxy.country, proxy.country_code]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(query))
  })
})
const selectedMemberCount = computed(() => selectedMemberProxyIds.value.size)
const visibleSelectedMemberCount = computed(() => filteredDetailProxies.value.filter((proxy) => selectedMemberProxyIds.value.has(proxy.id)).length)
const allVisibleMembersSelected = computed(() => filteredDetailProxies.value.length > 0 && visibleSelectedMemberCount.value === filteredDetailProxies.value.length)
const someVisibleMembersSelected = computed(() => visibleSelectedMemberCount.value > 0 && !allVisibleMembersSelected.value)

async function loadPools() { loading.value = true; try { pools.value = await adminAPI.proxyPools.list() } catch { appStore.showError(t('admin.proxyPools.loadFailed')) } finally { loading.value = false } }
function resetForm() {
  Object.assign(form, {
    name: '', description: '', status: 'active', health_interval_seconds: 300, failure_threshold: 2, auto_rebind: true,
    quality_mode: 'hybrid', active_interval_seconds: 1800, passive_window_seconds: 300, quarantine_seconds: 120,
    soft_tps: 500, hard_tps: 1000, consecutive_soft: 2, consecutive_errors: 2, min_healthy_proxies: 1,
    min_generation_ms: 1000, min_output_tokens: 32, quality_model: 'grok-4.5', thinking_guard: true,
    consecutive_missing_thinking: 1, thinking_cross_verify: true, soft_cross_verify: true, max_output_tokens_probe: 384,
    disable_account_on_hard: false
  })
}
function openCreate() { editingPool.value = null; resetForm(); showForm.value = true }
function openEdit(pool: ProxyPoolWithStats) {
  editingPool.value = pool
  Object.assign(form, {
    name: pool.name, description: pool.description || '', status: pool.status, health_interval_seconds: pool.health_interval_seconds,
    failure_threshold: pool.failure_threshold, auto_rebind: pool.auto_rebind, quality_mode: pool.quality_mode || 'hybrid',
    active_interval_seconds: pool.active_interval_seconds || 1800, passive_window_seconds: pool.passive_window_seconds || 300,
    quarantine_seconds: pool.quarantine_seconds || 120, soft_tps: pool.soft_tps || 500, hard_tps: pool.hard_tps || 1000,
    consecutive_soft: pool.consecutive_soft || 2, consecutive_errors: pool.consecutive_errors || 2,
    min_healthy_proxies: pool.min_healthy_proxies || 1, min_generation_ms: pool.min_generation_ms || 1000,
    min_output_tokens: pool.min_output_tokens || 32, quality_model: pool.quality_model || 'grok-4.5',
    thinking_guard: pool.thinking_guard !== false, consecutive_missing_thinking: pool.consecutive_missing_thinking || 1,
    thinking_cross_verify: pool.thinking_cross_verify !== false, soft_cross_verify: pool.soft_cross_verify !== false,
    max_output_tokens_probe: pool.max_output_tokens_probe || 384, disable_account_on_hard: pool.disable_account_on_hard === true
  })
  showForm.value = true
}
async function savePool() {
  if (form.soft_tps >= form.hard_tps) {
    appStore.showError(t('admin.proxyPools.qualityThresholdInvalid'))
    return
  }
  saving.value = true
  try {
    const {
      quality_mode, active_interval_seconds, passive_window_seconds, quarantine_seconds, soft_tps, hard_tps,
      consecutive_soft, consecutive_errors, min_healthy_proxies, min_generation_ms, min_output_tokens, quality_model,
      thinking_guard, consecutive_missing_thinking, thinking_cross_verify, soft_cross_verify, max_output_tokens_probe,
      disable_account_on_hard,
      ...base
    } = form
    const payload = {
      ...base,
      status: form.status as 'active' | 'disabled',
      quality_policy: {
        quality_mode, active_interval_seconds, passive_window_seconds, quarantine_seconds, soft_tps, hard_tps,
        consecutive_soft, consecutive_errors, min_healthy_proxies, min_generation_ms, min_output_tokens, quality_model,
        thinking_guard, consecutive_missing_thinking, thinking_cross_verify, soft_cross_verify, max_output_tokens_probe,
        disable_account_on_hard
      }
    }
    if (editingPool.value) await adminAPI.proxyPools.update(editingPool.value.id, payload)
    else await adminAPI.proxyPools.create(payload)
    showForm.value = false
    appStore.showSuccess(t('admin.proxyPools.saved'))
    await loadPools()
  } catch { appStore.showError(t('admin.proxyPools.saveFailed')) } finally { saving.value = false }
}
async function deletePool() { if (!deletingPool.value) return; try { await adminAPI.proxyPools.remove(deletingPool.value.id); deletingPool.value = null; appStore.showSuccess(t('admin.proxyPools.deleted')); await loadPools() } catch { appStore.showError(t('admin.proxyPools.deleteFailed')) } }
async function openDetail(pool: ProxyPoolWithStats) {
  memberSearch.value = ''
  memberStatusFilter.value = 'all'
  selectedMemberProxyIds.value = new Set()
  showBatchRemoveDialog.value = false
  showBindGroups.value = false
  selectedGroupIds.value = new Set()
  groupSearch.value = ''
  unbindingGroup.value = null
  detailPool.value = pool
  try {
    await refreshDetail()
  } catch {
    appStore.showError(t('admin.proxyPools.loadFailed'))
  }
}
function closeDetail() { detailPool.value = null; detailProxies.value = []; boundGroups.value = []; groupOptions.value = []; logs.value = []; memberSearch.value = ''; memberStatusFilter.value = 'all'; selectedMemberProxyIds.value = new Set(); selectedGroupIds.value = new Set(); groupSearch.value = ''; showBindGroups.value = false; showBatchRemoveDialog.value = false; unbindingGroup.value = null }
async function refreshDetail() {
  if (!detailPool.value) return
  const [proxies, rebindEntries, groups] = await Promise.all([adminAPI.proxyPools.listProxies(detailPool.value.id), adminAPI.proxyPools.rebindLogs(detailPool.value.id), adminAPI.proxyPools.listGroups(detailPool.value.id)])
  detailProxies.value = proxies
  logs.value = rebindEntries
  boundGroups.value = groups
  const memberIds = new Set(proxies.map((proxy) => proxy.id))
  selectedMemberProxyIds.value = new Set([...selectedMemberProxyIds.value].filter((id) => memberIds.has(id)))
}
async function openBindGroups() {
  if (!detailPool.value || bindingGroups.value) return
  groupSearch.value = ''
  selectedGroupIds.value = new Set()
  bindingGroups.value = true
  try {
    groupOptions.value = await adminAPI.proxyPools.listGroupOptions(detailPool.value.id)
    showBindGroups.value = true
  } catch {
    appStore.showError(t('admin.proxyPools.loadFailed'))
  } finally {
    bindingGroups.value = false
  }
}
function toggleGroup(id: number) {
  const group = selectableGroups.value.find((candidate) => candidate.id === id)
  if (!group) return
  const next = new Set(selectedGroupIds.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selectedGroupIds.value = next
}
function toggleAllVisibleGroups(checked: boolean) {
  const next = new Set(selectedGroupIds.value)
  for (const group of selectableGroups.value) {
    checked ? next.add(group.id) : next.delete(group.id)
  }
  selectedGroupIds.value = next
}
async function bindSelectedGroups() {
  if (!detailPool.value || selectedGroupIds.value.size === 0 || bindingGroups.value) return
  bindingGroups.value = true
  try {
    const result = await adminAPI.proxyPools.bindGroups(detailPool.value.id, [...selectedGroupIds.value])
    showBindGroups.value = false
    selectedGroupIds.value = new Set()
    appStore.showSuccess(t('admin.proxyPools.bindGroupsSuccess', { groups: result.bound_groups, accounts: result.synced_accounts }))
    await Promise.all([refreshDetail(), loadPools()])
  } catch {
    appStore.showError(t('admin.proxyPools.bindGroupsFailed'))
  } finally {
    bindingGroups.value = false
  }
}
async function confirmUnbindGroup() {
  if (!detailPool.value || !unbindingGroup.value || unbindingGroups.value) return
  const group = unbindingGroup.value
  unbindingGroups.value = true
  try {
    const result = await adminAPI.proxyPools.unbindGroups(detailPool.value.id, [group.id])
    unbindingGroup.value = null
    appStore.showSuccess(t('admin.proxyPools.unbindGroupsSuccess', { accounts: result.detached_accounts }))
    await Promise.all([refreshDetail(), loadPools()])
  } catch {
    appStore.showError(t('admin.proxyPools.unbindGroupsFailed'))
  } finally {
    unbindingGroups.value = false
  }
}
async function runRebind() {
  if (!detailPool.value) return
  rebinding.value = true
  try {
    const result = await adminAPI.proxyPools.rebind(detailPool.value.id)
    appStore.showSuccess(t(result.started ? 'admin.proxyPools.checkStarted' : 'admin.proxyPools.checkRunning'))
    await Promise.all([refreshDetail(), loadPools()])
  } catch {
    appStore.showError(t('admin.proxyPools.rebindFailed'))
  } finally {
    rebinding.value = false
  }
}
async function openAssign() {
  selectedProxyIds.value = new Set()
  proxySearch.value = ''
  try {
    allProxies.value = await adminAPI.proxies.getAll()
    showAssign.value = true
  } catch {
    appStore.showError(t('admin.proxies.failedToLoad'))
  }
}
function toggleProxy(id: number) { const next = new Set(selectedProxyIds.value); next.has(id) ? next.delete(id) : next.add(id); selectedProxyIds.value = next }
function toggleAllVisibleProxies(checked: boolean) {
  const next = new Set(selectedProxyIds.value)
  for (const proxy of assignableProxies.value) {
    checked ? next.add(proxy.id) : next.delete(proxy.id)
  }
  selectedProxyIds.value = next
}
function toggleMember(id: number) { const next = new Set(selectedMemberProxyIds.value); next.has(id) ? next.delete(id) : next.add(id); selectedMemberProxyIds.value = next }
function toggleAllVisibleMembers(checked: boolean) {
  const next = new Set(selectedMemberProxyIds.value)
  for (const proxy of filteredDetailProxies.value) {
    checked ? next.add(proxy.id) : next.delete(proxy.id)
  }
  selectedMemberProxyIds.value = next
}
function selectInvalidMembers() {
  selectedMemberProxyIds.value = new Set(invalidMemberProxies.value.map((proxy) => proxy.id))
  memberStatusFilter.value = 'invalid'
}
async function assignProxies() { if (!detailPool.value) return; assigning.value = true; try { await adminAPI.proxyPools.assignProxies(detailPool.value.id, [...selectedProxyIds.value]); showAssign.value = false; await Promise.all([refreshDetail(), loadPools()]) } catch { appStore.showError(t('admin.proxyPools.assignFailed')) } finally { assigning.value = false } }
async function removeProxy(id: number) { if (!detailPool.value) return; try { await adminAPI.proxyPools.removeProxies(detailPool.value.id, [id]); await Promise.all([refreshDetail(), loadPools()]) } catch { appStore.showError(t('admin.proxyPools.removeFailed')) } }
async function confirmBatchRemove() {
  if (!detailPool.value || selectedMemberProxyIds.value.size === 0 || batchRemoving.value) return
  batchRemoving.value = true
  try {
    const removed = await adminAPI.proxyPools.removeProxies(detailPool.value.id, [...selectedMemberProxyIds.value])
    selectedMemberProxyIds.value = new Set()
    showBatchRemoveDialog.value = false
    appStore.showSuccess(t('admin.proxyPools.removeSelectedDone', { count: removed }))
    await Promise.all([refreshDetail(), loadPools()])
  } catch {
    appStore.showError(t('admin.proxyPools.removeFailed'))
  } finally {
    batchRemoving.value = false
  }
}
function healthClass(health: string) { return ['badge', health === 'healthy' ? 'badge-success' : health === 'unhealthy' ? 'badge-danger' : 'badge-gray'] }
function healthLabel(health: string) { return health === 'healthy' ? t('admin.proxyPools.healthy') : health === 'unhealthy' ? t('admin.proxyPools.unhealthy') : t('admin.proxyPools.unknown') }
function grokQualityClass(status: string) { return ['badge', status === 'pass' ? 'badge-success' : status === 'warn' ? 'badge-warning' : status === 'fail' || status === 'challenge' ? 'badge-danger' : 'badge-gray'] }
function grokQualityLabel(status: string) {
  if (status === 'pass') return t('admin.proxyPools.grokQualityPassed')
  if (status === 'warn') return t('admin.proxyPools.grokQualityWarn')
  if (status === 'challenge') return t('admin.proxyPools.grokQualityChallenge')
  if (status === 'fail') return t('admin.proxyPools.grokQualityFailed')
  return t('admin.proxyPools.grokQualityPending')
}
function qualityLabel(status: string) {
  const key = `admin.proxyPools.qualityClass${status.charAt(0).toUpperCase()}${status.slice(1)}`
  return t(key)
}
function isActiveQualityQuarantine(proxy: ProxyPoolProxy) {
  return !!proxy.quarantined_until && new Date(proxy.quarantined_until).getTime() > Date.now()
}
function isAwaitingQualityRecovery(proxy: ProxyPoolProxy) {
  return !!proxy.quarantined_until && proxy.pool_health === 'unhealthy' && (proxy.quality_class === 'hard' || proxy.quality_class === 'error')
}

onMounted(loadPools)
</script>
