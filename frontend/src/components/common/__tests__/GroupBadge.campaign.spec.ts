import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import GroupBadge from '../GroupBadge.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null }),
}))

describe('GroupBadge campaign-adjusted rate', () => {
  it('shows the final campaign rate with the existing rate presentation', () => {
    const wrapper = mount(GroupBadge, {
      props: {
        name: 'Example group',
        platform: 'openai',
        rateMultiplier: 0.15,
        userRateMultiplier: 0.14,
        campaignFactor: 0.96,
      },
      global: {
        stubs: {
          PlatformIcon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('0.14x')
    expect(wrapper.text()).toContain('0.1344x')
    expect(wrapper.text()).not.toContain('0.15x')
    expect(wrapper.text()).not.toContain('→')
    expect(wrapper.get('[title]').attributes('title')).toBe('campaign.membershipFactor')
  })

  it('does not apply the campaign factor to subscription groups', () => {
    const wrapper = mount(GroupBadge, {
      props: {
        name: 'Subscription group',
        platform: 'openai',
        subscriptionType: 'subscription',
        rateMultiplier: 0.15,
        campaignFactor: 0.96,
      },
      global: {
        stubs: {
          PlatformIcon: true,
        },
      },
    })

    expect(wrapper.text()).not.toContain('→')
    expect(wrapper.text()).not.toContain('0.144x')
  })

  it('keeps the original rate when the factor is one', () => {
    const wrapper = mount(GroupBadge, {
      props: {
        name: 'Example group',
        platform: 'openai',
        rateMultiplier: 0.15,
        campaignFactor: 1,
      },
      global: {
        stubs: {
          PlatformIcon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('0.15x')
    expect(wrapper.text()).not.toContain('→')
  })
})
