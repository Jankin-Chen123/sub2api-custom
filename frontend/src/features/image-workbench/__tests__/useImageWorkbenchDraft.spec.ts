import { computed, reactive, ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ImageWorkbenchFormState } from '../useImageWorkbenchDraft'
import { draftReferenceToFile, useImageWorkbenchDraft } from '../useImageWorkbenchDraft'
import { extractErrorMessage } from '../imageWorkbenchFormatters'

const loadDraftMock = vi.fn()
const saveDraftMock = vi.fn()
const clearDraftMock = vi.fn()

vi.mock('@/utils/imageWorkbenchDraft', () => ({
  loadImageWorkbenchDraft: (...args: unknown[]) => loadDraftMock(...args),
  saveImageWorkbenchDraft: (...args: unknown[]) => saveDraftMock(...args),
  clearImageWorkbenchDraft: (...args: unknown[]) => clearDraftMock(...args),
}))

describe('draftReferenceToFile', () => {
  it('reconstructs a File from a persisted base64 data URL reference', async () => {
    const file = draftReferenceToFile({
      name: 'sketch.png',
      type: 'image/png',
      lastModified: 1700000000000,
      dataURL: 'data:image/png;base64,aGVsbG8=',
      isFile: true,
    })

    expect(file).toBeInstanceOf(File)
    expect(file?.name).toBe('sketch.png')
    expect(file?.type).toBe('image/png')
    expect(file?.lastModified).toBe(1700000000000)
    expect(file?.size).toBe(5)
    const content = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(String(reader.result || ''))
      reader.onerror = () => reject(reader.error)
      reader.readAsText(file as Blob)
    })
    expect(content).toBe('hello')
  })

  it('returns null for non-file or malformed references', () => {
    expect(
      draftReferenceToFile({
        name: 'url.png',
        type: 'image/png',
        lastModified: 1,
        dataURL: 'data:image/png;base64,aGVsbG8=',
        isFile: false,
      })
    ).toBeNull()
    expect(
      draftReferenceToFile({
        name: 'bad.png',
        type: 'image/png',
        lastModified: 1,
        dataURL: 'invalid-data-url',
        isFile: true,
      })
    ).toBeNull()
  })
})

describe('useImageWorkbenchDraft', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    saveDraftMock.mockResolvedValue(undefined)
    clearDraftMock.mockResolvedValue(undefined)
  })

  it('restores valid draft fields and resets incompatible dimensions to model default', async () => {
    loadDraftMock.mockResolvedValue({
      form: {
        apiKeyId: 12,
        model: 'gpt-image-2-1k',
        quality: 'high',
        size: '9999x9999',
        aspectRatio: 'invalid-ratio',
        width: 9999,
        height: 9999,
        prompt: 'a clean desk diagram',
        referenceUrls: 'https://example.com/a.png',
      },
      referenceUrlInput: 'https://example.com/pending.png',
      references: [
        {
          name: 'ref.png',
          type: 'image/png',
          lastModified: 1234,
          dataURL: 'data:image/png;base64,aGVsbG8=',
          isFile: true,
        },
      ],
    })

    const form = reactive<ImageWorkbenchFormState>({
      apiKeyId: 0,
      model: 'gpt-image-2-1k',
      quality: 'medium',
      size: '1024x1024',
      aspectRatio: '',
      width: 1024,
      height: 1024,
      prompt: '',
      referenceUrls: '',
    })
    const referenceFiles = ref<File[]>([])
    const referenceDataURLs = ref<string[]>([])
    const referenceUrlInput = ref('')
    const userId = ref(7)
    const selectedModelMaxEdge = computed(() => 3840)
    const selectedModelMaxPixels = computed(() => 1048576)

    const { restoreDraft, persistDraftNow, clearDraft } = useImageWorkbenchDraft({
      form,
      referenceFiles,
      referenceDataURLs,
      referenceUrlInput,
      userId,
      selectedModelMaxEdge,
      selectedModelMaxPixels,
    })

    await restoreDraft()

    expect(form.apiKeyId).toBe(12)
    expect(form.quality).toBe('high')
    expect(form.aspectRatio).toBe('')
    expect(form.width).toBe(1024)
    expect(form.height).toBe(1024)
    expect(form.size).toBe('1024x1024')
    expect(form.prompt).toBe('a clean desk diagram')
    expect(referenceUrlInput.value).toBe('https://example.com/pending.png')
    expect(referenceDataURLs.value).toEqual(['data:image/png;base64,aGVsbG8='])
    expect(referenceFiles.value).toHaveLength(1)
    expect(referenceFiles.value[0]?.name).toBe('ref.png')

    persistDraftNow()
    expect(saveDraftMock).toHaveBeenCalledWith(
      7,
      expect.objectContaining({
        referenceUrlInput: 'https://example.com/pending.png',
        form: expect.objectContaining({ apiKeyId: 12, prompt: 'a clean desk diagram' }),
      })
    )

    clearDraft()
    expect(clearDraftMock).toHaveBeenCalledWith(7)
  })
})

describe('extractErrorMessage', () => {
  it('extracts messages from Error objects, message records, strings, and falls back otherwise', () => {
    expect(extractErrorMessage(new Error('boom'), 'fallback')).toBe('boom')
    expect(extractErrorMessage({ message: 'request failed' }, 'fallback')).toBe('request failed')
    expect(extractErrorMessage('direct string error', 'fallback')).toBe('direct string error')
    expect(extractErrorMessage({ message: '' }, 'fallback')).toBe('fallback')
    expect(extractErrorMessage(null, 'fallback')).toBe('fallback')
  })
})
