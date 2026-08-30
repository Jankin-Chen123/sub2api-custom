<template>
  <AppLayout>
    <div class="subscription-page mx-auto max-w-[1440px]">
      <!-- Loading State -->
      <div v-if="loading" class="flex min-h-[28rem] items-center justify-center">
        <div
          class="h-9 w-9 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
          role="status"
          :aria-label="t('common.loading')"
        ></div>
      </div>

      <div v-else class="space-y-8">
        <!-- Page heading -->
        <header class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-[2rem]">
              {{ t('userSubscriptions.title') }}
            </h1>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
              {{ t('userSubscriptions.description') }}
            </p>
          </div>
          <div class="flex items-center gap-2 sm:gap-3">
            <div ref="managementRef" class="relative">
              <button
                type="button"
                data-test="manage-subscriptions"
                class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white/80 px-3 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:border-primary-400 hover:bg-primary-50/60 hover:text-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500/30 dark:border-dark-700 dark:bg-dark-900/70 dark:text-dark-200 dark:hover:border-primary-500 dark:hover:bg-primary-950/30 dark:hover:text-primary-300"
                :aria-expanded="managementOpen"
                aria-controls="subscription-management-menu"
                @click="managementOpen = !managementOpen"
              >
                <Icon name="creditCard" size="sm" />
                <span>{{ t('userSubscriptions.manage') }}</span>
                <span class="rounded-full bg-primary-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-400/10 dark:text-primary-300">
                  {{ visibleSubscriptions.length }}
                </span>
                <Icon :name="managementOpen ? 'chevronUp' : 'chevronDown'" size="xs" class="ml-0.5" />
              </button>

              <Transition name="dropdown">
                <div
                  v-if="managementOpen"
                  id="subscription-management-menu"
                  class="absolute right-0 z-30 mt-2 w-[min(26rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xl dark:border-dark-700 dark:bg-dark-900"
                  role="dialog"
                  :aria-label="t('userSubscriptions.manage')"
                >
                  <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-800">
                    <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('userSubscriptions.manageTitle') }}</p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.manageHint') }}</p>
                  </div>

                  <div v-if="visibleSubscriptions.length" class="max-h-[min(28rem,70vh)] space-y-2 overflow-y-auto p-2">
                    <div
                      v-for="subscription in visibleSubscriptions"
                      :key="subscription.id"
                      :data-test="`managed-subscription-${subscription.id}`"
                      class="rounded-lg border border-gray-100 bg-gray-50/70 p-3 dark:border-dark-800 dark:bg-dark-800/60"
                    >
                      <div class="flex items-start justify-between gap-3">
                        <div class="flex min-w-0 items-start gap-2.5">
                          <span :class="['mt-1.5 h-2 w-2 shrink-0 rounded-full', platformAccentDotClass(subscription.group?.platform || '')]"></span>
                          <div class="min-w-0">
                            <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">
                              {{ subscription.group?.name || `Group #${subscription.group_id}` }}
                            </p>
                            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                              #{{ subscription.id }} · {{ subscriptionRemainingLabel(subscription) }}
                            </p>
                          </div>
                        </div>
                        <span
                          :class="[
                            'shrink-0 rounded-md border px-2 py-1 text-[10px] font-semibold',
                            subscription.status === 'active'
                              ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-300'
                              : 'border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-300'
                          ]"
                        >
                          {{ t(`userSubscriptions.status.${subscription.status}`) }}
                        </span>
                      </div>

                      <div class="mt-2 flex items-center justify-between gap-3 pl-4.5">
                        <p class="min-w-0 text-xs text-gray-500 dark:text-dark-400">
                          {{ subscriptionExpiryLabel(subscription) }}
                        </p>
                        <span v-if="subscription.status === 'active'" class="shrink-0 text-xs font-medium text-emerald-600 dark:text-emerald-300">
                          {{ t('userSubscriptions.current') }}
                        </span>
                        <button
                          v-else-if="subscription.status === 'pending'"
                          type="button"
                          :data-test="`activate-subscription-${subscription.id}`"
                          class="shrink-0 rounded-lg px-2.5 py-1.5 text-xs font-semibold transition-colors disabled:cursor-not-allowed disabled:bg-gray-200 disabled:text-gray-400 dark:disabled:bg-dark-700 dark:disabled:text-dark-500"
                          :class="hasActiveSubscription ? 'bg-gray-200 text-gray-400 dark:bg-dark-700 dark:text-dark-500' : 'bg-amber-500 text-white hover:bg-amber-600'"
                          :disabled="hasActiveSubscription || activatingSubscriptionId === subscription.id"
                          :title="hasActiveSubscription ? t('userSubscriptions.activateBlocked') : undefined"
                          @click="activateSubscription(subscription)"
                        >
                          {{ activatingSubscriptionId === subscription.id ? t('common.loading') : hasActiveSubscription ? t('userSubscriptions.activateAfterExpiry') : t('userSubscriptions.activate') }}
                        </button>
                      </div>
                      <p v-if="subscription.status === 'pending' && hasActiveSubscription" class="mt-2 pl-4.5 text-[11px] leading-4 text-gray-400 dark:text-dark-500">
                        {{ t('userSubscriptions.activateBlocked') }}
                      </p>
                    </div>
                  </div>
                  <p v-else class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">
                    {{ t('userSubscriptions.manageEmpty') }}
                  </p>
                </div>
              </Transition>
            </div>

            <div class="hidden items-center gap-2 text-xs text-gray-500 dark:text-dark-400 sm:flex">
              <Icon name="shield" size="sm" class="text-primary-500 dark:text-primary-400" />
              <span>{{ t('userSubscriptions.balanceProtected') }}</span>
            </div>
          </div>
        </header>

        <!-- Current subscription summary -->
        <section
          class="subscription-surface overflow-hidden rounded-2xl border border-gray-200/80 bg-white/80 p-5 shadow-sm dark:border-dark-700/70 dark:bg-dark-900/70 sm:p-6"
          aria-labelledby="current-subscription-title"
        >
          <div class="flex items-center gap-3">
            <div class="flex items-center gap-3">
              <span class="flex h-9 w-9 items-center justify-center rounded-xl bg-primary-500/15 text-primary-600 dark:text-primary-300">
                <Icon name="checkCircle" size="md" />
              </span>
              <h2 id="current-subscription-title" class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('userSubscriptions.currentSubscription') }}
              </h2>
            </div>
          </div>

          <div v-if="activeSubscription" class="mt-6 grid gap-5 lg:grid-cols-[minmax(16rem,1fr)_minmax(0,2fr)] lg:items-center">
            <div class="min-w-0">
              <p class="text-xs font-medium uppercase tracking-[0.16em] text-primary-600 dark:text-primary-400">
                {{ activeSubscription.group?.name || `Group #${activeSubscription.group_id}` }}
              </p>
              <div class="mt-2 flex flex-wrap items-center gap-2.5">
                <h3 class="text-xl font-semibold text-gray-900 dark:text-white sm:text-2xl">
                  {{ activeSubscription.group?.name || `Group #${activeSubscription.group_id}` }}
                </h3>
                <span
                  :class="[
                    'rounded-md border px-2 py-1 text-[11px] font-semibold',
                    activeSubscription.status === 'active'
                      ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-300'
                      : 'border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-300'
                  ]"
                >
                  {{ t(`userSubscriptions.status.${activeSubscription.status}`) }}
                </span>
              </div>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                {{ t('payment.planCard.rate') }}: ×{{ activeSubscription.group?.rate_multiplier ?? 1 }}
                <span v-if="subscriptionHasPeakRate(activeSubscription)" class="ml-2 text-amber-600 dark:text-amber-300">
                  · {{ subscriptionPeakRateLabel(activeSubscription) }}
                </span>
              </p>
            </div>

            <div class="grid gap-3 sm:grid-cols-3">
              <div class="subscription-stat rounded-xl bg-gray-50/90 p-3.5 dark:bg-dark-800/80">
                <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <Icon name="calendar" size="sm" class="text-primary-500 dark:text-primary-400" />
                  <span>{{ t('userSubscriptions.remaining') }}</span>
                </div>
                <p class="mt-2 text-sm font-semibold text-gray-900 dark:text-white">
                  {{ subscriptionRemainingLabel(activeSubscription) }}
                </p>
              </div>
              <div class="subscription-stat rounded-xl bg-gray-50/90 p-3.5 dark:bg-dark-800/80">
                <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <Icon name="clock" size="sm" class="text-primary-500 dark:text-primary-400" />
                  <span>{{ t('userSubscriptions.expires') }}</span>
                </div>
                <p class="mt-2 text-sm font-semibold text-gray-900 dark:text-white">
                  {{ subscriptionExpiryLabel(activeSubscription) }}
                </p>
              </div>
              <div class="subscription-stat rounded-xl bg-gray-50/90 p-3.5 dark:bg-dark-800/80">
                <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <Icon name="chartBar" size="sm" class="text-primary-500 dark:text-primary-400" />
                  <span>{{ t('userSubscriptions.quota') }}</span>
                </div>
                <p class="mt-2 text-sm font-semibold text-gray-900 dark:text-white">
                  {{ subscriptionQuotaLabel(activeSubscription) }}
                </p>
              </div>
            </div>
          </div>

          <div v-else class="mt-5 flex flex-col items-start gap-3 rounded-xl border border-dashed border-gray-200 bg-gray-50/70 px-4 py-5 sm:flex-row sm:items-center dark:border-dark-700 dark:bg-dark-800/50">
            <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-200 text-gray-500 dark:bg-dark-700 dark:text-dark-400">
              <Icon name="creditCard" size="md" />
            </span>
            <div>
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('userSubscriptions.noActiveSubscriptions') }}
              </h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ plans.length ? t('userSubscriptions.noPurchasedSubscriptionsDesc') : t('userSubscriptions.noActiveSubscriptionsDesc') }}
              </p>
            </div>
            <button
              v-if="visibleSubscriptions.length"
              type="button"
              data-test="open-manage-subscriptions"
              class="shrink-0 rounded-lg border border-primary-500/30 bg-primary-500/10 px-3 py-2 text-xs font-semibold text-primary-700 transition-colors hover:bg-primary-500/20 dark:text-primary-300"
              @click="managementOpen = true"
            >
              {{ t('userSubscriptions.openManager') }}
            </button>
          </div>
        </section>

        <!-- Purchasable plans -->
        <section v-if="plans.length" id="subscription-plans" class="scroll-mt-6" aria-labelledby="available-plans-title">
          <div>
            <h2 id="available-plans-title" class="text-lg font-semibold text-gray-900 dark:text-white sm:text-xl">
              {{ t('userSubscriptions.availablePlans') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('userSubscriptions.availablePlansDesc') }}
            </p>
          </div>

          <div class="mt-5 grid grid-cols-1 gap-5 sm:grid-cols-2 xl:grid-cols-4">
            <article
              v-for="plan in visiblePlans"
              :key="plan.id"
              :data-test="`subscription-plan-${plan.id}`"
              :class="[
                'subscription-plan-card group relative flex min-h-[31rem] flex-col overflow-hidden rounded-2xl border bg-white/90 p-5 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:shadow-xl dark:bg-dark-900/80 sm:p-6',
                isRecommendedPlan(plan)
                  ? 'border-primary-500/80 ring-1 ring-primary-500/30 dark:border-primary-400/80'
                  : 'border-gray-200/90 dark:border-dark-700/80'
              ]"
            >
              <div v-if="isRecommendedPlan(plan)" class="pointer-events-none absolute inset-x-0 top-0 h-1 bg-primary-500"></div>

              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <p class="truncate text-xs font-medium uppercase tracking-[0.14em] text-primary-600 dark:text-primary-400">
                      {{ plan.group_name || `Group #${plan.group_id}` }}
                    </p>
                    <span v-if="isRecommendedPlan(plan)" class="rounded-md border border-primary-500/30 bg-primary-500/10 px-2 py-0.5 text-[10px] font-semibold text-primary-700 dark:text-primary-300">
                      {{ t('userSubscriptions.recommended') }}
                    </span>
                  </div>
                  <h3 class="mt-3 break-words text-lg font-semibold text-gray-900 dark:text-white sm:text-xl">
                    {{ plan.name }}
                  </h3>
                </div>
                <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-500/10 text-primary-600 dark:bg-primary-400/10 dark:text-primary-300">
                  <Icon name="sparkles" size="sm" />
                </span>
              </div>

              <p class="mt-2 min-h-[2.5rem] text-sm leading-5 text-gray-500 dark:text-dark-400">
                {{ plan.description || t('userSubscriptions.planDescriptionFallback') }}
              </p>

              <div class="mt-5 flex items-end gap-1.5">
                <span class="text-sm font-medium text-gray-500 dark:text-dark-400">{{ planCurrencyPrefix(plan) }}</span>
                <span class="text-3xl font-bold tracking-tight text-gray-900 dark:text-white">{{ plan.price.toFixed(2) }}</span>
                <span class="mb-1 text-xs font-medium text-gray-500 dark:text-dark-400">/ {{ planCycleLabel(plan) }}</span>
              </div>
              <div v-if="plan.original_price && plan.original_price > plan.price" class="mt-1 flex items-center gap-2">
                <span class="text-xs text-gray-400 line-through dark:text-dark-500">{{ planCurrencyPrefix(plan) }}{{ plan.original_price.toFixed(2) }}</span>
                <span class="rounded-md bg-primary-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:text-primary-300">
                  -{{ Math.round((1 - plan.price / plan.original_price) * 100) }}%
                </span>
              </div>

              <div class="my-5 h-px bg-gray-200/80 dark:bg-dark-700/80"></div>

              <ul class="space-y-3">
                <li v-for="feature in planFeatureList(plan)" :key="feature" class="flex items-start gap-2.5 text-sm text-gray-600 dark:text-dark-300">
                  <span class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-primary-500/15 text-primary-600 dark:text-primary-300">
                    <Icon name="check" size="xs" :stroke-width="2" />
                  </span>
                  <span class="leading-5">{{ feature }}</span>
                </li>
              </ul>

              <div class="mt-auto pt-6">
                <button
                  type="button"
                  :data-test="`plan-purchase-${plan.id}`"
                  class="w-full rounded-xl bg-primary-500 px-4 py-3 text-sm font-semibold text-white shadow-sm transition-all hover:bg-primary-600 active:scale-[0.99] disabled:cursor-not-allowed disabled:opacity-60 dark:bg-primary-600 dark:hover:bg-primary-500"
                  :disabled="purchasingPlanId === plan.id"
                  @click="purchasePlan(plan)"
                >
                  {{ purchasingPlanId === plan.id ? t('common.loading') : t('userSubscriptions.purchase') }}
                </button>
              </div>
            </article>
          </div>
        </section>

        <!-- Usage details stay available for subscriptions with quotas. -->
        <section v-if="visibleSubscriptions.some(hasUsageLimit)" class="subscription-surface rounded-2xl border border-gray-200/80 bg-white/70 p-5 shadow-sm dark:border-dark-700/70 dark:bg-dark-900/60 sm:p-6" aria-labelledby="usage-title">
          <div class="flex items-center justify-between gap-4">
            <div>
              <h2 id="usage-title" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('userSubscriptions.usage') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.usageDesc') }}</p>
            </div>
            <Icon name="chartBar" size="md" class="text-primary-500 dark:text-primary-400" />
          </div>

          <div class="mt-5 grid gap-4 md:grid-cols-2">
            <article v-for="subscription in visibleSubscriptions.filter(hasUsageLimit)" :key="`usage-${subscription.id}`" class="rounded-xl border border-gray-200/80 bg-gray-50/70 p-4 dark:border-dark-700/70 dark:bg-dark-800/60">
              <div class="flex items-center justify-between gap-3">
                <h3 class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ subscription.group?.name || `Group #${subscription.group_id}` }}</h3>
                <span class="text-xs text-gray-500 dark:text-dark-400">{{ subscriptionExpiryLabel(subscription) }}</span>
              </div>
              <div class="mt-4 space-y-4">
                <div v-if="getDailyLimit(subscription)" class="space-y-1.5">
                  <div class="flex items-center justify-between text-xs"><span class="text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.daily') }}</span><span class="font-medium text-gray-700 dark:text-dark-300">${{ (subscription.daily_usage_usd || 0).toFixed(2) }} / ${{ getDailyLimit(subscription)!.toFixed(2) }}</span></div>
                  <div class="h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700"><div class="h-full rounded-full transition-all" :class="getProgressBarClass(subscription.daily_usage_usd, getDailyLimit(subscription))" :style="{ width: getProgressWidth(subscription.daily_usage_usd, getDailyLimit(subscription)) }"></div></div>
                </div>
                <div v-if="getWeeklyLimit(subscription)" class="space-y-1.5">
                  <div class="flex items-center justify-between text-xs"><span class="text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.weekly') }}</span><span class="font-medium text-gray-700 dark:text-dark-300">${{ (subscription.weekly_usage_usd || 0).toFixed(2) }} / ${{ getWeeklyLimit(subscription)!.toFixed(2) }}</span></div>
                  <div class="h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700"><div class="h-full rounded-full transition-all" :class="getProgressBarClass(subscription.weekly_usage_usd, getWeeklyLimit(subscription))" :style="{ width: getProgressWidth(subscription.weekly_usage_usd, getWeeklyLimit(subscription)) }"></div></div>
                </div>
                <div v-if="getMonthlyLimit(subscription)" class="space-y-1.5">
                  <div class="flex items-center justify-between text-xs"><span class="text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.monthly') }}</span><span class="font-medium text-gray-700 dark:text-dark-300">${{ (subscription.monthly_usage_usd || 0).toFixed(2) }} / ${{ getMonthlyLimit(subscription)!.toFixed(2) }}</span></div>
                  <div class="h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700"><div class="h-full rounded-full transition-all" :class="getProgressBarClass(subscription.monthly_usage_usd, getMonthlyLimit(subscription))" :style="{ width: getProgressWidth(subscription.monthly_usage_usd, getMonthlyLimit(subscription)) }"></div></div>
                </div>
              </div>
            </article>
          </div>
        </section>

        <!-- Reassurance strip -->
        <section class="grid gap-px overflow-hidden rounded-2xl border border-gray-200/80 bg-gray-200/80 shadow-sm dark:border-dark-700/70 dark:bg-dark-700/70 sm:grid-cols-2 xl:grid-cols-4" :aria-label="t('userSubscriptions.benefitsTitle')">
          <div class="flex items-center gap-3 bg-white/75 px-4 py-4 dark:bg-dark-900/70 sm:px-5">
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-500/10 text-primary-600 dark:text-primary-300"><Icon name="bolt" size="sm" /></span>
            <div><p class="text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('userSubscriptions.quickStart') }}</p><p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.quickStartDesc') }}</p></div>
          </div>
          <div class="flex items-center gap-3 bg-white/75 px-4 py-4 dark:bg-dark-900/70 sm:px-5">
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-500/10 text-primary-600 dark:text-primary-300"><Icon name="shield" size="sm" /></span>
            <div><p class="text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('userSubscriptions.securePayment') }}</p><p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.securePaymentDesc') }}</p></div>
          </div>
          <div class="flex items-center gap-3 bg-white/75 px-4 py-4 dark:bg-dark-900/70 sm:px-5">
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-500/10 text-primary-600 dark:text-primary-300"><Icon name="refresh" size="sm" /></span>
            <div><p class="text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('userSubscriptions.cancelAnytime') }}</p><p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.cancelAnytimeDesc') }}</p></div>
          </div>
          <div class="flex items-center gap-3 bg-white/75 px-4 py-4 dark:bg-dark-900/70 sm:px-5">
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-500/10 text-primary-600 dark:text-primary-300"><Icon name="chatBubble" size="sm" /></span>
            <div><p class="text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('userSubscriptions.support') }}</p><p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.supportDesc') }}</p></div>
          </div>
        </section>
      </div>
    </div>

    <ConfirmDialog
      :show="purchaseConfirmOpen"
      :title="t('userSubscriptions.purchaseConfirmTitle')"
      :message="purchaseConfirmMessage"
      :confirm-text="t('userSubscriptions.confirmPayment')"
      @confirm="confirmPurchasePlan"
      @cancel="closePurchaseConfirm"
    >
      <div v-if="pendingPurchasePlan" class="space-y-4">
        <div class="rounded-xl border border-primary-500/20 bg-primary-500/10 p-4 dark:border-primary-400/20 dark:bg-primary-950/40">
          <div class="flex items-start gap-3">
            <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary-500/15 text-primary-500 dark:text-primary-300"><Icon name="creditCard" size="md" /></span>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ pendingPurchasePlan.name }}</p>
                <span v-if="isRecommendedPlan(pendingPurchasePlan)" class="rounded-md bg-primary-500/15 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:text-primary-300">{{ t('userSubscriptions.recommended') }}</span>
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.planValidity', { days: pendingPurchasePlan.validity_days }) }}</p>
            </div>
            <span class="shrink-0 text-base font-bold text-primary-600 dark:text-primary-300">{{ formatPlanPrice(pendingPurchasePlan) }}</span>
          </div>
        </div>

        <div class="flex items-center justify-between border-t border-gray-200 pt-3 dark:border-dark-700">
          <span class="text-sm text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.amountDue') }}</span>
          <span class="text-lg font-bold text-primary-600 dark:text-primary-300">{{ formatPlanPrice(pendingPurchasePlan) }}</span>
        </div>
      </div>
    </ConfirmDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
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
import { currencySymbol } from '@/components/payment/currency'

const { t } = useI18n()
const appStore = useAppStore()

const subscriptions = ref<UserSubscription[]>([])
const plans = ref<SubscriptionPlan[]>([])
const loading = ref(true)
const purchasingPlanId = ref<number | null>(null)
const activatingSubscriptionId = ref<number | null>(null)
const purchaseConfirmOpen = ref(false)
const pendingPurchasePlan = ref<SubscriptionPlan | null>(null)
const managementOpen = ref(false)
const managementRef = ref<HTMLElement | null>(null)
const currentTime = ref(Date.now())
let expiryRefreshTimer: ReturnType<typeof setInterval> | undefined

const visibleSubscriptions = computed(() =>
  subscriptions.value.filter((subscription) => {
    // Expired rows are intentionally excluded from this page. This also covers
    // stale rows whose status has not yet been updated by the server.
    if (subscription.status === 'expired') return false
    if (subscription.status !== 'active' || !subscription.expires_at) return true
    return new Date(subscription.expires_at).getTime() > currentTime.value
  }),
)

const activeSubscription = computed(() =>
  visibleSubscriptions.value.find((subscription) => subscription.status === 'active') || null,
)
const hasActiveSubscription = computed(() => Boolean(activeSubscription.value))

const visiblePlans = computed(() => plans.value)

const recommendedPlanId = computed(() => {
  const candidates = visiblePlans.value
  const discountedPlan = candidates.find((plan) => (plan.original_price || 0) > plan.price)
  if (discountedPlan) return discountedPlan.id
  return candidates.length >= 3 ? candidates[Math.floor(candidates.length / 2)].id : null
})

const purchaseConfirmMessage = computed(() => {
  const planName = pendingPurchasePlan.value?.name || ''
  return t('userSubscriptions.purchaseConfirm', { name: planName })
})

function platformAccentDotClass(platform: string): string {
  switch (platform) {
    case 'anthropic': return 'bg-orange-500'
    case 'openai': return 'bg-emerald-500'
    case 'antigravity': return 'bg-purple-500'
    case 'gemini': return 'bg-blue-500'
    default: return 'bg-gray-400'
  }
}

function normalizeFeatures(features: unknown): string[] {
  if (Array.isArray(features)) return features.map(String).map((feature) => feature.trim()).filter(Boolean)
  if (typeof features === 'string') return features.split('\n').map((feature) => feature.trim()).filter(Boolean)
  return []
}

async function loadSubscriptions() {
  try {
    loading.value = true
    const [mySubscriptions, availablePlans] = await Promise.all([
      subscriptionsAPI.getMySubscriptions(),
      subscriptionsAPI.getAvailablePlans(),
    ])
    subscriptions.value = mySubscriptions
    plans.value = availablePlans.map((plan) => ({
      ...plan,
      features: normalizeFeatures(plan.features),
    }))
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function isRecommendedPlan(plan: SubscriptionPlan): boolean {
  return recommendedPlanId.value === plan.id
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

async function activateSubscription(subscription: UserSubscription) {
  if (subscription.status !== 'pending') return
  if (hasActiveSubscription.value) {
    appStore.showError(t('userSubscriptions.activateBlocked'))
    return
  }

  managementOpen.value = false
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

function handleManagementDocumentClick(event: MouseEvent) {
  if (managementRef.value && !managementRef.value.contains(event.target as Node)) {
    managementOpen.value = false
  }
}

function handleManagementKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    managementOpen.value = false
  }
}

function planCurrencyPrefix(plan: SubscriptionPlan): string {
  const rawCurrency = String(plan.currency || '').trim()
  if (!rawCurrency || rawCurrency === '$') return '$'
  return /^[A-Z]{3}$/i.test(rawCurrency) ? currencySymbol(rawCurrency) : rawCurrency
}

function formatPlanPrice(plan: SubscriptionPlan): string {
  return `${planCurrencyPrefix(plan)}${plan.price.toFixed(2)}`
}

function planCycleLabel(plan: SubscriptionPlan): string {
  const unit = String(plan.validity_unit || 'day').trim().toLowerCase().replace(/s$/, '')
  if (unit === 'month' || (plan.validity_days >= 25 && plan.validity_days <= 45)) return t('userSubscriptions.month')
  if (unit === 'week' || (plan.validity_days >= 6 && plan.validity_days <= 14)) return t('userSubscriptions.week')
  if (plan.validity_days > 180) return t('userSubscriptions.year')
  if (plan.validity_days > 45) return t('userSubscriptions.quarter')
  return t('userSubscriptions.planValidity', { days: plan.validity_days })
}

function planFeatureList(plan: SubscriptionPlan): string[] {
  const features = [
    `${t('payment.planCard.rate')}: ×${Number((plan.rate_multiplier ?? 1).toPrecision(10))}`,
    `${t('userSubscriptions.validity')}: ${t('userSubscriptions.planValidity', { days: plan.validity_days })}`,
  ]
  if (plan.daily_limit_usd != null) features.push(`${t('userSubscriptions.daily')}: ${formatPlanLimit(plan.daily_limit_usd)}`)
  else if (plan.weekly_limit_usd != null) features.push(`${t('userSubscriptions.weekly')}: ${formatPlanLimit(plan.weekly_limit_usd)}`)
  else if (plan.monthly_limit_usd != null) features.push(`${t('userSubscriptions.monthly')}: ${formatPlanLimit(plan.monthly_limit_usd)}`)
  else features.push(`${t('userSubscriptions.quota')}: ${t('userSubscriptions.unlimited')}`)
  return [...features, ...normalizeFeatures(plan.features)]
}

function formatPlanLimit(limit: number | null | undefined): string {
  return limit != null && limit > 0 ? `$${limit.toFixed(2)}` : t('userSubscriptions.unlimited')
}

function subscriptionHasPeakRate(subscription: UserSubscription): boolean {
  return hasPeakRate(subscription.group)
}

function subscriptionPeakRateLabel(subscription: UserSubscription): string {
  return formatPeakRateWindow(subscription.group, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
}

function subscriptionRemainingLabel(subscription: UserSubscription): string {
  if (subscription.status === 'pending') return t('userSubscriptions.readyToActivate')
  if (!subscription.expires_at) return t('userSubscriptions.noExpiration')
  const diff = new Date(subscription.expires_at).getTime() - currentTime.value
  if (diff <= 0) return t('userSubscriptions.status.expired')
  return t('userSubscriptions.planValidity', { days: Math.max(1, Math.ceil(diff / (1000 * 60 * 60 * 24))) })
}

function subscriptionExpiryLabel(subscription: UserSubscription): string {
  if (subscription.status === 'pending') return t('userSubscriptions.readyToActivate')
  return subscription.expires_at ? formatDateTimeToMinute(new Date(subscription.expires_at)) : t('userSubscriptions.noExpiration')
}

function subscriptionQuotaLabel(subscription: UserSubscription): string {
  if (getDailyLimit(subscription)) return `${t('userSubscriptions.daily')} ${formatPlanLimit(getDailyLimit(subscription))}`
  if (getWeeklyLimit(subscription)) return `${t('userSubscriptions.weekly')} ${formatPlanLimit(getWeeklyLimit(subscription))}`
  if (getMonthlyLimit(subscription)) return `${t('userSubscriptions.monthly')} ${formatPlanLimit(getMonthlyLimit(subscription))}`
  return t('userSubscriptions.unlimited')
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

function hasUsageLimit(subscription: UserSubscription): boolean {
  return Boolean(getDailyLimit(subscription) || getWeeklyLimit(subscription) || getMonthlyLimit(subscription))
}

function getProgressWidth(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return '0%'
  return `${Math.min(((used || 0) / limit) * 100, 100)}%`
}

function getProgressBarClass(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return 'bg-gray-400'
  const percentage = ((used || 0) / limit) * 100
  if (percentage >= 90) return 'bg-red-500'
  if (percentage >= 70) return 'bg-orange-500'
  return 'bg-primary-500'
}

onMounted(() => {
  loadSubscriptions()
  document.addEventListener('click', handleManagementDocumentClick)
  document.addEventListener('keydown', handleManagementKeydown)
  expiryRefreshTimer = setInterval(() => {
    currentTime.value = Date.now()
  }, 60_000)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleManagementDocumentClick)
  document.removeEventListener('keydown', handleManagementKeydown)
  if (expiryRefreshTimer) clearInterval(expiryRefreshTimer)
})
</script>
