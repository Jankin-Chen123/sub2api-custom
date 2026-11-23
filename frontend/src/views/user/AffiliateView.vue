<template>
  <AppLayout>
    <div class="space-y-5">
      <JournalPageHeader
        icon="share"
        variant="sky"
        compact
        :title="t('affiliate.title')"
        :description="t('affiliate.description')"
      >
        <template v-if="detail" #actions>
          <div class="inline-flex items-center gap-1.5 rounded-full border border-[#a9d7f5] bg-[#fffdf8]/95 px-3 py-1.5 text-xs font-bold text-[#1c5988] shadow-xs dark:border-sky-500/35 dark:bg-dark-800/85 dark:text-sky-200">
            <CraftIcon name="coins" size="xs" />
            <span>{{ t('affiliate.stats.rebateRate') }}</span>
            <span class="font-black text-[#c26328] dark:text-amber-300">{{ formattedRebateRate }}%</span>
          </div>
          <div
            v-if="campaignStatus?.current_membership"
            class="inline-flex items-center gap-1.5 rounded-full border border-[#e3c497] bg-[#fffaf0]/95 px-3 py-1.5 text-xs font-bold text-[#7c4516] shadow-xs dark:border-amber-500/35 dark:bg-dark-800/85 dark:text-amber-200"
          >
            <CampaignTierIcon :tier-key="campaignStatus.current_membership.tier_key" size="xs" />
            <span>{{ campaignTierLabel(campaignStatus.current_membership.tier_key, campaignStatus.current_membership.tier_name) }}{{ t('campaign.membershipUserSuffix') }}</span>
            <span class="font-semibold opacity-80">×{{ campaignStatus.current_membership.factor }}</span>
          </div>
        </template>
      </JournalPageHeader>

      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else-if="detail">
        <!-- Zone 2: Primary Above-the-Fold Workbench (Rebate Transfer + Invite Share) -->
        <div class="grid items-stretch gap-5 lg:grid-cols-12">
          <!-- Left: Unified Rebate Wallet & Transfer-to-Balance Card -->
          <section
            data-test="affiliate-transfer-panel"
            class="card flex flex-col overflow-hidden p-0 lg:col-span-5"
          >
            <div class="journal-card-band flex items-center justify-between gap-3 px-5 py-3.5">
              <div class="flex items-center gap-2.5">
                <span class="journal-icon-well journal-icon-well--sm">
                  <CraftIcon name="wallet" size="xs" />
                </span>
                <h3 class="journal-section-title text-base font-extrabold text-[#221d17] dark:text-white">
                  {{ t('affiliate.transfer.title') }}
                </h3>
              </div>
              <CraftSparks variant="right" size="sm" />
            </div>

            <div class="flex flex-1 flex-col justify-between gap-4 p-5">
              <div class="rounded-2xl border border-[#d6e9dc] bg-[#f4fbf6]/90 p-4 dark:border-emerald-900/40 dark:bg-emerald-950/25">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span class="text-xs font-bold text-[#4a6654] dark:text-emerald-200/80">
                    {{ t('affiliate.stats.availableQuota') }}
                  </span>
                  <span
                    v-if="detail.aff_frozen_quota > 0"
                    class="rounded-full border border-amber-200 bg-amber-50 px-2.5 py-0.5 text-[11px] font-bold text-amber-700 dark:border-amber-700/50 dark:bg-amber-900/30 dark:text-amber-300"
                  >
                    {{ t('affiliate.stats.frozenQuota') }}: {{ formatCurrency(detail.aff_frozen_quota) }}
                  </span>
                </div>
                <p class="mt-1.5 text-3xl font-black tracking-tight tabular-nums text-emerald-700 dark:text-emerald-400">
                  {{ formatCurrency(detail.aff_quota) }}
                </p>
                <p class="mt-1.5 text-xs leading-relaxed text-[#5a4e3e] dark:text-dark-300">
                  {{ t('affiliate.transfer.description') }}
                </p>
                <button
                  class="btn btn-primary mt-3.5 w-full"
                  :disabled="transferring || detail.aff_quota <= 0"
                  @click="transferQuota"
                >
                  <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
                  <Icon v-else name="dollar" size="sm" />
                  <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
                </button>
                <p
                  v-if="detail.aff_quota <= 0"
                  class="mt-2.5 rounded-xl border border-dashed border-[#e3c497] bg-[#fff7e6] px-3 py-2 text-xs font-medium text-[#92541a] dark:border-amber-800/50 dark:bg-amber-950/20 dark:text-amber-400"
                >
                  {{ t('affiliate.transfer.empty') }}
                </p>
              </div>

              <div class="grid grid-cols-3 divide-x divide-[#ebe0cc] rounded-2xl border border-[#e6d7be] bg-[#fffaf0]/90 py-3 dark:divide-dark-700 dark:border-dark-700 dark:bg-dark-900/70">
                <div class="px-3 text-center">
                  <p class="text-[11px] font-semibold text-[#6f6252] dark:text-dark-400">
                    {{ t('affiliate.stats.rebateRate') }}
                  </p>
                  <p class="mt-1 text-lg font-black tabular-nums text-[#c26328] dark:text-primary-400">
                    {{ formattedRebateRate }}%
                  </p>
                </div>
                <div class="px-3 text-center">
                  <p class="text-[11px] font-semibold text-[#6f6252] dark:text-dark-400">
                    {{ t('affiliate.stats.invitedUsers') }}
                  </p>
                  <p class="mt-1 text-lg font-black tabular-nums text-[#221d17] dark:text-white">
                    {{ formatCount(detail.aff_count) }}
                  </p>
                </div>
                <div class="px-3 text-center">
                  <p class="text-[11px] font-semibold text-[#6f6252] dark:text-dark-400">
                    {{ t('affiliate.stats.totalQuota') }}
                  </p>
                  <p class="mt-1 text-lg font-black tabular-nums text-[#221d17] dark:text-white">
                    {{ formatCurrency(detail.aff_history_quota) }}
                  </p>
                </div>
              </div>
            </div>
          </section>

          <!-- Right: Unified Invite Code, Link & Rules Card -->
          <section
            data-test="affiliate-share-panel"
            class="card flex flex-col overflow-hidden p-0 lg:col-span-7"
          >
            <div class="journal-card-band flex items-center justify-between gap-3 px-5 py-3.5">
              <div class="flex items-center gap-2.5">
                <span class="journal-icon-well journal-icon-well--sm">
                  <CraftIcon name="share" size="xs" />
                </span>
                <h3 class="journal-section-title text-base font-extrabold text-[#221d17] dark:text-white">
                  {{ t('affiliate.inviteLink') }}
                </h3>
              </div>
              <CraftSparks variant="right" size="sm" />
            </div>

            <div class="flex flex-1 flex-col justify-between gap-4 p-5">
              <div class="grid gap-3.5 sm:grid-cols-1">
                <div class="space-y-1.5">
                  <p class="text-xs font-bold text-[#4e4336] dark:text-gray-300">{{ t('affiliate.yourCode') }}</p>
                  <div class="flex flex-col items-stretch gap-2 rounded-xl border border-[#dcc6a4] bg-[#fffdf8]/95 px-3.5 py-2.5 shadow-[0_2px_0_rgba(143,108,65,0.08)] dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center">
                    <code class="min-w-0 break-all text-sm font-bold text-[#221d17] dark:text-white sm:flex-1 sm:truncate">{{ detail.aff_code }}</code>
                    <button class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="copyCode">
                      <Icon name="copy" size="sm" />
                      <span>{{ t('affiliate.copyCode') }}</span>
                    </button>
                  </div>
                </div>

                <div class="space-y-1.5">
                  <p class="text-xs font-bold text-[#4e4336] dark:text-gray-300">{{ t('affiliate.inviteLink') }}</p>
                  <div class="flex flex-col items-stretch gap-2 rounded-xl border border-[#dcc6a4] bg-[#fffdf8]/95 px-3.5 py-2.5 shadow-[0_2px_0_rgba(143,108,65,0.08)] dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center">
                    <code class="min-w-0 break-all text-sm text-[#4a4236] dark:text-gray-300 sm:flex-1 sm:truncate">{{ campaignInviteLink }}</code>
                    <button class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="copyInviteLink">
                      <Icon name="copy" size="sm" />
                      <span>{{ t('affiliate.copyLink') }}</span>
                    </button>
                  </div>
                </div>
              </div>

              <div class="rounded-2xl border-2 border-dashed border-[#d9c4a3] bg-[#fffaf0]/90 p-4 dark:border-primary-900/50 dark:bg-dark-900/75">
                <p class="flex items-center gap-1.5 text-xs font-extrabold text-[#8a4d17] dark:text-primary-200">
                  <CraftIcon name="sparkles" size="xs" />
                  <span>{{ t('affiliate.tips.title') }}</span>
                </p>
                <ul class="mt-2.5 grid gap-2 text-xs leading-relaxed text-[#5a4e3e] dark:text-primary-300 sm:grid-cols-2">
                  <li class="flex items-start gap-2">
                    <span class="mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-[#dcc29c] bg-[#fff3d6] text-[11px] font-extrabold text-[#8a4d17] dark:border-amber-700/60 dark:bg-amber-900/40 dark:text-amber-200">1</span>
                    <span>{{ t('affiliate.tips.line1') }}</span>
                  </li>
                  <li class="flex items-start gap-2">
                    <span class="mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-[#dcc29c] bg-[#fff3d6] text-[11px] font-extrabold text-[#8a4d17] dark:border-amber-700/60 dark:bg-amber-900/40 dark:text-amber-200">2</span>
                    <span>{{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</span>
                  </li>
                  <li class="flex items-start gap-2">
                    <span class="mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-[#dcc29c] bg-[#fff3d6] text-[11px] font-extrabold text-[#8a4d17] dark:border-amber-700/60 dark:bg-amber-900/40 dark:text-amber-200">3</span>
                    <span>{{ t('affiliate.tips.line3') }}</span>
                  </li>
                  <li v-if="detail.aff_frozen_quota > 0" class="flex items-start gap-2">
                    <span class="mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-[#dcc29c] bg-[#fff3d6] text-[11px] font-extrabold text-[#8a4d17] dark:border-amber-700/60 dark:bg-amber-900/40 dark:text-amber-200">4</span>
                    <span>{{ t('affiliate.tips.line4') }}</span>
                  </li>
                </ul>
              </div>
            </div>
          </section>
        </div>

        <!-- Zone 3: Campaign Invite Progress & Membership Tiers + Invitees -->
        <div v-if="campaignStatus" class="card overflow-hidden p-0" data-test="campaign-invite-progress">
          <div class="journal-card-band flex flex-col gap-2 px-5 py-3.5 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex items-start gap-3">
              <span class="journal-icon-well journal-icon-well--sm mt-0.5 shrink-0">
                <CraftIcon name="crown" size="xs" />
              </span>
              <div>
                <div class="flex items-center gap-1.5">
                  <h3 class="journal-section-title text-base font-extrabold text-[#221d17] dark:text-white">{{ campaignStatus.name }}</h3>
                  <CraftSparks variant="right" size="sm" />
                </div>
                <p class="mt-0.5 text-xs text-[#6f6252] dark:text-dark-400">{{ t('campaign.inviteQualification') }}</p>
              </div>
            </div>
            <span class="self-start rounded-full border border-[#e3c497] bg-[#fff5de] px-3 py-1 text-xs font-bold text-[#8a4d17] dark:border-primary-800/60 dark:bg-primary-900/20 dark:text-primary-300">
              {{ t(`campaign.phase.${campaignStatus.phase}`) }}
            </span>
          </div>

          <div class="space-y-4 p-5">
            <div class="rounded-2xl border border-[#e3d2b6] bg-[#fffaf0]/90 p-4 dark:border-dark-700 dark:bg-dark-900">
              <div class="flex items-baseline justify-between gap-3">
                <span class="text-sm font-bold text-[#4e4336] dark:text-gray-300">{{ t('campaign.validInvites') }}</span>
                <span class="text-2xl font-black text-[#c26328] dark:text-primary-400">{{ formatCount(campaignStatus.valid_invite_count) }}</span>
              </div>
              <div v-if="campaignStatus.next_tier" class="mt-2.5">
                <div class="flex justify-between gap-3 text-xs font-medium text-[#6f6252] dark:text-dark-400">
                  <span>{{ t('campaign.nextTier') }}: {{ campaignTierLabel(campaignStatus.next_tier.key, campaignStatus.next_tier.name) }}</span>
                  <span>{{ t('campaign.tierProgress', { current: campaignStatus.next_tier_progress, threshold: campaignStatus.next_tier.threshold }) }}</span>
                </div>
                <div class="mt-2 h-2 overflow-hidden rounded-full bg-[#efe2cb] dark:bg-dark-700">
                  <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${campaignProgressPercent}%` }"></div>
                </div>
                <p class="mt-2 text-xs text-[#6f6252] dark:text-dark-400">
                  {{ t('campaign.tierRemaining', { count: campaignStatus.next_tier_remaining }) }}
                </p>
              </div>
              <p v-else class="mt-2 text-sm font-semibold text-emerald-700 dark:text-emerald-400">
                {{ t('campaign.allTiersReached') }}
              </p>

              <div
                v-if="campaignStatus.current_membership"
                class="journal-callout-sky mt-3 flex flex-wrap items-center gap-2 rounded-xl px-3.5 py-2.5 text-xs sm:text-sm"
              >
                <CampaignTierIcon :tier-key="campaignStatus.current_membership.tier_key" size="xs" />
                <span class="font-semibold text-[var(--craft-ink)]">{{ t('campaign.currentMembership') }}: {{ campaignTierLabel(campaignStatus.current_membership.tier_key, campaignStatus.current_membership.tier_name) }}</span>
                <span class="text-[var(--craft-sky-deep)]">{{ t('campaign.membershipFactor', { factor: campaignStatus.current_membership.factor }) }} · {{ t('campaign.membershipRemaining', { days: membershipRemainingDays(campaignStatus.current_membership.expires_at) }) }}</span>
              </div>
            </div>

            <div>
              <p class="text-sm font-bold text-[#4e4336] dark:text-gray-300">{{ t('campaign.tiersTitle') }}</p>
              <div class="mt-2.5 grid gap-3 sm:grid-cols-3">
                <div
                  v-for="tier in campaignStatus.tiers"
                  :key="tier.key"
                  class="relative overflow-hidden rounded-2xl border border-[var(--craft-border-strong)] bg-[var(--craft-surface)] p-4 text-[var(--craft-ink)] shadow-[0_4px_0_rgba(143,108,65,0.08)] dark:bg-gradient-to-br dark:from-[#1d2230] dark:via-[#212738] dark:to-[#272d3f]"
                  :data-test="`campaign-tier-${tier.key}`"
                >
                  <div class="absolute -right-8 -top-10 h-24 w-24 rounded-full blur-2xl" :class="campaignTierTheme(tier.key).glow"></div>
                  <div class="relative flex items-center gap-3">
                    <span
                      class="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl"
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
                  <div class="relative mt-3.5 flex items-center justify-between gap-2 border-t border-dashed border-[#e5d5ba] pt-2.5 text-xs font-medium text-[#6f6252] dark:border-white/10 dark:text-slate-300">
                    <span>{{ tier.threshold }} {{ t('campaign.validInvites') }}</span>
                    <span>{{ tier.duration_days }} {{ t('campaign.days') }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="card overflow-hidden p-0">
          <div class="journal-card-band flex items-center justify-between gap-3 px-5 py-3.5">
            <div class="flex items-center gap-2.5">
              <span class="journal-icon-well journal-icon-well--sm">
                <CraftIcon name="users" size="xs" />
              </span>
              <h3 class="journal-section-title text-base font-extrabold text-[#221d17] dark:text-white">
                {{ t('affiliate.invitees.title') }}
              </h3>
              <span class="rounded-full border border-[#e3c497] bg-[#fff5de] px-2 py-0.5 text-xs font-bold text-[#8a4d17] dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300">
                {{ formatCount(detail.invitees.length) }}
              </span>
            </div>
            <CraftSparks variant="right" size="sm" />
          </div>
          <div class="p-5">
            <div v-if="detail.invitees.length === 0" class="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-[#dcc6a4] bg-[#fffaf0]/80 p-6 text-center dark:border-dark-700 dark:bg-dark-800/50">
              <div class="relative mb-2.5 inline-flex">
                <div class="journal-icon-well -rotate-3">
                  <CraftIcon name="inbox" size="lg" />
                </div>
                <CraftSparks variant="right" size="sm" class="ml-1 self-start" />
              </div>
              <p class="text-sm font-bold text-[#4e4336] dark:text-dark-300">
                {{ t('affiliate.invitees.empty') }}
              </p>
            </div>
            <div v-else class="overflow-x-auto">
              <table class="w-full min-w-[560px] text-left text-sm">
                <thead>
                  <tr class="border-b border-[#e8dac0] bg-[#fff4de]/90 text-xs font-extrabold uppercase tracking-wide text-[#6b5336] dark:border-dark-700 dark:bg-dark-800/70 dark:text-dark-300">
                    <th class="px-3 py-2.5 font-extrabold">{{ t('affiliate.invitees.columns.email') }}</th>
                    <th class="px-3 py-2.5 font-extrabold">{{ t('affiliate.invitees.columns.username') }}</th>
                    <th class="px-3 py-2.5 text-right font-extrabold">{{ t('affiliate.invitees.columns.rebate') }}</th>
                    <th class="px-3 py-2.5 font-extrabold">{{ t('affiliate.invitees.columns.joinedAt') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="item in detail.invitees"
                    :key="item.user_id"
                    class="border-b border-dashed border-[#efe2cb] last:border-b-0 dark:border-dark-800"
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
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import CraftIcon from '@/components/craft/CraftIcon.vue'
import CraftSparks from '@/components/craft/CraftSparks.vue'
import JournalPageHeader from '@/components/craft/JournalPageHeader.vue'
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
      text: 'text-[#b5661b] dark:text-amber-300',
      icon: 'border border-[#ecc67c] bg-gradient-to-br from-[#fffdf4] via-[#fff0cb] to-[#ffe0a1] shadow-[0_3px_0_rgba(180,122,32,0.16)] dark:border-amber-500/35 dark:from-amber-950/70 dark:to-orange-900/50',
      glow: 'bg-amber-400/10 dark:bg-amber-400/20'
    }
  }
  if (key === 'diamond') {
    return {
      text: 'text-[#6a48be] dark:text-violet-300',
      icon: 'border border-[#cab8f7] bg-gradient-to-br from-[#fbf8ff] via-[#eee4ff] to-[#dcd0ff] shadow-[0_3px_0_rgba(116,86,214,0.15)] dark:border-violet-500/35 dark:from-violet-950/70 dark:to-indigo-900/50',
      glow: 'bg-violet-400/10 dark:bg-violet-400/20'
    }
  }
  return {
    text: 'text-[#2176b5] dark:text-sky-300',
    icon: 'border border-[#9ed0f5] bg-gradient-to-br from-[#f2faff] via-[#def2ff] to-[#c4e7ff] shadow-[0_3px_0_rgba(42,139,212,0.14)] dark:border-sky-500/35 dark:from-sky-950/70 dark:to-cyan-900/50',
    glow: 'bg-sky-400/10 dark:bg-sky-400/20'
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
