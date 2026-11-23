<template>
  <section class="space-y-3.5">
    <!-- Section Title + Inline Controls -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-1.5">
        <h2 class="text-xl font-black tracking-tight text-[#27221d] dark:text-[#f5ede0] sm:text-2xl">
          {{ t('dashboard.usageAnalysis') }}
        </h2>
        <CraftSparks variant="right" size="md" />
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <DateRangePicker
          :start-date="startDate"
          :end-date="endDate"
          :applied-period="appliedPeriod"
          @update:startDate="$emit('update:startDate', $event)"
          @update:endDate="$emit('update:endDate', $event)"
          @change="$emit('dateRangeChange', $event)"
        />
        <div class="w-24">
          <Select
            :model-value="granularity"
            :options="[
              { value: 'day', label: t('dashboard.day') },
              { value: 'hour', label: t('dashboard.hour') },
            ]"
            @update:model-value="$emit('update:granularity', $event)"
            @change="$emit('granularityChange')"
          />
        </div>
        <button
          type="button"
          :disabled="loading"
          class="btn btn-secondary btn-sm !rounded-xl !px-3.5 !py-2 font-bold"
          @click="$emit('refresh')"
        >
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          <span>{{ t('common.refresh') }}</span>
        </button>
      </div>
    </div>

    <!-- Charts Grid -->
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-12">
      <!-- Token Usage Trend Chart (7 cols) -->
      <div class="lg:col-span-7">
        <TokenUsageTrend :trend-data="trend" :loading="loading" />
      </div>

      <!-- Model Distribution Donut Card (5 cols) -->
      <div class="card relative flex flex-col justify-between overflow-hidden p-4 sm:p-5 lg:col-span-5">
        <div
          v-if="loading"
          class="absolute inset-0 z-10 flex items-center justify-center bg-[#fffdf8]/70 backdrop-blur-sm dark:bg-[#1d2230]/75"
        >
          <LoadingSpinner size="md" />
        </div>

        <div>
          <h3 class="mb-4 text-sm font-extrabold text-[#27221d] dark:text-[#f5ede0]">
            {{ t('dashboard.modelDistribution') }}
          </h3>

          <div class="flex flex-col items-center gap-5 sm:flex-row">
            <!-- Donut with Centered Total -->
            <div class="relative h-44 w-44 shrink-0">
              <Doughnut v-if="modelData" :data="modelData" :options="doughnutOptions" />
              <div
                v-if="modelData"
                class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center leading-tight"
              >
                <span class="text-[11px] font-bold text-[#756758] dark:text-[#9aa4b8]">
                  {{ t('dashboard.totalTokensShort') }}
                </span>
                <span class="mt-0.5 text-lg font-black tabular-nums text-[#27221d] dark:text-[#f5ede0]">
                  {{ formatTokens(totalModelTokens) }}
                </span>
              </div>
              <div
                v-else
                class="flex h-full items-center justify-center text-sm text-[#756758] dark:text-[#9aa4b8]"
              >
                {{ t('dashboard.noDataAvailable') }}
              </div>
            </div>

            <!-- Candy Legend List -->
            <ul v-if="models.length > 0" class="max-h-44 w-full min-w-0 flex-1 space-y-2 overflow-auto pr-1 text-xs">
              <li
                v-for="(m, index) in models"
                :key="m.model"
                class="flex items-center justify-between gap-2"
              >
                <span class="flex min-w-0 items-center gap-2">
                  <span
                    class="h-2.5 w-2.5 shrink-0 rounded-full"
                    :style="{ backgroundColor: CANDY_PALETTE[index % CANDY_PALETTE.length] }"
                  />
                  <span class="truncate font-semibold text-[#27221d] dark:text-[#f5ede0]" :title="m.model">
                    {{ m.model }}
                  </span>
                </span>
                <span class="shrink-0 font-bold tabular-nums text-[#756758] dark:text-[#9aa4b8]">
                  {{ sharePercent(m.total_tokens) }}%
                </span>
              </li>
            </ul>
          </div>
        </div>

        <!-- Expandable Model Breakdown Table -->
        <details v-if="models.length > 0" class="mt-4 border-t border-[#efe2cc] pt-3 text-xs dark:border-[rgba(255,236,204,0.09)]">
          <summary class="cursor-pointer select-none font-bold text-[#2a8bd4] hover:underline dark:text-[#6ec4fc]">
            {{ t('dashboard.modelDetails') }}
          </summary>
          <div class="mt-2.5 max-h-40 overflow-auto">
            <table class="w-full text-xs">
              <thead>
                <tr class="text-[#756758] dark:text-[#9aa4b8]">
                  <th class="pb-2 text-left font-bold">{{ t('dashboard.model') }}</th>
                  <th class="pb-2 text-right font-bold">{{ t('dashboard.requests') }}</th>
                  <th class="pb-2 text-right font-bold">{{ t('dashboard.tokens') }}</th>
                  <th class="pb-2 text-right font-bold">{{ t('dashboard.actual') }}</th>
                  <th class="pb-2 text-right font-bold">{{ t('dashboard.standard') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="model in models"
                  :key="model.model"
                  class="border-t border-[#f2e7d4] dark:border-[rgba(255,236,204,0.07)]"
                >
                  <td class="max-w-[110px] truncate py-1.5 font-semibold text-[#27221d] dark:text-[#f5ede0]" :title="model.model">
                    {{ model.model }}
                  </td>
                  <td class="py-1.5 text-right tabular-nums text-[#6e5d4b] dark:text-[#aeb7ca]">{{ formatNumber(model.requests) }}</td>
                  <td class="py-1.5 text-right tabular-nums text-[#6e5d4b] dark:text-[#aeb7ca]">{{ formatTokens(model.total_tokens) }}</td>
                  <td class="py-1.5 text-right font-semibold tabular-nums text-[#1f9d68] dark:text-[#34d399]">${{ formatCost(model.actual_cost) }}</td>
                  <td class="py-1.5 text-right tabular-nums text-[#857563] dark:text-[#7c879d]">${{ formatCost(model.cost) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </details>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutationObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import CraftSparks from '@/components/craft/CraftSparks.vue'
import { Doughnut } from 'vue-chartjs'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import type { TrendDataPoint, ModelStat } from '@/types'
import { formatCostFixed as formatCost, formatNumberLocaleString as formatNumber, formatTokensK as formatTokens } from '@/utils/format'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, ArcElement, Title, Tooltip, Legend, Filler } from 'chart.js'
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, ArcElement, Title, Tooltip, Legend, Filler)

const props = defineProps<{
  loading: boolean
  startDate: string
  endDate: string
  appliedPeriod?: 'dates' | '24h'
  granularity: string
  trend: TrendDataPoint[]
  models: ModelStat[]
}>()

defineEmits(['update:startDate', 'update:endDate', 'update:granularity', 'dateRangeChange', 'granularityChange', 'refresh'])

const { t } = useI18n()

const CANDY_PALETTE = ['#44a4f8', '#ff9e72', '#ffd25a', '#64d8c8', '#a392f5', '#f68db3', '#7cc7f9', '#8ed973']

const isDarkMode = ref(document.documentElement.classList.contains('dark'))
useMutationObserver(
  document.documentElement,
  () => {
    isDarkMode.value = document.documentElement.classList.contains('dark')
  },
  { attributes: true, attributeFilter: ['class'] }
)

const totalModelTokens = computed(() =>
  (props.models ?? []).reduce((sum, item) => sum + (item.total_tokens || 0), 0)
)

function sharePercent(tokens: number): string {
  const total = totalModelTokens.value
  if (!total) return '0.0'
  return ((tokens / total) * 100).toFixed(1)
}

const modelData = computed(() =>
  !props.models?.length
    ? null
    : {
        labels: props.models.map((m: ModelStat) => m.model),
        datasets: [
          {
            data: props.models.map((m: ModelStat) => m.total_tokens),
            backgroundColor: props.models.map((_, i) => CANDY_PALETTE[i % CANDY_PALETTE.length]),
            borderColor: isDarkMode.value ? '#1d2230' : '#fffdf8',
            borderWidth: 3,
            hoverOffset: 4,
          },
        ],
      }
)

const doughnutOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  cutout: '66%',
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: { label?: string; parsed: number }) =>
          `${context.label}: ${formatTokens(context.parsed)} tokens`,
      },
    },
  },
}))
</script>
