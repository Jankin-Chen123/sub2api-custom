<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <!-- Current Balance Card -->
      <div class="card overflow-hidden">
        <div class="bg-gradient-to-br from-primary-500 to-primary-600 px-6 py-8 text-center">
          <div
            class="mb-4 inline-flex h-16 w-16 items-center justify-center rounded-2xl bg-white/20 backdrop-blur-sm"
          >
            <Icon name="creditCard" size="xl" class="text-white" />
          </div>
          <p class="text-sm font-medium text-primary-100">{{ t('redeem.currentBalance') }}</p>
          <p class="mt-2 text-4xl font-bold text-white">
            ${{ user?.balance?.toFixed(2) || '0.00' }}
          </p>
          <p class="mt-2 text-sm text-primary-100">
            {{ t('redeem.concurrency') }}: {{ user?.concurrency || 0 }} {{ t('redeem.requests') }}
          </p>
        </div>
      </div>

      <div class="grid items-stretch gap-6 lg:grid-cols-2">
        <div class="flex h-full min-h-0 flex-col gap-6">

      <!-- Redeem Form -->
      <div class="card">
        <div class="p-6">
          <form @submit.prevent="handleRedeem" class="space-y-5">
            <div>
              <label for="code" class="input-label">
                {{ t('redeem.redeemCodeLabel') }}
              </label>
              <div class="relative mt-1">
                <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4">
                  <Icon name="gift" size="md" class="text-gray-400 dark:text-dark-500" />
                </div>
                <input
                  id="code"
                  v-model="redeemCode"
                  type="text"
                  required
                  :placeholder="t('redeem.redeemCodePlaceholder')"
                  :disabled="submitting"
                  class="input py-3 pl-12 text-lg"
                />
              </div>
              <p class="input-hint">
                {{ t('redeem.redeemCodeHint') }}
              </p>
            </div>

            <button
              type="submit"
              :disabled="!redeemCode || submitting"
              class="btn btn-primary w-full py-3"
            >
              <svg
                v-if="submitting"
                class="-ml-1 mr-2 h-5 w-5 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                ></circle>
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
              <Icon v-else name="checkCircle" size="md" class="mr-2" />
              {{ submitting ? t('redeem.redeeming') : t('redeem.redeemButton') }}
            </button>
          </form>
        </div>
      </div>

      <!-- Success Message -->
      <transition name="fade">
        <div
          v-if="redeemResult"
          class="card border-emerald-200 bg-emerald-50 dark:border-emerald-800/50 dark:bg-emerald-900/20"
        >
          <div class="p-6">
            <div class="flex items-start gap-4">
              <div
                class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-emerald-100 dark:bg-emerald-900/30"
              >
                <Icon name="checkCircle" size="md" class="text-emerald-600 dark:text-emerald-400" />
              </div>
              <div class="flex-1">
                <h3 class="text-sm font-semibold text-emerald-800 dark:text-emerald-300">
                  {{ t('redeem.redeemSuccess') }}
                </h3>
                <div class="mt-2 text-sm text-emerald-700 dark:text-emerald-400">
                  <p>{{ redeemResult.message }}</p>
                  <div class="mt-3 space-y-1">
                    <p v-if="redeemResult.type === 'balance'" class="font-medium">
                      {{ t('redeem.added') }}: ${{ redeemResult.value.toFixed(2) }}
                    </p>
                    <p v-else-if="redeemResult.type === 'concurrency'" class="font-medium">
                      {{ t('redeem.added') }}: {{ redeemResult.value }}
                      {{ t('redeem.concurrentRequests') }}
                    </p>
                    <p v-else-if="redeemResult.type === 'subscription'" class="font-medium">
                      {{ t('redeem.subscriptionAssigned') }}
                      <span v-if="redeemResult.group_name"> - {{ redeemResult.group_name }}</span>
                      <span v-if="redeemResult.validity_days">
                        ({{
                          t('redeem.subscriptionDays', { days: redeemResult.validity_days })
                        }})</span
                      >
                    </p>
                    <p v-if="redeemResult.new_balance !== undefined">
                      {{ t('redeem.newBalance') }}:
                      <span class="font-semibold">${{ redeemResult.new_balance.toFixed(2) }}</span>
                    </p>
                    <p v-if="redeemResult.new_concurrency !== undefined">
                      {{ t('redeem.newConcurrency') }}:
                      <span class="font-semibold"
                        >{{ redeemResult.new_concurrency }} {{ t('redeem.requests') }}</span
                      >
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </transition>

      <!-- Error Message -->
      <transition name="fade">
        <div
          v-if="errorMessage"
          class="card border-red-200 bg-red-50 dark:border-red-800/50 dark:bg-red-900/20"
        >
          <div class="p-6">
            <div class="flex items-start gap-4">
              <div
                class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-red-100 dark:bg-red-900/30"
              >
                <Icon
                  name="exclamationCircle"
                  size="md"
                  class="text-red-600 dark:text-red-400"
                />
              </div>
              <div class="flex-1">
                <h3 class="text-sm font-semibold text-red-800 dark:text-red-300">
                  {{ t('redeem.redeemFailed') }}
                </h3>
                <p class="mt-2 text-sm text-red-700 dark:text-red-400">
                  {{ errorMessage }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </transition>

          <!-- Recent balance activity -->
          <div class="card flex h-[380px] min-h-0 flex-none flex-col">
            <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('redeem.recentActivity') }}
              </h2>
            </div>
            <div class="flex min-h-0 flex-1 flex-col p-6">
              <div v-if="loadingActivity" class="flex min-h-0 flex-1 items-center justify-center py-8">
                <svg class="h-6 w-6 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
              </div>

              <div v-else-if="activities.length > 0" class="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
                <div
                  v-for="item in activities"
                  :key="item.id"
                  class="flex items-center justify-between gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-800"
                >
                  <div class="flex min-w-0 items-center gap-3">
                    <div
                      :class="[
                        'flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl',
                        getActivityIconClass(item.source)
                      ]"
                    >
                      <Icon :name="getActivityIcon(item.source)" size="md" />
                    </div>
                    <div class="min-w-0">
                      <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                        {{ getActivityTitle(item.source) }}
                      </p>
                      <p class="text-xs text-gray-500 dark:text-dark-400">
                        {{ formatDateTime(item.occurred_at) }}
                      </p>
                    </div>
                  </div>
                  <p class="flex-shrink-0 text-sm font-semibold text-emerald-600 dark:text-emerald-400">
                    +${{ item.amount.toFixed(2) }}
                  </p>
                </div>
              </div>

              <div v-else class="empty-state min-h-0 flex-1 py-8">
                <div
                  class="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-100 dark:bg-dark-800"
                >
                  <Icon name="clock" size="xl" class="text-gray-400 dark:text-dark-500" />
                </div>
                <p class="text-sm text-gray-500 dark:text-dark-400">
                  {{ t('redeem.activity.empty') }}
                </p>
              </div>
            </div>
          </div>

        </div>
        <!-- Daily lucky-wheel check-in -->
        <DailyCheckinWheel class="h-full" @reward-added="fetchActivity" />
      </div>

      <!-- Information Card -->
      <div
        class="card border-primary-200 bg-primary-50 dark:border-primary-800/50 dark:bg-primary-900/20"
      >
        <div class="p-6">
          <div class="flex items-start gap-4">
            <div
              class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-primary-100 dark:bg-primary-900/30"
            >
              <Icon name="infoCircle" size="md" class="text-primary-600 dark:text-primary-400" />
            </div>
            <div class="flex-1">
              <h3 class="text-sm font-semibold text-primary-800 dark:text-primary-300">
                {{ t('redeem.aboutCodes') }}
              </h3>
              <ul
                class="mt-2 list-inside list-disc space-y-1 text-sm text-primary-700 dark:text-primary-400"
              >
                <li>{{ t('redeem.codeRule1') }}</li>
                <li>{{ t('redeem.codeRule2') }}</li>
                <li>
                  {{ t('redeem.codeRule3') }}
                  <span
                    v-if="contactInfo"
                    class="ml-1.5 inline-flex items-center rounded-md bg-primary-200/50 px-2 py-0.5 text-xs font-medium text-primary-800 dark:bg-primary-800/40 dark:text-primary-200"
                  >
                    {{ contactInfo }}
                  </span>
                </li>
                <li>{{ t('redeem.codeRule4') }}</li>
              </ul>
            </div>
          </div>
        </div>
      </div>

    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useSubscriptionStore } from '@/stores/subscriptions'
import {
  authAPI,
  checkinAPI,
  paymentAPI,
  redeemAPI,
  type CheckinHistoryItem,
  type RedeemHistoryItem
} from '@/api'
import type { PaymentOrder } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import DailyCheckinWheel from '@/components/user/checkin/DailyCheckinWheel.vue'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()

const user = computed(() => authStore.user)

const redeemCode = ref('')
const submitting = ref(false)
const redeemResult = ref<{
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
  group_name?: string
  validity_days?: number
} | null>(null)
const errorMessage = ref('')

type BalanceActivitySource = 'redeem' | 'recharge' | 'checkin'

interface BalanceActivity {
  id: string
  source: BalanceActivitySource
  amount: number
  occurred_at: string
}

// Balance activity combines code redemptions, completed balance recharges, and check-in rewards.
const activities = ref<BalanceActivity[]>([])
const loadingActivity = ref(false)
const contactInfo = ref('')

const getActivityTitle = (source: BalanceActivitySource) => {
  if (source === 'redeem') return t('redeem.activity.redeem')
  if (source === 'recharge') return t('redeem.activity.recharge')
  return t('redeem.activity.checkin')
}

const getActivityIcon = (source: BalanceActivitySource) => {
  if (source === 'redeem') return 'gift'
  if (source === 'recharge') return 'creditCard'
  return 'checkCircle'
}

const getActivityIconClass = (source: BalanceActivitySource) => {
  if (source === 'redeem') return 'bg-purple-100 text-purple-600 dark:bg-purple-900/30 dark:text-purple-400'
  if (source === 'recharge') return 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400'
  return 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400'
}

const redeemActivities = (items: RedeemHistoryItem[]): BalanceActivity[] =>
  items
    .filter((item) => item.type === 'balance' && item.value > 0)
    .map((item) => ({
      id: `redeem-${item.id}`,
      source: 'redeem',
      amount: item.value,
      occurred_at: item.used_at
    }))

const rechargeActivities = (items: PaymentOrder[]): BalanceActivity[] =>
  items
    .filter((item) => item.order_type === 'balance' && item.amount > 0)
    .map((item) => ({
      id: `recharge-${item.id}`,
      source: 'recharge',
      amount: item.amount,
      occurred_at: item.completed_at || item.paid_at || item.created_at
    }))

const checkinActivities = (items: CheckinHistoryItem[]): BalanceActivity[] =>
  items
    .filter((item) => (item.total_amount ?? item.amount + (item.bonus_amount || 0)) > 0)
    .map((item) => ({
      id: `checkin-${item.id}`,
      source: 'checkin',
      amount: item.total_amount ?? item.amount + (item.bonus_amount || 0),
      occurred_at: item.checked_at
    }))

const fetchActivity = async () => {
  loadingActivity.value = true
  try {
    const [redeemResult, rechargeResult, checkinResult] = await Promise.allSettled([
      redeemAPI.getHistory(),
      paymentAPI.getMyOrders({ page: 1, page_size: 25, status: 'COMPLETED', order_type: 'balance' }),
      checkinAPI.getHistory()
    ])

    const next: BalanceActivity[] = []
    if (redeemResult.status === 'fulfilled') next.push(...redeemActivities(redeemResult.value))
    if (rechargeResult.status === 'fulfilled') next.push(...rechargeActivities(rechargeResult.value.data.items))
    if (checkinResult.status === 'fulfilled') next.push(...checkinActivities(checkinResult.value))

    activities.value = next
      .sort((a, b) => new Date(b.occurred_at).getTime() - new Date(a.occurred_at).getTime())
      .slice(0, 25)
  } catch (error) {
    console.error('Failed to fetch balance activity:', error)
    activities.value = []
  } finally {
    loadingActivity.value = false
  }
}

const handleRedeem = async () => {
  if (!redeemCode.value.trim()) {
    appStore.showError(t('redeem.pleaseEnterCode'))
    return
  }

  submitting.value = true
  errorMessage.value = ''
  redeemResult.value = null

  try {
    const result = await redeemAPI.redeem(redeemCode.value.trim())

    redeemResult.value = result

    // Refresh user data to get updated balance/concurrency
    await authStore.refreshUser()

    // If subscription type, immediately refresh subscription status
    if (result.type === 'subscription') {
      try {
        await subscriptionStore.fetchActiveSubscriptions(true) // force refresh
      } catch (error) {
        console.error('Failed to refresh subscriptions after redeem:', error)
        appStore.showWarning(t('redeem.subscriptionRefreshFailed'))
      }
    }

    // Clear the input
    redeemCode.value = ''

    // Refresh the combined balance activity list.
    await fetchActivity()

    // Show success toast
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
  } catch (error: any) {
    errorMessage.value = error.response?.data?.detail || t('redeem.failedToRedeem')

    appStore.showError(t('redeem.redeemFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  fetchActivity()
  try {
    const settings = await authAPI.getPublicSettings()
    contactInfo.value = settings.contact_info || ''
  } catch (error) {
    console.error('Failed to load contact info:', error)
  }
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
