<template>
  <aside class="rounded-xl border border-[#e3d2b6] bg-[#fffaf0]/85 p-3 dark:border-dark-700 dark:bg-dark-900/60">
    <div class="mb-3 flex items-center justify-between">
      <h3 class="font-extrabold text-[#221d17] dark:text-white">{{ t('imageWorkbench.preview.batchTitle') }}</h3>
      <span class="rounded-full bg-[#ffe6d1] px-2.5 py-0.5 text-xs font-extrabold text-[#b45309] dark:bg-primary-950/50 dark:text-primary-300">{{ jobs.length }}</span>
    </div>
    <div v-if="loadingJobs && jobs.length === 0" class="py-8 text-center text-sm text-[#7b6b57] dark:text-gray-400">{{ t('common.loading') }}</div>
    <div v-else-if="jobs.length === 0" class="rounded-2xl border-2 border-dashed border-[#dcc6a4] bg-[#fffdf8] p-5 text-center text-sm font-semibold text-[#6f6252] dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400">
      <div class="journal-icon-well journal-icon-well--sm mx-auto mb-2.5 -rotate-3">
        <CraftIcon name="inbox" size="md" />
      </div>
      <p>{{ t('imageWorkbench.jobs.empty') }}</p>
    </div>
    <div v-else class="max-h-[420px] space-y-2 overflow-y-auto pr-1">
      <article
        v-for="job in jobs"
        :key="job.id"
        class="overflow-hidden rounded-xl border transition"
        :class="job.id === selectedJobId ? 'border-[#e39c64] bg-[#fff4e3] shadow-[0_2px_0_rgba(227,156,100,0.2)] dark:border-primary-500 dark:bg-primary-950/30' : 'border-[#e5d5ba] bg-[#fffdf8] hover:border-[#d8b482] dark:border-dark-700 dark:bg-dark-800'"
      >
        <button class="block w-full p-3 text-left" type="button" @click="$emit('select-job', job)">
          <div class="flex items-start justify-between gap-2">
            <span class="truncate text-xs font-bold text-[#221d17] dark:text-gray-200">{{ jobDisplayName(job) }}</span>
            <span :class="statusClass(job.status)" class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium">{{ statusText(job.status) }}</span>
          </div>
          <p class="mt-2 line-clamp-2 text-xs text-[#6f6252] dark:text-gray-400">{{ job.actual_size || job.requested_size || job.id }}</p>
          <p class="mt-2 text-[10px] text-[#8a7a65] dark:text-gray-400">{{ formatDate(job.created_at) }}</p>
          <div class="mt-2 flex items-end justify-between gap-2">
            <p class="inline-flex rounded-full bg-blue-50 px-2 py-0.5 text-[10px] font-semibold tabular-nums text-blue-600 dark:bg-blue-950/40 dark:text-blue-300">
              {{ t('imageWorkbench.preview.syncWait', { time: formatJobWaitTime(job) }) }}
            </p>
            <div class="flex shrink-0 items-center gap-1 text-[10px] font-semibold" :data-testid="`batch-metadata-${job.id}`">
              <span class="rounded-full bg-violet-50 px-2 py-0.5 text-violet-600 dark:bg-violet-950/40 dark:text-violet-300">{{ t('imageWorkbench.metadata.resolution', { value: jobResolution(job) }) }}</span>
              <span class="rounded-full bg-amber-50 px-2 py-0.5 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300">{{ t('imageWorkbench.metadata.quality', { value: jobQuality(job) }) }}</span>
            </div>
          </div>
        </button>
        <div class="flex justify-end border-t border-dashed border-[#efe2cb] px-3 py-2 dark:border-dark-700">
          <button
            class="inline-flex items-center gap-1 rounded-md border border-red-200 bg-red-50 px-2 py-1 text-[11px] font-semibold text-red-600 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-not-allowed disabled:border-gray-200 disabled:bg-gray-50 disabled:text-gray-400 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300 dark:hover:bg-red-950/50 dark:disabled:border-dark-700 dark:disabled:bg-dark-900 dark:disabled:text-gray-600"
            type="button"
            :disabled="!canDeleteJob(job) || deletingJobId === job.id"
            :title="canDeleteJob(job) ? t('imageWorkbench.actions.delete') : t('imageWorkbench.actions.deleteUnavailable')"
            :data-testid="`delete-batch-${job.id}`"
            @click="$emit('delete-job', job)"
          >
            <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9 14.4 18m-4.8 0L9.26 9m9.97-3.21c.34.05.68.1 1.02.16m-1.02-.16L18.1 19.67A2.25 2.25 0 0 1 15.86 21H8.14a2.25 2.25 0 0 1-2.24-2.08L4.77 5.79m14.46 0A48.1 48.1 0 0 0 15.75 5m-10.98.79c.34-.06.68-.11 1.02-.16m0 0A48.11 48.11 0 0 1 8.25 5m7.5 0V3.92c0-1.18-.91-2.16-2.09-2.2a52.86 52.86 0 0 0-3.32 0c-1.18.04-2.09 1.02-2.09 2.2V5m7.5 0a48.67 48.67 0 0 0-7.5 0" />
            </svg>
            {{ deletingJobId === job.id ? t('imageWorkbench.actions.deleting') : t('imageWorkbench.actions.delete') }}
          </button>
        </div>
      </article>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import CraftIcon from '@/components/craft/CraftIcon.vue'
import type { ImageWorkbenchJob, ImageWorkbenchStatus } from '@/api'

defineProps<{
  jobs: ImageWorkbenchJob[]
  loadingJobs: boolean
  selectedJobId: string
  deletingJobId: string
  formatDate: (value: string) => string
  formatJobWaitTime: (job: ImageWorkbenchJob) => string
  jobDisplayName: (job: ImageWorkbenchJob) => string
  statusClass: (status: ImageWorkbenchStatus) => string
  statusText: (status: ImageWorkbenchStatus) => string
  jobResolution: (job: ImageWorkbenchJob) => string
  jobQuality: (job: ImageWorkbenchJob) => string
  canDeleteJob: (job: ImageWorkbenchJob) => boolean
}>()

defineEmits<{
  (e: 'select-job', job: ImageWorkbenchJob): void
  (e: 'delete-job', job: ImageWorkbenchJob): void
}>()

const { t } = useI18n()
</script>
