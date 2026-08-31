import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AffiliateView from '../AffiliateView.vue'
import type { NewcomerCampaignStatus } from '@/types/campaign'

const { copyToClipboard, getAffiliateDetail } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getAffiliateDetail: vi.fn(),
}))
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const refreshUser = vi.hoisted(() => vi.fn())
const campaignStoreState = vi.hoisted(() => ({ status: null as NewcomerCampaignStatus | null }))
const fetchCampaignStatus = vi.hoisted(() => vi.fn())
const translate = vi.hoisted(() => vi.fn((key: string, params?: Record<string, unknown>) => {
  if (key === 'campaign.tierRequirement') return `${params?.factor ?? ''}`
  if (key === 'campaign.membershipExclusiveRate') return `享受原倍率 ${params?.percent ?? ''}% 的专属倍率`
  return key
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    transferAffiliateQuota: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ refreshUser }),
}))

vi.mock('@/stores/campaign', () => ({
  useCampaignStore: () => ({
    status: campaignStoreState.status,
    fetchStatus: fetchCampaignStatus,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: translate }),
  }
})

const campaignFixture: NewcomerCampaignStatus = {
  campaign_key: 'newcomer_202609',
  name: '2026 年 9 月迎新活动',
  phase: 'active',
  starts_at: '2026-08-31T16:00:00.000Z',
  ends_at: '2026-10-01T16:00:00.000Z',
  first_recharge: { eligible: false, reward_status: 'ineligible', reward_amount: 2 },
  invite_link: 'http://localhost/register?aff=campaign',
  valid_invite_count: 3,
  next_tier: { key: 'gold', name: '黄金', threshold: 5, factor: 0.96, duration_days: 45 },
  next_tier_progress: 3,
  next_tier_remaining: 2,
  tiers: [
    { key: 'premium', name: '高级', threshold: 2, factor: 0.98, duration_days: 30 },
    { key: 'gold', name: '黄金', threshold: 5, factor: 0.96, duration_days: 45 },
    { key: 'diamond', name: '钻石', threshold: 10, factor: 0.94, duration_days: 60 },
  ],
}

describe('AffiliateView', () => {
  const affiliateCode = 'affiliate-code-that-is-long-enough-to-overflow-a-mobile-viewport'

  beforeEach(() => {
    vi.clearAllMocks()
    copyToClipboard.mockResolvedValue(true)
    getAffiliateDetail.mockResolvedValue({
      user_id: 1,
      aff_code: affiliateCode,
      inviter_id: null,
      aff_count: 0,
      aff_quota: 0,
      aff_frozen_quota: 0,
      aff_history_quota: 0,
      effective_rebate_rate_percent: 10,
      invitees: [],
    })
    campaignStoreState.status = null
    fetchCampaignStatus.mockResolvedValue(null)
  })

  it('stacks long values and copy controls on mobile while retaining desktop rows', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const values = wrapper.findAll('code')
    expect(values).toHaveLength(2)
    for (const value of values) {
      expect(value.classes()).toEqual(expect.arrayContaining([
        'min-w-0',
        'break-all',
        'sm:flex-1',
        'sm:truncate',
      ]))
      expect(Array.from(value.element.parentElement?.classList ?? [])).toEqual(expect.arrayContaining([
        'flex-col',
        'items-stretch',
        'sm:flex-row',
        'sm:items-center',
      ]))
    }

    const copyButtons = wrapper.findAll('button').filter((button) =>
      ['affiliate.copyCode', 'affiliate.copyLink'].includes(button.text()),
    )
    expect(copyButtons).toHaveLength(2)
    for (const button of copyButtons) {
      expect(button.classes()).toEqual(expect.arrayContaining([
        'w-full',
        'sm:w-auto',
        'sm:shrink-0',
      ]))
    }

    await copyButtons[0].trigger('click')
    await copyButtons[1].trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenNthCalledWith(1, affiliateCode, 'affiliate.codeCopied')
    expect(copyToClipboard).toHaveBeenNthCalledWith(
      2,
      `${window.location.origin}/register?aff=${encodeURIComponent(affiliateCode)}`,
      'affiliate.linkCopied',
    )
  })
})

describe('AffiliateView newcomer campaign progress', () => {
  beforeEach(() => {
    campaignStoreState.status = campaignFixture
    fetchCampaignStatus.mockReset().mockResolvedValue(campaignFixture)
  })

  it('renders server-provided valid invite progress and tier configuration', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    const campaign = wrapper.get('[data-test="campaign-invite-progress"]')
    expect(campaign.text()).toContain('campaign.validInvites')
    expect(campaign.text()).toContain('3')
    expect(campaign.text()).toContain('campaign.tierProgress')
    expect(campaign.text()).toContain('96%')
    expect(campaign.text()).toContain('享受原倍率 96% 的专属倍率')
    expect(campaign.find('.bg-primary-500').attributes('style')).toContain('width: 60%')
    expect(wrapper.text()).toContain('http://localhost/register?aff=campaign')
    expect(fetchCampaignStatus).toHaveBeenCalledWith(true)
  })

  it('does not invent campaign progress when the campaign endpoint has no state', async () => {
    campaignStoreState.status = null
    fetchCampaignStatus.mockResolvedValue(null)

    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="campaign-invite-progress"]').exists()).toBe(false)
  })
})
