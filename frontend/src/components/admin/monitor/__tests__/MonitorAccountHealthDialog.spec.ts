import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { AccountHealthItem, ChannelMonitor } from '@/api/admin/channelMonitor'
import MonitorAccountHealthDialog from '@/components/admin/monitor/MonitorAccountHealthDialog.vue'

const { listAccountHealth } = vi.hoisted(() => ({ listAccountHealth: vi.fn() }))

vi.mock('@/api/admin/channelMonitor', async () => {
  const actual = await vi.importActual<typeof import('@/api/admin/channelMonitor')>('@/api/admin/channelMonitor')
  return {
    ...actual,
    channelMonitorAPI: { listAccountHealth },
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    props: ['show', 'title'],
    template: '<div v-if="show" data-testid="dialog"><h1>{{ title }}</h1><slot /><slot name="footer" /></div>',
  },
}))

function makeMonitor(): ChannelMonitor {
  return {
    id: 42,
    name: 'primary',
    provider: 'openai',
    api_mode: 'chat_completions',
    endpoint: 'https://api.example.com',
    api_key_masked: 'sk-t***',
    primary_model: 'gpt-4o-mini',
    extra_models: ['gpt-4o'],
    group_name: 'main',
    enabled: true,
    interval_seconds: 60,
    jitter_seconds: 0,
    last_checked_at: null,
    created_by: 1,
    created_at: '2026-07-16T00:00:00Z',
    updated_at: '2026-07-16T00:00:00Z',
    primary_status: 'operational',
    primary_latency_ms: 100,
    availability_7d: 100,
    extra_models_status: [],
    template_id: null,
    extra_headers: {},
    body_override_mode: 'off',
    body_override: null,
    check_mode: 'probe',
    account_id: null,
  }
}

function makeItem(overrides: Partial<AccountHealthItem> = {}): AccountHealthItem {
  return {
    account_id: 17,
    account_name: 'upstream-a',
    group_id: 9,
    provider: 'openai',
    model: 'gpt-4o-mini',
    score: 82.5,
    health_state: 'healthy',
    ewma_success_rate: 0.95,
    ewma_latency_ms: 180,
    sample_count: 8,
    consecutive_successes: 7,
    consecutive_failures: 0,
    last_status: 'operational',
    last_probe_at: '2026-08-26T10:00:00Z',
    updated_at: '2026-08-26T10:00:00Z',
    expires_at: '2026-08-26T10:15:00Z',
    stale: false,
    ...overrides,
  }
}

describe('MonitorAccountHealthDialog', () => {
  it('loads and renders account health snapshots with model filtering', async () => {
    listAccountHealth.mockResolvedValue({
      items: [makeItem(), makeItem({ account_id: 18, account_name: 'upstream-b', model: 'gpt-4o', health_state: 'degraded', score: 61 })],
    })
    const wrapper = mount(MonitorAccountHealthDialog, {
      props: { show: true, monitor: makeMonitor() },
    })
    await flushPromises()

    expect(listAccountHealth).toHaveBeenCalledWith(42)
    expect(wrapper.text()).toContain('upstream-a')
    expect(wrapper.text()).toContain('82.5')
    expect(wrapper.text()).toContain('admin.channelMonitor.accountHealth.states.healthy')

    await wrapper.get('select').setValue('gpt-4o')
    expect(wrapper.text()).not.toContain('upstream-a')
    expect(wrapper.text()).toContain('upstream-b')
  })

  it('marks stale snapshots as unknown in the state badge', async () => {
    listAccountHealth.mockResolvedValue({ items: [makeItem({ stale: true, health_state: 'unknown' })] })
    const wrapper = mount(MonitorAccountHealthDialog, {
      props: { show: true, monitor: makeMonitor() },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('admin.channelMonitor.accountHealth.stale')
    expect(wrapper.text()).toContain('admin.channelMonitor.accountHealth.states.unknown')
  })
})
