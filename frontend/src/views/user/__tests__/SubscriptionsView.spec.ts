import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SubscriptionsView from '../SubscriptionsView.vue'

const getMySubscriptions = vi.hoisted(() => vi.fn())
const getAvailablePlans = vi.hoisted(() => vi.fn())
const purchasePlan = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/api/subscriptions', () => ({
  default: {
    getMySubscriptions,
    getAvailablePlans,
    purchasePlan,
    activateSubscription: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    cachedPublicSettings: null,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const messages: Record<string, string> = {
          'common.cancel': 'Cancel',
          'common.confirm': 'Confirm',
          'common.loading': 'Loading',
          'userSubscriptions.availablePlans': 'Available Subscriptions',
          'userSubscriptions.availablePlansDesc': 'Purchase a subscription with your balance.',
          'userSubscriptions.purchase': 'Buy Subscription',
          'userSubscriptions.purchaseConfirmTitle': 'Confirm Subscription Purchase',
          'userSubscriptions.purchaseConfirm': 'Use your balance to buy “{name}”?',
          'userSubscriptions.purchaseSuccess': 'Subscription purchased.',
          'userSubscriptions.purchaseFailed': 'Purchase failed.',
          'userSubscriptions.planValidity': '{days} days',
          'userSubscriptions.validity': 'Validity',
          'userSubscriptions.noActiveSubscriptions': 'No Active Subscriptions',
          'userSubscriptions.noPurchasedSubscriptionsDesc': 'Purchased cards appear here.',
          'userSubscriptions.daily': 'Daily',
          'userSubscriptions.weekly': 'Weekly',
          'userSubscriptions.monthly': 'Monthly',
          'userSubscriptions.unlimited': 'Unlimited',
        }
        return (messages[key] || key).replace(/\{(\w+)\}/g, (_, name: string) => String(params?.[name] ?? ''))
      },
    }),
  }
})

const plan = {
  id: 7,
  group_id: 3,
  group_name: 'OpenAI',
  name: 'Starter',
  description: '',
  price: 5,
  currency: '$',
  validity_days: 30,
  validity_unit: 'day',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  features: [],
  for_sale: true,
  sort_order: 1,
}

describe('SubscriptionsView purchase confirmation', () => {
  beforeEach(() => {
    getMySubscriptions.mockResolvedValue([])
    getAvailablePlans.mockResolvedValue([plan])
    purchasePlan.mockResolvedValue({})
    showError.mockReset()
    showSuccess.mockReset()
  })

  afterEach(() => {
    document.body.innerHTML = ''
    document.body.classList.remove('modal-open')
  })

  it('opens an in-app confirmation dialog and purchases only after confirmation', async () => {
    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(purchasePlan).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('Confirm Subscription Purchase')
    expect(document.body.textContent).toContain('Use your balance to buy “Starter”?')
    expect(document.body.textContent).toContain('$5.00')
    expect(document.body.textContent).not.toContain('(5)')

    const confirmButton = Array.from(document.body.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Confirm',
    )
    expect(confirmButton).toBeDefined()
    confirmButton?.click()
    await flushPromises()

    expect(purchasePlan).toHaveBeenCalledWith(7)
    expect(showSuccess).toHaveBeenCalled()
    wrapper.unmount()
  })
})
