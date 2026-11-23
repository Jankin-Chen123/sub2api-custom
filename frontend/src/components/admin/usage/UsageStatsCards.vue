<template>
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <div class="journal-kpi-warm flex items-center gap-3.5 rounded-[18px] p-4">
      <div class="journal-icon-well">
        <CraftIcon name="calendar" size="lg" />
      </div>
      <div class="min-w-0">
        <p class="text-xs font-bold text-[#6e5d4b] dark:text-[#aeb7ca]">{{ t('usage.totalRequests') }}</p>
        <p class="mt-0.5 text-xl font-black tabular-nums text-[#27221d] dark:text-[#f5ede0]">{{ stats?.total_requests?.toLocaleString() || '0' }}</p>
        <p class="text-xs font-medium text-[#857563] dark:text-[#8c96ab]">{{ t('usage.inSelectedRange') }}</p>
      </div>
    </div>
    <div class="journal-kpi-warm flex items-center gap-3.5 rounded-[18px] p-4">
      <div class="journal-icon-well">
        <CraftIcon name="layers" size="lg" />
      </div>
      <div class="min-w-0">
        <p class="text-xs font-bold text-[#6e5d4b] dark:text-[#aeb7ca]">{{ t('usage.totalTokens') }}</p>
        <p class="mt-0.5 text-xl font-black tabular-nums text-[#27221d] dark:text-[#f5ede0]">{{ formatTokens(stats?.total_tokens || 0) }}</p>
        <p class="flex flex-wrap items-center gap-x-1 text-xs font-medium text-[#857563] dark:text-[#8c96ab]">
          <span>{{ t('usage.in') }}: {{ formatTokens(stats?.total_input_tokens || 0) }}</span>
          <span>/</span>
          <span>{{ t('usage.out') }}: {{ formatTokens(stats?.total_output_tokens || 0) }}</span>
          <span>/</span>
          <span class="group relative inline-flex cursor-help items-center gap-0.5" tabindex="0">
            <span>{{ cacheLabel() }}: {{ formatTokens(stats?.total_cache_tokens || 0) }}</span>
            <svg
              class="h-3.5 w-3.5 text-[#857563] dark:text-[#8c96ab]"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span
              class="pointer-events-none absolute left-1/2 top-full z-30 mt-2 hidden w-56 -translate-x-1/2 rounded-xl border border-[var(--craft-border-strong)] bg-[var(--craft-surface)] p-3 text-left text-xs text-[var(--craft-ink-soft)] shadow-lg group-hover:block group-focus:block"
            >
              <span class="mb-2 block font-bold text-[var(--craft-ink)]">
                {{ cacheDetailLabel() }}
              </span>
              <span class="flex items-center justify-between gap-3">
                <span>{{ t('usage.cacheCreationTokensLabel') }}</span>
                <span class="tabular-nums">
                  {{ formatTokens(stats?.total_cache_creation_tokens || 0) }}
                </span>
              </span>
              <span class="mt-1 flex items-center justify-between gap-3">
                <span>{{ t('usage.cacheReadTokensLabel') }}</span>
                <span class="tabular-nums">
                  {{ formatTokens(stats?.total_cache_read_tokens || 0) }}
                </span>
              </span>
            </span>
          </span>
        </p>
      </div>
    </div>
    <div class="journal-kpi-warm flex items-center gap-3.5 rounded-[18px] p-4">
      <div class="journal-icon-well">
        <CraftIcon name="coins" size="lg" />
      </div>
      <div class="min-w-0 flex-1">
        <p class="text-xs font-bold text-[#6e5d4b] dark:text-[#aeb7ca]">{{ t('usage.totalCost') }}</p>
        <p class="mt-0.5 text-xl font-black tabular-nums text-[#1f9d68] dark:text-[#34d399]">
          ${{ (stats?.total_actual_cost || 0).toFixed(4) }}
        </p>
        <p class="text-xs font-medium text-[#857563] dark:text-[#8c96ab]">
          <template v-if="showAccountCost && totalAccountCost != null">
            <span class="text-orange-500">{{ t('usage.accountCost') }} ${{ totalAccountCost.toFixed(4) }}</span>
            <span> · </span>
          </template>
          <span>
            {{ t('usage.standardCost') }}
            <span :class="{ 'line-through': strikeStandardCost }">${{ (stats?.total_cost || 0).toFixed(4) }}</span>
          </span>
        </p>
      </div>
    </div>
    <div class="journal-kpi-warm flex items-center gap-3.5 rounded-[18px] p-4">
      <div class="journal-icon-well">
        <CraftIcon name="clock" size="lg" />
      </div>
      <div class="min-w-0">
        <p class="text-xs font-bold text-[#6e5d4b] dark:text-[#aeb7ca]">{{ t('usage.avgDuration') }}</p>
        <p class="mt-0.5 text-xl font-black tabular-nums text-[#27221d] dark:text-[#f5ede0]">{{ formatDuration(stats?.average_duration_ms || 0) }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import type { UsageStatsResponse } from '@/types'
import CraftIcon from '@/components/craft/CraftIcon.vue'

const props = withDefaults(defineProps<{
  stats: (AdminUsageStatsResponse | UsageStatsResponse) | null
  showAccountCost?: boolean
  strikeStandardCost?: boolean
}>(), {
  showAccountCost: true,
  strikeStandardCost: false,
})

const { t } = useI18n()

const totalAccountCost = computed(() => {
  const stats = props.stats as (AdminUsageStatsResponse & { total_account_cost?: number }) | null
  return stats?.total_account_cost ?? null
})
const showAccountCost = computed(() => props.showAccountCost)
const strikeStandardCost = computed(() => props.strikeStandardCost)

const formatDuration = (ms: number) =>
  ms < 1000 ? `${ms.toFixed(0)}ms` : `${(ms / 1000).toFixed(2)}s`

const formatTokens = (value: number) => {
  if (value >= 1e9) return (value / 1e9).toFixed(2) + 'B'
  if (value >= 1e6) return (value / 1e6).toFixed(2) + 'M'
  if (value >= 1e3) return (value / 1e3).toFixed(2) + 'K'
  return value.toLocaleString()
}

const cacheLabel = () => t('usage.cacheTotal')
const cacheDetailLabel = () => t('usage.cacheBreakdown')
</script>
