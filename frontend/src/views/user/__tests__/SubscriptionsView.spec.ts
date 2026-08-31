import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SubscriptionsView from '../SubscriptionsView.vue'

const getMySubscriptions = vi.hoisted(() => vi.fn())
const getAvailablePlans = vi.hoisted(() => vi.fn())
const purchasePlan = vi.hoisted(() => vi.fn())
const activateSubscription = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const refreshUser = vi.hoisted(() => vi.fn())
const adjustBalance = vi.hoisted(() => vi.fn())
const applyActivatedSubscription = vi.hoisted(() => vi.fn())
const copyToClipboard = vi.hoisted(() => vi.fn())

vi.mock('@/api/subscriptions', () => ({
  default: {
    getMySubscriptions,
    getAvailablePlans,
    purchasePlan,
    activateSubscription,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    cachedPublicSettings: null,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser,
    adjustBalance,
  }),
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    applyActivatedSubscription,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
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
          'userSubscriptions.confirmPayment': 'Confirm Payment',
          'userSubscriptions.activate': 'Activate Now',
          'userSubscriptions.activateAfterExpiry': 'Wait for expiry',
          'userSubscriptions.activateBlocked': 'Another subscription is active. Wait for it to expire before activating this card.',
          'userSubscriptions.copyKey': 'Copy API key',
          'userSubscriptions.keyCopied': 'API key copied',
          'userSubscriptions.current': 'Currently active',
          'userSubscriptions.manage': 'Manage subscriptions',
          'userSubscriptions.manageTitle': 'Purchased subscriptions',
          'userSubscriptions.manageHint': 'Only one subscription card can be active at a time.',
          'userSubscriptions.manageEmpty': 'No subscriptions to manage.',
          'userSubscriptions.purchaseSuccess': 'Subscription purchased.',
          'userSubscriptions.purchaseFailed': 'Purchase failed.',
          'userSubscriptions.planValidity': '{days} days',
          'userSubscriptions.validity': 'Validity',
          'userSubscriptions.noActiveSubscriptions': 'No Active Subscriptions',
          'userSubscriptions.noPurchasedSubscriptionsDesc': 'Purchased cards appear here.',
          'userSubscriptions.status.active': 'Active',
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
    purchasePlan.mockResolvedValue({
      id: 81,
      user_id: 1,
      group_id: 3,
      status: 'pending',
      starts_at: '2099-01-01T00:00:00Z',
      expires_at: null,
      validity_days: 30,
      daily_usage_usd: 0,
      weekly_usage_usd: 0,
      monthly_usage_usd: 0,
      group: { name: 'Starter', platform: 'openai', rate_multiplier: 1 },
    })
    activateSubscription.mockReset()
    activateSubscription.mockResolvedValue({})
    refreshUser.mockReset()
    refreshUser.mockResolvedValue({ balance: 95 })
    adjustBalance.mockReset()
    applyActivatedSubscription.mockReset()
    copyToClipboard.mockReset()
    copyToClipboard.mockResolvedValue(true)
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

    await wrapper.get('[data-test="plan-purchase-7"]').trigger('click')
    await flushPromises()

    expect(purchasePlan).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('Confirm Subscription Purchase')
    expect(document.body.textContent).toContain('Use your balance to buy “Starter”?')
    expect(document.body.textContent).toContain('$5.00')
    expect(document.body.textContent).not.toContain('Payment method')
    expect(document.body.textContent).not.toContain('Account balance')
    expect(document.body.textContent).not.toContain('(5)')

    const confirmButton = Array.from(document.body.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Confirm Payment',
    )
    expect(confirmButton).toBeDefined()
    confirmButton?.click()
    await flushPromises()

    expect(purchasePlan).toHaveBeenCalledWith(7)
    expect(adjustBalance).toHaveBeenCalledWith(-5)
    expect(refreshUser).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('does not render expired subscriptions in the current subscriptions list', async () => {
    getMySubscriptions.mockResolvedValue([
      {
        id: 12,
        user_id: 1,
        group_id: 3,
        status: 'expired',
        expires_at: '2020-01-01T00:00:00Z',
        validity_days: 30,
        group: { name: 'Expired subscription', platform: 'openai' },
      },
    ])

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('No Active Subscriptions')
    expect(wrapper.text()).not.toContain('Expired subscription')
    expect(wrapper.find('button').exists()).toBe(true)
    wrapper.unmount()
  })

  it('shows pending cards in the management menu and refreshes after activation', async () => {
    const pendingSubscription = {
      id: 21,
      user_id: 1,
      group_id: 3,
      status: 'pending',
      expires_at: null,
      validity_days: 30,
      daily_usage_usd: 0,
      weekly_usage_usd: 0,
      monthly_usage_usd: 0,
      group: { name: 'Starter', platform: 'openai', rate_multiplier: 1 },
    }
    const activeSubscription = {
      ...pendingSubscription,
      status: 'active',
      expires_at: '2099-01-01T00:00:00Z',
      api_key: {
        id: 91,
        key: 'sk-subscription-ready',
      },
    }
    getMySubscriptions.mockResolvedValue([pendingSubscription])
    activateSubscription.mockResolvedValue(activeSubscription)

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('[data-test="manage-subscriptions"]').trigger('click')
    const activateButton = wrapper.get('[data-test="activate-subscription-21"]')
    expect(activateButton.text()).toContain('Activate Now')
    await activateButton.trigger('click')
    await flushPromises()

    expect(activateSubscription).toHaveBeenCalledWith(21)
    expect(applyActivatedSubscription).toHaveBeenCalledWith(activeSubscription)
    expect(wrapper.text()).toContain('Active')
    const copyButton = wrapper.get('[data-test="copy-subscription-key"]')
    expect(copyButton.text()).toContain('Copy API key')
    await copyButton.trigger('click')
    await flushPromises()
    expect(copyToClipboard).toHaveBeenCalledWith('sk-subscription-ready', 'API key copied')
    wrapper.unmount()
  })

  it('disables other pending cards while one subscription is active', async () => {
    const activeSubscription = {
      id: 31,
      user_id: 1,
      group_id: 3,
      status: 'active',
      starts_at: '2099-01-01T00:00:00Z',
      expires_at: '2099-02-01T00:00:00Z',
      validity_days: 31,
      daily_usage_usd: 0,
      weekly_usage_usd: 0,
      monthly_usage_usd: 0,
      group: { name: 'Current', platform: 'openai', rate_multiplier: 1 },
    }
    const pendingSubscription = {
      id: 32,
      user_id: 1,
      group_id: 4,
      status: 'pending',
      starts_at: '2099-01-01T00:00:00Z',
      expires_at: null,
      validity_days: 30,
      daily_usage_usd: 0,
      weekly_usage_usd: 0,
      monthly_usage_usd: 0,
      group: { name: 'Next', platform: 'anthropic', rate_multiplier: 1 },
    }
    getMySubscriptions.mockResolvedValue([pendingSubscription, activeSubscription])

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('[data-test="manage-subscriptions"]').trigger('click')
    const activateButton = wrapper.get('[data-test="activate-subscription-32"]')
    expect(activateButton.attributes('disabled')).toBeDefined()
    expect(activateButton.text()).toContain('Wait for expiry')
    expect(wrapper.get('[data-test="managed-subscription-31"]').text()).toContain('Currently active')

    wrapper.unmount()
  })
})
