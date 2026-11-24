<template>
  <div class="card overflow-hidden p-0">
    <div class="journal-card-band flex items-center justify-between px-5 py-3.5">
      <div class="flex items-center gap-2.5">
        <span class="journal-icon-well journal-icon-well--sm">
          <CraftIcon name="layers" size="xs" />
        </span>
        <div>
          <div class="flex items-center gap-1.5">
            <h2 class="journal-section-title text-lg font-extrabold text-[#221d17] dark:text-white">{{ t('imageWorkbench.library.title') }}</h2>
            <CraftSparks variant="right" size="sm" />
          </div>
          <p class="mt-0.5 text-xs font-medium text-[#6f6252] dark:text-gray-400">{{ t('imageWorkbench.library.subtitle') }}</p>
        </div>
      </div>
      <span class="rounded-full bg-[#ffe6d1] px-2.5 py-0.5 text-xs font-extrabold text-[#b45309] dark:bg-primary-950/50 dark:text-primary-300">{{ completedJobs.length }}</span>
    </div>
    <div class="p-4">
      <div v-if="completedJobs.length === 0" class="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-[#dcc6a4] bg-[#fffaf0]/80 p-8 text-center dark:border-dark-600 dark:bg-dark-800/40">
        <div class="relative mb-3 inline-flex">
          <div class="journal-icon-well -rotate-3">
            <CraftIcon name="palette" size="lg" />
          </div>
          <CraftSparks variant="right" size="sm" class="ml-1 self-start" />
        </div>
        <p class="text-sm font-bold text-[#4e4336] dark:text-gray-400">{{ t('imageWorkbench.library.empty') }}</p>
      </div>
      <div v-else class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <article v-for="job in completedJobs" :key="job.id" class="group overflow-hidden rounded-2xl border border-[#e3d2b6] bg-[#fffdf8] shadow-[0_4px_0_rgba(143,108,65,0.08)] transition-all hover:-translate-y-0.5 hover:shadow-[0_7px_0_rgba(143,108,65,0.12)] dark:border-dark-700 dark:bg-dark-900">
          <button class="block w-full text-left" type="button" :aria-label="t('imageWorkbench.actions.previewArtwork', { name: jobDisplayName(job) })" @click="$emit('preview-artwork', job)">
            <div class="aspect-square overflow-hidden bg-[#f9f1e1] dark:bg-dark-800">
              <img v-if="previewURLs[job.id]" :src="previewURLs[job.id]" :alt="jobDisplayName(job)" class="h-full w-full object-cover transition group-hover:scale-105" />
              <span v-else class="flex h-full items-center justify-center text-xs text-[#7b6b57] dark:text-gray-400">{{ t('imageWorkbench.actions.loadPreview') }}</span>
            </div>
          </button>
          <div class="p-3">
            <form v-if="renamingJobId === job.id" class="space-y-2" @submit.prevent="$emit('save-job-name', job)">
              <input v-model="renameDraftModel" class="input" maxlength="80" :aria-label="t('imageWorkbench.library.nameLabel')" :placeholder="t('imageWorkbench.library.namePlaceholder')" data-testid="work-name-input" />
              <div class="flex gap-2">
                <button class="btn btn-primary btn-sm flex-1" type="submit" :disabled="savingJobNameId === job.id || !renameDraft.trim()">{{ t('imageWorkbench.actions.saveName') }}</button>
                <button class="btn btn-secondary btn-sm flex-1" type="button" :disabled="savingJobNameId === job.id" @click="$emit('cancel-rename')">{{ t('imageWorkbench.actions.cancel') }}</button>
              </div>
            </form>
            <template v-else>
              <button class="block w-full truncate text-left text-xs font-bold text-[#221d17] hover:text-[#c26328] dark:text-gray-200 dark:hover:text-blue-300" type="button" @click="$emit('preview-artwork', job)">{{ jobDisplayName(job) }}</button>
              <div class="mt-2 flex items-end justify-between gap-2">
                <p class="min-w-0 truncate text-[10px] text-[#8a7a65] dark:text-gray-400">{{ job.actual_size || job.requested_size }}</p>
                <div class="flex shrink-0 items-center gap-1 text-[10px] font-semibold" :data-testid="`library-metadata-${job.id}`">
                  <span class="rounded-full bg-violet-50 px-2 py-0.5 text-violet-600 dark:bg-violet-950/40 dark:text-violet-300">{{ t('imageWorkbench.metadata.resolution', { value: jobResolution(job) }) }}</span>
                  <span class="rounded-full bg-amber-50 px-2 py-0.5 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300">{{ t('imageWorkbench.metadata.quality', { value: jobQuality(job) }) }}</span>
                </div>
              </div>
              <div class="mt-3 grid grid-cols-2 gap-2 border-t border-dashed border-[#e8dac0] pt-3 dark:border-dark-700">
                <button class="inline-flex items-center justify-center gap-1.5 rounded-lg border border-blue-200 bg-blue-50 px-2 py-1.5 text-xs font-semibold text-blue-700 transition hover:border-blue-300 hover:bg-blue-100 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-300 dark:hover:bg-blue-950/70" type="button" :data-testid="`rename-work-${job.id}`" @click="$emit('start-rename', job)">
                  <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                    <path stroke-linecap="round" stroke-linejoin="round" d="m16.86 4.49 1.69-1.69a1.88 1.88 0 1 1 2.65 2.65L10.58 16.07a4.5 4.5 0 0 1-1.9 1.13l-2.68.8.8-2.68a4.5 4.5 0 0 1 1.13-1.9L16.86 4.5Zm0 0L19.5 7.13M18 14v4.75A2.25 2.25 0 0 1 15.75 21h-10.5A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
                  </svg>
                  {{ t('imageWorkbench.actions.rename') }}
                </button>
                <button class="inline-flex items-center justify-center gap-1.5 rounded-lg border border-red-200 bg-red-50 px-2 py-1.5 text-xs font-semibold text-red-600 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-wait disabled:opacity-60 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300 dark:hover:bg-red-950/50" type="button" :disabled="deletingJobId === job.id" :data-testid="`delete-library-${job.id}`" @click="$emit('delete-job', job)">
                  <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9 14.4 18m-4.8 0L9.26 9m9.97-3.21c.34.05.68.1 1.02.16m-1.02-.16L18.1 19.67A2.25 2.25 0 0 1 15.86 21H8.14a2.25 2.25 0 0 1-2.24-2.08L4.77 5.79m14.46 0A48.1 48.1 0 0 0 15.75 5m-10.98.79c.34-.06.68-.11 1.02-.16m0 0A48.11 48.11 0 0 1 8.25 5m7.5 0V3.92c0-1.18-.91-2.16-2.09-2.2a52.86 52.86 0 0 0-3.32 0c-1.18.04-2.09 1.02-2.09 2.2V5m7.5 0a48.67 48.67 0 0 0-7.5 0" />
                  </svg>
                  {{ deletingJobId === job.id ? t('imageWorkbench.actions.deleting') : t('imageWorkbench.actions.delete') }}
                </button>
              </div>
            </template>
          </div>
        </article>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import CraftIcon from '@/components/craft/CraftIcon.vue'
import CraftSparks from '@/components/craft/CraftSparks.vue'
import type { ImageWorkbenchJob } from '@/api'

const props = defineProps<{
  completedJobs: ImageWorkbenchJob[]
  previewURLs: Record<string, string>
  renamingJobId: string
  renameDraft: string
  savingJobNameId: string
  deletingJobId: string
  jobDisplayName: (job: ImageWorkbenchJob) => string
  jobResolution: (job: ImageWorkbenchJob) => string
  jobQuality: (job: ImageWorkbenchJob) => string
}>()

const emit = defineEmits<{
  (e: 'preview-artwork', job: ImageWorkbenchJob): void
  (e: 'start-rename', job: ImageWorkbenchJob): void
  (e: 'cancel-rename'): void
  (e: 'save-job-name', job: ImageWorkbenchJob): void
  (e: 'delete-job', job: ImageWorkbenchJob): void
  (e: 'update:renameDraft', value: string): void
}>()

const { t } = useI18n()

const renameDraftModel = computed({
  get: () => props.renameDraft,
  set: (value: string) => emit('update:renameDraft', value),
})
</script>
