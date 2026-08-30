import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import NewcomerCampaignView from '../NewcomerCampaignView.vue'

const getConfig = vi.hoisted(() => vi.fn())
const updateConfig = vi.hoisted(() => vi.fn())
const reconcile = vi.hoisted(() => vi.fn())
const getUserMembership = vi.hoisted(() => vi.fn())
const setUserMembership = vi.hoisted(() => vi.fn())
const clearUserMembership = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin/campaign', () => ({
  default: {
    getConfig,
    updateConfig,
    reconcile,
    getUserMembership,
    setUserMembership,
    clearUserMembership,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, values?: Record<string, unknown>) => `${key}${values ? JSON.stringify(values) : ''}`,
    }),
  }
})

const baseConfig = {
  campaign_key: 'newcomer_202609',
  name: '2026 年 9 月迎新活动',
  phase: 'active',
  starts_at: '2026-09-01T00:00:00+08:00',
  ends_at: '2026-10-01T00:00:00+08:00',
  tiers: [
    { key: 'premium', name: '高级', threshold: 2, factor: 0.98, duration_days: 30 },
    { key: 'gold', name: '黄金', threshold: 5, factor: 0.96, duration_days: 45 },
    { key: 'diamond', name: '钻石', threshold: 10, factor: 0.94, duration_days: 60 },
  ],
}

function mountView() {
  return shallowMount(NewcomerCampaignView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
      },
    },
  })
}

describe('NewcomerCampaignView date and threshold editing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getConfig.mockResolvedValue(structuredClone(baseConfig))
    updateConfig.mockResolvedValue(structuredClone(baseConfig))
  })

  it('renders an exclusive API end date as an inclusive Shanghai date and submits next-day UTC midnight', async () => {
    const wrapper = mountView()
    await flushPromises()

    const dates = wrapper.findAll('input[type="date"]')
    expect((dates[0].element as HTMLInputElement).value).toBe('2026-09-01')
    expect((dates[1].element as HTMLInputElement).value).toBe('2026-09-30')

    await dates[0].setValue('2027-01-01')
    await dates[1].setValue('2027-01-08')
    const numbers = wrapper.findAll('input[type="number"]')
    await numbers[0].setValue(3)
    await numbers[3].setValue(6)
    await numbers[6].setValue(12)
    await wrapper.findAll('form')[0].trigger('submit')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith(
      [
        expect.objectContaining({ key: 'premium', threshold: 3 }),
        expect.objectContaining({ key: 'gold', threshold: 6 }),
        expect.objectContaining({ key: 'diamond', threshold: 12 }),
      ],
      '2026-12-31T16:00:00.000Z',
      '2027-01-08T16:00:00.000Z'
    )
  })

  it('rejects an invalid date range without submitting', async () => {
    const wrapper = mountView()
    await flushPromises()
    const dates = wrapper.findAll('input[type="date"]')
    await dates[0].setValue('2027-02-01')
    await dates[1].setValue('2027-01-31')
    await wrapper.findAll('form')[0].trigger('submit')

    expect(updateConfig).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalled()
  })
})
