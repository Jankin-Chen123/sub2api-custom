<template>
  <section class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex items-center gap-1.5">
        <h2 class="text-xl font-black tracking-tight text-[#27221d] dark:text-[#f5ede0] sm:text-2xl">
          {{ t('dashboard.recentUsage') }}
        </h2>
        <CraftSparks variant="right" size="md" />
      </div>

      <div class="flex items-center gap-2">
        <span class="badge badge-gray text-xs">{{ t('dashboard.last7Days') }}</span>
        <router-link
          to="/usage"
          class="inline-flex items-center gap-1 rounded-full border border-[#8ccbf5] bg-[#dff2fe] px-3.5 py-1.5 text-xs font-extrabold text-[#1e4d73] shadow-sm transition-transform hover:-translate-y-0.5 dark:border-[#3e76a6] dark:bg-[#224568] dark:text-[#b6e1ff]"
        >
          <span>{{ t('dashboard.viewAllUsage') }}</span>
          <Icon name="chevronRight" size="sm" />
        </router-link>
      </div>
    </div>

    <div class="card overflow-hidden !rounded-[18px]">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner size="lg" />
      </div>
      <div v-else-if="data.length === 0" class="px-6 py-10">
        <EmptyState :title="t('dashboard.noUsageRecords')" :description="t('dashboard.startUsingApi')" />
      </div>
      <template v-else>
        <!-- Desktop Warm-Header Journal Table -->
        <div class="hidden overflow-x-auto sm:block">
          <table class="w-full text-sm">
            <thead>
              <tr class="bg-[#f7ead2] text-xs font-extrabold text-[#4c4237] dark:bg-[#262c3d] dark:text-[#d8cfc0]">
                <th class="px-5 py-3 text-left">{{ t('dashboard.model') }}</th>
                <th class="px-4 py-3 text-left">{{ t('usage.time') }}</th>
                <th class="px-4 py-3 text-right">{{ t('dashboard.tokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('dashboard.actual') }}</th>
                <th class="px-5 py-3 text-right">{{ t('dashboard.standard') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-[#efe2cc] dark:divide-[rgba(255,236,204,0.08)]">
              <tr
                v-for="log in data"
                :key="log.id"
                class="transition-colors hover:bg-[#faf2e2]/65 dark:hover:bg-[#252b3d]/65"
              >
                <td class="px-5 py-3">
                  <div class="flex items-center gap-2.5">
                    <CraftIcon :name="providerForModel(log.model)" size="sm" />
                    <span class="font-bold text-[#27221d] dark:text-[#f5ede0]">{{ log.model }}</span>
                  </div>
                </td>
                <td class="px-4 py-3 text-xs tabular-nums text-[#756758] dark:text-[#9aa4b8]">
                  {{ formatDateTime(log.created_at) }}
                </td>
                <td class="px-4 py-3 text-right font-semibold tabular-nums text-[#4c4237] dark:text-[#d8cfc0]">
                  {{ formatTokensCompact(log.input_tokens + log.output_tokens) }}
                </td>
                <td class="px-4 py-3 text-right font-extrabold tabular-nums text-[#1f9d68] dark:text-[#34d399]">
                  ${{ formatCost(log.actual_cost) }}
                </td>
                <td class="px-5 py-3 text-right tabular-nums text-[#857563] dark:text-[#8690a6]">
                  ${{ formatCost(log.total_cost) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Mobile List -->
        <div class="divide-y divide-[#efe2cc] dark:divide-[rgba(255,236,204,0.08)] sm:hidden">
          <div
            v-for="log in data"
            :key="log.id"
            class="flex items-center justify-between gap-3 px-4 py-3"
          >
            <div class="flex min-w-0 items-center gap-2.5">
              <CraftIcon :name="providerForModel(log.model)" size="sm" />
              <div class="min-w-0">
                <p class="truncate text-sm font-bold text-[#27221d] dark:text-[#f5ede0]">{{ log.model }}</p>
                <p class="text-xs tabular-nums text-[#756758] dark:text-[#9aa4b8]">
                  {{ formatDateTime(log.created_at) }} · {{ formatTokensCompact(log.input_tokens + log.output_tokens) }} tokens
                </p>
              </div>
            </div>
            <div class="shrink-0 text-right tabular-nums">
              <p class="text-sm font-extrabold text-[#1f9d68] dark:text-[#34d399]">${{ formatCost(log.actual_cost) }}</p>
              <p class="text-xs text-[#857563] dark:text-[#8690a6]">${{ formatCost(log.total_cost) }}</p>
            </div>
          </div>
        </div>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import CraftSparks from '@/components/craft/CraftSparks.vue'
import CraftIcon from '@/components/craft/CraftIcon.vue'
import { formatDateTime } from '@/utils/format'
import type { UsageLog } from '@/types'

defineProps<{
  data: UsageLog[]
  loading: boolean
}>()

const { t } = useI18n()

const formatCost = (c: number) => c.toFixed(4)

function formatTokensCompact(tokens: number): string {
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M`
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}K`
  return tokens.toLocaleString()
}

function providerForModel(model: string): string {
  const lower = model.toLowerCase()
  if (lower.includes('claude')) return 'anthropic'
  if (lower.includes('gpt') || lower.includes('o1') || lower.includes('o3') || lower.includes('o4')) return 'openai'
  if (lower.includes('gemini')) return 'gemini'
  return 'generic'
}
</script>
