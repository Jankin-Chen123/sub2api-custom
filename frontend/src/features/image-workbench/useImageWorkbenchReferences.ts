import type { ComputedRef, Ref } from 'vue'
import { validateReferenceFiles } from '@/views/user/imageWorkbenchValidation'
import { readFileAsDataURL } from './imageWorkbenchFormatters'

export interface ImageWorkbenchReferenceFormState {
  referenceUrls: string
}

export interface UseImageWorkbenchReferencesOptions {
  form: ImageWorkbenchReferenceFormState
  referenceFiles: Ref<File[]>
  referenceDataURLs: Ref<string[]>
  referenceUrlInput: Ref<string>
  referenceUrlList: ComputedRef<string[]>
  referenceCount: ComputedRef<number>
  onError: (message: string) => void
  onChanged: () => void
  onClearFileInput: () => void
  translateError: (key: string) => string
}

const REFERENCE_VALIDATION_ERROR_KEYS = {
  too_many_reference_files: 'imageWorkbench.errors.tooManyReferenceFiles',
  reference_file_not_image: 'imageWorkbench.errors.invalidReferenceType',
  reference_file_too_large: 'imageWorkbench.errors.fileTooLarge',
  mask_file_not_png: 'imageWorkbench.errors.invalidMask',
  mask_file_too_large: 'imageWorkbench.errors.fileTooLarge',
} as const

export function useImageWorkbenchReferences(options: UseImageWorkbenchReferencesOptions) {
  const {
    form,
    referenceFiles,
    referenceDataURLs,
    referenceUrlInput,
    referenceUrlList,
    referenceCount,
    onError,
    onChanged,
    onClearFileInput,
    translateError,
  } = options

  async function processReferenceFiles(files: File[]) {
    if (!files.length) return
    if (referenceCount.value + files.length > 9) {
      onError(translateError('imageWorkbench.errors.tooManyReferences'))
      return
    }
    const validationError = validateReferenceFiles(files)
    if (validationError) {
      onError(translateError(REFERENCE_VALIDATION_ERROR_KEYS[validationError]))
      return
    }
    referenceFiles.value = [...referenceFiles.value, ...files]
    referenceDataURLs.value = [
      ...referenceDataURLs.value,
      ...(await Promise.all(files.map(readFileAsDataURL))),
    ]
    onChanged()
  }

  async function onReferenceFiles(event: Event) {
    const input = event.target as HTMLInputElement
    await processReferenceFiles(Array.from(input.files || []))
    input.value = ''
  }

  async function onReferenceDrop(event: DragEvent) {
    await processReferenceFiles(Array.from(event.dataTransfer?.files || []))
  }

  function importReferenceUrl() {
    const url = referenceUrlInput.value.trim()
    if (!url) return
    if (!/^https?:\/\//i.test(url)) {
      onError(translateError('imageWorkbench.errors.invalidReferenceUrl'))
      return
    }
    if (referenceCount.value >= 9) {
      onError(translateError('imageWorkbench.errors.tooManyReferences'))
      return
    }
    form.referenceUrls = [...referenceUrlList.value, url].join('\n')
    referenceUrlInput.value = ''
    onChanged()
  }

  function clearReferences() {
    form.referenceUrls = ''
    referenceUrlInput.value = ''
    referenceFiles.value = []
    referenceDataURLs.value = []
    onClearFileInput()
    onChanged()
  }

  return {
    onReferenceFiles,
    onReferenceDrop,
    importReferenceUrl,
    clearReferences,
  }
}
