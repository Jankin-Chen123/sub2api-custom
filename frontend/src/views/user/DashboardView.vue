<template>
  <AppLayout>
    <div class="mx-auto max-w-[1200px] space-y-6 pb-4">
      <div v-if="loading" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>
      <template v-else-if="stats">
        <UserDashboardStats
          section="overview"
          :stats="stats"
          :balance="user?.balance || 0"
          :is-simple="authStore.isSimpleMode"
          :platform-quotas="platformQuotas"
        >
          <template #actions>
            <UserDashboardQuickActions />
          </template>
        </UserDashboardStats>

        <UserDashboardCharts
          v-model:startDate="startDate"
          v-model:endDate="endDate"
          v-model:granularity="granularity"
          :applied-period="chartPeriod"
          :loading="loadingCharts"
          :trend="trendData"
          :models="modelStats"
          @dateRangeChange="onDateRangeChange"
          @granularityChange="loadCharts"
          @refresh="refreshAll"
        />

        <UserDashboardStats
          section="platforms"
          :stats="stats"
          :balance="user?.balance || 0"
          :is-simple="authStore.isSimpleMode"
          :platform-quotas="platformQuotas"
        />

        <UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'
import { getMyPlatformQuotas } from '@/api/user'
import { formatDateLocalInput } from '@/utils/format'

const authStore = useAuthStore()
const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null)
const loading = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)

function lastSevenDays() {
  const end = new Date()
  const start = new Date(end)
  start.setDate(start.getDate() - 6)
  return { start: formatDateLocalInput(start), end: formatDateLocalInput(end) }
}
const initialRange = lastSevenDays()
const startDate = ref(initialRange.start)
const endDate = ref(initialRange.end)
const chartPeriod = ref<'dates' | '24h'>('dates')
let latestChartRequest = 0
const granularity = ref('day')

const loadStats = async () => {
  loading.value = true
  try {
    await authStore.refreshUser()
    stats.value = await usageAPI.getDashboardStats()
  } catch (error) {
    console.error('Failed to load dashboard stats:', error)
  } finally {
    loading.value = false
  }
}

function onDateRangeChange(range: { preset: string | null }) {
  chartPeriod.value = range.preset === 'last24Hours' ? '24h' : 'dates'
  return loadCharts()
}

const loadCharts = async () => {
  const request = ++latestChartRequest
  const range = chartPeriod.value === '24h'
    ? { period: '24h' as const }
    : { start_date: startDate.value, end_date: endDate.value }
  loadingCharts.value = true
  try {
    const res = await Promise.all([
      usageAPI.getDashboardTrend({
        ...range,
        granularity: granularity.value as 'day' | 'hour',
      }),
      usageAPI.getDashboardModels(range),
    ])
    if (request !== latestChartRequest) return
    trendData.value = res[0].trend || []
    modelStats.value = res[1].models || []
  } catch (error) {
    if (request !== latestChartRequest) return
    trendData.value = []
    modelStats.value = []
    console.error('Failed to load charts:', error)
  } finally {
    if (request === latestChartRequest) loadingCharts.value = false
  }
}

const loadRecent = async () => {
  loadingUsage.value = true
  try {
    const { start, end } = lastSevenDays()
    const res = await usageAPI.getByDateRange(start, end)
    recentUsage.value = res.items.slice(0, 5)
  } catch (error) {
    console.error('Failed to load recent usage:', error)
  } finally {
    loadingUsage.value = false
  }
}

const loadPlatformQuotas = async () => {
  try {
    const data = await getMyPlatformQuotas()
    platformQuotas.value = data.platform_quotas ?? []
  } catch (error) {
    console.warn('Failed to load platform quotas:', error)
    platformQuotas.value = []
  }
}

const refreshAll = () => {
  loadStats()
  loadCharts()
  loadRecent()
  loadPlatformQuotas()
}

onMounted(() => {
  refreshAll()
})
</script>
