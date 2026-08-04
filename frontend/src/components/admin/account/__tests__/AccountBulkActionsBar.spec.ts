import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

const mountBar = (selectedIds: number[] = []) => mount(AccountBulkActionsBar, {
  props: { selectedIds },
  global: {
    stubs: { Icon: true }
  }
})

describe('AccountBulkActionsBar', () => {
  it('keeps the proxy pool action visible but disabled before accounts are selected', () => {
    const wrapper = mountBar()
    const button = wrapper.get('[data-test="bind-proxy-pool"]')

    expect(button.attributes('disabled')).toBeDefined()
    expect(button.attributes('title')).toBe('admin.accounts.bulkActions.selectBeforeBind')
  })

  it('enables the proxy pool action and emits after accounts are selected', async () => {
    const wrapper = mountBar([101, 102])
    const button = wrapper.get('[data-test="bind-proxy-pool"]')

    expect(button.attributes('disabled')).toBeUndefined()
    await button.trigger('click')

    expect(wrapper.emitted('bind-proxy-pool')).toHaveLength(1)
  })
})
