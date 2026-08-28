<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <div v-else class="space-y-8">
        <!-- Purchasable plans -->
        <section v-if="plans.length">
          <div class="mb-4">
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white">
              {{ t('userSubscriptions.availablePlans') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('userSubscriptions.availablePlansDesc') }}
            </p>
          </div>
          <div class="mx-auto grid max-w-5xl grid-cols-1 gap-6 md:grid-cols-2">
            <div
              v-for="plan in plans"
              :key="plan.id"
              class="flex h-full min-h-[380px] flex-col rounded-2xl border border-primary-100 bg-white p-6 shadow-sm dark:border-primary-900/40 dark:bg-dark-800"
            >
              <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <p class="text-xs font-medium uppercase tracking-wide text-primary-600 dark:text-primary-400">
                    {{ plan.group_name || `Group #${plan.group_id}` }}
                  </p>
                  <h3 class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ plan.name }}</h3>
                </div>
                <div class="text-right">
                  <div class="text-2xl font-bold text-primary-600 dark:text-primary-400">
                    {{ formatPlanPrice(plan) }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-dark-400">
                    {{ t('userSubscriptions.planValidity', { days: plan.validity_days }) }}
                  </div>
                </div>
              </div>
              <p v-if="plan.description" class="mt-3 text-sm text-gray-600 dark:text-dark-300">{{ plan.description }}</p>
              <div class="mt-4 grid grid-cols-3 gap-2 text-xs">
                <div class="rounded-lg bg-gray-50 p-2 dark:bg-dark-700">
                  <div class="text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.daily') }}</div>
                  <div class="mt-1 font-semibold text-gray-800 dark:text-gray-200">{{ formatPlanLimit(plan.daily_limit_usd) }}</div>
                </div>
                <div class="rounded-lg bg-gray-50 p-2 dark:bg-dark-700">
                  <div class="text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.weekly') }}</div>
                  <div class="mt-1 font-semibold text-gray-800 dark:text-gray-200">{{ formatPlanLimit(plan.weekly_limit_usd) }}</div>
                </div>
                <div class="rounded-lg bg-gray-50 p-2 dark:bg-dark-700">
                  <div class="text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.monthly') }}</div>
                  <div class="mt-1 font-semibold text-gray-800 dark:text-gray-200">{{ formatPlanLimit(plan.monthly_limit_usd) }}</div>
                </div>
              </div>
              <ul v-if="plan.features?.length" class="mt-4 space-y-1 text-sm text-gray-600 dark:text-dark-300">
                <li v-for="feature in plan.features" :key="feature" class="flex gap-2">
                  <span class="text-emerald-500">✓</span><span>{{ feature }}</span>
                </li>
              </ul>
              <button
                class="mt-auto w-full rounded-xl bg-primary-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="purchasingPlanId === plan.id"
                @click="purchasePlan(plan)"
              >
                {{ purchasingPlanId === plan.id ? t('common.loading') : t('userSubscriptions.purchase') }}
              </button>
            </div>
          </div>
        </section>

        <!-- Empty State -->
        <div v-if="subscriptions.length === 0" class="card p-12 text-center">
          <div
            class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
          >
            <Icon name="creditCard" size="xl" class="text-gray-400" />
          </div>
          <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('userSubscriptions.noActiveSubscriptions') }}
          </h3>
          <p class="text-gray-500 dark:text-dark-400">
            {{ plans.length ? t('userSubscriptions.noPurchasedSubscriptionsDesc') : t('userSubscriptions.noActiveSubscriptionsDesc') }}
          </p>
        </div>

        <!-- Subscriptions Grid -->
      <div v-else class="mx-auto grid max-w-5xl gap-6 lg:grid-cols-2">
        <div
          v-for="subscription in subscriptions"
          :key="subscription.id"
          class="flex min-h-[380px] flex-col overflow-hidden rounded-2xl border bg-white dark:bg-dark-800"
          :class="platformBorderClass(subscription.group?.platform || '')"
        >
          <!-- Header -->
          <div
            class="flex items-center justify-between border-b border-gray-100 p-4 dark:border-dark-700"
          >
            <div class="flex items-center gap-3">
              <div :class="['h-1.5 w-1.5 shrink-0 rounded-full', platformAccentDotClass(subscription.group?.platform || '')]" />
              <div>
                <div class="flex items-center gap-2">
                  <h3 class="font-semibold text-gray-900 dark:text-white">
                    {{ subscription.group?.name || `Group #${subscription.group_id}` }}
                  </h3>
                  <span :class="['rounded-md border px-2 py-0.5 text-[11px] font-medium', platformBadgeClass(subscription.group?.platform || '')]">
                    {{ platformLabel(subscription.group?.platform || '') }}
                  </span>
                </div>
                <p v-if="subscription.group?.description" class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                  {{ subscription.group.description }}
                </p>
                <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-gray-400 dark:text-gray-500">
                  <span>{{ t('payment.planCard.rate') }}: ×{{ subscription.group?.rate_multiplier ?? 1 }}</span>
                  <span v-if="subscriptionHasPeakRate(subscription)" class="text-amber-700 dark:text-amber-300">
                    {{ t('payment.planCard.peakRate') }}: {{ subscriptionPeakRateLabel(subscription) }}
                  </span>
                </div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span
                :class="[
                  'rounded-full px-2 py-0.5 text-xs font-medium',
                  subscription.status === 'active'
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
                    : subscription.status === 'expired'
                      ? 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400'
                      : subscription.status === 'pending'
                        ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                        : 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                ]"
              >
                {{ t(`userSubscriptions.status.${subscription.status}`) }}
              </span>
              <button
                v-if="subscription.status === 'pending'"
                class="rounded-lg bg-amber-500 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-amber-600 disabled:opacity-60"
                :disabled="activatingSubscriptionId === subscription.id"
                @click="activateSubscription(subscription)"
              >
                {{ activatingSubscriptionId === subscription.id ? t('common.loading') : t('userSubscriptions.activate') }}
              </button>
            </div>
          </div>

          <!-- Usage Progress -->
          <div class="flex flex-1 flex-col gap-4 p-4">
            <!-- Expiration Info -->
            <div v-if="subscription.status === 'pending'" class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.validity') }}</span>
              <span class="font-medium text-amber-600 dark:text-amber-400">
                {{ t('userSubscriptions.readyToActivate') }} · {{ t('userSubscriptions.planValidity', { days: subscription.validity_days }) }}
              </span>
            </div>
            <div v-else-if="subscription.expires_at" class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{
                t('userSubscriptions.expires')
              }}</span>
              <span :class="getExpirationClass(subscription.expires_at)">
                {{ formatExpirationDate(subscription.expires_at) }}
              </span>
            </div>
            <div v-else class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{
                t('userSubscriptions.expires')
              }}</span>
              <span class="text-gray-700 dark:text-gray-300">{{
                t('userSubscriptions.noExpiration')
              }}</span>
            </div>

            <!-- Daily Usage -->
            <div v-if="getDailyLimit(subscription)" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.daily') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  ${{ (subscription.daily_usage_usd || 0).toFixed(2) }} / ${{
                    getDailyLimit(subscription)!.toFixed(2)
                  }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      subscription.daily_usage_usd,
                      getDailyLimit(subscription)
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      subscription.daily_usage_usd,
                      getDailyLimit(subscription)
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.daily_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{ formatDailyUsageWindow(subscription) }}
              </p>
            </div>

            <!-- Weekly Usage -->
            <div v-if="getWeeklyLimit(subscription)" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.weekly') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  ${{ (subscription.weekly_usage_usd || 0).toFixed(2) }} / ${{
                    getWeeklyLimit(subscription)!.toFixed(2)
                  }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      subscription.weekly_usage_usd,
                      getWeeklyLimit(subscription)
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      subscription.weekly_usage_usd,
                      getWeeklyLimit(subscription)
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.weekly_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.weekly_window_start, 168)
                  })
                }}
              </p>
            </div>

            <!-- Monthly Usage -->
            <div v-if="getMonthlyLimit(subscription)" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.monthly') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  ${{ (subscription.monthly_usage_usd || 0).toFixed(2) }} / ${{
                    getMonthlyLimit(subscription)!.toFixed(2)
                  }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      subscription.monthly_usage_usd,
                      getMonthlyLimit(subscription)
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      subscription.monthly_usage_usd,
                      getMonthlyLimit(subscription)
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.monthly_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.monthly_window_start, 720)
                  })
                }}
              </p>
            </div>

            <!-- No limits configured - Unlimited badge -->
            <div
              v-if="
                !getDailyLimit(subscription) &&
                !getWeeklyLimit(subscription) &&
                !getMonthlyLimit(subscription)
              "
              class="mt-auto flex items-center justify-center rounded-xl bg-gradient-to-r from-emerald-50 to-teal-50 py-6 dark:from-emerald-900/20 dark:to-teal-900/20"
            >
              <div class="flex items-center gap-3">
                <span class="text-4xl text-emerald-600 dark:text-emerald-400">∞</span>
                <div>
                  <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">
                    {{ t('userSubscriptions.unlimited') }}
                  </p>
                  <p class="text-xs text-emerald-600/70 dark:text-emerald-400/70">
                    {{ t('userSubscriptions.unlimitedDesc') }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :show="purchaseConfirmOpen"
      :title="t('userSubscriptions.purchaseConfirmTitle')"
      :message="purchaseConfirmMessage"
      @confirm="confirmPurchasePlan"
      @cancel="closePurchaseConfirm"
    >
      <div
        v-if="pendingPurchasePlan"
        class="rounded-xl border border-primary-100 bg-primary-50/70 p-4 dark:border-primary-900/50 dark:bg-primary-900/20"
      >
        <div class="flex items-start justify-between gap-4">
          <span class="font-semibold text-gray-900 dark:text-white">{{ pendingPurchasePlan.name }}</span>
          <span class="shrink-0 text-lg font-bold text-primary-600 dark:text-primary-400">
            {{ formatPlanPrice(pendingPurchasePlan) }}
          </span>
        </div>
        <div class="mt-2 flex items-center justify-between text-sm text-gray-500 dark:text-dark-300">
          <span>{{ t('userSubscriptions.validity') }}</span>
          <span>{{ t('userSubscriptions.planValidity', { days: pendingPurchasePlan.validity_days }) }}</span>
        </div>
      </div>
    </ConfirmDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI from '@/api/subscriptions'
import type { UserSubscription } from '@/types'
import type { SubscriptionPlan } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'
import { hasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { platformBorderClass, platformBadgeClass, platformLabel } from '@/utils/platformColors'
import {
  getExpirationDateRelation,
  getRemainingDurationParts,
  isOneTimeDailyQuota,
  type RemainingDurationParts
} from '@/utils/subscriptionQuota'

function platformAccentDotClass(p: string): string {
  switch (p) {
    case 'anthropic': return 'bg-orange-500'
    case 'openai': return 'bg-emerald-500'
    case 'antigravity': return 'bg-purple-500'
    case 'gemini': return 'bg-blue-500'
    default: return 'bg-gray-400'
  }
}

const { t } = useI18n()
const appStore = useAppStore()

const subscriptions = ref<UserSubscription[]>([])
const plans = ref<SubscriptionPlan[]>([])
const loading = ref(true)
const purchasingPlanId = ref<number | null>(null)
const activatingSubscriptionId = ref<number | null>(null)
const purchaseConfirmOpen = ref(false)
const pendingPurchasePlan = ref<SubscriptionPlan | null>(null)

const purchaseConfirmMessage = computed(() => {
  const planName = pendingPurchasePlan.value?.name || ''
  return t('userSubscriptions.purchaseConfirm', { name: planName })
})

function subscriptionHasPeakRate(subscription: UserSubscription): boolean {
  return hasPeakRate(subscription.group)
}

function subscriptionPeakRateLabel(subscription: UserSubscription): string {
  return formatPeakRateWindow(subscription.group, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
}

async function loadSubscriptions() {
  try {
    loading.value = true
    const [mySubscriptions, availablePlans] = await Promise.all([
      subscriptionsAPI.getMySubscriptions(),
      subscriptionsAPI.getAvailablePlans()
    ])
    subscriptions.value = mySubscriptions
    plans.value = availablePlans
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function purchasePlan(plan: SubscriptionPlan) {
  pendingPurchasePlan.value = plan
  purchaseConfirmOpen.value = true
}

function closePurchaseConfirm() {
  purchaseConfirmOpen.value = false
  pendingPurchasePlan.value = null
}

async function confirmPurchasePlan() {
  const plan = pendingPurchasePlan.value
  if (!plan) return

  closePurchaseConfirm()
  purchasingPlanId.value = plan.id
  try {
    await subscriptionsAPI.purchasePlan(plan.id)
    appStore.showSuccess(t('userSubscriptions.purchaseSuccess'))
    await loadSubscriptions()
  } catch (error) {
    console.error('Failed to purchase subscription:', error)
    appStore.showError(extractApiErrorMessage(error, t('userSubscriptions.purchaseFailed')))
  } finally {
    purchasingPlanId.value = null
  }
}

function formatPlanPrice(plan: SubscriptionPlan): string {
  return `${plan.currency || '$'}${plan.price.toFixed(2)}`
}

async function activateSubscription(subscription: UserSubscription) {
  activatingSubscriptionId.value = subscription.id
  try {
    await subscriptionsAPI.activateSubscription(subscription.id)
    appStore.showSuccess(t('userSubscriptions.activateSuccess'))
    await loadSubscriptions()
  } catch (error) {
    console.error('Failed to activate subscription:', error)
    appStore.showError(extractApiErrorMessage(error, t('userSubscriptions.activateFailed')))
  } finally {
    activatingSubscriptionId.value = null
  }
}

function getDailyLimit(subscription: UserSubscription): number | null | undefined {
  return subscription.daily_limit_usd ?? subscription.group?.daily_limit_usd
}

function getWeeklyLimit(subscription: UserSubscription): number | null | undefined {
  return subscription.weekly_limit_usd ?? subscription.group?.weekly_limit_usd
}

function getMonthlyLimit(subscription: UserSubscription): number | null | undefined {
  return subscription.monthly_limit_usd ?? subscription.group?.monthly_limit_usd
}

function formatPlanLimit(limit: number | null | undefined): string {
  return limit != null && limit > 0 ? `$${limit.toFixed(2)}` : t('userSubscriptions.unlimited')
}

function getProgressWidth(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return '0%'
  const percentage = Math.min(((used || 0) / limit) * 100, 100)
  return `${percentage}%`
}

function getProgressBarClass(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return 'bg-gray-400'
  const percentage = ((used || 0) / limit) * 100
  if (percentage >= 90) return 'bg-red-500'
  if (percentage >= 70) return 'bg-orange-500'
  return 'bg-green-500'
}

function formatExpirationDate(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  const relation = getExpirationDateRelation(expires, now)

  if (relation === null) return ''

  if (relation === 'expired') {
    return t('userSubscriptions.status.expired')
  }

  const dateStr = formatDateTimeToMinute(expires)

  if (relation === 'today') {
    return `${dateStr} (${t('common.today')})`
  }
  if (relation === 'tomorrow') {
    return `${dateStr} (${t('common.tomorrow')})`
  }

  return t('userSubscriptions.daysRemaining', { days }) + ` (${dateStr})`
}

function getExpirationClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (diff <= 0) return 'text-red-600 dark:text-red-400 font-medium'
  if (days <= 3) return 'text-red-600 dark:text-red-400'
  if (days <= 7) return 'text-orange-600 dark:text-orange-400'
  return 'text-gray-700 dark:text-gray-300'
}

function formatDurationParts(parts: RemainingDurationParts): string {
  if (parts.days > 0) {
    return `${parts.days}d ${parts.hours}h`
  }

  if (parts.hours > 0) {
    return `${parts.hours}h ${parts.minutes}m`
  }

  return `${parts.minutes}m`
}

function formatDailyUsageWindow(subscription: UserSubscription): string {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    if (!parts) return t('userSubscriptions.windowNotActive')
    return t('userSubscriptions.quotaEndsIn', { time: formatDurationParts(parts) })
  }

  return t('userSubscriptions.resetIn', {
    time: formatResetTime(subscription.daily_window_start, 24)
  })
}

function formatResetTime(windowStart: string | null, windowHours: number): string {
  if (!windowStart) return t('userSubscriptions.windowNotActive')

  const start = new Date(windowStart)
  const end = new Date(start.getTime() + windowHours * 60 * 60 * 1000)
  const parts = getRemainingDurationParts(end)

  return parts ? formatDurationParts(parts) : t('userSubscriptions.windowNotActive')
}

onMounted(() => {
  loadSubscriptions()
})
</script>
