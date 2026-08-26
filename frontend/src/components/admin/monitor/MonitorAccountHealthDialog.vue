<template>
  <BaseDialog
    :show="show"
    :title="t('admin.channelMonitor.accountHealth.title')"
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ monitor?.name || '-' }}
          <span class="mx-1 text-gray-300 dark:text-dark-600">·</span>
          {{ t('admin.channelMonitor.accountHealth.description') }}
        </p>
        <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
          <span>{{ t('admin.channelMonitor.accountHealth.modelFilter') }}</span>
          <select
            v-model="selectedModel"
            class="rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100"
          >
            <option value="">{{ t('admin.channelMonitor.accountHealth.allModels') }}</option>
            <option v-for="model in models" :key="model" :value="model">{{ model }}</option>
          </select>
        </label>
      </div>

      <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <div
          v-for="summary in summaries"
          :key="summary.state"
          class="rounded-xl border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800/70"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ summary.label }}</span>
            <span class="text-base font-semibold" :class="summary.class">{{ summary.count }}</span>
          </div>
        </div>
      </div>

      <div v-if="loading" class="space-y-2">
        <div v-for="i in 4" :key="i" class="h-11 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
      </div>
      <div v-else-if="error" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
        {{ error }}
      </div>
      <div v-else-if="filteredItems.length === 0" class="rounded-lg border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
        {{ t('admin.channelMonitor.accountHealth.empty') }}
      </div>
      <div v-else class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
        <table class="w-full min-w-[920px] divide-y divide-gray-200 text-left text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-xs uppercase tracking-wide text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th class="px-3 py-2.5 font-medium">{{ t('admin.channelMonitor.accountHealth.columns.account') }}</th>
              <th class="px-3 py-2.5 font-medium">{{ t('admin.channelMonitor.accountHealth.columns.model') }}</th>
              <th class="px-3 py-2.5 font-medium">{{ t('admin.channelMonitor.accountHealth.columns.score') }}</th>
              <th class="px-3 py-2.5 font-medium">{{ t('admin.channelMonitor.accountHealth.columns.state') }}</th>
              <th class="px-3 py-2.5 font-medium">{{ t('admin.channelMonitor.accountHealth.columns.successRate') }}</th>
              <th class="px-3 py-2.5 font-medium">{{ t('admin.channelMonitor.accountHealth.columns.latency') }}</th>
              <th class="px-3 py-2.5 font-medium">{{ t('admin.channelMonitor.accountHealth.columns.samples') }}</th>
              <th class="px-3 py-2.5 font-medium">{{ t('admin.channelMonitor.accountHealth.columns.failures') }}</th>
              <th class="px-3 py-2.5 font-medium">{{ t('admin.channelMonitor.accountHealth.columns.lastProbe') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="item in filteredItems" :key="`${item.account_id}-${item.model}`" class="bg-white dark:bg-dark-900">
              <td class="px-3 py-3">
                <div class="font-medium text-gray-900 dark:text-white">{{ item.account_name }}</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">#{{ item.account_id }}</div>
              </td>
              <td class="px-3 py-3 text-gray-700 dark:text-gray-200">{{ item.model }}</td>
              <td class="px-3 py-3 font-semibold" :class="scoreClass(item)">{{ formatScore(item.score) }}</td>
              <td class="px-3 py-3">
                <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium" :class="stateClass(item)">
                  {{ stateLabel(item) }}
                </span>
              </td>
              <td class="px-3 py-3 text-gray-700 dark:text-gray-200">{{ formatRate(item.ewma_success_rate) }}</td>
              <td class="px-3 py-3 text-gray-700 dark:text-gray-200">{{ formatLatency(item.ewma_latency_ms) }}</td>
              <td class="px-3 py-3 text-gray-700 dark:text-gray-200">{{ item.sample_count }}</td>
              <td class="px-3 py-3 text-gray-700 dark:text-gray-200">{{ item.consecutive_failures }}</td>
              <td class="px-3 py-3 text-xs text-gray-500 dark:text-gray-400">
                <span>{{ formatDate(item.last_probe_at) }}</span>
                <span v-if="item.stale" class="ml-1 text-amber-600 dark:text-amber-400">{{ t('admin.channelMonitor.accountHealth.stale') }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-between gap-3">
        <span class="text-xs text-gray-400 dark:text-gray-500">
          {{ t('admin.channelMonitor.accountHealth.sampleHint') }}
        </span>
        <button type="button" class="btn btn-primary" @click="emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountHealthItem, AccountHealthState, ChannelMonitor } from '@/api/admin/channelMonitor'
import { channelMonitorAPI } from '@/api/admin/channelMonitor'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{
  show: boolean
  monitor: ChannelMonitor | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const items = ref<AccountHealthItem[]>([])
const selectedModel = ref('')
const loading = ref(false)
const error = ref('')

const models = computed(() => {
  const values = [props.monitor?.primary_model || '', ...(props.monitor?.extra_models || [])]
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))]
})

const filteredItems = computed(() => {
  if (!selectedModel.value) return items.value
  return items.value.filter((item) => item.model === selectedModel.value)
})

const summaries = computed(() => {
  const counts: Record<AccountHealthState, number> = {
    healthy: 0,
    degraded: 0,
    unhealthy: 0,
    unknown: 0,
  }
  for (const item of filteredItems.value) counts[item.health_state] = (counts[item.health_state] || 0) + 1
  return [
    { state: 'healthy' as const, count: counts.healthy, label: t('admin.channelMonitor.accountHealth.states.healthy'), class: 'text-emerald-600 dark:text-emerald-300' },
    { state: 'degraded' as const, count: counts.degraded, label: t('admin.channelMonitor.accountHealth.states.degraded'), class: 'text-amber-600 dark:text-amber-300' },
    { state: 'unhealthy' as const, count: counts.unhealthy, label: t('admin.channelMonitor.accountHealth.states.unhealthy'), class: 'text-red-600 dark:text-red-300' },
    { state: 'unknown' as const, count: counts.unknown, label: t('admin.channelMonitor.accountHealth.states.unknown'), class: 'text-gray-500 dark:text-gray-300' },
  ]
})

async function load() {
  if (!props.show || !props.monitor) return
  loading.value = true
  error.value = ''
  try {
    const response = await channelMonitorAPI.listAccountHealth(props.monitor.id)
    items.value = response.items || []
  } catch {
    items.value = []
    error.value = t('admin.channelMonitor.accountHealth.loadError')
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.monitor?.id] as const,
  ([show]) => {
    if (show) {
      selectedModel.value = ''
      void load()
    } else {
      items.value = []
      error.value = ''
    }
  },
  { immediate: true }
)

function stateLabel(item: AccountHealthItem): string {
  return t(`admin.channelMonitor.accountHealth.states.${item.health_state}`)
}

function stateClass(item: AccountHealthItem): string {
  switch (item.health_state) {
    case 'healthy': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
    case 'degraded': return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
    case 'unhealthy': return 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300'
    default: return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
}

function scoreClass(item: AccountHealthItem): string {
  if (item.health_state === 'healthy') return 'text-emerald-600 dark:text-emerald-300'
  if (item.health_state === 'degraded') return 'text-amber-600 dark:text-amber-300'
  if (item.health_state === 'unhealthy') return 'text-red-600 dark:text-red-300'
  return 'text-gray-600 dark:text-gray-300'
}

function formatScore(score: number): string {
  return `${Math.max(0, Math.min(100, score)).toFixed(1)}`
}

function formatRate(rate: number): string {
  return `${(Math.max(0, Math.min(1, rate)) * 100).toFixed(1)}%`
}

function formatLatency(value: number | null): string {
  return value == null ? '-' : `${Math.round(value)} ms`
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}
</script>
