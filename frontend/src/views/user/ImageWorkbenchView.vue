<template>
  <AppLayout>
    <div class="-mx-4 -mt-2 -mb-4 flex h-[calc(100dvh-4.5rem-1px)] max-w-[1560px] min-h-0 flex-col gap-3 overflow-hidden px-4 pb-4 pt-0 md:-mx-6 md:-mt-4 md:-mb-6 md:h-[calc(100dvh-4.5rem-1px)] lg:-mx-8 lg:-mt-6 lg:-mb-8 lg:h-[calc(100dvh-4.5rem-1px)] lg:px-4">
      <section class="rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-4">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex min-w-0 items-center gap-2.5">
            <img src="/brand-icon.png" :alt="t('imageWorkbench.header.brandAlt')" class="h-10 w-10 shrink-0 rounded-lg object-cover shadow-sm" />
            <div class="min-w-0">
              <h1 class="truncate text-xl font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.title') }}</h1>
              <p class="truncate text-xs text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.header.subtitle') }}</p>
            </div>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <button class="btn btn-secondary" type="button" :disabled="completedJobs.length === 0 || downloadingAll" @click="downloadAll">
              {{ downloadingAll ? t('imageWorkbench.actions.downloading') : t('imageWorkbench.actions.downloadAll') }}
            </button>
            <button class="btn btn-primary" type="button" :disabled="submitting || !canSubmit" @click="submitJob">
              {{ submitting ? t('imageWorkbench.actions.submitting') : t('imageWorkbench.actions.generateShort') }}
            </button>
          </div>
        </div>

        <dl class="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
          <div class="rounded-lg border border-gray-200 bg-gray-50/70 px-2.5 py-2 dark:border-dark-700 dark:bg-dark-900/50">
            <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.header.model') }}</dt>
            <dd class="mt-0.5 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ headerModel }}</dd>
          </div>
          <div class="rounded-lg border border-gray-200 bg-gray-50/70 px-2.5 py-2 dark:border-dark-700 dark:bg-dark-900/50">
            <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.header.canvas') }}</dt>
            <dd class="mt-0.5 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ headerCanvasSize }}</dd>
          </div>
          <div class="rounded-lg border border-gray-200 bg-gray-50/70 px-2.5 py-2 dark:border-dark-700 dark:bg-dark-900/50">
            <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.header.queue') }}</dt>
            <dd class="mt-0.5 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ queueSummary }}</dd>
          </div>
          <div class="rounded-lg border border-gray-200 bg-gray-50/70 px-2.5 py-2 dark:border-dark-700 dark:bg-dark-900/50">
            <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.header.interface') }}</dt>
            <dd class="mt-0.5 flex items-center gap-1.5 truncate text-sm font-semibold" :class="interfaceConnected ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-900 dark:text-white'">
              <span class="h-2 w-2 shrink-0 rounded-full" :class="interfaceConnected ? 'bg-emerald-500' : 'bg-gray-400'" />
              {{ interfaceStatus }}
            </dd>
          </div>
        </dl>

        <div v-if="workbenchAnnouncements.length" class="mt-2 flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-2.5 py-2 text-xs text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300" role="status" aria-live="polite">
          <svg class="h-3.5 w-3.5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 8.25v3.75m0 3.75h.008M10.34 3.94 2.52 17.5a1.5 1.5 0 0 0 1.3 2.25h16.36a1.5 1.5 0 0 0 1.3-2.25L13.66 3.94a1.91 1.91 0 0 0-3.32 0Z" />
          </svg>
          <div class="h-5 min-w-0 flex-1 overflow-hidden">
            <div class="transition-transform duration-500 ease-out" :style="announcementTrackStyle">
              <div v-for="announcement in workbenchAnnouncements" :key="announcement.id" class="h-5 truncate leading-5">
                {{ announcement.content }}
              </div>
            </div>
          </div>
        </div>
      </section>

      <div class="grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)_minmax(0,1fr)] gap-3 lg:grid-cols-[340px_minmax(0,1fr)] lg:grid-rows-1">
        <form class="image-workbench-compact flex min-h-0 flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-4" @submit.prevent="submitJob">
          <div class="-mx-3 -mt-3 flex shrink-0 items-center justify-between border-b border-gray-100 bg-white px-3 pb-2 pt-3 dark:border-dark-700 dark:bg-dark-800 sm:-mx-4 sm:-mt-4 sm:px-4 sm:pt-4">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.form.settingsTitle') }}</h2>
            <button class="btn btn-secondary btn-sm" type="button" @click="resetForm">{{ t('imageWorkbench.form.reset') }}</button>
          </div>

          <div class="min-h-0 flex-1 space-y-3 overflow-y-auto overscroll-contain pr-1 pt-3 sm:pt-4">
          <div>
            <label class="input-label">{{ t('imageWorkbench.form.apiKey') }}</label>
            <div v-if="loadingKeys" class="input flex items-center text-gray-400" aria-live="polite">{{ t('common.loading') }}</div>
            <p v-else-if="eligibleKeys.length === 0" class="input flex items-center text-amber-600 dark:text-amber-400" data-testid="no-image-api-key">
              {{ t('imageWorkbench.form.noApiKey') }}
            </p>
            <select v-else v-model.number="form.apiKeyId" class="input" required>
              <option v-for="key in eligibleKeys" :key="key.id" :value="key.id">
                {{ key.name }} · {{ key.group?.name || '#' + key.group_id }}
              </option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <div>
              <label class="input-label">{{ t('imageWorkbench.form.model') }}</label>
              <select v-model="form.model" class="input">
                <option v-for="model in models" :key="model.value" :value="model.value">{{ model.label }}</option>
              </select>
            </div>
            <div>
              <label class="input-label">{{ t('imageWorkbench.form.quality') }}</label>
              <select v-model="form.quality" class="input">
                <option v-for="quality in qualities" :key="quality" :value="quality">{{ t('imageWorkbench.form.qualityOptions.' + quality) }}</option>
              </select>
            </div>
          </div>

          <div>
            <label class="input-label">{{ t('imageWorkbench.form.dimensions') }}</label>
            <div class="grid grid-cols-4 gap-1.5">
              <button
                v-for="ratio in aspectRatios"
                :key="ratio || 'auto'"
                class="rounded-md border px-2 py-1.5 text-xs font-semibold transition"
                :class="form.aspectRatio === ratio ? 'border-blue-400 bg-blue-50 text-blue-700 dark:border-blue-700 dark:bg-blue-950/30 dark:text-blue-300' : 'border-gray-200 bg-white text-gray-500 hover:border-blue-300 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300'"
                type="button"
                @click="selectAspectRatio(ratio)"
              >
                {{ ratio || t('imageWorkbench.form.unspecified') }}
              </button>
            </div>

            <div class="mt-3 grid grid-cols-2 gap-2">
              <div>
                <div class="flex items-center justify-between text-xs font-semibold text-gray-700 dark:text-gray-300">
                  <label>{{ t('imageWorkbench.form.width') }}</label>
                  <span class="text-blue-600 dark:text-blue-400">{{ form.width }}px</span>
                </div>
                <input v-model.number="form.width" class="mt-1.5 w-full accent-blue-500" type="range" min="16" max="3840" step="16" @change="normalizeDimensions" />
                <input v-model.number="form.width" class="input mt-1.5" type="number" min="16" max="3840" step="16" @change="normalizeDimensions" />
              </div>
              <div>
                <div class="flex items-center justify-between text-xs font-semibold text-gray-700 dark:text-gray-300">
                  <label>{{ t('imageWorkbench.form.height') }}</label>
                  <span class="text-blue-600 dark:text-blue-400">{{ form.height }}px</span>
                </div>
                <input v-model.number="form.height" class="mt-1.5 w-full accent-blue-500" type="range" min="16" max="3840" step="16" @change="normalizeDimensions" />
                <input v-model.number="form.height" class="input mt-1.5" type="number" min="16" max="3840" step="16" @change="normalizeDimensions" />
              </div>
            </div>

            <p v-if="dimensionErrorMessage" class="mt-2 text-xs leading-5 text-red-600 dark:text-red-400">{{ dimensionErrorMessage }}</p>
            <p v-else class="input-hint">{{ t('imageWorkbench.form.dimensionLimitsHint', { maxPixels: formatInteger(selectedModelMaxPixels) }) }}</p>
            <p v-if="experimentalDimensions" class="mt-1 text-xs leading-5 text-amber-600 dark:text-amber-400">{{ t('imageWorkbench.form.experimentalHint') }}</p>
          </div>

          <div>
            <label class="input-label">{{ t('imageWorkbench.form.prompt') }}</label>
            <textarea
              v-model="form.prompt"
              class="input min-h-24 resize-y"
              maxlength="12000"
              required
              :placeholder="t('imageWorkbench.form.promptPlaceholder')"
              @keydown.ctrl.enter.prevent="submitJob"
              @keydown.meta.enter.prevent="submitJob"
            />
            <div class="mt-1 text-right text-xs text-gray-400">{{ form.prompt.length }}/12000</div>
          </div>

          <div>
            <label class="input-label">{{ t('imageWorkbench.form.referenceTitle') }}</label>
            <div
              class="cursor-pointer rounded-lg border border-dashed px-3 py-2.5 transition"
              :class="isReferenceDragOver ? 'border-blue-500 bg-blue-100 dark:border-blue-400 dark:bg-blue-950/40' : 'border-blue-300 bg-blue-50/80 hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-950/20'"
              @click="referenceInputRef?.click()"
              @dragenter.prevent="isReferenceDragOver = true"
              @dragover.prevent="isReferenceDragOver = true"
              @dragleave.prevent="isReferenceDragOver = false"
              @drop.prevent="onReferenceDrop"
            >
              <input ref="referenceInputRef" class="hidden" type="file" accept="image/*" multiple @change="onReferenceFiles" />
              <p class="text-xs font-semibold text-blue-600 dark:text-blue-300">{{ t('imageWorkbench.form.dropReference') }}</p>
              <p class="mt-1 text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.form.dropReferenceHint') }}</p>
            </div>
            <div class="mt-2 flex gap-2">
              <input v-model.trim="referenceUrlInput" class="input min-w-0 flex-1" :placeholder="t('imageWorkbench.form.pasteReferenceUrl')" @keydown.enter.prevent="importReferenceUrl" />
              <button class="btn btn-secondary shrink-0" type="button" @click="importReferenceUrl">{{ t('imageWorkbench.form.importReference') }}</button>
            </div>
            <button class="btn btn-secondary mt-2 w-full" type="button" :disabled="referenceCount === 0" @click="clearReferences">{{ t('imageWorkbench.form.clearReferences') }}</button>
            <div v-if="referenceCount > 0" class="mt-2 flex flex-wrap gap-2">
              <span v-for="file in referenceFiles" :key="file.name + file.lastModified" class="max-w-full truncate rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ file.name }}</span>
              <span v-for="url in referenceUrlList" :key="url" class="max-w-full truncate rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ url }}</span>
              <span v-if="referenceDataURLs.length && !referenceFiles.length" class="rounded-full bg-blue-50 px-2 py-1 text-xs text-blue-700 dark:bg-blue-950/40 dark:text-blue-300">
                {{ t('imageWorkbench.editor.currentImageReference') }}
              </span>
            </div>
          </div>

          <div class="rounded-lg border border-gray-200 bg-gray-50 p-2 text-xs text-gray-600 dark:border-dark-700 dark:bg-dark-900/50 dark:text-gray-300">
            <span v-if="loadingEstimate">{{ t('imageWorkbench.form.loadingEstimate') }}</span>
            <span v-else-if="costEstimate">{{ t('imageWorkbench.form.estimatedCost', { cost: formatCost(costEstimate.estimated_cost) }) }}</span>
            <span v-else>{{ t('imageWorkbench.form.estimateUnavailable') }}</span>
          </div>
          </div>

          <button class="btn btn-primary mt-3 w-full shrink-0 py-2 text-xs shadow-none disabled:bg-gray-500 disabled:bg-none disabled:from-gray-400 disabled:to-gray-500 disabled:text-white disabled:opacity-100 disabled:shadow-none" type="submit" :disabled="submitting || !canSubmit">
            {{ submitting ? t('imageWorkbench.actions.submitting') : t('imageWorkbench.actions.generate') }}
          </button>
        </form>

        <section class="min-h-0 min-w-0 space-y-3 overflow-y-auto overscroll-contain pr-1">
          <div class="rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
              <div>
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.preview.title') }}</h2>
                <p class="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.preview.subtitle') }}</p>
              </div>
              <div class="flex flex-wrap gap-2">
                <button class="btn btn-secondary btn-sm" type="button" :disabled="dimensionValidation.code !== null" @click="createBlankCanvas">
                  {{ t('imageWorkbench.actions.newCanvas') }}
                </button>
                <button class="btn btn-secondary btn-sm" :disabled="!currentJob || currentJob.status !== 'completed'" @click="openEditor(currentJob!)">
                  {{ t('imageWorkbench.editor.expand') }}
                </button>
                <button class="btn btn-secondary btn-sm" :disabled="!currentJob || currentJob.status !== 'completed'" @click="downloadCurrent">
                  {{ t('imageWorkbench.actions.downloadCurrent') }}
                </button>
              </div>
            </div>

            <div class="grid min-h-[420px] gap-3 lg:grid-cols-[minmax(0,1fr)_250px]">
              <div class="flex min-h-[420px] items-center justify-center rounded-xl border border-gray-200 bg-[linear-gradient(135deg,#f8fafc_25%,#eef2f7_25%,#eef2f7_50%,#f8fafc_50%,#f8fafc_75%,#eef2f7_75%)] bg-[length:28px_28px] p-3 dark:border-dark-700 dark:bg-dark-900">
                <div v-if="blankCanvasOpen || !currentPreviewURL" class="relative max-h-[420px] max-w-full overflow-hidden rounded-xl bg-white shadow-sm transition-[width,aspect-ratio] duration-500 ease-[cubic-bezier(0.22,1,0.36,1)] will-change-[width,aspect-ratio] dark:bg-dark-800" :style="activeCanvasStyle" data-testid="preview-canvas">
                  <div class="flex h-full min-h-24 items-center justify-center px-6 text-center text-gray-400 dark:text-gray-500">
                    <div>
                      <p class="text-lg font-medium text-gray-600 dark:text-gray-300">{{ activeCanvasTitle }}</p>
                      <p class="mt-1 text-sm">{{ activeCanvasHint }}</p>
                    </div>
                  </div>
                  <div class="absolute inset-x-3 bottom-3 flex items-center justify-between rounded-lg bg-gray-500/80 px-3 py-2 text-xs text-white">
                    <span>{{ activeCanvasStatus }}</span>
                    <span>{{ activeCanvasSize }}</span>
                  </div>
                </div>
                <div v-else class="relative flex max-h-[420px] max-w-full items-center justify-center overflow-hidden rounded-xl bg-white shadow-sm dark:bg-dark-800">
                  <img :src="currentPreviewURL" :alt="currentJob ? jobDisplayName(currentJob) : 'generated image'" class="max-h-[420px] max-w-full object-contain" />
                  <div class="absolute inset-x-3 bottom-3 flex items-center justify-between rounded-lg bg-black/65 px-3 py-2 text-xs text-white">
                    <span>{{ currentJob?.status === 'completed' ? t('imageWorkbench.preview.ready') : statusText(currentJob?.status || 'in_progress') }}</span>
                    <span>{{ currentJob?.actual_size || currentJob?.requested_size || '—' }}</span>
                  </div>
                </div>
              </div>

              <aside class="rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/60">
                <div class="mb-3 flex items-center justify-between">
                  <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.preview.batchTitle') }}</h3>
                  <span class="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-semibold text-blue-700 dark:bg-blue-950/50 dark:text-blue-300">{{ jobs.length }}</span>
                </div>
                <div v-if="loadingJobs && jobs.length === 0" class="py-8 text-center text-sm text-gray-400">{{ t('common.loading') }}</div>
                <div v-else-if="jobs.length === 0" class="rounded-lg border border-dashed border-gray-300 bg-white p-6 text-center text-sm text-gray-400 dark:border-dark-600 dark:bg-dark-800">
                  {{ t('imageWorkbench.jobs.empty') }}
                </div>
                <div v-else class="max-h-[420px] space-y-2 overflow-y-auto pr-1">
                  <article
                    v-for="job in jobs"
                    :key="job.id"
                    class="overflow-hidden rounded-lg border transition"
                    :class="job.id === selectedJobId ? 'border-blue-400 bg-blue-50 dark:border-blue-700 dark:bg-blue-950/30' : 'border-gray-200 bg-white hover:border-blue-300 dark:border-dark-700 dark:bg-dark-800'"
                  >
                    <button class="block w-full p-3 text-left" type="button" @click="selectJob(job)">
                      <div class="flex items-start justify-between gap-2">
                        <span class="truncate text-xs font-medium text-gray-800 dark:text-gray-200">{{ jobDisplayName(job) }}</span>
                        <span :class="statusClass(job.status)" class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium">{{ statusText(job.status) }}</span>
                      </div>
                      <p class="mt-2 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">{{ job.actual_size || job.requested_size || job.id }}</p>
                      <p class="mt-2 text-[10px] text-gray-400">{{ formatDate(job.created_at) }}</p>
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
                    <div class="flex justify-end border-t border-gray-100 px-3 py-2 dark:border-dark-700">
                      <button
                        class="inline-flex items-center gap-1 rounded-md border border-red-200 bg-red-50 px-2 py-1 text-[11px] font-semibold text-red-600 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-not-allowed disabled:border-gray-200 disabled:bg-gray-50 disabled:text-gray-400 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300 dark:hover:bg-red-950/50 dark:disabled:border-dark-700 dark:disabled:bg-dark-900 dark:disabled:text-gray-600"
                        type="button"
                        :disabled="!canDeleteJob(job) || deletingJobId === job.id"
                        :title="canDeleteJob(job) ? t('imageWorkbench.actions.delete') : t('imageWorkbench.actions.deleteUnavailable')"
                        :data-testid="`delete-batch-${job.id}`"
                        @click="deleteJob(job)"
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
            </div>
          </div>

          <div class="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-3 flex items-center justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.library.title') }}</h2>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.library.subtitle') }}</p>
              </div>
              <span class="text-sm text-gray-400">{{ completedJobs.length }}</span>
            </div>
            <div v-if="completedJobs.length === 0" class="rounded-xl border border-dashed border-gray-300 p-8 text-center text-sm text-gray-400 dark:border-dark-600">
              {{ t('imageWorkbench.library.empty') }}
            </div>
            <div v-else class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <article v-for="job in completedJobs" :key="job.id" class="group overflow-hidden rounded-xl border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-900">
                <button class="block w-full text-left" type="button" :aria-label="t('imageWorkbench.actions.previewArtwork', { name: jobDisplayName(job) })" @click="previewArtwork(job)">
                  <div class="aspect-square overflow-hidden bg-gray-100 dark:bg-dark-800">
                    <img v-if="previewURLs[job.id]" :src="previewURLs[job.id]" :alt="jobDisplayName(job)" class="h-full w-full object-cover transition group-hover:scale-105" />
                    <span v-else class="flex h-full items-center justify-center text-xs text-gray-400">{{ t('imageWorkbench.actions.loadPreview') }}</span>
                  </div>
                </button>
                <div class="p-3">
                  <form v-if="renamingJobId === job.id" class="space-y-2" @submit.prevent="saveJobName(job)">
                    <input v-model="renameDraft" class="input" maxlength="80" :aria-label="t('imageWorkbench.library.nameLabel')" :placeholder="t('imageWorkbench.library.namePlaceholder')" data-testid="work-name-input" />
                    <div class="flex gap-2">
                      <button class="btn btn-primary btn-sm flex-1" type="submit" :disabled="savingJobNameId === job.id || !renameDraft.trim()">{{ t('imageWorkbench.actions.saveName') }}</button>
                      <button class="btn btn-secondary btn-sm flex-1" type="button" :disabled="savingJobNameId === job.id" @click="cancelRename">{{ t('imageWorkbench.actions.cancel') }}</button>
                    </div>
                  </form>
                  <template v-else>
                    <button class="block w-full truncate text-left text-xs font-semibold text-gray-800 hover:text-blue-600 dark:text-gray-200 dark:hover:text-blue-300" type="button" @click="previewArtwork(job)">{{ jobDisplayName(job) }}</button>
                    <div class="mt-2 flex items-end justify-between gap-2">
                      <p class="min-w-0 truncate text-[10px] text-gray-400">{{ job.actual_size || job.requested_size }}</p>
                      <div class="flex shrink-0 items-center gap-1 text-[10px] font-semibold" :data-testid="`library-metadata-${job.id}`">
                        <span class="rounded-full bg-violet-50 px-2 py-0.5 text-violet-600 dark:bg-violet-950/40 dark:text-violet-300">{{ t('imageWorkbench.metadata.resolution', { value: jobResolution(job) }) }}</span>
                        <span class="rounded-full bg-amber-50 px-2 py-0.5 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300">{{ t('imageWorkbench.metadata.quality', { value: jobQuality(job) }) }}</span>
                      </div>
                    </div>
                    <div class="mt-3 grid grid-cols-2 gap-2 border-t border-gray-200 pt-3 dark:border-dark-700">
                      <button class="inline-flex items-center justify-center gap-1.5 rounded-lg border border-blue-200 bg-blue-50 px-2 py-1.5 text-xs font-semibold text-blue-700 transition hover:border-blue-300 hover:bg-blue-100 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-300 dark:hover:bg-blue-950/70" type="button" :data-testid="`rename-work-${job.id}`" @click="startRename(job)">
                        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                          <path stroke-linecap="round" stroke-linejoin="round" d="m16.86 4.49 1.69-1.69a1.88 1.88 0 1 1 2.65 2.65L10.58 16.07a4.5 4.5 0 0 1-1.9 1.13l-2.68.8.8-2.68a4.5 4.5 0 0 1 1.13-1.9L16.86 4.5Zm0 0L19.5 7.13M18 14v4.75A2.25 2.25 0 0 1 15.75 21h-10.5A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
                        </svg>
                        {{ t('imageWorkbench.actions.rename') }}
                      </button>
                      <button class="inline-flex items-center justify-center gap-1.5 rounded-lg border border-red-200 bg-red-50 px-2 py-1.5 text-xs font-semibold text-red-600 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-wait disabled:opacity-60 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300 dark:hover:bg-red-950/50" type="button" :disabled="deletingJobId === job.id" :data-testid="`delete-library-${job.id}`" @click="deleteJob(job)">
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
        </section>
      </div>
    </div>

    <div v-if="previewingJob" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 p-4 backdrop-blur-sm" data-testid="artwork-preview" @click.self="closeArtworkPreview">
      <section class="flex max-h-[94vh] w-full max-w-6xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl dark:bg-dark-800" role="dialog" aria-modal="true" :aria-label="jobDisplayName(previewingJob)">
        <header class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700 sm:px-5">
          <div class="min-w-0">
            <h2 class="truncate text-lg font-semibold text-gray-900 dark:text-white">{{ jobDisplayName(previewingJob) }}</h2>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ previewingJob.actual_size || previewingJob.requested_size }}</p>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <button class="btn btn-secondary btn-sm" type="button" @click="downloadResult(previewingJob)">{{ t('imageWorkbench.actions.downloadCurrent') }}</button>
            <button class="btn btn-secondary btn-sm" type="button" :aria-label="t('imageWorkbench.actions.closePreview')" @click="closeArtworkPreview">×</button>
          </div>
        </header>
        <div class="flex min-h-0 flex-1 items-center justify-center overflow-auto bg-[linear-gradient(135deg,#f8fafc_25%,#eef2f7_25%,#eef2f7_50%,#f8fafc_50%,#f8fafc_75%,#eef2f7_75%)] bg-[length:28px_28px] p-4 dark:bg-dark-900 sm:p-6">
          <img v-if="previewURLs[previewingJob.id]" :src="previewURLs[previewingJob.id]" :alt="jobDisplayName(previewingJob)" class="max-h-[78vh] max-w-full rounded-xl object-contain shadow-lg" />
          <p v-else class="text-sm text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.actions.loadPreview') }}</p>
        </div>
      </section>
    </div>

    <div v-if="editor.open" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 p-4" @keydown.esc="closeEditor">
      <section class="flex max-h-[94vh] w-full max-w-[1500px] flex-col overflow-hidden rounded-2xl bg-white shadow-2xl dark:bg-dark-800">
        <header class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.editor.title') }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.editor.subtitle') }}</p>
          </div>
          <button class="btn btn-secondary btn-sm" aria-label="Close" @click="closeEditor">×</button>
        </header>

        <div class="grid min-h-0 flex-1 gap-4 overflow-auto p-4 lg:grid-cols-[minmax(0,1fr)_380px]">
          <div class="flex min-h-[420px] items-center justify-center rounded-xl bg-gray-100 p-4 dark:bg-dark-900">
            <div class="relative w-full max-w-[900px] overflow-hidden rounded-lg bg-white shadow-sm dark:bg-dark-800" :style="editorStageStyle">
              <img ref="editorImageRef" :src="editor.originalDataURL" alt="editing image" class="absolute inset-0 h-full w-full object-contain" @load="setupEditorCanvas" />
              <canvas
                ref="editorCanvasRef"
                class="absolute inset-0 h-full w-full touch-none"
                @pointerdown.prevent="startDrawing"
                @pointermove.prevent="draw"
                @pointerup.prevent="stopDrawing"
                @pointerleave="stopDrawing"
                @pointercancel="stopDrawing"
              />
            </div>
          </div>

          <aside class="flex flex-col rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/60">
            <div class="grid grid-cols-2 gap-2">
              <button class="btn btn-primary" :class="{ 'ring-2 ring-blue-300': editor.mode === 'brush' }" @click="editor.mode = 'brush'">{{ t('imageWorkbench.editor.brush') }}</button>
              <button class="btn btn-secondary" @click="clearEditorMarks">{{ t('imageWorkbench.editor.clear') }}</button>
            </div>
            <label class="mt-4 text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.editor.brushSize') }} · {{ editor.brushSize }}px</label>
            <input v-model.number="editor.brushSize" class="mt-2 w-full accent-blue-500" type="range" min="8" max="160" step="4" />
            <label class="mt-4 text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.editor.prompt') }}</label>
            <textarea v-model="editor.prompt" class="input mt-2 min-h-44 resize-y bg-white dark:bg-dark-800" :placeholder="t('imageWorkbench.editor.promptPlaceholder')" maxlength="12000" />
            <p class="mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.editor.hint') }}</p>
            <div class="mt-auto space-y-2 pt-5">
              <button class="btn btn-secondary w-full" :disabled="!editor.originalDataURL" @click="setCurrentAsReference">{{ t('imageWorkbench.editor.setReference') }}</button>
              <button class="btn btn-primary w-full" :disabled="editingSubmitting || !editor.prompt.trim() || !editor.hasMarks" @click="submitEditFromEditor">
                {{ editingSubmitting ? t('imageWorkbench.actions.submitting') : t('imageWorkbench.editor.submit') }}
              </button>
            </div>
          </aside>
        </div>
      </section>
    </div>
    <canvas ref="editorMaskCanvasRef" class="hidden" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { imageWorkbenchAPI, keysAPI } from '@/api'
import type { ImageWorkbenchCostEstimate, ImageWorkbenchJob, ImageWorkbenchModel, ImageWorkbenchQuality, ImageWorkbenchStatus } from '@/api'
import type { ApiKey } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { validateReferenceFiles } from './imageWorkbenchValidation'
import {
  IMAGE_DIMENSION_MAX_EDGE,
  IMAGE_DIMENSION_MAX_PIXELS,
  IMAGE_DIMENSION_MIN_EDGE,
  IMAGE_DIMENSION_MIN_PIXELS,
  IMAGE_DIMENSION_STEP,
  isExperimentalImageDimensions,
  validateImageDimensions
} from '@/utils/imageWorkbenchDimensions'
import {
  deleteCachedImageWorkbenchEntry,
  getCachedImageWorkbenchBlob,
  listCachedImageWorkbenchEntries,
  putCachedImageWorkbenchBlob
} from '@/utils/imageWorkbenchCache'
import type { ImageWorkbenchDraft, ImageWorkbenchDraftReference } from '@/utils/imageWorkbenchDraft'
import {
  clearImageWorkbenchDraft,
  loadImageWorkbenchDraft,
  saveImageWorkbenchDraft
} from '@/utils/imageWorkbenchDraft'

const { t, locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const workbenchAnnouncements = computed(() => appStore.cachedPublicSettings?.image_workbench_announcements ?? [])
const announcementIntervalSeconds = computed(() => {
  const configured = Number(appStore.cachedPublicSettings?.image_workbench_announcement_interval_seconds)
  return Math.min(3600, Math.max(1, Number.isFinite(configured) && configured > 0 ? Math.floor(configured) : 5))
})
const announcementIndex = ref(0)
const announcementTrackStyle = computed(() => ({
  transform: `translateY(-${announcementIndex.value * 1.25}rem)`
}))

const models: Array<{ value: ImageWorkbenchModel; label: string; defaultSize: string }> = [
  { value: 'gpt-image-2-1k', label: '1K · gpt-image-2-1k', defaultSize: '1024x1024' },
  { value: 'gpt-image-2-2k', label: '2K · gpt-image-2-2k', defaultSize: '2048x2048' },
  { value: 'gpt-image-2-4k', label: '4K · gpt-image-2-4k', defaultSize: '3840x2160' }
]
const qualities: ImageWorkbenchQuality[] = ['auto', 'low', 'medium', 'high']
const aspectRatios = ['', '1:1', '3:4', '4:3', '3:2', '2:3', '9:16', '16:9', '4:7']
const modelMaxPixels: Record<ImageWorkbenchModel, number> = {
  'gpt-image-2-1k': 1_048_576,
  'gpt-image-2-2k': 4_194_304,
  'gpt-image-2-4k': IMAGE_DIMENSION_MAX_PIXELS
}
const EDITOR_REFERENCE_MAX_BYTES = 10 * 1024 * 1024

const form = reactive({
  apiKeyId: 0,
  model: 'gpt-image-2-1k' as ImageWorkbenchModel,
  quality: 'auto' as ImageWorkbenchQuality,
  size: '1024x1024',
  aspectRatio: '',
  width: 1024,
  height: 1024,
  prompt: '',
  referenceUrls: ''
})

const editor = reactive({
  open: false,
  job: null as ImageWorkbenchJob | null,
  originalDataURL: '',
  width: 1,
  height: 1,
  prompt: '',
  brushSize: 64,
  mode: 'brush' as const,
  drawing: false,
  hasMarks: false
})

const apiKeys = ref<ApiKey[]>([])
const jobs = ref<ImageWorkbenchJob[]>([])
const selectedJobId = ref('')
const blankCanvasOpen = ref(true)
const renamingJobId = ref('')
const renameDraft = ref('')
const savingJobNameId = ref('')
const deletingJobId = ref('')
const previewingJobId = ref('')
const loadingKeys = ref(false)
const loadingJobs = ref(false)
const submitting = ref(false)
const downloadingAll = ref(false)
const editingSubmitting = ref(false)
const loadingEstimate = ref(false)
const costEstimate = ref<ImageWorkbenchCostEstimate | null>(null)
const referenceFiles = ref<File[]>([])
const referenceDataURLs = ref<string[]>([])
const referenceUrlInput = ref('')
const isReferenceDragOver = ref(false)
const previewURLs = reactive<Record<string, string>>({})
const previewDataURLs = reactive<Record<string, string>>({})
const previewBlobs = new Map<string, { version: string; blob: Blob }>()
const pendingBlobLoads = new Map<string, Promise<Blob>>()
const referenceInputRef = ref<HTMLInputElement | null>(null)
const editorCanvasRef = ref<HTMLCanvasElement | null>(null)
const editorMaskCanvasRef = ref<HTMLCanvasElement | null>(null)
const editorImageRef = ref<HTMLImageElement | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null
let elapsedTimer: ReturnType<typeof setInterval> | null = null
let announcementTimer: ReturnType<typeof setInterval> | null = null
let draftSaveTimer: ReturnType<typeof setTimeout> | null = null
let draftReady = false
const elapsedClock = ref(Date.now())
type BrushPoint = { x: number; y: number }
type BrushStroke = { points: BrushPoint[]; size: number }
const brushStrokes: BrushStroke[] = []

const selectedModel = computed(() => models.find(item => item.value === form.model) || models[0]!)
const imageCacheUserId = computed(() => Number(authStore.user?.id || 0))
const eligibleKeys = computed(() => apiKeys.value.filter(key => key.status === 'active' && key.group?.platform === 'openai' && key.group.allow_image_generation))
const currentJob = computed(() => jobs.value.find(job => job.id === selectedJobId.value) || null)
const previewingJob = computed(() => jobs.value.find(job => job.id === previewingJobId.value) || null)
const currentPreviewURL = computed(() => selectedJobId.value ? previewURLs[selectedJobId.value] : '')
const headerModel = computed(() => selectedModel.value.value.replace(/-\d+k$/i, ''))
const headerCanvasSize = computed(() => `${form.width} × ${form.height}`)
const pendingJobs = computed(() => jobs.value.filter(job => job.status === 'queued' || job.status === 'in_progress'))
const queueSummary = computed(() => t('imageWorkbench.header.queueValue', { count: pendingJobs.value.length }))
const interfaceConnected = computed(() => form.apiKeyId > 0 && eligibleKeys.value.some(key => key.id === form.apiKeyId))
const interfaceStatus = computed(() => interfaceConnected.value ? t('imageWorkbench.header.connected') : t('imageWorkbench.header.disconnected'))
const blankCanvasSize = computed(() => `${form.width}x${form.height}`)
const activeCanvasSize = computed(() => {
  if (!blankCanvasOpen.value && currentJob.value) return currentJob.value.actual_size || currentJob.value.requested_size || blankCanvasSize.value
  return blankCanvasSize.value
})
const activeCanvasStyle = computed(() => {
  const [width, height] = parseSize(activeCanvasSize.value)
  return {
    aspectRatio: `${width} / ${height}`,
    width: `min(100%, ${Math.round((420 * width) / height)}px)`
  }
})
const activeCanvasTitle = computed(() => !blankCanvasOpen.value && currentJob.value ? jobDisplayName(currentJob.value) : t('imageWorkbench.preview.blankCanvasTitle'))
const activeCanvasHint = computed(() => !blankCanvasOpen.value && currentJob.value ? t('imageWorkbench.preview.taskCanvasHint') : t('imageWorkbench.preview.blankCanvasHint'))
const activeCanvasStatus = computed(() => !blankCanvasOpen.value && currentJob.value ? statusText(currentJob.value.status) : t('imageWorkbench.preview.blankCanvasStatus'))
const completedJobs = computed(() => jobs.value.filter(job => job.status === 'completed'))
const referenceUrlList = computed(() => form.referenceUrls.split(/\r?\n/).map(value => value.trim()).filter(Boolean))
const referenceCount = computed(() => referenceUrlList.value.length + referenceFiles.value.length + referenceDataURLs.value.length)
const selectedModelMaxPixels = computed(() => modelMaxPixels[form.model])
const dimensionValidation = computed(() => validateImageDimensions(Number(form.width), Number(form.height), selectedModelMaxPixels.value))
const dimensionErrorMessage = computed(() => {
  const code = dimensionValidation.value.code
  if (!code) return ''
  const keys: Record<string, string> = {
    not_positive_integer: 'imageWorkbench.form.dimensionErrors.positiveInteger',
    not_multiple_of_16: 'imageWorkbench.form.dimensionErrors.multipleOf16',
    edge_too_large: 'imageWorkbench.form.dimensionErrors.maxEdge',
    aspect_ratio_too_wide: 'imageWorkbench.form.dimensionErrors.aspectRatio',
    pixels_too_few: 'imageWorkbench.form.dimensionErrors.minPixels',
    pixels_too_many: 'imageWorkbench.form.dimensionErrors.maxPixels'
  }
  return t(keys[code] || 'imageWorkbench.form.dimensionErrors.invalid', {
    step: IMAGE_DIMENSION_STEP,
    minEdge: IMAGE_DIMENSION_MIN_EDGE,
    maxEdge: IMAGE_DIMENSION_MAX_EDGE,
    minPixels: formatInteger(IMAGE_DIMENSION_MIN_PIXELS),
    maxPixels: formatInteger(selectedModelMaxPixels.value)
  })
})
const experimentalDimensions = computed(() => dimensionValidation.value.code === null && isExperimentalImageDimensions(form.width, form.height))
const canSubmit = computed(() => form.apiKeyId > 0 && form.prompt.trim().length > 0 && eligibleKeys.value.some(key => key.id === form.apiKeyId) && dimensionValidation.value.code === null)
const editorStageStyle = computed(() => ({ aspectRatio: editor.width + ' / ' + editor.height }))

function currentDraft(): ImageWorkbenchDraft {
  return {
    form: {
      apiKeyId: Number(form.apiKeyId) || 0,
      model: form.model,
      quality: form.quality,
      size: form.size,
      aspectRatio: form.aspectRatio,
      width: Number(form.width) || 1024,
      height: Number(form.height) || 1024,
      prompt: form.prompt,
      referenceUrls: form.referenceUrls
    },
    referenceUrlInput: referenceUrlInput.value,
    references: referenceDataURLs.value.map((dataURL, index) => {
      const file = referenceFiles.value[index]
      return {
        name: file?.name || `reference-${index + 1}.png`,
        type: file?.type || dataURL.match(/^data:([^;,]+)/i)?.[1] || 'image/png',
        lastModified: file?.lastModified || Date.now(),
        dataURL,
        isFile: Boolean(file)
      }
    })
  }
}

function draftReferenceToFile(reference: ImageWorkbenchDraftReference): File | null {
  if (!reference.dataURL || !reference.isFile) return null
  const separator = reference.dataURL.indexOf(',')
  if (separator < 0) return null
  try {
    const payload = reference.dataURL.slice(separator + 1)
    const binary = atob(payload)
    const bytes = new Uint8Array(binary.length)
    for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index)
    return new File([bytes], reference.name, {
      type: reference.type || 'image/png',
      lastModified: reference.lastModified || Date.now()
    })
  } catch {
    return null
  }
}

async function restoreDraft() {
  const userId = imageCacheUserId.value
  if (userId <= 0) {
    draftReady = true
    return
  }
  try {
    const draft = await loadImageWorkbenchDraft(userId)
    if (draft) {
      const savedForm = draft.form || {} as ImageWorkbenchDraft['form']
      form.apiKeyId = Number(savedForm.apiKeyId) || 0
      form.model = models.some(model => model.value === savedForm.model) ? savedForm.model as ImageWorkbenchModel : 'gpt-image-2-1k'
      form.quality = qualities.includes(savedForm.quality as ImageWorkbenchQuality) ? savedForm.quality as ImageWorkbenchQuality : 'auto'
      form.aspectRatio = aspectRatios.includes(savedForm.aspectRatio) ? savedForm.aspectRatio : ''
      await nextTick()
      form.width = clampDimension(Number(savedForm.width) || 1024)
      form.height = clampDimension(Number(savedForm.height) || 1024)
      form.size = `${form.width}x${form.height}`
      form.prompt = String(savedForm.prompt || '').slice(0, 12000)
      form.referenceUrls = String(savedForm.referenceUrls || '')
      referenceUrlInput.value = String(draft.referenceUrlInput || '')

      const references = (draft.references || []).slice(0, 9).filter(reference => Boolean(reference.dataURL))
      referenceDataURLs.value = references.map(reference => reference.dataURL)
      referenceFiles.value = references
        .map(draftReferenceToFile)
        .filter((file): file is File => file !== null)
    }
  } catch {
    // Draft persistence is optional; a malformed or unavailable draft must not block the workbench.
  } finally {
    draftReady = true
  }
}

function persistDraftNow() {
  if (!draftReady) return
  const userId = imageCacheUserId.value
  if (userId > 0) void saveImageWorkbenchDraft(userId, currentDraft()).catch(() => undefined)
}

function scheduleDraftSave() {
  if (!draftReady) return
  if (draftSaveTimer) clearTimeout(draftSaveTimer)
  draftSaveTimer = setTimeout(() => {
    draftSaveTimer = null
    persistDraftNow()
  }, 250)
}

watch(() => form.model, () => {
  const [width, height] = parseSize(selectedModel.value.defaultSize)
  form.width = width
  form.height = height
  form.size = selectedModel.value.defaultSize
  form.aspectRatio = ''
})
watch([form, referenceFiles, referenceDataURLs, referenceUrlInput], scheduleDraftSave, { deep: true })
watch([() => form.width, () => form.height], () => {
  form.size = clampDimension(Number(form.width)) + 'x' + clampDimension(Number(form.height))
})
watch([() => form.apiKeyId, () => form.model, () => interfaceConnected.value], () => { void loadCostEstimate() }, { immediate: true })

async function loadKeys() {
  loadingKeys.value = true
  try {
    const response = await keysAPI.list(1, 100, { status: 'active', sort_by: 'created_at', sort_order: 'desc' })
    apiKeys.value = response.items || []
    if (!eligibleKeys.value.some(key => key.id === form.apiKeyId)) form.apiKeyId = eligibleKeys.value[0]?.id || 0
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.loadKeys'))
  } finally {
    loadingKeys.value = false
  }
}

async function loadCostEstimate() {
  const apiKeyId = form.apiKeyId
  const model = form.model
  if (apiKeyId <= 0 || !eligibleKeys.value.some(key => key.id === apiKeyId)) {
    costEstimate.value = null
    loadingEstimate.value = false
    return
  }
  loadingEstimate.value = true
  try {
    const estimate = await imageWorkbenchAPI.estimateCost({ api_key_id: apiKeyId, model })
    if (form.apiKeyId === apiKeyId && form.model === model) costEstimate.value = estimate
  } catch {
    if (form.apiKeyId === apiKeyId && form.model === model) costEstimate.value = null
  } finally {
    if (form.apiKeyId === apiKeyId && form.model === model) loadingEstimate.value = false
  }
}

function jobImageVersion(job: ImageWorkbenchJob) {
  return job.content_url || job.updated_at || job.id
}

function hasCurrentPreview(job: ImageWorkbenchJob) {
  return previewBlobs.get(job.id)?.version === jobImageVersion(job) && Boolean(previewURLs[job.id])
}

function releasePreview(jobId: string) {
  const url = previewURLs[jobId]
  if (url) URL.revokeObjectURL(url)
  delete previewURLs[jobId]
  delete previewDataURLs[jobId]
  previewBlobs.delete(jobId)
}

function setPreviewBlob(job: ImageWorkbenchJob, blob: Blob) {
  const version = jobImageVersion(job)
  const existing = previewBlobs.get(job.id)
  if (existing?.version === version && previewURLs[job.id]) return
  if (existing?.version !== version) delete previewDataURLs[job.id]
  if (previewURLs[job.id]) URL.revokeObjectURL(previewURLs[job.id])
  previewBlobs.set(job.id, { version, blob })
  previewURLs[job.id] = URL.createObjectURL(blob)
}

async function restoreCachedLibrary() {
  const userId = imageCacheUserId.value
  if (userId <= 0) return
  try {
    const entries = (await listCachedImageWorkbenchEntries(userId)).slice(0, 30)
    if (!entries.length) return
    jobs.value = entries.map(entry => entry.job)
    entries.forEach(entry => setPreviewBlob(entry.job, entry.blob))
    const preferred = jobs.value.find(job => job.status === 'completed') || jobs.value[0]
    if (preferred) selectedJobId.value = preferred.id
  } catch {
    // IndexedDB is an optional acceleration layer; the server remains the source of truth.
  }
}

async function hydrateCachedPreviews(imageJobs: ImageWorkbenchJob[]) {
  const userId = imageCacheUserId.value
  if (userId <= 0) return
  await Promise.all(imageJobs.map(async job => {
    if (hasCurrentPreview(job)) return
    try {
      const blob = await getCachedImageWorkbenchBlob(userId, job)
      if (blob) setPreviewBlob(job, blob)
    } catch {
      // Cache read failures fall back to the authenticated content endpoint on demand.
    }
  }))
}

async function getImageBlob(job: ImageWorkbenchJob): Promise<Blob> {
  const version = jobImageVersion(job)
  const memoryEntry = previewBlobs.get(job.id)
  if (memoryEntry?.version === version) return memoryEntry.blob
  if (memoryEntry) releasePreview(job.id)

  const userId = imageCacheUserId.value
  const loadKey = `${userId}:${job.id}:${version}`
  const pending = pendingBlobLoads.get(loadKey)
  if (pending) return pending

  const load = (async () => {
    let blob: Blob | null = null
    if (userId > 0) {
      try {
        blob = await getCachedImageWorkbenchBlob(userId, job)
      } catch {
        blob = null
      }
    }
    if (!blob) {
      blob = await imageWorkbenchAPI.getContent(job.id)
      if (userId > 0) {
        try {
          await putCachedImageWorkbenchBlob(userId, job, blob)
        } catch {
          // Quota, privacy mode, and storage errors must not prevent normal previews/downloads.
        }
      }
    }
    previewBlobs.set(job.id, { version, blob })
    return blob
  })()

  pendingBlobLoads.set(loadKey, load)
  try {
    return await load
  } finally {
    pendingBlobLoads.delete(loadKey)
  }
}

async function loadJobs() {
  loadingJobs.value = true
  try {
    const response = await imageWorkbenchAPI.listJobs(30, 0)
    const previousSelectedJobId = selectedJobId.value
    jobs.value = response.data || []
    const currentJobIds = new Set(jobs.value.map(job => job.id))
    Object.keys(previewURLs).forEach(jobId => {
      if (!currentJobIds.has(jobId)) releasePreview(jobId)
    })
    await hydrateCachedPreviews(jobs.value.filter(job => job.status === 'completed'))
    const preferred = jobs.value.find(job => job.id === previousSelectedJobId)
      || jobs.value.find(job => job.status === 'completed')
      || jobs.value[0]
    if (preferred) await selectJob(preferred)
    else {
      selectedJobId.value = ''
      blankCanvasOpen.value = true
    }
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.loadJobs'))
  } finally {
    loadingJobs.value = false
  }
}

async function submitJob() {
  normalizeDimensionValues(false)
  if (dimensionValidation.value.code) {
    appStore.showError(dimensionErrorMessage.value)
    return
  }
  if (!canSubmit.value || submitting.value) return
  submitting.value = true
  blankCanvasOpen.value = false
  try {
    const urls = form.referenceUrls.split(/\r?\n/).map(value => value.trim()).filter(Boolean)
    const images = [...urls, ...referenceDataURLs.value]
    const operation = images.length > 0 ? 'edit' : 'generation'
    const job = await imageWorkbenchAPI.createJob({
      api_key_id: form.apiKeyId,
      operation,
      model: form.model,
      prompt: form.prompt.trim(),
      size: form.size || selectedModel.value.defaultSize,
      aspect_ratio: undefined,
      quality: form.quality,
      images: images.length ? images : undefined
    })
    jobs.value = [job, ...jobs.value.filter(item => item.id !== job.id)]
    selectedJobId.value = job.id
    form.prompt = ''
    form.referenceUrls = ''
    referenceUrlInput.value = ''
    referenceFiles.value = []
    referenceDataURLs.value = []
    if (referenceInputRef.value) referenceInputRef.value.value = ''
    void clearImageWorkbenchDraft(imageCacheUserId.value)
    appStore.showSuccess(t('imageWorkbench.messages.submitted'))
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.submit'))
  } finally {
    submitting.value = false
  }
}

function createBlankCanvas() {
  if (dimensionValidation.value.code !== null) return
  blankCanvasOpen.value = true
  selectedJobId.value = ''
}

function parseSize(value: string): [number, number] {
  const match = value.trim().match(/^(\d+)x(\d+)$/i)
  if (!match) return [1024, 1024]
  return [Number(match[1]), Number(match[2])]
}

function clampDimension(value: number) {
  const numeric = Number.isFinite(value) ? Math.round(value / IMAGE_DIMENSION_STEP) * IMAGE_DIMENSION_STEP : 1024
  return Math.min(IMAGE_DIMENSION_MAX_EDGE, Math.max(IMAGE_DIMENSION_MIN_EDGE, numeric))
}

function snapDimension(value: number, direction: 'up' | 'down' | 'nearest' = 'nearest') {
  const raw = Number.isFinite(value) ? value / IMAGE_DIMENSION_STEP : 1024 / IMAGE_DIMENSION_STEP
  const snapped = direction === 'up' ? Math.ceil(raw) : direction === 'down' ? Math.floor(raw) : Math.round(raw)
  return snapped * IMAGE_DIMENSION_STEP
}

function updateFormSize() {
  form.size = form.width + 'x' + form.height
}

function normalizeDimensionValues(clearAspectRatio: boolean) {
  form.width = clampDimension(Number(form.width))
  form.height = clampDimension(Number(form.height))
  if (clearAspectRatio) form.aspectRatio = ''
  updateFormSize()
}

function normalizeDimensions() {
  normalizeDimensionValues(true)
}

function resetForm() {
  form.apiKeyId = eligibleKeys.value[0]?.id || 0
  form.model = 'gpt-image-2-1k'
  form.quality = 'auto'
  form.size = '1024x1024'
  form.aspectRatio = ''
  form.width = 1024
  form.height = 1024
  form.prompt = ''
  clearReferences()
  blankCanvasOpen.value = false
  selectedJobId.value = ''
  void clearImageWorkbenchDraft(imageCacheUserId.value)
}

function selectAspectRatio(value: string) {
  form.aspectRatio = value
  if (!value) return
  const [widthRatio, heightRatio] = value.split(':').map(Number)
  if (!widthRatio || !heightRatio) return
  let width = clampDimension(Number(form.width))
  let height = snapDimension(width * heightRatio / widthRatio)
  if (height > IMAGE_DIMENSION_MAX_EDGE) {
    width = clampDimension(snapDimension(width * IMAGE_DIMENSION_MAX_EDGE / height, 'down'))
    height = snapDimension(width * heightRatio / widthRatio)
  }
  if (height < IMAGE_DIMENSION_MIN_EDGE) {
    width = clampDimension(snapDimension(width * IMAGE_DIMENSION_MIN_EDGE / Math.max(height, 1), 'up'))
    height = snapDimension(width * heightRatio / widthRatio, 'up')
  }
  for (let index = 0; index < 4; index += 1) {
    const pixels = width * height
    if (pixels < IMAGE_DIMENSION_MIN_PIXELS) {
      const factor = Math.sqrt(IMAGE_DIMENSION_MIN_PIXELS / pixels)
      const nextWidth = clampDimension(snapDimension(width * factor, 'up'))
      const nextHeight = clampDimension(snapDimension(height * factor, 'up'))
      if (nextWidth === width && nextHeight === height) break
      width = nextWidth
      height = nextHeight
      continue
    }
    if (pixels > selectedModelMaxPixels.value) {
      const factor = Math.sqrt(selectedModelMaxPixels.value / pixels)
      const nextWidth = clampDimension(snapDimension(width * factor, 'down'))
      const nextHeight = clampDimension(snapDimension(height * factor, 'down'))
      if (nextWidth === width && nextHeight === height) break
      width = nextWidth
      height = nextHeight
      continue
    }
    break
  }
  form.width = clampDimension(width)
  form.height = clampDimension(height)
  updateFormSize()
}

async function onReferenceFiles(event: Event) {
  const input = event.target as HTMLInputElement
  await processReferenceFiles(Array.from(input.files || []))
  input.value = ''
}

async function onReferenceDrop(event: DragEvent) {
  isReferenceDragOver.value = false
  await processReferenceFiles(Array.from(event.dataTransfer?.files || []))
}

async function processReferenceFiles(files: File[]) {
  if (!files.length) return
  if (referenceCount.value + files.length > 9) {
    appStore.showError(t('imageWorkbench.errors.tooManyReferences'))
    return
  }
  const validationError = validateReferenceFiles(files)
  if (validationError) {
    const errorKey = {
      too_many_reference_files: 'imageWorkbench.errors.tooManyReferenceFiles',
      reference_file_not_image: 'imageWorkbench.errors.invalidReferenceType',
      reference_file_too_large: 'imageWorkbench.errors.fileTooLarge',
      mask_file_not_png: 'imageWorkbench.errors.invalidMask',
      mask_file_too_large: 'imageWorkbench.errors.fileTooLarge'
    } as const
    appStore.showError(t(errorKey[validationError]))
    return
  }
  referenceFiles.value = [...referenceFiles.value, ...files]
  referenceDataURLs.value = [...referenceDataURLs.value, ...await Promise.all(files.map(readFileAsDataURL))]
  persistDraftNow()
}

function importReferenceUrl() {
  const url = referenceUrlInput.value.trim()
  if (!url) return
  if (!/^https?:\/\//i.test(url)) {
    appStore.showError(t('imageWorkbench.errors.invalidReferenceUrl'))
    return
  }
  if (referenceCount.value >= 9) {
    appStore.showError(t('imageWorkbench.errors.tooManyReferences'))
    return
  }
  form.referenceUrls = [...referenceUrlList.value, url].join('\n')
  referenceUrlInput.value = ''
  persistDraftNow()
}

function clearReferences() {
  form.referenceUrls = ''
  referenceUrlInput.value = ''
  referenceFiles.value = []
  referenceDataURLs.value = []
  if (referenceInputRef.value) referenceInputRef.value.value = ''
  persistDraftNow()
}

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}

async function refreshPendingJobs() {
  const pending = jobs.value.filter(job => job.status === 'queued' || job.status === 'in_progress')
  if (!pending.length) return
  await Promise.all(pending.map(async job => {
    try {
      const updated = await imageWorkbenchAPI.getJob(job.id)
      const index = jobs.value.findIndex(item => item.id === job.id)
      if (index >= 0) jobs.value[index] = updated
      if (updated.status === 'completed' && updated.id === selectedJobId.value && !previewURLs[updated.id]) await loadPreview(updated)
    } catch {
      // Transient polling errors are retried on the next interval.
    }
  }))
}

async function selectJob(job: ImageWorkbenchJob) {
  blankCanvasOpen.value = false
  selectedJobId.value = job.id
  if (job.status === 'completed' && !previewURLs[job.id]) await loadPreview(job)
}

async function previewArtwork(job: ImageWorkbenchJob) {
  if (job.status !== 'completed') return
  await selectJob(job)
  previewingJobId.value = job.id
}

function closeArtworkPreview() {
  previewingJobId.value = ''
}

async function loadPreview(job: ImageWorkbenchJob) {
  if (hasCurrentPreview(job)) return
  try {
    const blob = await getImageBlob(job)
    setPreviewBlob(job, blob)
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.preview'))
  }
}

async function downloadCurrent() {
  if (currentJob.value) await downloadResult(currentJob.value)
}

async function downloadAll() {
  if (downloadingAll.value || completedJobs.value.length === 0) return
  downloadingAll.value = true
  const jobsToDownload = [...completedJobs.value]
  try {
    for (const job of jobsToDownload) await downloadResult(job)
    appStore.showSuccess(t('imageWorkbench.messages.downloadAllSuccess', { count: jobsToDownload.length }))
  } finally {
    downloadingAll.value = false
  }
}

async function downloadResult(job: ImageWorkbenchJob) {
  try {
    const blob = await getImageBlob(job)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = downloadFileName(job)
    anchor.click()
    URL.revokeObjectURL(url)
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.download'))
  }
}

async function openEditor(job: ImageWorkbenchJob) {
  if (job.status !== 'completed') return
  if (!previewDataURLs[job.id]) {
    try {
      previewDataURLs[job.id] = await createEditableReferenceDataURL(await getImageBlob(job))
    } catch (error: any) {
      appStore.showError(error?.message || t('imageWorkbench.errors.preview'))
      return
    }
  }
  const dataURL = previewDataURLs[job.id]
  if (!dataURL) return
  editor.open = true
  editor.job = job
  editor.originalDataURL = dataURL
  editor.prompt = ''
  editor.hasMarks = false
  await nextTick()
  if (editorImageRef.value?.complete) setupEditorCanvas()
}

function startRename(job: ImageWorkbenchJob) {
  renamingJobId.value = job.id
  renameDraft.value = jobDisplayName(job)
}

function cancelRename() {
  renamingJobId.value = ''
  renameDraft.value = ''
}

async function saveJobName(job: ImageWorkbenchJob) {
  const name = renameDraft.value.trim()
  if (!name || savingJobNameId.value) return
  savingJobNameId.value = job.id
  try {
    const updated = await imageWorkbenchAPI.renameJob(job.id, name)
    const index = jobs.value.findIndex(item => item.id === job.id)
    if (index >= 0) jobs.value[index] = updated
    const preview = previewBlobs.get(job.id)
    if (preview && imageCacheUserId.value > 0) {
      try {
        await putCachedImageWorkbenchBlob(imageCacheUserId.value, updated, preview.blob)
      } catch {
        // A cache metadata refresh is optional; the server name is authoritative.
      }
    }
    cancelRename()
    appStore.showSuccess(t('imageWorkbench.messages.renamed'))
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.rename'))
  } finally {
    savingJobNameId.value = ''
  }
}

function canDeleteJob(job: ImageWorkbenchJob) {
  return job.status === 'completed' || job.status === 'failed'
}

async function deleteJob(job: ImageWorkbenchJob) {
  if (!canDeleteJob(job) || deletingJobId.value) return
  if (!window.confirm(t('imageWorkbench.actions.deleteConfirm', { name: jobDisplayName(job) }))) return
  deletingJobId.value = job.id
  try {
    await imageWorkbenchAPI.deleteJob(job.id)
    jobs.value = jobs.value.filter(item => item.id !== job.id)
    if (renamingJobId.value === job.id) cancelRename()
    if (previewingJobId.value === job.id) closeArtworkPreview()
    releasePreview(job.id)
    if (imageCacheUserId.value > 0) {
      try {
        await deleteCachedImageWorkbenchEntry(imageCacheUserId.value, job.id)
      } catch {
        // The server deletion is authoritative; a stale optional cache entry must not block the UI.
      }
    }
    if (selectedJobId.value === job.id) {
      const preferred = jobs.value.find(item => item.status === 'completed') || jobs.value[0]
      if (preferred) await selectJob(preferred)
      else {
        selectedJobId.value = ''
        blankCanvasOpen.value = true
      }
    }
    appStore.showSuccess(t('imageWorkbench.messages.deleted'))
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.delete'))
  } finally {
    deletingJobId.value = ''
  }
}

function closeEditor() {
  editor.open = false
  editor.job = null
  clearEditorMarks()
}

function setupEditorCanvas() {
  const image = editorImageRef.value
  const canvas = editorCanvasRef.value
  const maskCanvas = editorMaskCanvasRef.value
  if (!image || !canvas || !maskCanvas || !image.naturalWidth || !image.naturalHeight) return
  editor.width = image.naturalWidth
  editor.height = image.naturalHeight
  canvas.width = image.naturalWidth
  canvas.height = image.naturalHeight
  maskCanvas.width = image.naturalWidth
  maskCanvas.height = image.naturalHeight
  clearEditorMarks()
}

function pointFromEvent(event: PointerEvent) {
  const canvas = editorCanvasRef.value
  if (!canvas) return null
  const rect = canvas.getBoundingClientRect()
  return {
    x: Math.max(0, Math.min(canvas.width, (event.clientX - rect.left) * canvas.width / rect.width)),
    y: Math.max(0, Math.min(canvas.height, (event.clientY - rect.top) * canvas.height / rect.height))
  }
}

function drawSmoothStroke(ctx: CanvasRenderingContext2D, stroke: BrushStroke, color: string) {
  const points = stroke.points
  if (!points.length) return
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  ctx.strokeStyle = color
  ctx.lineWidth = stroke.size
  ctx.beginPath()
  ctx.moveTo(points[0].x, points[0].y)
  if (points.length === 1) {
    ctx.arc(points[0].x, points[0].y, stroke.size / 2, 0, Math.PI * 2)
    ctx.fillStyle = color
    ctx.fill()
    return
  }
  if (points.length === 2) {
    ctx.lineTo(points[1].x, points[1].y)
  } else {
    for (let index = 1; index < points.length - 1; index += 1) {
      const current = points[index]!
      const next = points[index + 1]!
      const midpointX = (current.x + next.x) / 2
      const midpointY = (current.y + next.y) / 2
      ctx.quadraticCurveTo(current.x, current.y, midpointX, midpointY)
    }
    const last = points[points.length - 1]!
    ctx.quadraticCurveTo(last.x, last.y, last.x, last.y)
  }
  ctx.stroke()
}

function redrawBrushStrokes() {
  const canvas = editorCanvasRef.value
  const maskCanvas = editorMaskCanvasRef.value
  if (!canvas || !maskCanvas) return
  const visual = canvas.getContext('2d')
  const mask = maskCanvas.getContext('2d')
  if (!visual || !mask) return
  visual.clearRect(0, 0, canvas.width, canvas.height)
  mask.clearRect(0, 0, maskCanvas.width, maskCanvas.height)
  mask.save()
  mask.globalCompositeOperation = 'source-over'
  mask.fillStyle = '#ffffff'
  mask.fillRect(0, 0, maskCanvas.width, maskCanvas.height)
  mask.globalCompositeOperation = 'destination-out'
  for (const stroke of brushStrokes) {
    drawSmoothStroke(visual, stroke, 'rgba(239, 68, 68, 0.72)')
    drawSmoothStroke(mask, stroke, '#000000')
  }
  mask.restore()
}

function startDrawing(event: PointerEvent) {
  if (editor.mode !== 'brush') return
  const point = pointFromEvent(event)
  if (!point) return
  editor.drawing = true
  brushStrokes.push({ points: [point], size: Math.max(1, Number(editor.brushSize) || 1) })
  editor.hasMarks = true
  redrawBrushStrokes()
  editorCanvasRef.value?.setPointerCapture(event.pointerId)
}

function draw(event: PointerEvent) {
  if (!editor.drawing) return
  const point = pointFromEvent(event)
  if (!point) return
  const stroke = brushStrokes[brushStrokes.length - 1]
  if (!stroke) return
  const previous = stroke.points[stroke.points.length - 1]
  if (!previous || Math.hypot(point.x - previous.x, point.y - previous.y) >= 0.5) {
    stroke.points.push(point)
    redrawBrushStrokes()
  }
}

function stopDrawing() {
  editor.drawing = false
}

function clearEditorMarks() {
  const canvas = editorCanvasRef.value
  const maskCanvas = editorMaskCanvasRef.value
  canvas?.getContext('2d')?.clearRect(0, 0, canvas.width, canvas.height)
  maskCanvas?.getContext('2d')?.clearRect(0, 0, maskCanvas.width, maskCanvas.height)
  brushStrokes.length = 0
  editor.hasMarks = false
  editor.drawing = false
}

function setCurrentAsReference() {
  if (!editor.originalDataURL) return
  referenceFiles.value = []
  referenceDataURLs.value = [editor.originalDataURL]
  form.referenceUrls = ''
  persistDraftNow()
  closeEditor()
  appStore.showSuccess(t('imageWorkbench.editor.referenceAdded'))
}

async function submitEditFromEditor() {
  if (!editor.job || !editor.originalDataURL || !editor.prompt.trim() || !editor.hasMarks || editingSubmitting.value) return
  const maskCanvas = editorMaskCanvasRef.value
  if (!maskCanvas) return
  editingSubmitting.value = true
  try {
    const mask = maskCanvas.toDataURL('image/png')
    if (dataURLByteLength(mask) > EDITOR_REFERENCE_MAX_BYTES) {
      appStore.showError(t('imageWorkbench.errors.fileTooLarge'))
      return
    }
    const job = await imageWorkbenchAPI.createJob({
      api_key_id: form.apiKeyId,
      operation: 'edit',
      model: editor.job.model,
      prompt: editor.prompt.trim(),
      size: editor.job.actual_size || editor.job.requested_size || undefined,
      quality: form.quality,
      images: [editor.originalDataURL],
      mask
    })
    jobs.value = [job, ...jobs.value.filter(item => item.id !== job.id)]
    blankCanvasOpen.value = false
    selectedJobId.value = job.id
    closeEditor()
    appStore.showSuccess(t('imageWorkbench.editor.submitted'))
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.submit'))
  } finally {
    editingSubmitting.value = false
  }
}

async function createEditableReferenceDataURL(blob: Blob) {
  const bitmap = await createImageBitmap(blob)
  let canvas = document.createElement('canvas')
  canvas.width = bitmap.width
  canvas.height = bitmap.height
  const context = canvas.getContext('2d')
  if (!context) {
    bitmap.close()
    throw new Error('Canvas 2D context is unavailable')
  }
  context.drawImage(bitmap, 0, 0)
  bitmap.close()

  let dataURL = canvas.toDataURL('image/png')
  let byteLength = dataURLByteLength(dataURL)
  while (byteLength > EDITOR_REFERENCE_MAX_BYTES) {
    const scale = Math.min(0.9, Math.sqrt(EDITOR_REFERENCE_MAX_BYTES / byteLength) * 0.95)
    const width = Math.max(IMAGE_DIMENSION_MIN_EDGE, Math.floor(canvas.width * scale))
    const height = Math.max(IMAGE_DIMENSION_MIN_EDGE, Math.floor(canvas.height * scale))
    if (width === canvas.width && height === canvas.height) break

    const resized = document.createElement('canvas')
    resized.width = width
    resized.height = height
    const resizedContext = resized.getContext('2d')
    if (!resizedContext) throw new Error('Canvas 2D context is unavailable')
    resizedContext.drawImage(canvas, 0, 0, width, height)
    canvas = resized
    dataURL = canvas.toDataURL('image/png')
    byteLength = dataURLByteLength(dataURL)
  }
  if (byteLength > EDITOR_REFERENCE_MAX_BYTES) throw new Error(t('imageWorkbench.errors.fileTooLarge'))
  return dataURL
}

function dataURLByteLength(dataURL: string) {
  const separator = dataURL.indexOf(',')
  const payload = separator >= 0 ? dataURL.slice(separator + 1) : dataURL
  const padding = payload.endsWith('==') ? 2 : payload.endsWith('=') ? 1 : 0
  return Math.max(0, Math.floor(payload.length * 3 / 4) - padding)
}

function statusText(status: ImageWorkbenchStatus) { return t('imageWorkbench.status.' + status) }
function statusClass(status: ImageWorkbenchStatus) {
  if (status === 'completed') return 'bg-green-100 text-green-700 dark:bg-green-950/40 dark:text-green-300'
  if (status === 'failed' || status === 'submission_unknown') return 'bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-300'
  return 'bg-blue-100 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300'
}
function jobDisplayName(job: ImageWorkbenchJob) { return job.name?.trim() || t('imageWorkbench.library.untitled') }
function jobResolution(job: ImageWorkbenchJob) { return job.model.match(/-(\d+k)$/i)?.[1]?.toUpperCase() || '—' }
function jobQuality(job: ImageWorkbenchJob) {
  const quality = qualities.includes(job.quality as ImageWorkbenchQuality) ? job.quality as ImageWorkbenchQuality : 'auto'
  return t('imageWorkbench.form.qualityOptions.' + quality)
}
function downloadFileName(job: ImageWorkbenchJob) {
  const safeName = [...jobDisplayName(job)]
    .map(character => character.charCodeAt(0) < 32 || /[\\/:*?"<>|]/.test(character) ? '_' : character)
    .join('')
    .trim()
  return `${safeName || job.id}.png`
}
function formatCost(value: number) { return Number(value || 0).toFixed(4) }
function formatInteger(value: number) { return new Intl.NumberFormat(locale.value).format(value) }
function formatDate(value: string) { return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
function isTerminalJob(status: ImageWorkbenchStatus) {
  return status === 'completed' || status === 'failed' || status === 'submission_unknown'
}
function formatJobWaitTime(job: ImageWorkbenchJob) {
  const createdAt = Date.parse(job.created_at)
  if (!Number.isFinite(createdAt)) return '00:00'
  const completedAt = isTerminalJob(job.status) ? Date.parse(job.updated_at) : Number.NaN
  const endAt = Number.isFinite(completedAt) ? completedAt : elapsedClock.value
  const elapsedSeconds = Math.max(0, Math.floor((endAt - createdAt) / 1000))
  const minutes = Math.floor(elapsedSeconds / 60)
  const seconds = elapsedSeconds % 60
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

function restartAnnouncementRotation() {
  if (announcementTimer) clearInterval(announcementTimer)
  announcementTimer = null
  announcementIndex.value = 0
  if (workbenchAnnouncements.value.length <= 1) return
  announcementTimer = setInterval(() => {
    announcementIndex.value = (announcementIndex.value + 1) % workbenchAnnouncements.value.length
  }, announcementIntervalSeconds.value * 1000)
}

watch([workbenchAnnouncements, announcementIntervalSeconds], restartAnnouncementRotation, { deep: true })

onMounted(async () => {
  await restoreDraft()
  const keysPromise = loadKeys()
  await restoreCachedLibrary()
  await Promise.all([keysPromise, loadJobs()])
  pollTimer = setInterval(refreshPendingJobs, 2000)
  elapsedTimer = setInterval(() => { elapsedClock.value = Date.now() }, 1000)
  restartAnnouncementRotation()
})

onBeforeUnmount(() => {
  if (draftSaveTimer) clearTimeout(draftSaveTimer)
  draftSaveTimer = null
  persistDraftNow()
  if (pollTimer) clearInterval(pollTimer)
  if (elapsedTimer) clearInterval(elapsedTimer)
  if (announcementTimer) clearInterval(announcementTimer)
  Object.keys(previewURLs).forEach(releasePreview)
  previewBlobs.clear()
  pendingBlobLoads.clear()
})
</script>

<style scoped>
.image-workbench-compact :deep(.input) {
  border-radius: 0.5rem;
  font-size: 0.75rem;
  line-height: 1.25rem;
  padding: 0.375rem 0.75rem;
}

.image-workbench-compact :deep(.input-label) {
  font-size: 0.75rem;
  line-height: 1rem;
  margin-bottom: 0.25rem;
}

.image-workbench-compact :deep(.input-hint) {
  font-size: 0.6875rem;
  line-height: 1rem;
  margin-top: 0.25rem;
}
</style>
