<template>
  <AppLayout>
    <div class="-mx-4 -mt-2 -mb-4 flex h-[calc(100dvh-4.5rem-1px)] max-w-[1560px] min-h-0 flex-col gap-3 overflow-hidden px-4 pb-4 pt-0 md:-mx-6 md:-mt-4 md:-mb-6 md:h-[calc(100dvh-4.5rem-1px)] lg:-mx-8 lg:-mt-6 lg:-mb-8 lg:h-[calc(100dvh-4.5rem-1px)] lg:px-4">
      <ImageWorkbenchHeader
        :header-model="headerModel"
        :header-canvas-size="headerCanvasSize"
        :queue-summary="queueSummary"
        :interface-connected="interfaceConnected"
        :interface-status="interfaceStatus"
        :completed-count="completedJobs.length"
        :downloading-all="downloadingAll"
        :submitting="submitting"
        :can-submit="canSubmit"
        :announcements="workbenchAnnouncements"
        :announcement-track-style="announcementTrackStyle"
        @download-all="downloadAll"
        @submit="submitJob"
      />

      <div class="grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)_minmax(0,1fr)] gap-3 lg:grid-cols-[340px_minmax(0,1fr)] lg:grid-rows-1">
        <ImageWorkbenchFormPanel
          ref="formPanelRef"
          v-model:api-key-id="form.apiKeyId"
          v-model:model="form.model"
          v-model:quality="form.quality"
          v-model:width="form.width"
          v-model:height="form.height"
          v-model:prompt="form.prompt"
          v-model:reference-url-input="referenceUrlInput"
          :loading-keys="loadingKeys"
          :eligible-keys="eligibleKeys"
          :models="models"
          :available-qualities="availableQualities"
          :available-aspect-ratios="availableAspectRatios"
          :aspect-ratio="form.aspectRatio"
          :selected-model-max-edge="selectedModelMaxEdge"
          :selected-model-max-pixels="selectedModelMaxPixels"
          :selected-model-tier="selectedModel.tier"
          :dimension-error-message="dimensionErrorMessage"
          :experimental-dimensions="experimentalDimensions"
          :reference-count="referenceCount"
          :reference-files="referenceFiles"
          :reference-url-list="referenceUrlList"
          :referenceDataURLsCount="referenceDataURLs.length"
          :loading-estimate="loadingEstimate"
          :cost-estimate="costEstimate"
          :submitting="submitting"
          :can-submit="canSubmit"
          :format-integer="formatInteger"
          @submit="submitJob"
          @reset="resetForm"
          @select-aspect-ratio="selectAspectRatio"
          @normalize-dimensions="normalizeDimensions"
          @reference-files="onReferenceFiles"
          @drop-reference="onReferenceDrop"
          @import-reference-url="importReferenceUrl"
          @clear-references="clearReferences"
        />

        <section class="min-h-0 min-w-0 space-y-3 overflow-y-auto overscroll-contain pr-1">
          <div class="card overflow-hidden p-0">
            <div class="journal-card-band flex flex-wrap items-center justify-between gap-2 px-4 py-3">
              <div class="flex items-center gap-2.5">
                <span class="journal-icon-well journal-icon-well--sm">
                  <CraftIcon name="sparkles" size="xs" />
                </span>
                <div>
                  <div class="flex items-center gap-1.5">
                    <h2 class="journal-section-title text-base font-extrabold text-[#221d17] dark:text-white">{{ t('imageWorkbench.preview.title') }}</h2>
                    <CraftSparks variant="right" size="sm" />
                  </div>
                  <p class="mt-0.5 text-[11px] font-medium text-[#6f6252] dark:text-gray-400">{{ t('imageWorkbench.preview.subtitle') }}</p>
                </div>
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

            <div class="p-3">
            <div class="grid min-h-[420px] gap-3 lg:grid-cols-[minmax(0,1fr)_250px]">
              <div class="flex min-h-[420px] items-center justify-center rounded-xl border border-[#e3d2b6] bg-[linear-gradient(135deg,#fdf7eb_25%,#f5ecdc_25%,#f5ecdc_50%,#fdf7eb_50%,#fdf7eb_75%,#f5ecdc_75%)] bg-[length:28px_28px] p-3 dark:border-dark-700 dark:bg-none dark:bg-[#161a24]">
                <div v-if="blankCanvasOpen || !currentPreviewURL" class="relative max-h-[420px] max-w-full overflow-hidden rounded-2xl border border-[#e8dac0] bg-[#fffdf8] shadow-[0_6px_0_rgba(143,108,65,0.1)] transition-[width,aspect-ratio] duration-500 ease-[cubic-bezier(0.22,1,0.36,1)] will-change-[width,aspect-ratio] dark:border-dark-700 dark:bg-dark-800" :style="activeCanvasStyle" data-testid="preview-canvas">
                  <div class="flex h-full min-h-24 items-center justify-center px-6 pb-12 pt-5 text-center text-[#6f6252] dark:text-gray-400">
                    <div>
                      <div class="journal-icon-well mx-auto mb-3 -rotate-3">
                        <CraftIcon name="palette" size="lg" />
                      </div>
                      <p class="text-lg font-extrabold text-[#221d17] dark:text-gray-200">{{ activeCanvasTitle }}</p>
                      <p class="mt-1 text-sm text-[#6f6252] dark:text-gray-400">{{ activeCanvasHint }}</p>
                    </div>
                  </div>
                  <div class="absolute inset-x-3 bottom-3 flex items-center justify-between rounded-lg bg-[#3a3027]/85 px-3 py-2 text-xs font-semibold text-[#fffaf0]">
                    <span>{{ activeCanvasStatus }}</span>
                    <span>{{ activeCanvasSize }}</span>
                  </div>
                </div>
                <div v-else class="relative flex max-h-[420px] max-w-full items-center justify-center overflow-hidden rounded-2xl border border-[#e8dac0] bg-[#fffdf8] shadow-[0_6px_0_rgba(143,108,65,0.1)] dark:border-dark-700 dark:bg-dark-800">
                  <img :src="currentPreviewURL" :alt="currentJob ? jobDisplayName(currentJob) : 'generated image'" class="max-h-[420px] max-w-full object-contain" />
                  <div class="absolute inset-x-3 bottom-3 flex items-center justify-between rounded-lg bg-[#3a3027]/85 px-3 py-2 text-xs font-semibold text-[#fffaf0]">
                    <span>{{ currentJob?.status === 'completed' ? t('imageWorkbench.preview.ready') : statusText(currentJob?.status || 'in_progress') }}</span>
                    <span>{{ currentJob?.actual_size || currentJob?.requested_size || '—' }}</span>
                  </div>
                </div>
              </div>

              <ImageWorkbenchBatchList
                :jobs="jobs"
                :loading-jobs="loadingJobs"
                :selected-job-id="selectedJobId"
                :deleting-job-id="deletingJobId"
                :format-date="formatDate"
                :format-job-wait-time="formatJobWaitTime"
                :job-display-name="jobDisplayName"
                :status-class="statusClass"
                :status-text="statusText"
                :job-resolution="jobResolution"
                :job-quality="jobQuality"
                :can-delete-job="canDeleteJob"
                @select-job="selectJob"
                @delete-job="deleteJob"
              />
            </div>
            </div>
          </div>

          <ImageWorkbenchLibrary
            v-model:rename-draft="renameDraft"
            :completed-jobs="completedJobs"
            :previewURLs="previewURLs"
            :renaming-job-id="renamingJobId"
            :saving-job-name-id="savingJobNameId"
            :deleting-job-id="deletingJobId"
            :job-display-name="jobDisplayName"
            :job-resolution="jobResolution"
            :job-quality="jobQuality"
            @preview-artwork="previewArtwork"
            @start-rename="startRename"
            @cancel-rename="cancelRename"
            @save-job-name="saveJobName"
            @delete-job="deleteJob"
          />
        </section>
      </div>
    </div>

    <ImageWorkbenchArtworkPreviewModal
      :job="previewingJob"
      :preview-url="previewingJob ? previewURLs[previewingJob.id] || '' : ''"
      :display-name="previewingJob ? jobDisplayName(previewingJob) : ''"
      @close="closeArtworkPreview"
      @download="downloadResult"
    />

    <div v-if="editor.open" class="fixed inset-0 z-50 flex items-center justify-center bg-[#231d17]/75 p-4" @keydown.esc="closeEditor">
      <section class="flex max-h-[94vh] w-full max-w-[1500px] flex-col overflow-hidden rounded-2xl border border-[#e3d2b6] bg-[#fffdf8] shadow-2xl dark:border-dark-700 dark:bg-dark-800">
        <header class="journal-card-band flex items-center justify-between px-5 py-4">
          <div>
            <h2 class="text-xl font-extrabold text-[#221d17] dark:text-white">{{ t('imageWorkbench.editor.title') }}</h2>
            <p class="mt-1 text-xs font-medium text-[#6f6252] dark:text-gray-400">{{ t('imageWorkbench.editor.subtitle') }}</p>
          </div>
          <button class="btn btn-secondary btn-sm" aria-label="Close" @click="closeEditor">×</button>
        </header>

        <div class="grid min-h-0 flex-1 gap-4 overflow-auto p-4 lg:grid-cols-[minmax(0,1fr)_380px]">
          <div class="flex min-h-[420px] items-center justify-center rounded-xl border border-[#e3d2b6] bg-[#f8efe0] p-4 dark:border-dark-700 dark:bg-dark-900">
            <div class="relative w-full max-w-[900px] overflow-hidden rounded-lg bg-[#fffdf8] shadow-sm dark:bg-dark-800" :style="editorStageStyle">
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

          <aside class="flex flex-col rounded-xl border border-[#e3d2b6] bg-[#fffaf0]/90 p-4 dark:border-dark-700 dark:bg-dark-900/60">
            <div class="grid grid-cols-2 gap-2">
              <button class="btn btn-primary" :class="{ 'ring-2 ring-amber-300': editor.mode === 'brush' }" @click="editor.mode = 'brush'">{{ t('imageWorkbench.editor.brush') }}</button>
              <button class="btn btn-secondary" @click="clearEditorMarks">{{ t('imageWorkbench.editor.clear') }}</button>
            </div>
            <label class="mt-4 text-sm font-medium text-[#4e4336] dark:text-gray-300">{{ t('imageWorkbench.editor.brushSize') }} · {{ editor.brushSize }}px</label>
            <input v-model.number="editor.brushSize" class="mt-2 w-full accent-[#e5894b]" type="range" min="8" max="160" step="4" />
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
import CraftIcon from '@/components/craft/CraftIcon.vue'
import CraftSparks from '@/components/craft/CraftSparks.vue'
import ImageWorkbenchArtworkPreviewModal from '@/features/image-workbench/components/ImageWorkbenchArtworkPreviewModal.vue'
import ImageWorkbenchBatchList from '@/features/image-workbench/components/ImageWorkbenchBatchList.vue'
import ImageWorkbenchFormPanel from '@/features/image-workbench/components/ImageWorkbenchFormPanel.vue'
import ImageWorkbenchHeader from '@/features/image-workbench/components/ImageWorkbenchHeader.vue'
import ImageWorkbenchLibrary from '@/features/image-workbench/components/ImageWorkbenchLibrary.vue'
import { imageWorkbenchAPI, keysAPI } from '@/api'
import type { ImageWorkbenchCostEstimate, ImageWorkbenchJob, ImageWorkbenchModel, ImageWorkbenchQuality, ImageWorkbenchStatus } from '@/api'
import type { ApiKey } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import {
  IMAGE_DIMENSION_MAX_EDGE,
  IMAGE_DIMENSION_MIN_EDGE,
  IMAGE_DIMENSION_MIN_PIXELS,
  IMAGE_DIMENSION_STEP,
  isExperimentalImageDimensions,
  maxImageDimensionForPixelBudget,
  validateImageDimensions
} from '@/utils/imageWorkbenchDimensions'
import {
  IMAGE_WORKBENCH_MODELS,
  imageWorkbenchModelCapabilities,
  isImageWorkbenchQualityAllowed
} from '@/utils/imageWorkbenchModelCapabilities'
import {
  clampDimension as clampImageDimension,
  computeAspectRatioSize,
  dataURLByteLength,
  dimensionErrorKey,
  extractErrorMessage,
  formatJobWaitTime as formatWorkbenchJobWaitTime,
  jobQualityOption,
  jobResolution,
  parseSize,
  sanitizeDownloadFileName,
  statusClass
} from '@/features/image-workbench/imageWorkbenchFormatters'
import { useImageMaskEditor } from '@/features/image-workbench/useImageMaskEditor'
import { useImageWorkbenchDraft } from '@/features/image-workbench/useImageWorkbenchDraft'
import { useImageWorkbenchPreviews } from '@/features/image-workbench/useImageWorkbenchPreviews'
import { useImageWorkbenchReferences } from '@/features/image-workbench/useImageWorkbenchReferences'

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

const models = IMAGE_WORKBENCH_MODELS
const EDITOR_REFERENCE_MAX_BYTES = 10 * 1024 * 1024

const form = reactive({
  apiKeyId: 0,
  model: 'gpt-image-2-1k' as ImageWorkbenchModel,
  quality: 'medium' as ImageWorkbenchQuality,
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
const formPanelRef = ref<InstanceType<typeof ImageWorkbenchFormPanel> | null>(null)
const editorCanvasRef = ref<HTMLCanvasElement | null>(null)
const editorMaskCanvasRef = ref<HTMLCanvasElement | null>(null)
const editorImageRef = ref<HTMLImageElement | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null
let elapsedTimer: ReturnType<typeof setInterval> | null = null
let announcementTimer: ReturnType<typeof setInterval> | null = null
const elapsedClock = ref(Date.now())
const {
  setupEditorCanvas,
  redrawBrushStrokes,
  startDrawing,
  draw,
  stopDrawing,
  clearEditorMarks,
  createEditableReferenceDataURL
} = useImageMaskEditor({
  editor,
  editorCanvasRef,
  editorMaskCanvasRef,
  editorImageRef,
  maxReferenceBytes: EDITOR_REFERENCE_MAX_BYTES,
  fileTooLargeMessage: () => t('imageWorkbench.errors.fileTooLarge')
})
watch(() => editor.open, (isOpen) => {
  if (isOpen && editor.hasMarks) redrawBrushStrokes()
})

const selectedModel = computed(() => imageWorkbenchModelCapabilities(form.model))
const availableQualities = computed(() => selectedModel.value.qualities)
const availableAspectRatios = computed(() => ['', ...selectedModel.value.aspectRatios])
const imageCacheUserId = computed(() => Number(authStore.user?.id || 0))
const eligibleKeys = computed(() => apiKeys.value.filter(key => key.status === 'active' && key.group?.platform === 'openai' && key.group.allow_image_generation))
const currentJob = computed(() => jobs.value.find(job => job.id === selectedJobId.value) || null)
const previewingJob = computed(() => jobs.value.find(job => job.id === previewingJobId.value) || null)
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
const referenceCount = computed(() => referenceUrlList.value.length + referenceDataURLs.value.length)
const selectedModelMaxPixels = computed(() => selectedModel.value.maxPixels)
const selectedModelMaxEdge = computed(() => maxImageDimensionForPixelBudget(selectedModelMaxPixels.value))
const {
  restoreDraft,
  persistDraftNow,
  clearDraft,
  disposeDraftAutoSave
} = useImageWorkbenchDraft({
  form,
  referenceFiles,
  referenceDataURLs,
  referenceUrlInput,
  userId: imageCacheUserId,
  selectedModelMaxEdge,
  selectedModelMaxPixels
})
const {
  onReferenceFiles,
  onReferenceDrop,
  importReferenceUrl,
  clearReferences
} = useImageWorkbenchReferences({
  form,
  referenceFiles,
  referenceDataURLs,
  referenceUrlInput,
  referenceUrlList,
  referenceCount,
  onError: (message) => appStore.showError(message),
  onChanged: persistDraftNow,
  onClearFileInput: () => formPanelRef.value?.clearFileInput(),
  translateError: (key) => t(key)
})
const {
  previewURLs,
  prunePreviews,
  restoreCachedLibrary: restoreCachedPreviewLibrary,
  hydrateCachedPreviews,
  loadPreview,
  downloadResult,
  ensureEditableDataURL,
  refreshCachedJobMetadata,
  removeCachedJob,
  disposePreviews
} = useImageWorkbenchPreviews({
  userId: imageCacheUserId,
  createEditableReferenceDataURL,
  getDownloadFileName: downloadFileName,
  onError: (message) => appStore.showError(message),
  previewErrorMessage: () => t('imageWorkbench.errors.preview'),
  downloadErrorMessage: () => t('imageWorkbench.errors.download')
})
const currentPreviewURL = computed(() => selectedJobId.value ? previewURLs[selectedJobId.value] : '')
const dimensionValidation = computed(() => validateImageDimensions(Number(form.width), Number(form.height), selectedModelMaxPixels.value))
const dimensionErrorMessage = computed(() => {
  const code = dimensionValidation.value.code
  if (!code) return ''
  return t(dimensionErrorKey(code), {
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

watch(() => form.model, () => {
  const [width, height] = parseSize(selectedModel.value.defaultSize)
  if (!isImageWorkbenchQualityAllowed(form.model, form.quality)) form.quality = selectedModel.value.defaultQuality
  form.width = width
  form.height = height
  form.size = selectedModel.value.defaultSize
  form.aspectRatio = ''
})
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
  } catch (error: unknown) {
    appStore.showError(extractErrorMessage(error, t('imageWorkbench.errors.loadKeys')))
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

async function restoreCachedLibrary() {
  const restored = await restoreCachedPreviewLibrary()
  if (!restored.jobs.length) return
  jobs.value = restored.jobs
  if (restored.preferredJobId) selectedJobId.value = restored.preferredJobId
}

async function loadJobs() {
  loadingJobs.value = true
  try {
    const response = await imageWorkbenchAPI.listJobs(30, 0)
    const previousSelectedJobId = selectedJobId.value
    jobs.value = response.data || []
    const currentJobIds = new Set(jobs.value.map(job => job.id))
    prunePreviews(currentJobIds)
    await hydrateCachedPreviews(jobs.value.filter(job => job.status === 'completed'))
    const preferred = jobs.value.find(job => job.id === previousSelectedJobId)
      || jobs.value.find(job => job.status === 'completed')
      || jobs.value[0]
    if (preferred) await selectJob(preferred)
    else {
      selectedJobId.value = ''
      blankCanvasOpen.value = true
    }
  } catch (error: unknown) {
    appStore.showError(extractErrorMessage(error, t('imageWorkbench.errors.loadJobs')))
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
    formPanelRef.value?.clearFileInput()
    clearDraft()
    appStore.showSuccess(t('imageWorkbench.messages.submitted'))
  } catch (error: unknown) {
    appStore.showError(extractErrorMessage(error, t('imageWorkbench.errors.submit')))
  } finally {
    submitting.value = false
  }
}

function createBlankCanvas() {
  if (dimensionValidation.value.code !== null) return
  blankCanvasOpen.value = true
  selectedJobId.value = ''
}

function clampDimension(value: number) {
  return clampImageDimension(value, selectedModelMaxEdge.value)
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
  const defaultModel = imageWorkbenchModelCapabilities(form.model)
  form.quality = defaultModel.defaultQuality
  form.size = defaultModel.defaultSize
  form.aspectRatio = ''
  const [defaultWidth, defaultHeight] = parseSize(defaultModel.defaultSize)
  form.width = defaultWidth
  form.height = defaultHeight
  form.prompt = ''
  clearReferences()
  blankCanvasOpen.value = false
  selectedJobId.value = ''
  clearDraft()
}

function selectAspectRatio(value: string) {
  form.aspectRatio = value
  if (!value) return
  const resolved = computeAspectRatioSize(Number(form.width), value, selectedModelMaxEdge.value, selectedModelMaxPixels.value)
  if (!resolved) return
  form.width = resolved[0]
  form.height = resolved[1]
  updateFormSize()
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

async function openEditor(job: ImageWorkbenchJob) {
  if (job.status !== 'completed') return
  const dataURL = await ensureEditableDataURL(job)
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
    await refreshCachedJobMetadata(updated)
    cancelRename()
    appStore.showSuccess(t('imageWorkbench.messages.renamed'))
  } catch (error: unknown) {
    appStore.showError(extractErrorMessage(error, t('imageWorkbench.errors.rename')))
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
    await removeCachedJob(job.id)
    if (selectedJobId.value === job.id) {
      const preferred = jobs.value.find(item => item.status === 'completed') || jobs.value[0]
      if (preferred) await selectJob(preferred)
      else {
        selectedJobId.value = ''
        blankCanvasOpen.value = true
      }
    }
    appStore.showSuccess(t('imageWorkbench.messages.deleted'))
  } catch (error: unknown) {
    appStore.showError(extractErrorMessage(error, t('imageWorkbench.errors.delete')))
  } finally {
    deletingJobId.value = ''
  }
}

function closeEditor() {
  editor.open = false
  editor.job = null
  clearEditorMarks()
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
  } catch (error: unknown) {
    appStore.showError(extractErrorMessage(error, t('imageWorkbench.errors.submit')))
  } finally {
    editingSubmitting.value = false
  }
}

function statusText(status: ImageWorkbenchStatus) { return t('imageWorkbench.status.' + status) }
function jobDisplayName(job: ImageWorkbenchJob) { return job.name?.trim() || t('imageWorkbench.library.untitled') }
function jobQuality(job: ImageWorkbenchJob) { return t('imageWorkbench.form.qualityOptions.' + jobQualityOption(job)) }
function downloadFileName(job: ImageWorkbenchJob) { return sanitizeDownloadFileName(jobDisplayName(job), job.id) }
function formatInteger(value: number) { return new Intl.NumberFormat(locale.value).format(value) }
function formatDate(value: string) { return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
function formatJobWaitTime(job: ImageWorkbenchJob) { return formatWorkbenchJobWaitTime(job, elapsedClock.value) }

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
  disposeDraftAutoSave()
  if (pollTimer) clearInterval(pollTimer)
  if (elapsedTimer) clearInterval(elapsedTimer)
  if (announcementTimer) clearInterval(announcementTimer)
  disposePreviews()
})
</script>
