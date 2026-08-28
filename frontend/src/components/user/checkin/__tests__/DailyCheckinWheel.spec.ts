import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DailyCheckinWheel from '../DailyCheckinWheel.vue'

const getStatus = vi.hoisted(() => vi.fn())
const draw = vi.hoisted(() => vi.fn())
const refreshUser = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('@/api', () => ({
  checkinAPI: { getStatus, draw }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ refreshUser })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError })
}))

const prizes = [
  { id: 1, name: 'Small', amount: 0.01, probability: 70, color: '#60A5FA', sort_order: 0 },
  { id: 2, name: 'Large', amount: 1, probability: 30, color: '#F97316', sort_order: 1 }
]

describe('DailyCheckinWheel', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    getStatus.mockResolvedValue({
      date: '2026-08-28',
      checked_today: false,
      can_checkin: true,
      prizes
    })
    draw.mockResolvedValue({
      id: 9,
      prize_id: 2,
      prize_name: 'Large',
      amount: 1,
      probability: 30,
      new_balance: 5,
      checked_at: '2026-08-28T00:00:00Z'
    })
    refreshUser.mockResolvedValue(undefined)
  })

  afterEach(() => vi.useRealTimers())

  it('draws once, animates to the server-selected prize, and refreshes balance', async () => {
    const wrapper = mount(DailyCheckinWheel)
    await flushPromises()

    const button = wrapper.get('button')
    expect(button.text()).toBe('redeem.checkin.checkinButton')
    await button.trigger('click')
    await flushPromises()

    expect(draw).toHaveBeenCalledTimes(1)
    expect(button.attributes('disabled')).toBeDefined()

    await vi.advanceTimersByTimeAsync(4700)
    await flushPromises()

    expect(refreshUser).toHaveBeenCalledTimes(1)
    expect(document.body.textContent).toContain('+$1.00')
    expect(document.body.querySelectorAll('.confetti-piece')).toHaveLength(40)
    expect(button.text()).toBe('redeem.checkin.checkedToday')
  })

  it('disables the draw button when today has already been claimed', async () => {
    getStatus.mockResolvedValue({
      date: '2026-08-28',
      checked_today: true,
      can_checkin: false,
      prizes,
      today_result: {
        id: 8,
        prize_id: 1,
        prize_name: 'Small',
        amount: 0.01,
        probability: 70,
        new_balance: 0,
        checked_at: '2026-08-28T00:00:00Z'
      }
    })

    const wrapper = mount(DailyCheckinWheel)
    await flushPromises()

    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('Small')
    expect(draw).not.toHaveBeenCalled()
  })
})
