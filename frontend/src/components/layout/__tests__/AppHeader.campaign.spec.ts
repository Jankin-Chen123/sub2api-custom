import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import AppHeader from '../AppHeader.vue'
import type { NewcomerCampaignStatus } from '@/types/campaign'

const campaignStoreState = vi.hoisted(() => ({ status: null as NewcomerCampaignStatus | null }))
const fetchCampaignStatus = vi.hoisted(() => vi.fn())
const translate = vi.hoisted(() => vi.fn((key: string) => key))
const routerPush = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: translate }) }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({ name: 'Dashboard', params: {}, meta: { titleKey: 'nav.dashboard' } }),
  useRouter: () => ({ push: routerPush }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    contactInfo: '',
    docUrl: '',
    cachedPublicSettings: null,
    toggleMobileSidebar: vi.fn(),
  }),
  useAuthStore: () => ({
    user: { username: 'demo', email: 'demo@example.com', role: 'user', balance: 0, frozen_balance: 0 },
    isAdmin: false,
    isSimpleMode: false,
    logout: vi.fn().mockResolvedValue(undefined),
  }),
  useOnboardingStore: () => ({ replay: vi.fn() }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null }),
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))

vi.mock('@/stores/campaign', () => ({
  useCampaignStore: () => ({
    status: campaignStoreState.status,
    fetchStatus: fetchCampaignStatus,
  }),
}))

const statusFixture = (firstRecharge: NewcomerCampaignStatus['first_recharge']): NewcomerCampaignStatus => ({
  campaign_key: 'newcomer_202609',
  name: '2026 年 9 月迎新活动',
  phase: 'active',
  starts_at: '2026-08-31T16:00:00.000Z',
  ends_at: '2026-10-01T16:00:00.000Z',
  first_recharge: firstRecharge,
  invite_link: '',
  valid_invite_count: 5,
  next_tier: undefined,
  next_tier_progress: 5,
  next_tier_remaining: 0,
  current_membership: {
    tier_key: 'gold',
    tier_name: '黄金',
    factor: 0.96,
    starts_at: '2026-09-03T00:00:00.000Z',
    expires_at: '2026-10-18T00:00:00.000Z',
  },
  tiers: [],
})

describe('AppHeader newcomer campaign controls', () => {
  beforeEach(() => {
    campaignStoreState.status = statusFixture({ eligible: true, reward_status: 'pending', reward_amount: 2 })
    fetchCampaignStatus.mockReset().mockResolvedValue(campaignStoreState.status)
    routerPush.mockReset()
  })

  it('shows the authorized direct recharge link and membership details without concurrency', async () => {
    const wrapper = shallowMount(AppHeader, {
      global: {
        stubs: {
          'router-link': { template: '<a :to="to"><slot /></a>', props: ['to'] },
          Icon: true,
          LocaleSwitcher: true,
          SubscriptionProgressMini: true,
          AnnouncementBell: true,
          Transition: false,
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-test="first-recharge-shortcut"]').attributes('to')).toBe('/purchase?campaign=first-recharge')
    expect(wrapper.get('[data-test="campaign-membership-badge"]').text()).toContain('黄金')
    await wrapper.get('button[aria-label="common.userMenu"]').trigger('click')
    expect(wrapper.get('[data-test="campaign-membership-menu"]').text()).toContain('campaign.membershipFactor')
    expect(wrapper.text()).not.toContain('concurrency')
  })

  it('hides the first-recharge link when server state is not eligible', async () => {
    campaignStoreState.status = statusFixture({ eligible: false, reward_status: 'ineligible', reward_amount: 2 })
    fetchCampaignStatus.mockResolvedValue(campaignStoreState.status)

    const wrapper = shallowMount(AppHeader, {
      global: {
        stubs: {
          'router-link': { template: '<a :to="to"><slot /></a>', props: ['to'] },
          Icon: true,
          LocaleSwitcher: true,
          SubscriptionProgressMini: true,
          AnnouncementBell: true,
          Transition: false,
        },
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="first-recharge-shortcut"]').exists()).toBe(false)
  })
})
