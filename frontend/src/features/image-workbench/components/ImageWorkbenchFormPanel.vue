<template>
  <form class="card image-workbench-compact flex min-h-0 flex-col overflow-hidden p-3 sm:p-4" @submit.prevent="$emit('submit')">
    <div class="journal-card-band -mx-3 -mt-3 flex shrink-0 items-center justify-between px-3 py-2.5 sm:-mx-4 sm:-mt-4 sm:px-4 sm:py-3">
      <div class="flex items-center gap-2">
        <span class="journal-icon-well journal-icon-well--sm">
          <CraftIcon name="palette" size="xs" />
        </span>
        <h2 class="journal-section-title text-base font-extrabold text-[#221d17] dark:text-white">{{ t('imageWorkbench.form.settingsTitle') }}</h2>
      </div>
      <button class="btn btn-secondary btn-sm" type="button" @click="$emit('reset')">{{ t('imageWorkbench.form.reset') }}</button>
    </div>

    <div class="min-h-0 flex-1 space-y-3 overflow-y-auto overscroll-contain pr-1 pt-3 sm:pt-4">
      <div>
        <label class="input-label">{{ t('imageWorkbench.form.apiKey') }}</label>
        <div v-if="loadingKeys" class="input flex items-center text-gray-400" aria-live="polite">{{ t('common.loading') }}</div>
        <p v-else-if="eligibleKeys.length === 0" class="input flex items-center text-amber-600 dark:text-amber-400" data-testid="no-image-api-key">
          {{ t('imageWorkbench.form.noApiKey') }}
        </p>
        <select v-else v-model.number="apiKeyIdModel" class="input" required>
          <option v-for="key in eligibleKeys" :key="key.id" :value="key.id">
            {{ key.name }} · {{ key.group?.name || '#' + key.group_id }}
          </option>
        </select>
      </div>

      <div class="grid grid-cols-2 gap-2">
        <div>
          <label class="input-label">{{ t('imageWorkbench.form.model') }}</label>
          <select v-model="modelValue" class="input" data-testid="image-model-select">
            <option v-for="item in models" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div>
          <label class="input-label">{{ t('imageWorkbench.form.quality') }}</label>
          <select v-model="qualityModel" class="input" data-testid="image-quality-select">
            <option v-for="item in availableQualities" :key="item" :value="item">{{ t('imageWorkbench.form.qualityOptions.' + item) }}</option>
          </select>
        </div>
      </div>

      <div>
        <label class="input-label">{{ t('imageWorkbench.form.dimensions') }}</label>
        <div class="grid grid-cols-4 gap-1.5">
          <button
            v-for="ratio in availableAspectRatios"
            :key="ratio || 'auto'"
            :data-testid="ratio ? `image-ratio-${ratio.replace(':', '-')}` : 'image-ratio-unspecified'"
            class="rounded-lg border px-2 py-1.5 text-xs font-bold transition"
            :class="aspectRatio === ratio ? 'border-[#e39c64] bg-[#fff1de] text-[#8a4515] shadow-[0_2px_0_rgba(227,156,100,0.35)] dark:border-primary-500 dark:bg-primary-950/40 dark:text-primary-200' : 'border-[#e3d2b6] bg-[#fffdf8] text-[#6f6252] hover:border-[#d8b482] hover:bg-[#fff8ec] dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300'"
            type="button"
            @click="$emit('select-aspect-ratio', ratio)"
          >
            {{ ratio || t('imageWorkbench.form.unspecified') }}
          </button>
        </div>

        <div class="mt-3 grid grid-cols-2 gap-2">
          <div>
            <div class="flex items-center justify-between text-xs font-semibold text-[#5a4e3e] dark:text-gray-300">
              <label>{{ t('imageWorkbench.form.width') }}</label>
              <span class="font-bold text-[#c26328] dark:text-primary-400">{{ width }}px</span>
            </div>
            <input v-model.number="widthModel" class="mt-1.5 w-full accent-[#e5894b]" type="range" min="16" :max="selectedModelMaxEdge" step="16" @change="$emit('normalize-dimensions')" />
            <input v-model.number="widthModel" class="input mt-1.5" type="number" min="16" :max="selectedModelMaxEdge" step="16" @change="$emit('normalize-dimensions')" />
          </div>
          <div>
            <div class="flex items-center justify-between text-xs font-semibold text-[#5a4e3e] dark:text-gray-300">
              <label>{{ t('imageWorkbench.form.height') }}</label>
              <span class="font-bold text-[#c26328] dark:text-primary-400">{{ height }}px</span>
            </div>
            <input v-model.number="heightModel" class="mt-1.5 w-full accent-[#e5894b]" type="range" min="16" :max="selectedModelMaxEdge" step="16" @change="$emit('normalize-dimensions')" />
            <input v-model.number="heightModel" class="input mt-1.5" type="number" min="16" :max="selectedModelMaxEdge" step="16" @change="$emit('normalize-dimensions')" />
          </div>
        </div>

        <p v-if="dimensionErrorMessage" class="mt-2 text-xs leading-5 text-red-600 dark:text-red-400">{{ dimensionErrorMessage }}</p>
        <p v-else class="input-hint">{{ t('imageWorkbench.form.dimensionLimitsHint', { tier: selectedModelTier, maxEdge: selectedModelMaxEdge, maxPixels: formatInteger(selectedModelMaxPixels) }) }}</p>
        <p v-if="experimentalDimensions" class="mt-1 text-xs leading-5 text-amber-600 dark:text-amber-400">{{ t('imageWorkbench.form.experimentalHint') }}</p>
      </div>

      <div>
        <label class="input-label">{{ t('imageWorkbench.form.prompt') }}</label>
        <textarea
          v-model="promptModel"
          class="input min-h-24 resize-y"
          maxlength="12000"
          required
          :placeholder="t('imageWorkbench.form.promptPlaceholder')"
          @keydown.ctrl.enter.prevent="$emit('submit')"
          @keydown.meta.enter.prevent="$emit('submit')"
        />
        <div class="mt-1 text-right text-xs text-gray-400">{{ prompt.length }}/12000</div>
      </div>

      <div>
        <label class="input-label">{{ t('imageWorkbench.form.referenceTitle') }}</label>
        <div
          class="cursor-pointer rounded-xl border-2 border-dashed px-3 py-2.5 transition"
          :class="isReferenceDragOver ? 'border-[#e39c64] bg-[#fff1de] dark:border-primary-400 dark:bg-primary-950/40' : 'border-[#7dbde8] bg-[#eef8ff]/80 hover:bg-[#e3f3ff] dark:border-sky-700 dark:bg-sky-950/20'"
          @click="referenceInputRef?.click()"
          @dragenter.prevent="isReferenceDragOver = true"
          @dragover.prevent="isReferenceDragOver = true"
          @dragleave.prevent="isReferenceDragOver = false"
          @drop.prevent="handleReferenceDrop"
        >
          <input ref="referenceInputRef" class="hidden" type="file" accept="image/*" multiple @change="$emit('reference-files', $event)" />
          <p class="text-xs font-bold text-[#256b9c] dark:text-blue-300">{{ t('imageWorkbench.form.dropReference') }}</p>
          <p class="mt-1 text-[11px] font-medium text-[#6f6252] dark:text-gray-400">{{ t('imageWorkbench.form.dropReferenceHint') }}</p>
        </div>
        <div class="mt-2 flex gap-2">
          <input v-model.trim="referenceUrlInputModel" class="input min-w-0 flex-1" :placeholder="t('imageWorkbench.form.pasteReferenceUrl')" @keydown.enter.prevent="$emit('import-reference-url')" />
          <button class="btn btn-secondary shrink-0" type="button" @click="$emit('import-reference-url')">{{ t('imageWorkbench.form.importReference') }}</button>
        </div>
        <button class="btn btn-secondary mt-2 w-full" type="button" :disabled="referenceCount === 0" @click="$emit('clear-references')">{{ t('imageWorkbench.form.clearReferences') }}</button>
        <div v-if="referenceCount > 0" class="mt-2 flex flex-wrap gap-2">
          <span v-for="file in referenceFiles" :key="file.name + file.lastModified" class="max-w-full truncate rounded-full border border-[#e3d2b6] bg-[#fffaf0] px-2.5 py-1 text-xs font-medium text-[#5a4e3e] dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300">{{ file.name }}</span>
          <span v-for="url in referenceUrlList" :key="url" class="max-w-full truncate rounded-full border border-[#e3d2b6] bg-[#fffaf0] px-2.5 py-1 text-xs font-medium text-[#5a4e3e] dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300">{{ url }}</span>
          <span v-if="referenceDataURLsCount && !referenceFiles.length" class="rounded-full border border-[#7dbde8] bg-[#eef8ff] px-2.5 py-1 text-xs font-semibold text-[#256b9c] dark:border-sky-800 dark:bg-blue-950/40 dark:text-blue-300">
            {{ t('imageWorkbench.editor.currentImageReference') }}
          </span>
        </div>
      </div>

      <div class="flex items-center gap-2 rounded-xl border border-dashed border-[#dcc6a4] bg-[#fffaf0]/90 px-3 py-2 text-xs font-semibold text-[#5a4e3e] dark:border-dark-700 dark:bg-dark-900/50 dark:text-gray-300">
        <CraftIcon name="receipt" size="xs" />
        <span v-if="loadingEstimate">{{ t('imageWorkbench.form.loadingEstimate') }}</span>
        <span v-else-if="costEstimate">{{ t('imageWorkbench.form.estimatedCost', { cost: formatCost(costEstimate.estimated_cost) }) }}</span>
        <span v-else>{{ t('imageWorkbench.form.estimateUnavailable') }}</span>
      </div>
    </div>

    <button class="btn btn-primary mt-3 w-full shrink-0 py-2 text-xs shadow-none disabled:bg-gray-500 disabled:bg-none disabled:from-gray-400 disabled:to-gray-500 disabled:text-white disabled:opacity-100 disabled:shadow-none" type="submit" :disabled="submitting || !canSubmit">
      {{ submitting ? t('imageWorkbench.actions.submitting') : t('imageWorkbench.actions.generate') }}
    </button>
  </form>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import CraftIcon from '@/components/craft/CraftIcon.vue'
import type { ImageWorkbenchCostEstimate, ImageWorkbenchModel, ImageWorkbenchQuality } from '@/api'
import type { ApiKey } from '@/types'
import type { ImageWorkbenchModelCapabilities } from '@/utils/imageWorkbenchModelCapabilities'
import { formatCost } from '../imageWorkbenchFormatters'

const props = defineProps<{
  loadingKeys: boolean
  eligibleKeys: ApiKey[]
  apiKeyId: number
  model: ImageWorkbenchModel
  quality: ImageWorkbenchQuality
  models: ReadonlyArray<ImageWorkbenchModelCapabilities>
  availableQualities: ReadonlyArray<ImageWorkbenchQuality>
  availableAspectRatios: ReadonlyArray<string>
  aspectRatio: string
  width: number
  height: number
  selectedModelMaxEdge: number
  selectedModelMaxPixels: number
  selectedModelTier: string
  dimensionErrorMessage: string
  experimentalDimensions: boolean
  prompt: string
  referenceUrlInput: string
  referenceCount: number
  referenceFiles: File[]
  referenceUrlList: string[]
  referenceDataURLsCount: number
  loadingEstimate: boolean
  costEstimate: ImageWorkbenchCostEstimate | null
  submitting: boolean
  canSubmit: boolean
  formatInteger: (value: number) => string
}>()

const emit = defineEmits<{
  (e: 'submit'): void
  (e: 'reset'): void
  (e: 'select-aspect-ratio', ratio: string): void
  (e: 'normalize-dimensions'): void
  (e: 'reference-files', event: Event): void
  (e: 'drop-reference', event: DragEvent): void
  (e: 'import-reference-url'): void
  (e: 'clear-references'): void
  (e: 'update:apiKeyId', value: number): void
  (e: 'update:model', value: ImageWorkbenchModel): void
  (e: 'update:quality', value: ImageWorkbenchQuality): void
  (e: 'update:width', value: number): void
  (e: 'update:height', value: number): void
  (e: 'update:prompt', value: string): void
  (e: 'update:referenceUrlInput', value: string): void
}>()

const { t } = useI18n()
const isReferenceDragOver = ref(false)
const referenceInputRef = ref<HTMLInputElement | null>(null)

const apiKeyIdModel = computed({
  get: () => props.apiKeyId,
  set: (value: number) => emit('update:apiKeyId', value),
})
const modelValue = computed({
  get: () => props.model,
  set: (value: ImageWorkbenchModel) => emit('update:model', value),
})
const qualityModel = computed({
  get: () => props.quality,
  set: (value: ImageWorkbenchQuality) => emit('update:quality', value),
})
const widthModel = computed({
  get: () => props.width,
  set: (value: number) => emit('update:width', value),
})
const heightModel = computed({
  get: () => props.height,
  set: (value: number) => emit('update:height', value),
})
const promptModel = computed({
  get: () => props.prompt,
  set: (value: string) => emit('update:prompt', value),
})
const referenceUrlInputModel = computed({
  get: () => props.referenceUrlInput,
  set: (value: string) => emit('update:referenceUrlInput', value),
})

function handleReferenceDrop(event: DragEvent) {
  isReferenceDragOver.value = false
  emit('drop-reference', event)
}

function clearFileInput() {
  if (referenceInputRef.value) referenceInputRef.value.value = ''
}

defineExpose({ clearFileInput })
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
