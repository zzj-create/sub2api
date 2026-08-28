import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import AccountQualityCell from '../AccountQualityCell.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => `formatted:${value}`
}))

afterEach(() => {
  document.body.innerHTML = ''
})

describe('AccountQualityCell', () => {
  it('shows account t/s and exposes the complete quality snapshot', async () => {
    const account = {
      id: 42,
      platform: 'grok',
      type: 'oauth',
      grok_quality: {
        account_id: 42,
        pool_id: 7,
        pool_name: 'Grok pool',
        proxy_id: 9,
        proxy_name: 'TN exit',
        quality_class: 'healthy',
        output_tps: 125.75,
        output_tokens: 64,
        duration_ms: 2000,
        first_token_ms: 250,
        has_thinking: true,
        source: 'passive',
        reason: 'quality observation recorded',
        observed_at: '2026-08-07T05:00:00Z'
      }
    } as Account

    const wrapper = mount(AccountQualityCell, {
      props: { account },
      attachTo: document.body
    })

    expect(wrapper.text()).toContain('125.75 t/s')
    await wrapper.get('[data-test="account-quality-trigger"]').trigger('click')

    const details = document.body.querySelector('[data-test="account-quality-details"]')
    expect(details).not.toBeNull()
    expect(details?.textContent).toContain('125.75 t/s')
    expect(details?.textContent).toContain('64')
    expect(details?.textContent).toContain('2,000 ms')
    expect(details?.textContent).toContain('250 ms')
    expect(details?.textContent).toContain('Grok pool')
    expect(details?.textContent).toContain('TN exit')
    expect(details?.textContent).toContain('quality observation recorded')

    wrapper.unmount()
  })

  it('shows the SSO risk result beside model quality', async () => {
    const botFlag = 2
    const risk = 0.95
    const httpStatus = 200
    const account = {
      id: 44,
      platform: 'grok',
      type: 'oauth',
      grok_quality: {
        account_id: 44,
        pool_id: 7,
        proxy_id: 9,
        quality_class: 'unknown',
        output_tps: 0,
        output_tokens: 0,
        duration_ms: 0,
        first_token_ms: 0,
        source: 'sso',
        observed_at: '2026-08-07T05:00:00Z',
        sso_state: 'flagged_ip',
        sso_reason: 'eapi_ip_bot_farm free-tier',
        sso_bot_flag_source: botFlag,
        sso_risk: risk,
        sso_policy: 'free-tier',
        sso_event: '',
        sso_http_status: httpStatus,
        sso_checked_at: '2026-08-17T05:00:00Z'
      }
    } as Account

    const wrapper = mount(AccountQualityCell, {
      props: { account },
      attachTo: document.body
    })

    expect(wrapper.get('[data-test="account-sso-quality-state"]').text()).toContain('flagged_ip')
    await wrapper.get('[data-test="account-quality-trigger"]').trigger('click')
    const details = document.body.querySelector('[data-test="account-quality-details"]')
    expect(details?.textContent).toContain('eapi_ip_bot_farm free-tier')
    expect(details?.textContent).toContain('95%')
    expect(details?.textContent).toContain('formatted:2026-08-17T05:00:00Z')

    wrapper.unmount()
  })

  it('makes an unobserved Grok account explicit', () => {
    const wrapper = mount(AccountQualityCell, {
      props: {
        account: { id: 43, platform: 'grok', type: 'oauth' } as Account
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.grokQuality.notObserved')
  })
})
