<template>
  <section class="journal-banner-apricot overflow-visible rounded-[22px] p-2">
    <div class="journal-banner-stitch rounded-[16px] p-3 sm:p-4">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-center gap-3">
          <JournalBannerSticker name="palette" size="sm" />
          <div class="min-w-0">
            <div class="flex items-center gap-1.5">
              <h1 class="truncate text-xl font-black tracking-tight text-[#27221d] dark:text-white">{{ t('imageWorkbench.title') }}</h1>
              <CraftSparks variant="right" size="sm" />
            </div>
            <p class="truncate text-xs font-medium text-[#5c5042] dark:text-gray-400">{{ t('imageWorkbench.header.subtitle') }}</p>
          </div>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <button class="btn btn-secondary" type="button" :disabled="completedCount === 0 || downloadingAll" @click="$emit('download-all')">
            {{ downloadingAll ? t('imageWorkbench.actions.downloading') : t('imageWorkbench.actions.downloadAll') }}
          </button>
          <button class="btn btn-primary" type="button" :disabled="submitting || !canSubmit" @click="$emit('submit')">
            {{ submitting ? t('imageWorkbench.actions.submitting') : t('imageWorkbench.actions.generateShort') }}
          </button>
        </div>
      </div>

      <dl class="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
        <div class="journal-kpi-warm rounded-xl px-3 py-2">
          <dt class="text-xs font-semibold text-[#6f6252] dark:text-gray-400">{{ t('imageWorkbench.header.model') }}</dt>
          <dd class="mt-0.5 truncate text-sm font-bold text-[#221d17] dark:text-white">{{ headerModel }}</dd>
        </div>
        <div class="journal-kpi-warm rounded-xl px-3 py-2">
          <dt class="text-xs font-semibold text-[#6f6252] dark:text-gray-400">{{ t('imageWorkbench.header.canvas') }}</dt>
          <dd class="mt-0.5 truncate text-sm font-bold text-[#221d17] dark:text-white">{{ headerCanvasSize }}</dd>
        </div>
        <div class="journal-kpi-warm rounded-xl px-3 py-2">
          <dt class="text-xs font-semibold text-[#6f6252] dark:text-gray-400">{{ t('imageWorkbench.header.queue') }}</dt>
          <dd class="mt-0.5 truncate text-sm font-bold text-[#221d17] dark:text-white">{{ queueSummary }}</dd>
        </div>
        <div class="journal-kpi-warm rounded-xl px-3 py-2">
          <dt class="text-xs font-semibold text-[#6f6252] dark:text-gray-400">{{ t('imageWorkbench.header.interface') }}</dt>
          <dd class="mt-0.5 flex items-center gap-1.5 truncate text-sm font-bold" :class="interfaceConnected ? 'text-emerald-600 dark:text-emerald-400' : 'text-[#221d17] dark:text-white'">
            <span class="h-2 w-2 shrink-0 rounded-full" :class="interfaceConnected ? 'bg-emerald-500' : 'bg-gray-400'" />
            {{ interfaceStatus }}
          </dd>
        </div>
      </dl>

      <div v-if="announcements.length" class="mt-2 flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-2.5 py-2 text-xs text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300" role="status" aria-live="polite">
        <svg class="h-3.5 w-3.5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 8.25v3.75m0 3.75h.008M10.34 3.94 2.52 17.5a1.5 1.5 0 0 0 1.3 2.25h16.36a1.5 1.5 0 0 0 1.3-2.25L13.66 3.94a1.91 1.91 0 0 0-3.32 0Z" />
        </svg>
        <div class="h-5 min-w-0 flex-1 overflow-hidden">
          <div class="transition-transform duration-500 ease-out" :style="announcementTrackStyle">
            <div v-for="announcement in announcements" :key="announcement.id" class="h-5 truncate leading-5">
              {{ announcement.content }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import CraftSparks from '@/components/craft/CraftSparks.vue'
import JournalBannerSticker from '@/components/craft/JournalBannerSticker.vue'
import type { ImageWorkbenchAnnouncement } from '@/types'

defineProps<{
  headerModel: string
  headerCanvasSize: string
  queueSummary: string
  interfaceConnected: boolean
  interfaceStatus: string
  completedCount: number
  downloadingAll: boolean
  submitting: boolean
  canSubmit: boolean
  announcements: ImageWorkbenchAnnouncement[]
  announcementTrackStyle: { transform: string }
}>()

defineEmits<{
  (e: 'download-all'): void
  (e: 'submit'): void
}>()

const { t } = useI18n()
</script>
