<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else-if="detail">
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div class="card p-5">
            <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
              <Icon name="dollar" size="sm" class="text-primary-500" />
              {{ t('affiliate.stats.rebateRate') }}
            </p>
            <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
              {{ formattedRebateRate }}<span class="ml-0.5 text-base font-medium">%</span>
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              {{ t('affiliate.stats.rebateRateHint') }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.invitedUsers') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCount(detail.aff_count) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.availableQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
              {{ formatCurrency(detail.aff_quota) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.totalQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(detail.aff_history_quota) }}
            </p>
            <p v-if="detail.aff_frozen_quota > 0" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
              {{ t('affiliate.stats.frozenQuota') }}: {{ formatCurrency(detail.aff_frozen_quota) }}
            </p>
          </div>
        </div>

        <div v-if="campaignStatus" class="card border border-primary-200 p-6 dark:border-primary-900/40" data-test="campaign-invite-progress">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ campaignStatus.name }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('campaign.inviteQualification') }}</p>
            </div>
            <span class="self-start rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">
              {{ t(`campaign.phase.${campaignStatus.phase}`) }}
            </span>
          </div>

          <div class="mt-5 rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
            <div class="flex items-baseline justify-between gap-3">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('campaign.validInvites') }}</span>
              <span class="text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ formatCount(campaignStatus.valid_invite_count) }}</span>
            </div>
            <div v-if="campaignStatus.next_tier" class="mt-3">
              <div class="flex justify-between gap-3 text-xs text-gray-500 dark:text-dark-400">
                <span>{{ t('campaign.nextTier') }}: {{ campaignTierLabel(campaignStatus.next_tier.key, campaignStatus.next_tier.name) }}</span>
                <span>{{ t('campaign.tierProgress', { current: campaignStatus.next_tier_progress, threshold: campaignStatus.next_tier.threshold }) }}</span>
              </div>
              <div class="mt-2 h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
                <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${campaignProgressPercent}%` }"></div>
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                {{ t('campaign.tierRemaining', { count: campaignStatus.next_tier_remaining }) }}
              </p>
            </div>
            <p v-else class="mt-2 text-sm font-medium text-emerald-600 dark:text-emerald-400">
              {{ t('campaign.allTiersReached') }}
            </p>
          </div>

          <div class="mt-5">
            <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('campaign.tiersTitle') }}</p>
            <div class="mt-3 grid gap-3 sm:grid-cols-3">
              <div
                v-for="tier in campaignStatus.tiers"
                :key="tier.key"
                class="relative overflow-hidden rounded-xl border border-gray-200 bg-white p-4 text-gray-900 shadow-sm dark:border-slate-700/80 dark:bg-gradient-to-br dark:from-slate-950 dark:via-slate-900 dark:to-slate-800 dark:text-white"
                :data-test="`campaign-tier-${tier.key}`"
              >
                <div class="absolute -right-8 -top-10 h-24 w-24 rounded-full blur-2xl" :class="campaignTierTheme(tier.key).glow"></div>
                <div class="relative flex items-center gap-3">
                  <span
                    class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl shadow-lg ring-1 ring-white/25"
                    :class="campaignTierTheme(tier.key).icon"
                    aria-hidden="true"
                  >
                    <CampaignTierIcon :tier-key="tier.key" size="sm" />
                  </span>
                  <span class="min-w-0">
                    <span class="block truncate text-sm font-semibold text-gray-900 dark:text-white">{{ campaignTierLabel(tier.key, tier.name) }}{{ t('campaign.membershipUserSuffix') }}</span>
                    <span class="mt-0.5 flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                      <span class="text-xl font-semibold" :class="campaignTierTheme(tier.key).text">{{ membershipPercent(tier.factor) }}%</span>
                      <span class="text-[11px] font-normal text-gray-500 dark:text-slate-400">
                        {{ t('campaign.membershipExclusiveRate', { percent: membershipPercent(tier.factor) }) }}
                      </span>
                    </span>
                  </span>
                </div>
                <div class="relative mt-4 flex items-center justify-between gap-2 border-t border-gray-200 pt-3 text-xs text-gray-500 dark:border-white/10 dark:text-slate-300">
                  <span>{{ tier.threshold }} {{ t('campaign.validInvites') }}</span>
                  <span>{{ tier.duration_days }} {{ t('campaign.days') }}</span>
                </div>
              </div>
            </div>
          </div>

          <div v-if="campaignStatus.current_membership" class="mt-4 rounded-xl bg-primary-50 px-4 py-3 text-sm dark:bg-primary-900/20">
            <span class="font-semibold text-primary-800 dark:text-primary-200">{{ t('campaign.currentMembership') }}: {{ campaignTierLabel(campaignStatus.current_membership.tier_key, campaignStatus.current_membership.tier_name) }}</span>
            <span class="ml-2 text-primary-700 dark:text-primary-300">{{ t('campaign.membershipFactor', { factor: campaignStatus.current_membership.factor }) }} · {{ t('campaign.membershipRemaining', { days: membershipRemainingDays(campaignStatus.current_membership.expires_at) }) }}</span>
          </div>
        </div>

        <div class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.title') }}</h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.description') }}</p>

          <div class="mt-5 grid gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.yourCode') }}</p>
              <div class="flex flex-col items-stretch gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center">
                <code class="min-w-0 break-all text-sm font-semibold text-gray-900 dark:text-white sm:flex-1 sm:truncate">{{ detail.aff_code }}</code>
                <button class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="copyCode">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyCode') }}</span>
                </button>
              </div>
            </div>

            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.inviteLink') }}</p>
              <div class="flex flex-col items-stretch gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center">
                <code class="min-w-0 break-all text-sm text-gray-700 dark:text-gray-300 sm:flex-1 sm:truncate">{{ campaignInviteLink }}</code>
                <button class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="copyInviteLink">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyLink') }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="mt-5 rounded-xl border border-primary-200 bg-primary-50 p-4 dark:border-primary-900/40 dark:bg-primary-900/20">
            <p class="text-sm font-medium text-primary-800 dark:text-primary-200">{{ t('affiliate.tips.title') }}</p>
            <ul class="mt-2 space-y-1 text-sm text-primary-700 dark:text-primary-300">
              <li>1. {{ t('affiliate.tips.line1') }}</li>
              <li>2. {{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
              <li>3. {{ t('affiliate.tips.line3') }}</li>
              <li v-if="detail.aff_frozen_quota > 0">4. {{ t('affiliate.tips.line4') }}</li>
            </ul>
          </div>
        </div>

        <div class="card p-6">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.transfer.title') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.transfer.description') }}</p>
            </div>
            <button
              class="btn btn-primary"
              :disabled="transferring || detail.aff_quota <= 0"
              @click="transferQuota"
            >
              <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="dollar" size="sm" />
              <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
            </button>
          </div>
          <p v-if="detail.aff_quota <= 0" class="mt-3 text-sm text-amber-600 dark:text-amber-400">
            {{ t('affiliate.transfer.empty') }}
          </p>
        </div>

        <div class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.invitees.title') }}</h3>
          <div v-if="detail.invitees.length === 0" class="mt-4 rounded-xl border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
            {{ t('affiliate.invitees.empty') }}
          </div>
          <div v-else class="mt-4 overflow-x-auto">
            <table class="w-full min-w-[560px] text-left text-sm">
              <thead>
                <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.email') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.username') }}</th>
                  <th class="px-3 py-2 font-medium text-right">{{ t('affiliate.invitees.columns.rebate') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.joinedAt') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in detail.invitees"
                  :key="item.user_id"
                  class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"
                >
                  <td class="px-3 py-3 text-gray-900 dark:text-white">{{ item.email || '-' }}</td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ item.username || '-' }}</td>
                  <td class="px-3 py-3 text-right font-medium text-emerald-600 dark:text-emerald-400">{{ formatCurrency(item.total_rebate) }}</td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ formatDateTime(item.created_at) || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import CampaignTierIcon from '@/components/common/CampaignTierIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useCampaignStore } from '@/stores/campaign'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const campaignStore = useCampaignStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)
const campaignStatus = computed(() => campaignStore.status)

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

const campaignInviteLink = computed(() => campaignStatus.value?.invite_link || inviteLink.value)
const campaignProgressPercent = computed(() => {
  const next = campaignStatus.value?.next_tier
  if (!next) return 100
  return Math.min(100, Math.max(0, Math.round((campaignStatus.value?.next_tier_progress ?? 0) / next.threshold * 100)))
})

function campaignTierLabel(key: string, fallback: string): string {
  const translated = t(`campaign.tiers.${key}`)
  return translated === `campaign.tiers.${key}` ? fallback : translated
}

function membershipPercent(factor: number): number {
  return Math.round(Number(factor) * 100)
}

function campaignTierTheme(key: string): { text: string; icon: string; glow: string } {
  if (key === 'gold') {
    return {
      text: 'text-amber-600 dark:text-amber-300',
      icon: 'bg-gradient-to-br from-amber-300 via-orange-400 to-rose-500 text-white',
      glow: 'bg-amber-400/10 dark:bg-amber-400/20'
    }
  }
  if (key === 'diamond') {
    return {
      text: 'text-violet-600 dark:text-violet-300',
      icon: 'bg-gradient-to-br from-violet-400 via-indigo-500 to-blue-600 text-white',
      glow: 'bg-violet-400/10 dark:bg-violet-400/20'
    }
  }
  return {
    text: 'text-primary-600 dark:text-primary-300',
    icon: 'bg-gradient-to-br from-teal-400 via-cyan-500 to-sky-600 text-white',
    glow: 'bg-primary-400/10 dark:bg-primary-400/20'
  }
}

function membershipRemainingDays(expiresAt: string): number {
  return Math.max(0, Math.ceil((new Date(expiresAt).getTime() - Date.now()) / 86400000))
}

// Rebate rate is a percentage in the range [0, 100]; backend already clamps it.
// We trim trailing zeros (e.g. 20.00 → "20", 12.50 → "12.5") for a cleaner UI.
const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

function formatCount(value: number): string {
  return value.toLocaleString()
}

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!campaignInviteLink.value) return
  await copyToClipboard(campaignInviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

onMounted(() => {
  void loadAffiliateDetail()
  // The invitation page is an explicit activity entry point; do not show a
  // stale count after a payment/refund or an admin redeem-code review.
  void campaignStore.fetchStatus(true)
})
</script>
