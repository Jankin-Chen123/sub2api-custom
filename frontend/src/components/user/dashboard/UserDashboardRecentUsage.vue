<template>
  <section class="notebook-recent" aria-labelledby="recent-usage-title">
    <header class="notebook-section-header">
      <div class="flex items-baseline gap-2">
        <h2 id="recent-usage-title" class="notebook-heading">{{ t('dashboard.recentUsage') }}</h2>
        <span class="notebook-muted hidden text-xs sm:inline">{{ t('dashboard.last7Days') }}</span>
      </div>
      <RouterLink to="/usage" class="notebook-view-all notebook-view-all-desktop">
        {{ t('dashboard.viewAllUsage') }} <Icon name="chevronRight" size="sm" />
      </RouterLink>
    </header>
    <div class="notebook-records">
      <div v-if="loading" class="flex justify-center py-12"><LoadingSpinner size="lg" /></div>
      <EmptyState
        v-else-if="!data.length"
        class="py-8"
        :title="t('dashboard.noUsageRecords')"
        :description="t('dashboard.startUsingApi')"
      />
      <template v-else>
        <div class="notebook-records-labels" aria-hidden="true">
          <span>{{ t('dashboard.model') }}</span><span>{{ t('dashboard.time') }}</span><span>{{ t('dashboard.tokens') }}</span>
          <span>{{ t('dashboard.actual') }}</span><span>{{ t('dashboard.standard') }}</span>
        </div>
        <ul>
          <li v-for="log in data" :key="log.id" class="notebook-record">
            <span class="notebook-record-model"><Icon name="cube" size="sm" aria-hidden="true" /><strong>{{ log.model }}</strong></span>
            <time :datetime="log.created_at">{{ formatDateTime(log.created_at) }}</time>
            <span class="notebook-record-tokens"><span class="sr-only">Token </span>{{ formatNumber(log.input_tokens + log.output_tokens) }}</span>
            <strong class="notebook-record-fee"><span class="sr-only">{{ t('dashboard.actual') }} </span>${{ formatCost(log.actual_cost) }}</strong>
            <span class="notebook-record-standard"><span class="sm:sr-only">{{ t('dashboard.standard') }} </span>${{ formatCost(log.total_cost) }}</span>
          </li>
        </ul>
      </template>
    </div>
    <RouterLink to="/usage" class="notebook-view-all notebook-view-all-mobile">
      {{ t('dashboard.viewAllUsage') }} <Icon name="chevronRight" size="sm" />
    </RouterLink>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime, formatCostFixed as formatCost, formatNumberLocaleString as formatNumber } from '@/utils/format'
import type { UsageLog } from '@/types'

defineProps<{ data: UsageLog[]; loading: boolean }>()
const { t } = useI18n()
</script>
