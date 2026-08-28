import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CheckinPrizeManager from '../CheckinPrizeManager.vue'

const listPrizes = vi.hoisted(() => vi.fn())
const replacePrizes = vi.hoisted(() => vi.fn())
const getConfig = vi.hoisted(() => vi.fn())
const updateConfig = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { checkin: { listPrizes, replacePrizes, getConfig, updateConfig } }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

const prizes = [
  { id: 1, name: 'Small', amount: 0.01, probability: 70, color: '#60A5FA', sort_order: 0 },
  { id: 2, name: 'Large', amount: 1, probability: 30, color: '#F97316', sort_order: 1 }
]

describe('CheckinPrizeManager', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listPrizes.mockResolvedValue(prizes)
    replacePrizes.mockImplementation(async (input) => input)
    getConfig.mockResolvedValue({ streak_bonus_amount: 5, streak_target: 7 })
    updateConfig.mockImplementation(async (amount) => ({ streak_bonus_amount: amount, streak_target: 7 }))
  })

  it('saves a valid 100 percent prize configuration atomically', async () => {
    const wrapper = mount(CheckinPrizeManager, { global: { stubs: { Icon: true } } })
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((button) => button.text() === 'common.save')
    expect(saveButton).toBeDefined()
    expect(saveButton!.attributes('disabled')).toBeUndefined()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(replacePrizes).toHaveBeenCalledTimes(1)
    expect(updateConfig).toHaveBeenCalledWith(5)
    expect(replacePrizes.mock.calls[0][0]).toHaveLength(2)
    expect(showSuccess).toHaveBeenCalledTimes(1)
  })

  it('prevents saving while probabilities do not add up to 100 percent', async () => {
    const wrapper = mount(CheckinPrizeManager, { global: { stubs: { Icon: true } } })
    await flushPromises()

    const addButton = wrapper.findAll('button').find((button) => button.text().includes('admin.redeem.checkin.addPrize'))
    await addButton!.trigger('click')

    const saveButton = wrapper.findAll('button').find((button) => button.text() === 'common.save')
    expect(saveButton!.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('101%')
    expect(replacePrizes).not.toHaveBeenCalled()
  })
})
