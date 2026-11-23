<template>
  <div class="space-y-4">
    <!-- ==================== Overview Section ==================== -->
    <section v-if="section === 'all' || section === 'overview'" class="space-y-3.5">
      <div class="flex items-center gap-1.5">
        <h2 class="text-xl font-black tracking-tight text-[#27221d] dark:text-[#f5ede0] sm:text-2xl">
          {{ t('dashboard.accountOverview') }}
        </h2>
        <CraftSparks variant="right" size="md" />
      </div>

      <!-- Stitched Sky-Blue Wallet Balance Banner -->
      <div v-if="!isSimple" class="wallet-banner group relative overflow-visible rounded-[22px] p-3">
        <div class="wallet-stitch flex flex-wrap items-center justify-between gap-4 rounded-[16px] px-4 py-3.5 sm:px-6 sm:py-4">
          <div class="flex items-center gap-4 sm:gap-6">
            <JournalBannerSticker name="wallet" size="lg" />
            <div>
              <p class="text-xs font-bold tracking-wide text-[#24435c] dark:text-[#b6defa] sm:text-sm">
                {{ t('dashboard.accountBalance') }}
              </p>
              <p class="mt-0.5 text-3xl font-black tracking-tight tabular-nums text-[#201c18] dark:text-[#ffffff] sm:text-4xl">
                ${{ formatBalance(balance) }}
              </p>
            </div>
          </div>
          <CraftSparks variant="sticker" size="md" class="hidden shrink-0 sm:inline-block" />
        </div>
      </div>

      <dl class="account-summary card px-4 sm:px-6">
        <div>
          <dt>{{ t('dashboard.todayRequests') }}</dt>
          <dd>
            <strong>{{ formatNumber(stats?.today_requests || 0) }}</strong>
            <p>{{ t('common.total') }} · {{ formatNumber(stats?.total_requests || 0) }}</p>
          </dd>
        </div>
        <div>
          <dt>{{ t('dashboard.todayCost') }}</dt>
          <dd>
            <strong :title="t('dashboard.actual')">${{ formatCost(stats?.today_actual_cost || 0) }}</strong>
            <span :title="t('dashboard.standard')"> / ${{ formatCost(stats?.today_cost || 0) }}</span>
            <p data-testid="cumulative-cost" class="flex flex-wrap justify-end gap-x-1">
              <span>{{ t('common.total') }} ·</span>
              <span>{{ t('dashboard.actual') }} ${{ formatCost(stats?.total_actual_cost || 0) }}</span>
              <span>{{ t('dashboard.standard') }} ${{ formatCost(stats?.total_cost || 0) }}</span>
            </p>
          </dd>
        </div>
        <div>
          <dt>{{ t('dashboard.apiKeys') }}</dt>
          <dd>
            <strong>{{ formatNumber(stats?.total_api_keys || 0) }}</strong>
            <p>{{ stats?.active_api_keys || 0 }} {{ t('common.active') }}</p>
          </dd>
        </div>
      </dl>

      <slot name="actions" />

      <!-- Row 2: Unified 4-Column Token & Performance Strip -->
      <div class="card overflow-hidden !rounded-[18px]">
        <div class="grid grid-cols-2 divide-y divide-[#efe2cc] dark:divide-[rgba(255,236,204,0.09)] lg:grid-cols-4 lg:divide-x lg:divide-y-0">
          <!-- Today Tokens -->
          <div class="flex items-center gap-3 px-4 py-3.5">
            <CraftIcon name="layers" size="md" />
            <div class="min-w-0 flex-1">
              <p class="text-xs font-semibold text-[#756758] dark:text-[#9aa4b8]">{{ t('dashboard.todayTokens') }}</p>
              <p class="truncate text-lg font-black tabular-nums text-[#27221d] dark:text-[#f5ede0]">
                {{ formatTokens(stats?.today_tokens || 0) }}
              </p>
              <p class="truncate text-[11px] tabular-nums text-[#857563] dark:text-[#8690a6]">
                {{ t('dashboard.input') }} {{ formatTokens(stats?.today_input_tokens || 0) }} · {{ t('dashboard.output') }} {{ formatTokens(stats?.today_output_tokens || 0) }}
              </p>
            </div>
          </div>

          <!-- Total Tokens -->
          <div class="flex items-center gap-3 px-4 py-3.5">
            <CraftIcon name="bars" size="md" />
            <div class="min-w-0 flex-1">
              <p class="text-xs font-semibold text-[#756758] dark:text-[#9aa4b8]">{{ t('dashboard.totalTokens') }}</p>
              <p class="truncate text-lg font-black tabular-nums text-[#27221d] dark:text-[#f5ede0]">
                {{ formatTokens(stats?.total_tokens || 0) }}
              </p>
              <p class="truncate text-[11px] tabular-nums text-[#857563] dark:text-[#8690a6]">
                {{ t('dashboard.cache') }} {{ formatTokens((stats?.total_cache_creation_tokens || 0) + (stats?.total_cache_read_tokens || 0)) }}
              </p>
            </div>
          </div>

          <!-- Performance (RPM/TPM) -->
          <div class="flex items-center gap-3 px-4 py-3.5">
            <CraftIcon name="bolt" size="md" />
            <div class="min-w-0 flex-1">
              <p class="text-xs font-semibold text-[#756758] dark:text-[#9aa4b8]">{{ t('dashboard.performance') }}</p>
              <p class="truncate text-lg font-black tabular-nums text-[#27221d] dark:text-[#f5ede0]">
                {{ formatTokens(stats?.rpm || 0) }} <span class="text-xs font-bold text-[#756758] dark:text-[#9aa4b8]">RPM</span>
              </p>
              <p class="truncate text-[11px] font-semibold tabular-nums text-[#2a8bd4] dark:text-[#6ec4fc]">
                {{ formatTokens(stats?.tpm || 0) }} TPM
              </p>
            </div>
          </div>

          <!-- Avg Response Time -->
          <div class="flex items-center gap-3 px-4 py-3.5">
            <CraftIcon name="clock" size="md" />
            <div class="min-w-0 flex-1">
              <p class="text-xs font-semibold text-[#756758] dark:text-[#9aa4b8]">{{ t('dashboard.avgResponse') }}</p>
              <p class="truncate text-lg font-black tabular-nums text-[#27221d] dark:text-[#f5ede0]">
                {{ formatDuration(stats?.average_duration_ms || 0) }}
              </p>
              <p class="truncate text-[11px] text-[#857563] dark:text-[#8690a6]">
                {{ t('dashboard.averageTime') }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ==================== Platform Usage Section ==================== -->
    <section
      v-if="(section === 'all' || section === 'platforms') && !isSimple && platformCards.length > 0"
      class="space-y-3"
    >
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="flex items-center gap-1.5">
          <h3 class="text-lg font-black tracking-tight text-[#27221d] dark:text-[#f5ede0] sm:text-xl">
            {{ t('dashboard.platformUsage') }}
          </h3>
          <CraftSparks variant="right" size="sm" />
        </div>
        <span class="badge badge-gray px-3 py-1 text-xs font-bold">
          {{ t('dashboard.platformCount', { count: platformCount }) }}
        </span>
      </div>

      <div class="space-y-2.5">
        <div
          v-for="item in platformCards"
          :key="item.platform"
          data-testid="platform-card"
          :data-platform="item.platform"
          :class="[
            'card px-4 py-3.5 !rounded-[18px]',
            item.isOther ? '!border-dashed bg-[#faf2e2]/70 dark:bg-[#191e2b]' : ''
          ]"
        >
          <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <!-- Left: Icon + Platform Name + Compact Today Metrics -->
            <div class="flex min-w-0 items-center gap-3 md:w-[32%]">
              <CraftIcon :name="platformIcon(item.platform)" size="md" />
              <div class="min-w-0">
                <p class="truncate text-sm font-extrabold text-[#27221d] dark:text-[#f5ede0]">
                  {{ item.isOther ? t('dashboard.platformOther') : platformLabel(item.platform) }}
                </p>
                <p class="truncate text-xs tabular-nums text-[#756758] dark:text-[#9aa4b8]">
                  {{ t('dashboard.todayCost') }} ${{ formatCost(item.today_actual_cost) }} ·
                  {{ item.total_tokens > 0 ? formatTokens(item.total_tokens) : '-' }} {{ t('dashboard.tokens') }}
                </p>
              </div>
            </div>

            <!-- Middle: Candy Progress Track -->
            <div class="flex-1">
              <div class="h-3.5 w-full overflow-hidden rounded-full bg-[#efe5d2] p-0.5 dark:bg-[#283044]">
                <div
                  class="h-full rounded-full transition-all duration-500"
                  :class="platformBarTone(item)"
                  :style="{ width: `${platformBarPercent(item)}%` }"
                />
              </div>
            </div>

            <!-- Right: Actual Cost + Requests -->
            <div class="flex items-center justify-between gap-4 text-xs tabular-nums md:w-[24%] md:justify-end">
              <span class="text-[#756758] dark:text-[#9aa4b8]">
                {{ t('dashboard.requests') }} {{ item.total_requests > 0 ? formatNumber(item.total_requests) : '-' }}
              </span>
              <span class="text-sm font-black text-[#27221d] dark:text-[#f5ede0]" :title="t('dashboard.actual')">
                ${{ formatCost(item.total_actual_cost) }}
              </span>
            </div>
          </div>

          <!-- Quota Window Breakdown (preserved cleanly when quota is configured) -->
          <div
            v-if="hasAnyLimit(item.quota) && !item.isOther"
            class="mt-3 grid grid-cols-1 gap-2.5 border-t border-[#efe2cc] pt-3 dark:border-[rgba(255,236,204,0.09)] sm:grid-cols-[repeat(auto-fit,minmax(180px,1fr))]"
          >
            <p class="col-span-full text-[11px] font-bold uppercase tracking-wider text-[#756758] dark:text-[#9aa4b8]">
              {{ t('dashboard.platformQuota.title') }}
            </p>
            <template v-for="w in (['daily', 'weekly', 'monthly'] as const)" :key="w">
              <div v-if="quotaVal(item.quota, `${w}_limit_usd`) != null" class="space-y-1">
                <template v-if="(quotaVal(item.quota, `${w}_limit_usd`) as number) === 0">
                  <div class="flex items-center justify-between text-xs">
                    <span class="font-medium text-[#6e5d4b] dark:text-[#c7d0e0]">{{ t(`dashboard.platformQuota.${w}`) }}</span>
                    <span class="font-mono font-bold text-red-500">{{ t('dashboard.platformQuota.disabled') }}</span>
                  </div>
                  <div class="h-1.5 w-full overflow-hidden rounded-full bg-[#efe5d2] dark:bg-[#283044]">
                    <div class="h-full w-full rounded-full bg-red-500" />
                  </div>
                </template>
                <template v-else>
                  <div class="flex items-center justify-between text-xs">
                    <span class="font-medium text-[#6e5d4b] dark:text-[#c7d0e0]">{{ t(`dashboard.platformQuota.${w}`) }}</span>
                    <span class="font-mono text-[#27221d] dark:text-[#f5ede0]">
                      ${{ formatUsd((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0) }} / ${{ formatUsd(quotaVal(item.quota, `${w}_limit_usd`) as number) }}
                    </span>
                  </div>
                  <div class="h-1.5 w-full overflow-hidden rounded-full bg-[#efe5d2] dark:bg-[#283044]">
                    <div
                      class="h-full rounded-full transition-all"
                      :class="quotaBarClass(calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number))"
                      :style="{ width: calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number) + '%' }"
                    />
                  </div>
                  <p v-if="quotaVal(item.quota, `${w}_window_resets_at`)" class="text-[10px] text-[#857563] dark:text-[#8c96ab]">
                    {{ t('dashboard.platformQuota.resetsAt', { time: formatResetTime(quotaVal(item.quota, `${w}_window_resets_at`) as string) }) }}
                  </p>
                </template>
              </div>
            </template>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import CraftSparks from '@/components/craft/CraftSparks.vue'
import CraftIcon from '@/components/craft/CraftIcon.vue'
import JournalBannerSticker from '@/components/craft/JournalBannerSticker.vue'
import type { PlatformDashboardStats, UserDashboardStats as UserStatsType } from '@/api/usage'
import type { PlatformQuotaItem } from '@/types'

interface FusedPlatformCard {
  platform: string
  total_actual_cost: number
  today_actual_cost: number
  total_requests: number
  total_tokens: number
  isOther?: boolean
  quota?: PlatformQuotaItem
}

const props = withDefaults(
  defineProps<{
    stats: UserStatsType
    balance: number
    isSimple: boolean
    platformQuotas?: PlatformQuotaItem[] | null
    section?: 'overview' | 'platforms' | 'all'
  }>(),
  {
    section: 'all',
  }
)
const { t } = useI18n()

const PLATFORM_LABELS: Record<string, string> = {
  anthropic: 'Claude',
  openai: 'OpenAI',
  gemini: 'Gemini',
  antigravity: 'Antigravity',
  grok: 'Grok',
  kimi: 'Kimi',
  zhipu: 'Zhipu GLM',
  deepseek: 'DeepSeek',
  minimax: 'MiniMax',
  typesafe: 'TypeSafe / Jev',
}

const platformLabel = (p: string) => PLATFORM_LABELS[p] ?? p

const platformIcon = (p: string): string => {
  if (p === 'openai' || p === 'anthropic' || p === 'gemini') return p
  return 'generic'
}

const OTHER_THRESHOLD = 0.0001
const platformCards = computed<FusedPlatformCard[]>(() => {
  const byPlat = new Map<string, PlatformDashboardStats>()
  for (const item of props.stats?.by_platform ?? []) byPlat.set(item.platform, item)

  const byQuota = new Map<string, PlatformQuotaItem>()
  for (const q of props.platformQuotas ?? []) byQuota.set(q.platform, q)

  const platforms = new Set<string>(byPlat.keys())
  for (const [platform, q] of byQuota) {
    if (hasAnyLimit(q)) platforms.add(platform)
  }

  const PLATFORM_ORDER = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'typesafe']
  const cards: FusedPlatformCard[] = []

  for (const p of platforms) {
    const stat = byPlat.get(p)
    cards.push({
      platform: p,
      total_actual_cost: stat?.total_actual_cost ?? 0,
      today_actual_cost: stat?.today_actual_cost ?? 0,
      total_requests: stat?.total_requests ?? 0,
      total_tokens: stat?.total_tokens ?? 0,
      quota: byQuota.get(p),
    })
  }

  cards.sort((a, b) => {
    const ai = PLATFORM_ORDER.indexOf(a.platform)
    const bi = PLATFORM_ORDER.indexOf(b.platform)
    if (ai === -1 && bi === -1) return a.platform.localeCompare(b.platform)
    if (ai === -1) return 1
    if (bi === -1) return -1
    return ai - bi
  })

  const total = props.stats?.total_actual_cost ?? 0
  const today = props.stats?.today_actual_cost ?? 0
  const sumTotal = cards.reduce((s, c) => s + c.total_actual_cost, 0)
  const sumToday = cards.reduce((s, c) => s + c.today_actual_cost, 0)
  const diffTotal = Math.max(0, total - sumTotal)
  const diffToday = Math.max(0, today - sumToday)

  if (diffTotal > OTHER_THRESHOLD || diffToday > OTHER_THRESHOLD) {
    cards.push({
      platform: '__other__',
      total_actual_cost: diffTotal,
      today_actual_cost: diffToday,
      total_requests: 0,
      total_tokens: 0,
      isOther: true,
    })
  }

  return cards
})

const platformCount = computed(() => platformCards.value.filter((c) => !c.isOther).length)

const maxPlatformCost = computed(() =>
  Math.max(0.0001, ...platformCards.value.map((c) => c.total_actual_cost))
)

function platformBarPercent(item: FusedPlatformCard): number {
  if (hasAnyLimit(item.quota) && !item.isOther) {
    const windows = (['daily', 'weekly', 'monthly'] as const)
      .map((w) => {
        const limit = quotaVal(item.quota, `${w}_limit_usd`) as number | null | undefined
        const usage = (quotaVal(item.quota, `${w}_usage_usd`) as number | null | undefined) ?? 0
        if (limit == null) return null
        if (limit === 0) return 100
        return calcPercent(usage, limit)
      })
      .filter((v): v is number => v !== null)
    if (windows.length) return Math.max(...windows, 4)
  }
  const share = Math.round((item.total_actual_cost / maxPlatformCost.value) * 100)
  return item.total_actual_cost > 0 ? Math.max(8, Math.min(100, share)) : 0
}

function platformBarTone(item: FusedPlatformCard): string {
  if (item.platform === 'openai') return 'bg-gradient-to-r from-[#5cb9fa] to-[#3b9df8]'
  if (item.platform === 'anthropic') return 'bg-gradient-to-r from-[#ffb68d] to-[#fa9364]'
  if (item.platform === 'gemini') return 'bg-gradient-to-r from-[#67d8c2] to-[#3cb89f]'
  if (item.platform === 'antigravity' || item.platform === 'grok') return 'bg-gradient-to-r from-[#fcd365] to-[#f4b63d]'
  return 'bg-gradient-to-r from-[#b4a7f6] to-[#8c7ae6]'
}

type QuotaWindow = 'daily' | 'weekly' | 'monthly'
type QuotaField = `${QuotaWindow}_limit_usd` | `${QuotaWindow}_usage_usd` | `${QuotaWindow}_window_resets_at`

function quotaVal(q: PlatformQuotaItem | undefined, key: QuotaField): PlatformQuotaItem[QuotaField] {
  return q?.[key]
}

function hasAnyLimit(q: PlatformQuotaItem | undefined): boolean {
  if (!q) return false
  return q.daily_limit_usd != null || q.weekly_limit_usd != null || q.monthly_limit_usd != null
}

function calcPercent(usage: number, limit: number): number {
  if (!limit || limit <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((usage / limit) * 100)))
}

function quotaBarClass(p: number): string {
  if (p >= 95) return 'bg-red-500'
  if (p >= 75) return 'bg-amber-500'
  return 'bg-emerald-500'
}

const usdFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})
function formatUsd(n: number): string {
  if (!Number.isFinite(n)) return '0.00'
  return usdFormatter.format(n)
}

function formatResetTime(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString(undefined, {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

const formatBalance = (b: number) =>
  new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(b)

const formatNumber = (n: number) => n.toLocaleString()
const formatCost = (c: number) => c.toFixed(4)
const formatTokens = (t: number) => {
  if (t >= 1_000_000) return `${(t / 1_000_000).toFixed(1)}M`
  if (t >= 1000) return `${(t / 1000).toFixed(1)}K`
  return t.toString()
}
const formatDuration = (ms: number) => (ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${ms.toFixed(0)}ms`)
</script>

<style scoped>
.wallet-banner {
  background:
    radial-gradient(circle at 82% 28%, rgba(255, 255, 255, 0.34) 0%, transparent 48%),
    #94d4fc url('/sticker/paper.webp') repeat;
  background-size: 100% 100%, 155px;
  border: 1px solid #80c4ef;
  box-shadow: 0 6px 18px rgba(72, 134, 178, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.65);
}

.dark .wallet-banner {
  background:
    radial-gradient(circle at 82% 28%, rgba(110, 196, 252, 0.16) 0%, transparent 50%),
    linear-gradient(135deg, #21405e 0%, #1a334c 100%);
  border-color: rgba(110, 196, 252, 0.3);
  box-shadow: 0 8px 22px rgba(0, 0, 0, 0.32);
}

.wallet-stitch {
  border: 1.5px dashed rgba(255, 255, 255, 0.78);
}

.dark .wallet-stitch {
  border-color: rgba(138, 209, 255, 0.32);
}

.account-summary > div {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 2fr);
  align-items: center;
  gap: 0.75rem;
  padding-block: 0.65rem;
}

.account-summary > div + div {
  border-top: 1px solid var(--craft-border);
}

.account-summary dt {
  color: var(--craft-ink);
  font-size: 0.875rem;
  font-weight: 700;
}

.account-summary dd {
  color: var(--craft-ink-soft);
  text-align: end;
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.account-summary strong {
  color: var(--craft-ink);
  font-size: 1.25rem;
  font-weight: 800;
}
</style>
