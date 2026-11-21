<template>
  <AppLayout sticker-dashboard>
    <div class="sticker-dashboard-content">
      <h1 class="sticker-dashboard-title">{{ t('dashboard.overview') }}</h1>
      <div v-if="loading && !stats" class="flex justify-center py-14"><LoadingSpinner /></div>
      <div v-else-if="!stats" class="card p-6" role="alert">
        <p>{{ t('dashboard.loadFailed') }}</p>
        <button class="btn btn-secondary mt-3" @click="refreshAll">{{ t('common.refresh') }}</button>
      </div>
      <template v-else>
        <UserDashboardStats :stats="stats" :balance="user?.balance || 0" :is-simple="authStore.isSimpleMode" :platform-quotas="platformQuotas">
          <template #actions><UserDashboardQuickActions /></template>
          <template #analytics>
            <UserDashboardCharts
              v-model:startDate="startDate" v-model:endDate="endDate" v-model:granularity="granularity"
              :loading="loadingCharts" :trend="trendData" :models="modelStats"
              @dateRangeChange="loadCharts" @granularityChange="loadCharts" @refresh="refreshAll"
            />
          </template>
        </UserDashboardStats>
        <UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { usageAPI, type UserDashboardStats as AccountStats } from '@/api/usage'
import { getMyPlatformQuotas } from '@/api/user'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import { useAuthStore } from '@/stores/auth'
import type { ModelStat, PlatformQuotaItem, TrendDataPoint, UsageLog } from '@/types'
import { formatDateLocalInput } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()
const user = computed(() => authStore.user)
const stats = ref<AccountStats | null>(null)
const loading = ref(true)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)

function lastWeek() {
  const end = new Date()
  const start = new Date(end)
  start.setDate(start.getDate() - 6)
  return { start_date: formatDateLocalInput(start), end_date: formatDateLocalInput(end) }
}

const initialRange = lastWeek()
const startDate = ref(initialRange.start_date)
const endDate = ref(initialRange.end_date)
const granularity = ref<'day' | 'hour'>('day')
let chartRequest = 0

async function loadStats() {
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

async function loadCharts() {
  const request = ++chartRequest
  const range = { start_date: startDate.value, end_date: endDate.value }
  loadingCharts.value = true
  try {
    const [usage, distribution] = await Promise.all([
      usageAPI.getDashboardTrend({ ...range, granularity: granularity.value }),
      usageAPI.getDashboardModels(range),
    ])
    if (request === chartRequest) {
      trendData.value = usage.trend || []
      modelStats.value = distribution.models || []
    }
  } catch (error) {
    console.error('Failed to load charts:', error)
  } finally {
    if (request === chartRequest) loadingCharts.value = false
  }
}

async function loadRecent() {
  const range = lastWeek()
  loadingUsage.value = true
  try {
    const response = await usageAPI.getByDateRange(range.start_date, range.end_date)
    recentUsage.value = response.items.slice(0, 5)
  } catch (error) {
    console.error('Failed to load recent usage:', error)
  } finally {
    loadingUsage.value = false
  }
}

async function loadPlatformQuotas() {
  try {
    const response = await getMyPlatformQuotas()
    platformQuotas.value = response.platform_quotas ?? []
  } catch (error) {
    console.warn('Failed to load platform quotas:', error)
  }
}

function refreshAll() {
  return Promise.all([loadStats(), loadCharts(), loadRecent(), loadPlatformQuotas()])
}

onMounted(refreshAll)
</script>

<style src="@/styles/sticker-dashboard.css"></style>
