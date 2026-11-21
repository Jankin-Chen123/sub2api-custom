<template>
  <section class="notebook-analytics" :aria-label="t('dashboard.analytics')">
    <div class="notebook-section-header">
      <h2 class="notebook-heading">{{ t('dashboard.analytics') }}</h2>
      <div class="notebook-filters">
        <DateRangePicker
          :start-date="startDate"
          :end-date="endDate"
          @update:startDate="$emit('update:startDate', $event)"
          @update:endDate="$emit('update:endDate', $event)"
          @change="$emit('dateRangeChange')"
        />
        <Select
          :aria-label="t('dashboard.granularity')"
          :model-value="granularity"
          :options="[{ value: 'day', label: t('dashboard.day') }, { value: 'hour', label: t('dashboard.hour') }]"
          @update:model-value="updateGranularity"
          @change="$emit('granularityChange')"
        />
        <button type="button" :disabled="loading" class="notebook-refresh" :aria-label="t('common.refresh')" @click="$emit('refresh')">
          <Icon name="refresh" size="sm" :class="{ 'motion-safe:animate-spin': loading }" />
          <span>{{ t('common.refresh') }}</span>
        </button>
      </div>
    </div>

    <div class="notebook-chart-grid">
      <TokenUsageTrend sticker :trend-data="trend" :loading="loading" />
      <section class="card notebook-model-card">
        <h3>{{ t('dashboard.modelDistribution') }}</h3>
        <div v-if="loading" class="grid min-h-48 place-items-center"><LoadingSpinner /></div>
        <div v-else-if="!modelData" class="grid min-h-48 place-items-center text-sm">{{ t('dashboard.noDataAvailable') }}</div>
        <template v-else>
          <div class="notebook-models">
            <div class="notebook-donut">
              <Doughnut :data="modelData" :options="doughnutOptions" :aria-label="t('dashboard.modelDistribution')" />
              <div v-if="totalTokens > 0" class="notebook-donut-label">
                <span>{{ t('dashboard.tokens') }}</span>
                <strong>{{ formatTokens(totalTokens) }}</strong>
              </div>
            </div>
            <ul class="notebook-legend">
              <li v-for="(model, index) in models" :key="model.model">
                <i :style="{ background: palette[index % palette.length] }" aria-hidden="true"></i>
                <span :title="model.model">{{ model.model }}</span>
                <strong v-if="totalTokens > 0">{{ formatShare(model.total_tokens) }}</strong>
              </li>
            </ul>
          </div>
          <details class="notebook-model-details">
            <summary>{{ t('dashboard.modelDetails') }}</summary>
            <div class="overflow-x-auto">
              <table class="w-full text-xs">
                <thead>
                  <tr>
                    <th scope="col">{{ t('dashboard.model') }}</th>
                    <th scope="col">{{ t('dashboard.requests') }}</th>
                    <th scope="col">{{ t('dashboard.tokens') }}</th>
                    <th scope="col">{{ t('dashboard.actual') }}</th>
                    <th scope="col">{{ t('dashboard.standard') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="model in models" :key="model.model">
                    <td>{{ model.model }}</td>
                    <td>{{ formatNumber(model.requests) }}</td>
                    <td>{{ formatTokens(model.total_tokens) }}</td>
                    <td>${{ formatCost(model.actual_cost) }}</td>
                    <td>${{ formatCost(model.cost) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </details>
        </template>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArcElement, Chart, Tooltip, type ChartOptions } from 'chart.js'
import { Doughnut } from 'vue-chartjs'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import type { ModelStat, TrendDataPoint } from '@/types'
import { formatCostFixed as formatCost, formatNumberLocaleString as formatNumber, formatTokensK as formatTokens } from '@/utils/format'

Chart.register(ArcElement, Tooltip)
const props = defineProps<{
  loading: boolean
  startDate: string
  endDate: string
  granularity: 'day' | 'hour'
  trend: TrendDataPoint[]
  models: ModelStat[]
}>()
const emit = defineEmits<{
  'update:startDate': [value: string]
  'update:endDate': [value: string]
  'update:granularity': [value: 'day' | 'hour']
  dateRangeChange: []
  granularityChange: []
  refresh: []
}>()
const { t, locale } = useI18n()
const palette = ['#419aed', '#ffa176', '#f5ca55', '#66c6bc', '#9d86dc', '#d6759a', '#83a85e', '#bf885d']
const totalTokens = computed(() => props.models.reduce((sum, model) => sum + model.total_tokens, 0))
const percent = computed(() => new Intl.NumberFormat(locale.value, { style: 'percent', maximumFractionDigits: 1 }))
const formatShare = (tokens: number) => percent.value.format(tokens / totalTokens.value)
const modelData = computed(() => !props.models.length ? null : ({
  labels: props.models.map(model => model.model),
  datasets: [{
    data: props.models.map(model => model.total_tokens),
    backgroundColor: props.models.map((_, index) => palette[index % palette.length]),
    borderWidth: 0,
    spacing: 1,
  }],
}))
const doughnutOptions: ChartOptions<'doughnut'> = {
  responsive: true,
  maintainAspectRatio: false,
  cutout: '68%',
  animation: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? false : { duration: 220 },
  plugins: {
    legend: { display: false },
    tooltip: { callbacks: { label: context => formatTokens(context.parsed) + ' Token' } },
  },
}

function updateGranularity(value: string | number | boolean | null) {
  if (value === 'day' || value === 'hour') emit('update:granularity', value)
}
</script>
