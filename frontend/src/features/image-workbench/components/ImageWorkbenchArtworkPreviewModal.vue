<template>
  <div v-if="job" class="fixed inset-0 z-50 flex items-center justify-center bg-[#231d17]/75 p-4 backdrop-blur-sm" data-testid="artwork-preview" @click.self="$emit('close')">
    <section class="flex max-h-[94vh] w-full max-w-6xl flex-col overflow-hidden rounded-2xl border border-[#e3d2b6] bg-[#fffdf8] shadow-2xl dark:border-dark-700 dark:bg-dark-800" role="dialog" aria-modal="true" :aria-label="displayName">
      <header class="journal-card-band flex flex-wrap items-center justify-between gap-3 px-4 py-3 sm:px-5">
        <div class="min-w-0">
          <h2 class="truncate text-lg font-extrabold text-[#221d17] dark:text-white">{{ displayName }}</h2>
          <p class="mt-0.5 text-xs font-medium text-[#6f6252] dark:text-gray-400">{{ job.actual_size || job.requested_size }}</p>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <button class="btn btn-secondary btn-sm" type="button" @click="$emit('download', job)">{{ t('imageWorkbench.actions.downloadCurrent') }}</button>
          <button class="btn btn-secondary btn-sm" type="button" :aria-label="t('imageWorkbench.actions.closePreview')" @click="$emit('close')">×</button>
        </div>
      </header>
      <div class="flex min-h-0 flex-1 items-center justify-center overflow-auto bg-[linear-gradient(135deg,#fdf7eb_25%,#f5ecdc_25%,#f5ecdc_50%,#fdf7eb_50%,#fdf7eb_75%,#f5ecdc_75%)] bg-[length:28px_28px] p-4 dark:bg-dark-900 sm:p-6">
        <img v-if="previewUrl" :src="previewUrl" :alt="displayName" class="max-h-[78vh] max-w-full rounded-xl object-contain shadow-lg" />
        <p v-else class="text-sm text-[#6f6252] dark:text-gray-400">{{ t('imageWorkbench.actions.loadPreview') }}</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ImageWorkbenchJob } from '@/api'

defineProps<{
  job: ImageWorkbenchJob | null
  previewUrl: string
  displayName: string
}>()

defineEmits<{
  (e: 'close'): void
  (e: 'download', job: ImageWorkbenchJob): void
}>()

const { t } = useI18n()
</script>
