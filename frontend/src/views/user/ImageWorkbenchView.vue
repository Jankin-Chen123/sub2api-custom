<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6 px-4 py-6 sm:px-6 lg:px-8">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.title') }}</h1>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.description') }}</p>
        </div>
        <button class="btn btn-secondary" :disabled="loadingJobs" @click="loadJobs">
          {{ loadingJobs ? t('common.loading') : t('common.refresh') }}
        </button>
      </div>

      <div class="grid gap-3 md:grid-cols-3">
        <div class="rounded-xl border border-blue-200 bg-blue-50 p-4 text-sm text-blue-900 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200">
          {{ t('imageWorkbench.notices.originalResolution') }}
        </div>
        <div class="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
          {{ t('imageWorkbench.notices.billable') }}
        </div>
        <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-900/50 dark:text-gray-300">
          {{ t('imageWorkbench.notices.textLimit') }}
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-[minmax(0,420px)_minmax(0,1fr)]">
        <form class="space-y-5 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800" @submit.prevent="submitJob">
          <div>
            <label class="input-label">{{ t('imageWorkbench.form.apiKey') }}</label>
            <select v-model.number="form.apiKeyId" class="input" required>
              <option :value="0">{{ loadingKeys ? t('common.loading') : t('imageWorkbench.form.selectApiKey') }}</option>
              <option v-for="key in eligibleKeys" :key="key.id" :value="key.id">
                {{ key.name }} · {{ key.group?.name || `#${key.group_id}` }}
              </option>
            </select>
            <p v-if="!loadingKeys && eligibleKeys.length === 0" class="input-hint text-amber-600 dark:text-amber-400">
              {{ t('imageWorkbench.form.noApiKey') }}
            </p>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="input-label">{{ t('imageWorkbench.form.model') }}</label>
              <select v-model="form.model" class="input">
                <option v-for="model in models" :key="model.value" :value="model.value">{{ model.label }}</option>
              </select>
            </div>
            <div>
              <label class="input-label">{{ t('imageWorkbench.form.quality') }}</label>
              <select v-model="form.quality" class="input">
                <option v-for="quality in qualities" :key="quality" :value="quality">{{ t(`imageWorkbench.form.qualityOptions.${quality}`) }}</option>
              </select>
            </div>
          </div>

          <div class="grid gap-3 sm:grid-cols-[150px_minmax(0,1fr)]">
            <div>
              <label class="input-label">{{ t('imageWorkbench.form.dimensionMode') }}</label>
              <select v-model="form.dimensionMode" class="input">
                <option value="size">{{ t('imageWorkbench.form.dimensionModes.size') }}</option>
                <option value="aspect_ratio">{{ t('imageWorkbench.form.dimensionModes.aspectRatio') }}</option>
              </select>
            </div>
            <div v-if="form.dimensionMode === 'size'">
              <label class="input-label">{{ t('imageWorkbench.form.size') }}</label>
              <input v-model.trim="form.size" class="input" :placeholder="selectedModel.defaultSize" />
            </div>
            <div v-else>
              <label class="input-label">{{ t('imageWorkbench.form.aspectRatio') }}</label>
              <input v-model.trim="form.aspectRatio" class="input" placeholder="16:9" inputmode="numeric" />
            </div>
          </div>

          <div>
            <label class="input-label">{{ t('imageWorkbench.form.responseFormat') }}</label>
            <select v-model="form.responseFormat" class="input">
              <option value="url">{{ t('imageWorkbench.form.responseFormats.url') }}</option>
              <option value="b64_json">{{ t('imageWorkbench.form.responseFormats.b64Json') }}</option>
            </select>
          </div>

          <div>
            <label class="input-label">{{ t('imageWorkbench.form.prompt') }}</label>
            <textarea v-model="form.prompt" class="input min-h-36 resize-y" maxlength="12000" required :placeholder="t('imageWorkbench.form.promptPlaceholder')" :aria-describedby="'image-workbench-prompt-hint'" @keydown.ctrl.enter.prevent="submitJob" @keydown.meta.enter.prevent="submitJob" />
            <div class="mt-1 text-right text-xs text-gray-400">{{ form.prompt.length }}/12000</div>
            <p id="image-workbench-prompt-hint" class="input-hint">{{ t('imageWorkbench.form.promptSubmitHint') }}</p>
          </div>

          <div>
            <label class="input-label">{{ t('imageWorkbench.form.referenceUrls') }}</label>
            <textarea v-model="form.referenceUrls" class="input min-h-24 resize-y" :placeholder="t('imageWorkbench.form.referenceUrlsPlaceholder')" />
            <input class="mt-2 block w-full text-sm text-gray-500 file:mr-3 file:rounded-md file:border-0 file:bg-primary-50 file:px-3 file:py-2 file:text-primary-700 dark:text-gray-400" type="file" accept="image/*" multiple @change="onReferenceFiles" />
            <div v-if="referenceFiles.length" class="mt-2 flex flex-wrap gap-2">
              <span v-for="file in referenceFiles" :key="file.name" class="rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ file.name }}</span>
            </div>
          </div>

          <div>
            <label class="input-label">{{ t('imageWorkbench.form.mask') }}</label>
            <input v-model.trim="form.maskUrl" class="input" :placeholder="t('imageWorkbench.form.maskPlaceholder')" />
            <input class="mt-2 block w-full text-sm text-gray-500 file:mr-3 file:rounded-md file:border-0 file:bg-primary-50 file:px-3 file:py-2 file:text-primary-700 dark:text-gray-400" type="file" accept="image/png" @change="onMaskFile" />
            <p class="input-hint">{{ t('imageWorkbench.form.maskHint') }}</p>
          </div>

          <div v-if="form.model === 'gpt-image-2-4k'" class="rounded-lg bg-amber-50 p-3 text-xs leading-5 text-amber-800 dark:bg-amber-950/30 dark:text-amber-300">
            {{ t('imageWorkbench.notices.fourK') }}
          </div>

          <div class="rounded-lg border border-primary-200 bg-primary-50 p-3 text-sm text-primary-900 dark:border-primary-900/60 dark:bg-primary-950/30 dark:text-primary-200">
            <span v-if="loadingEstimate">{{ t('imageWorkbench.form.loadingEstimate') }}</span>
            <span v-else-if="costEstimate">{{ t('imageWorkbench.form.estimatedCost', { cost: formatCost(costEstimate.estimated_cost) }) }}</span>
            <span v-else>{{ t('imageWorkbench.form.estimateUnavailable') }}</span>
          </div>

          <button class="btn btn-primary w-full" type="submit" :disabled="submitting || !canSubmit">
            {{ submitting ? t('imageWorkbench.actions.submitting') : t('imageWorkbench.actions.generate') }}
          </button>
        </form>

        <section class="space-y-4">
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.jobs.title') }}</h2>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.jobs.autoRefresh') }}</span>
          </div>

          <div v-if="loadingJobs && jobs.length === 0" class="rounded-2xl border border-gray-200 bg-white p-10 text-center text-gray-500 dark:border-dark-700 dark:bg-dark-800">
            {{ t('common.loading') }}
          </div>
          <div v-else-if="jobs.length === 0" class="rounded-2xl border border-dashed border-gray-300 bg-white p-10 text-center text-gray-500 dark:border-dark-600 dark:bg-dark-800">
            {{ t('imageWorkbench.jobs.empty') }}
          </div>
          <article v-for="job in jobs" :key="job.id" class="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="grid md:grid-cols-[220px_minmax(0,1fr)]">
              <div class="flex min-h-48 items-center justify-center bg-gray-100 dark:bg-dark-900">
                <img v-if="previewURLs[job.id]" :src="previewURLs[job.id]" :alt="job.id" class="h-full max-h-72 w-full object-contain" />
                <button v-else-if="job.status === 'completed'" class="btn btn-secondary" @click="loadPreview(job)">{{ t('imageWorkbench.actions.loadPreview') }}</button>
                <span v-else class="text-sm text-gray-400">{{ statusText(job.status) }}</span>
              </div>
              <div class="space-y-4 p-5">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p class="font-mono text-xs text-gray-400">{{ job.id }}</p>
                    <h3 class="mt-1 font-medium text-gray-900 dark:text-white">{{ modelLabel(job.model) }}</h3>
                  </div>
                  <span :class="statusClass(job.status)" class="rounded-full px-2.5 py-1 text-xs font-medium">{{ statusText(job.status) }}</span>
                </div>
                <dl class="grid grid-cols-2 gap-3 text-sm">
                  <div><dt class="text-gray-400">{{ t('imageWorkbench.jobs.size') }}</dt><dd class="text-gray-800 dark:text-gray-200">{{ job.actual_size || job.requested_size || '—' }}</dd></div>
                  <div><dt class="text-gray-400">{{ t('imageWorkbench.jobs.cost') }}</dt><dd class="text-gray-800 dark:text-gray-200">${{ formatCost(job.settled_cost || job.estimated_cost) }}</dd></div>
                  <div class="col-span-2"><dt class="text-gray-400">{{ t('imageWorkbench.jobs.createdAt') }}</dt><dd class="text-gray-800 dark:text-gray-200">{{ formatDate(job.created_at) }}</dd></div>
                </dl>
                <div v-if="job.error" class="rounded-lg bg-red-50 p-3 text-xs text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ job.error.code }} · {{ job.error.message }}</div>
                <div v-if="job.status === 'completed'" class="flex gap-2">
                  <button class="btn btn-secondary" @click="loadPreview(job)">{{ t('imageWorkbench.actions.preview') }}</button>
                  <button class="btn btn-primary" @click="downloadResult(job)">{{ t('imageWorkbench.actions.download') }}</button>
                </div>
              </div>
            </div>
          </article>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { imageWorkbenchAPI, keysAPI } from '@/api'
import type { ImageWorkbenchCostEstimate, ImageWorkbenchJob, ImageWorkbenchModel, ImageWorkbenchQuality, ImageWorkbenchStatus } from '@/api'
import type { ApiKey } from '@/types'
import { useAppStore } from '@/stores/app'
import { validateMaskFile, validateReferenceFiles } from './imageWorkbenchValidation'

const { t, locale } = useI18n()
const appStore = useAppStore()

const models: Array<{ value: ImageWorkbenchModel; label: string; defaultSize: string }> = [
  { value: 'gpt-image-2-1k', label: '1K · gpt-image-2-1k', defaultSize: '1024x1024' },
  { value: 'gpt-image-2-2k', label: '2K · gpt-image-2-2k', defaultSize: '2048x2048' },
  { value: 'gpt-image-2-4k', label: '4K · gpt-image-2-4k', defaultSize: '3840x2160' }
]
const qualities: ImageWorkbenchQuality[] = ['auto', 'low', 'medium', 'high']

const form = reactive({
  apiKeyId: 0,
  model: 'gpt-image-2-1k' as ImageWorkbenchModel,
  quality: 'auto' as ImageWorkbenchQuality,
  dimensionMode: 'size' as 'size' | 'aspect_ratio',
  size: '1024x1024',
  aspectRatio: '1:1',
  responseFormat: 'url' as 'url' | 'b64_json',
  prompt: '',
  referenceUrls: '',
  maskUrl: ''
})
const apiKeys = ref<ApiKey[]>([])
const jobs = ref<ImageWorkbenchJob[]>([])
const loadingKeys = ref(false)
const loadingJobs = ref(false)
const submitting = ref(false)
const loadingEstimate = ref(false)
const costEstimate = ref<ImageWorkbenchCostEstimate | null>(null)
const referenceFiles = ref<File[]>([])
const referenceDataURLs = ref<string[]>([])
const maskDataURL = ref('')
const previewURLs = reactive<Record<string, string>>({})
let pollTimer: ReturnType<typeof setInterval> | null = null

const selectedModel = computed(() => models.find(item => item.value === form.model) || models[0]!)
const eligibleKeys = computed(() => apiKeys.value.filter(key => key.status === 'active' && key.group?.platform === 'openai' && key.group.allow_image_generation))
const canSubmit = computed(() => form.apiKeyId > 0 && form.prompt.trim().length > 0 && eligibleKeys.value.some(key => key.id === form.apiKeyId) && (form.dimensionMode === 'size' ? form.size.trim().length > 0 : form.aspectRatio.trim().length > 0))

watch(() => form.model, () => { form.size = selectedModel.value.defaultSize })
watch([() => form.apiKeyId, () => form.model], () => { void loadCostEstimate() }, { immediate: true })

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

async function loadJobs() {
  loadingJobs.value = true
  try {
    const response = await imageWorkbenchAPI.listJobs(30, 0)
    jobs.value = response.data || []
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.loadJobs'))
  } finally {
    loadingJobs.value = false
  }
}

async function submitJob() {
  if (!canSubmit.value || submitting.value) return
  submitting.value = true
  try {
    const urls = form.referenceUrls.split(/\r?\n/).map(value => value.trim()).filter(Boolean)
    const images = [...urls, ...referenceDataURLs.value]
    const operation = images.length > 0 || maskDataURL.value || form.maskUrl ? 'edit' : 'generation'
    const idempotencyKey = crypto.randomUUID()
    const job = await imageWorkbenchAPI.createJob({
      api_key_id: form.apiKeyId,
      operation,
      model: form.model,
      prompt: form.prompt.trim(),
      size: form.dimensionMode === 'size' ? (form.size || selectedModel.value.defaultSize) : undefined,
      aspect_ratio: form.dimensionMode === 'aspect_ratio' ? form.aspectRatio : undefined,
      quality: form.quality,
      response_format: form.responseFormat,
      images: images.length ? images : undefined,
      mask: maskDataURL.value || form.maskUrl || undefined
    }, idempotencyKey)
    jobs.value = [job, ...jobs.value.filter(item => item.id !== job.id)]
    form.prompt = ''
    form.referenceUrls = ''
    form.maskUrl = ''
    referenceFiles.value = []
    referenceDataURLs.value = []
    maskDataURL.value = ''
    appStore.showSuccess(t('imageWorkbench.messages.submitted'))
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.submit'))
  } finally {
    submitting.value = false
  }
}

async function onReferenceFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
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
    input.value = ''
    return
  }
  referenceFiles.value = files
  referenceDataURLs.value = await Promise.all(files.map(readFileAsDataURL))
}

async function onMaskFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) { maskDataURL.value = ''; return }
  const validationError = validateMaskFile(file)
  if (validationError) {
    appStore.showError(t(validationError === 'mask_file_too_large' ? 'imageWorkbench.errors.fileTooLarge' : 'imageWorkbench.errors.invalidMask'))
    input.value = ''
    return
  }
  maskDataURL.value = await readFileAsDataURL(file)
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
    } catch { /* transient polling errors are retried */ }
  }))
}

async function loadPreview(job: ImageWorkbenchJob) {
  if (previewURLs[job.id]) URL.revokeObjectURL(previewURLs[job.id])
  try {
    const blob = await imageWorkbenchAPI.getContent(job.id)
    previewURLs[job.id] = URL.createObjectURL(blob)
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.preview'))
  }
}

async function downloadResult(job: ImageWorkbenchJob) {
  try {
    const blob = await imageWorkbenchAPI.getContent(job.id)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${job.id}.png`
    anchor.click()
    URL.revokeObjectURL(url)
  } catch (error: any) {
    appStore.showError(error?.message || t('imageWorkbench.errors.download'))
  }
}

function statusText(status: ImageWorkbenchStatus) { return t(`imageWorkbench.status.${status}`) }
function statusClass(status: ImageWorkbenchStatus) {
  if (status === 'completed') return 'bg-green-100 text-green-700 dark:bg-green-950/40 dark:text-green-300'
  if (status === 'failed' || status === 'submission_unknown') return 'bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-300'
  return 'bg-blue-100 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300'
}
function modelLabel(model: ImageWorkbenchModel) { return models.find(item => item.value === model)?.label || model }
function formatCost(value: number) { return Number(value || 0).toFixed(4) }
function formatDate(value: string) { return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }

onMounted(async () => {
  await Promise.all([loadKeys(), loadJobs()])
  pollTimer = setInterval(refreshPendingJobs, 2000)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
  Object.values(previewURLs).forEach(url => URL.revokeObjectURL(url))
})
</script>
