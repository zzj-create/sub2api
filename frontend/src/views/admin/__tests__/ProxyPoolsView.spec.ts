import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

import ProxyPoolsView from '../ProxyPoolsView.vue'

const {
  listPools,
  listProxies,
  listGroups,
  listGroupOptions,
  bindGroups,
  unbindGroups,
  getAllProxies,
  assignProxies,
  removeProxies,
  rebindLogs,
  rebind,
  showError,
  showSuccess
} = vi.hoisted(() => ({
  listPools: vi.fn(),
  listProxies: vi.fn(),
  listGroups: vi.fn(),
  listGroupOptions: vi.fn(),
  bindGroups: vi.fn(),
  unbindGroups: vi.fn(),
  getAllProxies: vi.fn(),
  assignProxies: vi.fn(),
  removeProxies: vi.fn(),
  rebindLogs: vi.fn(),
  rebind: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    proxyPools: {
      list: listPools,
      listProxies,
      rebindLogs,
      rebind,
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn(),
      assignProxies,
      removeProxies,
      listGroups,
      listGroupOptions,
      bindGroups,
      unbindGroups
    },
    proxies: {
      getAll: getAllProxies
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const DataTableStub = defineComponent({
  props: {
    data: { type: Array, default: () => [] }
  },
  setup(props, { slots }) {
    return () => h('div', { class: 'data-table-stub' }, (props.data as Record<string, unknown>[]).map((row) =>
      h('div', { class: 'data-row' }, [
        slots['cell-name']?.({ row, value: row.name }),
        slots['cell-actions']?.({ row })
      ])
    ))
  }
})

const SelectStub = defineComponent({
  props: {
    modelValue: { type: [String, Number], default: '' },
    options: { type: Array, default: () => [] }
  },
  emits: ['update:modelValue'],
  template: `
    <select :value="modelValue" @change="$emit('update:modelValue', $event.target.value)">
      <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
    </select>
  `
})

const ConfirmDialogStub = defineComponent({
  props: {
    show: { type: Boolean, default: false },
    title: { type: String, default: '' },
    message: { type: String, default: '' }
  },
  emits: ['confirm', 'cancel'],
  template: `
    <section v-if="show" data-test="confirm-dialog">
      <span>{{ title }}</span><span>{{ message }}</span>
      <button data-test="confirm-remove" @click="$emit('confirm')">confirm</button>
      <button data-test="cancel-remove" @click="$emit('cancel')">cancel</button>
    </section>
  `
})

function makePool() {
  return {
    id: 1,
    name: 'primary-pool',
    description: 'Primary exits',
    status: 'active',
    health_interval_seconds: 300,
    failure_threshold: 2,
    auto_rebind: true,
    proxy_count: 2,
    healthy_proxy_count: 1,
    unhealthy_proxy_count: 1,
    bound_account_count: 4,
    bound_group_count: 0,
    created_at: '2026-08-04T00:00:00Z',
    updated_at: '2026-08-04T00:00:00Z'
  }
}

function mountView() {
  return mount(ProxyPoolsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
        DataTable: DataTableStub,
        BaseDialog: {
          props: ['show', 'title'],
          template: '<section v-if="show" class="dialog-stub"><slot /><slot name="footer" /></section>'
        },
        ConfirmDialog: ConfirmDialogStub,
        Select: SelectStub,
        Icon: true
      }
    }
  })
}

describe('ProxyPoolsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listPools.mockResolvedValue([makePool()])
    listProxies.mockResolvedValue([
      {
        id: 11,
        name: 'exit-a',
        protocol: 'http',
        host: 'proxy.example',
        port: 8080,
        status: 'active',
        pool_id: 1,
        pool_health: 'healthy',
        pool_failures: 0,
        pool_checked_at: '2026-08-04T00:00:00Z',
        grok_quality_status: 'pass',
        grok_quality_checked_at: '2026-08-04T00:00:00Z',
        grok_quality_http_status: 401,
        grok_quality_message: 'HTTP 401 (target reachable)',
        account_count: 4,
        latency_ms: 42,
        ip_address: '203.0.113.25',
        country: 'Example',
        country_code: 'EX'
      }
    ])
    rebindLogs.mockResolvedValue([])
    listGroups.mockResolvedValue([])
    listGroupOptions.mockResolvedValue([])
    bindGroups.mockResolvedValue({ bound_groups: 0, synced_accounts: 0 })
    unbindGroups.mockResolvedValue({ unbound_groups: 1, detached_accounts: 0 })
    rebind.mockResolvedValue({ started: true, already_running: false })
    getAllProxies.mockResolvedValue([
      { id: 11, name: 'exit-a', host: 'proxy.example', port: 8080 },
      { id: 12, name: 'exit-b', host: 'proxy-b.example', port: 8081 },
      { id: 13, name: 'exit-c', host: 'proxy-c.example', port: 8082 }
    ])
    assignProxies.mockResolvedValue({ assigned: 2 })
    removeProxies.mockResolvedValue(2)
  })

  it('loads proxy pools on mount', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listPools).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('primary-pool')
  })

  it('shows cached exit information in pool details', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button[title="admin.proxyPools.details"]').trigger('click')
    await flushPromises()

    expect(listProxies).toHaveBeenCalledWith(1)
    expect(rebindLogs).toHaveBeenCalledWith(1)
    expect(listGroups).toHaveBeenCalledWith(1)
    expect(wrapper.text()).toContain('203.0.113.25')
    expect(wrapper.text()).toContain('42ms')
    expect(wrapper.text()).toContain('Example')
    expect(wrapper.text()).toContain('admin.proxyPools.grokQualityPassed')
    expect(wrapper.text()).toContain('HTTP 401')
  })

  it('binds selected groups and refreshes the pool', async () => {
    listGroupOptions.mockResolvedValue([
      { id: 21, name: 'Grok accounts', platform: 'grok', status: 'active', account_count: 4 },
      { id: 22, name: 'Disabled group', platform: 'grok', status: 'inactive', account_count: 1 }
    ])
    bindGroups.mockResolvedValue({ bound_groups: 1, synced_accounts: 4 })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[title="admin.proxyPools.details"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="bind-groups"]').trigger('click')
    await flushPromises()
    await wrapper.get('input[data-test="bind-group-21"]').setValue(true)
    await wrapper.get('[data-test="submit-bind-groups"]').trigger('click')
    await flushPromises()

    expect(bindGroups).toHaveBeenCalledWith(1, [21])
    expect(showSuccess).toHaveBeenCalledWith('admin.proxyPools.bindGroupsSuccess')
  })

  it('does not allow a group already owned by another pool', async () => {
    listGroupOptions.mockResolvedValue([
      { id: 21, name: 'Other pool group', platform: 'grok', status: 'active', account_count: 2, bound_pool_id: 9, bound_pool_name: 'secondary' }
    ])
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[title="admin.proxyPools.details"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="bind-groups"]').trigger('click')
    await flushPromises()

    const checkbox = wrapper.get<HTMLInputElement>('input[data-test="bind-group-21"]')
    expect(checkbox.element.disabled).toBe(true)
  })

  it('unbinds a group from the pool', async () => {
    listGroups.mockResolvedValue([
      { id: 21, name: 'Grok accounts', platform: 'grok', status: 'active', account_count: 4, bound_pool_id: 1 }
    ])
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[title="admin.proxyPools.details"]').trigger('click')
    await flushPromises()
    await wrapper.get('button[title="admin.proxyPools.unbindGroup"]').trigger('click')
    await wrapper.get('[data-test="confirm-remove"]').trigger('click')
    await flushPromises()

    expect(unbindGroups).toHaveBeenCalledWith(1, [21])
    expect(showSuccess).toHaveBeenCalledWith('admin.proxyPools.unbindGroupsSuccess')
  })

  it('runs a forced health check and refreshes the detail', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[title="admin.proxyPools.details"]').trigger('click')
    await flushPromises()

    const checkButton = wrapper.findAll('button').find((button) => button.text().includes('admin.proxyPools.checkNow'))
    expect(checkButton).toBeDefined()
    await checkButton!.trigger('click')
    await flushPromises()

    expect(rebind).toHaveBeenCalledWith(1)
    expect(listProxies).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalledWith('admin.proxyPools.checkStarted')
  })

  it('treats an existing background health check as an idempotent success', async () => {
    rebind.mockResolvedValue({ started: false, already_running: true })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[title="admin.proxyPools.details"]').trigger('click')
    await flushPromises()

    const checkButton = wrapper.findAll('button').find((button) => button.text().includes('admin.proxyPools.checkNow'))
    await checkButton!.trigger('click')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('admin.proxyPools.checkRunning')
  })

  it('selects and assigns all available proxies', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[title="admin.proxyPools.details"]').trigger('click')
    await flushPromises()

    const addButton = wrapper.findAll('button').find((button) => button.text().includes('admin.proxyPools.addProxies'))
    expect(addButton).toBeDefined()
    await addButton!.trigger('click')
    await flushPromises()

    const selectAll = wrapper.get<HTMLInputElement>('input[data-test="select-all-proxies"]')
    await selectAll.setValue(true)

    const proxyCheckboxes = wrapper.findAll<HTMLInputElement>('input[data-test^="assign-proxy-"]')
    expect(proxyCheckboxes).toHaveLength(2)
    expect(proxyCheckboxes.every((input) => input.element.checked)).toBe(true)

    const assignButton = wrapper.findAll('button').find((button) => button.text().includes('admin.proxyPools.addSelected'))
    expect(assignButton).toBeDefined()
    await assignButton!.trigger('click')
    await flushPromises()

    expect(assignProxies).toHaveBeenCalledWith(1, [12, 13])
  })

  it('filters invalid members using connectivity and Grok quality together', async () => {
    listProxies.mockResolvedValue([
      { ...makeProxy(11, 'healthy-pass'), pool_health: 'healthy', grok_quality_status: 'pass' },
      { ...makeProxy(12, 'connection-failed'), pool_health: 'unhealthy', grok_quality_status: 'pass' },
      { ...makeProxy(13, 'grok-blocked'), pool_health: 'healthy', grok_quality_status: 'challenge' },
      { ...makeProxy(14, 'not-checked'), pool_health: 'unknown', grok_quality_status: 'unknown' }
    ])
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[title="admin.proxyPools.details"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="member-status-filter"]').setValue('invalid')

    const rows = wrapper.findAll('[data-test="pool-member-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('connection-failed')
    expect(rows[1].text()).toContain('grok-blocked')
    expect(wrapper.text()).not.toContain('healthy-pass')
    expect(wrapper.text()).not.toContain('not-checked')
  })

  it('combines member search with the invalid filter', async () => {
    listProxies.mockResolvedValue([
      { ...makeProxy(12, 'connection-failed'), host: 'dead.example', pool_health: 'unhealthy', grok_quality_status: 'pass' },
      { ...makeProxy(13, 'grok-blocked'), host: 'blocked.example', pool_health: 'healthy', grok_quality_status: 'challenge' }
    ])
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[title="admin.proxyPools.details"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="member-status-filter"]').setValue('invalid')
    await wrapper.get('[data-test="member-search"]').setValue('blocked')

    const rows = wrapper.findAll('[data-test="pool-member-row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('grok-blocked')
  })

  it('selects every invalid member and removes them in one request', async () => {
    listProxies.mockResolvedValue([
      { ...makeProxy(11, 'healthy-pass'), pool_health: 'healthy', grok_quality_status: 'pass' },
      { ...makeProxy(12, 'connection-failed'), pool_health: 'unhealthy', grok_quality_status: 'pass' },
      { ...makeProxy(13, 'grok-blocked'), pool_health: 'healthy', grok_quality_status: 'challenge' },
      { ...makeProxy(14, 'not-checked'), pool_health: 'unknown', grok_quality_status: 'unknown' }
    ])
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[title="admin.proxyPools.details"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="select-invalid-members"]').trigger('click')
    expect(wrapper.findAll('[data-test="pool-member-row"]')).toHaveLength(2)
    expect(wrapper.get<HTMLInputElement>('[data-test="select-member-12"]').element.checked).toBe(true)
    expect(wrapper.get<HTMLInputElement>('[data-test="select-member-13"]').element.checked).toBe(true)

    await wrapper.get('[data-test="remove-selected-members"]').trigger('click')
    await wrapper.get('[data-test="confirm-remove"]').trigger('click')
    await flushPromises()

    expect(removeProxies).toHaveBeenCalledWith(1, [12, 13])
    expect(showSuccess).toHaveBeenCalledWith('admin.proxyPools.removeSelectedDone')
  })
})

function makeProxy(id: number, name: string) {
  return {
    id,
    name,
    protocol: 'http',
    host: `${name}.example`,
    port: 8080 + id,
    status: 'active',
    pool_id: 1,
    pool_health: 'unknown',
    pool_failures: 0,
    grok_quality_status: 'unknown',
    account_count: 0
  }
}
