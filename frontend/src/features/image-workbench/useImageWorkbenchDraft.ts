import { nextTick, watch } from 'vue'
import type { ComputedRef, Ref } from 'vue'
import type { ImageWorkbenchModel, ImageWorkbenchQuality } from '@/api'
import { validateImageDimensions } from '@/utils/imageWorkbenchDimensions'
import {
  IMAGE_WORKBENCH_MODELS,
  imageWorkbenchModelCapabilities,
  isImageWorkbenchAspectRatioAllowed,
  isImageWorkbenchQualityAllowed,
} from '@/utils/imageWorkbenchModelCapabilities'
import type { ImageWorkbenchDraft, ImageWorkbenchDraftReference } from '@/utils/imageWorkbenchDraft'
import {
  clearImageWorkbenchDraft,
  loadImageWorkbenchDraft,
  saveImageWorkbenchDraft,
} from '@/utils/imageWorkbenchDraft'
import { clampDimension, parseSize } from './imageWorkbenchFormatters'

export interface ImageWorkbenchFormState {
  apiKeyId: number
  model: ImageWorkbenchModel
  quality: ImageWorkbenchQuality
  size: string
  aspectRatio: string
  width: number
  height: number
  prompt: string
  referenceUrls: string
}

export interface UseImageWorkbenchDraftOptions {
  form: ImageWorkbenchFormState
  referenceFiles: Ref<File[]>
  referenceDataURLs: Ref<string[]>
  referenceUrlInput: Ref<string>
  userId: Ref<number> | ComputedRef<number>
  selectedModelMaxEdge: Ref<number> | ComputedRef<number>
  selectedModelMaxPixels: Ref<number> | ComputedRef<number>
}

export function draftReferenceToFile(reference: ImageWorkbenchDraftReference): File | null {
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
      lastModified: reference.lastModified || Date.now(),
    })
  } catch {
    return null
  }
}

export function useImageWorkbenchDraft(options: UseImageWorkbenchDraftOptions) {
  const {
    form,
    referenceFiles,
    referenceDataURLs,
    referenceUrlInput,
    userId,
    selectedModelMaxEdge,
    selectedModelMaxPixels,
  } = options

  let draftSaveTimer: ReturnType<typeof setTimeout> | null = null
  let draftReady = false

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
        referenceUrls: form.referenceUrls,
      },
      referenceUrlInput: referenceUrlInput.value,
      references: referenceDataURLs.value.map((dataURL, index) => {
        const file = referenceFiles.value[index]
        return {
          name: file?.name || `reference-${index + 1}.png`,
          type: file?.type || dataURL.match(/^data:([^;,]+)/i)?.[1] || 'image/png',
          lastModified: file?.lastModified || Date.now(),
          dataURL,
          isFile: Boolean(file),
        }
      }),
    }
  }

  async function restoreDraft() {
    const currentUserId = userId.value
    if (currentUserId <= 0) {
      draftReady = true
      return
    }
    try {
      const draft = await loadImageWorkbenchDraft(currentUserId)
      if (draft) {
        const savedForm = draft.form || ({} as ImageWorkbenchDraft['form'])
        form.apiKeyId = Number(savedForm.apiKeyId) || 0
        form.model = IMAGE_WORKBENCH_MODELS.some(model => model.value === savedForm.model)
          ? (savedForm.model as ImageWorkbenchModel)
          : 'gpt-image-2-1k'
        const modelCapabilities = imageWorkbenchModelCapabilities(form.model)
        form.quality = isImageWorkbenchQualityAllowed(form.model, savedForm.quality)
          ? savedForm.quality
          : modelCapabilities.defaultQuality
        form.aspectRatio = isImageWorkbenchAspectRatioAllowed(form.model, savedForm.aspectRatio)
          ? savedForm.aspectRatio
          : ''
        await nextTick()
        const maxEdge = selectedModelMaxEdge.value
        form.width = clampDimension(Number(savedForm.width) || 1024, maxEdge)
        form.height = clampDimension(Number(savedForm.height) || 1024, maxEdge)
        if (validateImageDimensions(form.width, form.height, selectedModelMaxPixels.value).code) {
          const [defaultWidth, defaultHeight] = parseSize(modelCapabilities.defaultSize)
          form.width = defaultWidth
          form.height = defaultHeight
          form.aspectRatio = ''
        }
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
    const currentUserId = userId.value
    if (currentUserId > 0) void saveImageWorkbenchDraft(currentUserId, currentDraft()).catch(() => undefined)
  }

  function scheduleDraftSave() {
    if (!draftReady) return
    if (draftSaveTimer) clearTimeout(draftSaveTimer)
    draftSaveTimer = setTimeout(() => {
      draftSaveTimer = null
      persistDraftNow()
    }, 250)
  }

  function clearDraft() {
    void clearImageWorkbenchDraft(userId.value)
  }

  function disposeDraftAutoSave() {
    if (draftSaveTimer) clearTimeout(draftSaveTimer)
    draftSaveTimer = null
    persistDraftNow()
  }

  watch([form, referenceFiles, referenceDataURLs, referenceUrlInput], scheduleDraftSave, { deep: true })

  return {
    restoreDraft,
    persistDraftNow,
    clearDraft,
    disposeDraftAutoSave,
  }
}
