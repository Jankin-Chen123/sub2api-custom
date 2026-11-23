<template>
  <div class="card p-4 sm:p-5">
    <h3 class="mb-4 text-sm font-extrabold text-[#27221d] dark:text-[#f5ede0]">
      {{ t('admin.dashboard.tokenUsageTrend') }}
    </h3>
    <div v-if="loading" class="flex h-52 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="trendData.length > 0 && chartData" class="h-52">
      <Line :data="chartData" :options="lineOptions" />
    </div>
    <div
      v-else
      class="flex h-52 items-center justify-center text-sm text-[#756758] dark:text-[#9aa4b8]"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutationObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()

const props = defineProps<{
  trendData: TrendDataPoint[]
  loading?: boolean
}>()

const isDarkMode = ref(document.documentElement.classList.contains('dark'))
useMutationObserver(
  document.documentElement,
  () => {
    isDarkMode.value = document.documentElement.classList.contains('dark')
  },
  { attributes: true, attributeFilter: ['class'] }
)

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#cfd6e3' : '#5e5042',
  grid: isDarkMode.value ? 'rgba(255, 240, 214, 0.08)' : 'rgba(154, 127, 90, 0.14)',
  pointBg: isDarkMode.value ? '#1d2230' : '#fffdf8',
  input: '#3b9df8',
  output: '#fa986a',
  cacheCreation: '#f5c443',
  cacheRead: '#4cc9b0',
  cacheHitRate: '#9b87f5'
}))

const chartData = computed(() => {
  if (!props.trendData?.length) return null

  return {
    labels: props.trendData.map((d) => d.date),
    datasets: [
      {
        label: 'Input',
        data: props.trendData.map((d) => d.input_tokens),
        borderColor: chartColors.value.input,
        backgroundColor: 'rgba(59, 157, 248, 0.14)',
        pointBackgroundColor: chartColors.value.pointBg,
        pointBorderColor: chartColors.value.input,
        pointBorderWidth: 2,
        pointRadius: 3.5,
        pointHoverRadius: 5.5,
        borderWidth: 2.5,
        fill: true,
        tension: 0.35
      },
      {
        label: 'Output',
        data: props.trendData.map((d) => d.output_tokens),
        borderColor: chartColors.value.output,
        backgroundColor: 'rgba(250, 152, 106, 0.14)',
        pointBackgroundColor: chartColors.value.pointBg,
        pointBorderColor: chartColors.value.output,
        pointBorderWidth: 2,
        pointRadius: 3.5,
        pointHoverRadius: 5.5,
        borderWidth: 2.5,
        fill: true,
        tension: 0.35
      },
      {
        label: 'Cache Creation',
        data: props.trendData.map((d) => d.cache_creation_tokens),
        borderColor: chartColors.value.cacheCreation,
        backgroundColor: 'rgba(245, 196, 67, 0.12)',
        pointBackgroundColor: chartColors.value.pointBg,
        pointBorderColor: chartColors.value.cacheCreation,
        pointBorderWidth: 1.8,
        pointRadius: 2.5,
        pointHoverRadius: 4.5,
        borderWidth: 2,
        fill: false,
        tension: 0.35
      },
      {
        label: 'Cache Read',
        data: props.trendData.map((d) => d.cache_read_tokens),
        borderColor: chartColors.value.cacheRead,
        backgroundColor: 'rgba(76, 201, 176, 0.12)',
        pointBackgroundColor: chartColors.value.pointBg,
        pointBorderColor: chartColors.value.cacheRead,
        pointBorderWidth: 1.8,
        pointRadius: 2.5,
        pointHoverRadius: 4.5,
        borderWidth: 2,
        fill: false,
        tension: 0.35
      },
      {
        label: 'Cache Hit Rate',
        data: props.trendData.map((d) => {
          const totalPromptTokens = d.input_tokens + d.cache_read_tokens + d.cache_creation_tokens
          return totalPromptTokens > 0 ? (d.cache_read_tokens / totalPromptTokens) * 100 : 0
        }),
        borderColor: chartColors.value.cacheHitRate,
        backgroundColor: 'rgba(155, 135, 245, 0.12)',
        pointBackgroundColor: chartColors.value.pointBg,
        pointBorderColor: chartColors.value.cacheHitRate,
        pointBorderWidth: 1.8,
        pointRadius: 2.5,
        pointHoverRadius: 4.5,
        borderWidth: 2,
        borderDash: [5, 5],
        fill: false,
        tension: 0.35,
        yAxisID: 'yPercent'
      }
    ]
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'bottom' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        boxWidth: 8,
        boxHeight: 8,
        padding: 14,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          if (context.dataset.yAxisID === 'yPercent') {
            return `${context.dataset.label}: ${context.raw.toFixed(1)}%`
          }
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        },
        footer: (tooltipItems: any) => {
          const dataIndex = tooltipItems[0]?.dataIndex
          if (dataIndex !== undefined && props.trendData[dataIndex]) {
            const data = props.trendData[dataIndex]
            return `Actual: $${formatCost(data.actual_cost)} | Standard: $${formatCost(data.cost)}`
          }
          return ''
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    },
    yPercent: {
      position: 'right' as const,
      min: 0,
      max: 100,
      grid: {
        drawOnChartArea: false
      },
      ticks: {
        color: chartColors.value.cacheHitRate,
        font: {
          size: 10
        },
        callback: (value: string | number) => `${value}%`
      }
    }
  }
}))

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatCost = (value: number): string => {
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  } else if (value >= 1) {
    return value.toFixed(2)
  } else if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}
</script>
