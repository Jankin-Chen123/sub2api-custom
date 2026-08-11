import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ImageWorkbenchView from '../ImageWorkbenchView.vue'

const listKeys = vi.hoisted(() => vi.fn())
const listJobs = vi.hoisted(() => vi.fn())
const createJob = vi.hoisted(() => vi.fn())
const estimateCost = vi.hoisted(() => vi.fn())
const getJob = vi.hoisted(() => vi.fn())
const getContent = vi.hoisted(() => vi.fn())
const renameJob = vi.hoisted(() => vi.fn())
const deleteJob = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const getCachedImageWorkbenchBlob = vi.hoisted(() => vi.fn())
const listCachedImageWorkbenchEntries = vi.hoisted(() => vi.fn())
const putCachedImageWorkbenchBlob = vi.hoisted(() => vi.fn())
const deleteCachedImageWorkbenchEntry = vi.hoisted(() => vi.fn())
const loadImageWorkbenchDraft = vi.hoisted(() => vi.fn())
const saveImageWorkbenchDraft = vi.hoisted(() => vi.fn())
const clearImageWorkbenchDraft = vi.hoisted(() => vi.fn())

vi.mock('@/api', () => ({
  imageWorkbenchAPI: { createJob, estimateCost, listJobs, getJob, getContent, renameJob, deleteJob },
  keysAPI: { list: listKeys }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: 42 } })
}))

vi.mock('@/utils/imageWorkbenchCache', () => ({
  getCachedImageWorkbenchBlob,
  listCachedImageWorkbenchEntries,
  putCachedImageWorkbenchBlob,
  deleteCachedImageWorkbenchEntry
}))

vi.mock('@/utils/imageWorkbenchDraft', () => ({
  loadImageWorkbenchDraft,
  saveImageWorkbenchDraft,
  clearImageWorkbenchDraft
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, values?: Record<string, unknown>) => values ? `${key}:${String(values.time || values.value || '')}` : key,
      locale: { value: 'zh-CN' }
    })
  }
})

const eligibleKey = {
  id: 7,
  name: 'image-key',
  status: 'active',
  group_id: 3,
  group: { name: 'image-group', platform: 'openai', allow_image_generation: true }
}

const completedJob = {
  id: 'imgjob_completed',
  name: 'a mountain lake at sunset',
  status: 'completed',
  operation: 'generation',
  model: 'gpt-image-2-1k',
  requested_size: '1024x1024',
  actual_size: '1024x1024',
  quality: 'high',
  estimated_cost: 0.01,
  settled_cost: 0.01,
  created_at: '2026-08-04T00:00:00.000Z',
  updated_at: '2026-08-04T00:00:00.000Z'
} as const

async function mountWorkbench(jobs: unknown[] = [], keys: unknown[] = [eligibleKey], createdJob: unknown = { ...completedJob, id: 'imgjob_created' }) {
  listKeys.mockResolvedValue({ items: keys })
  listJobs.mockResolvedValue({ data: jobs, limit: 30, offset: 0 })
  getJob.mockResolvedValue(jobs[0])
  estimateCost.mockResolvedValue({ model: 'gpt-image-2-1k', size_tier: '1K', base_cost: 0.01, rate_multiplier: 1, estimated_cost: 0.01 })
  createJob.mockResolvedValue(createdJob)

  const wrapper = shallowMount(ImageWorkbenchView, {
    global: {
      stubs: { AppLayout: { template: '<div><slot /></div>' } }
    }
  })
  await flushPromises()
  return wrapper
}

function setInputFiles(input: HTMLInputElement, files: File[]) {
  Object.defineProperty(input, 'files', { configurable: true, value: files })
}

describe('ImageWorkbenchView', () => {
  beforeEach(() => {
    vi.useRealTimers()
    listKeys.mockReset()
    listJobs.mockReset()
    createJob.mockReset()
    estimateCost.mockReset()
    getJob.mockReset()
    getContent.mockReset()
    renameJob.mockReset()
    deleteJob.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    getCachedImageWorkbenchBlob.mockReset()
    listCachedImageWorkbenchEntries.mockReset()
    putCachedImageWorkbenchBlob.mockReset()
    deleteCachedImageWorkbenchEntry.mockReset()
    loadImageWorkbenchDraft.mockReset()
    saveImageWorkbenchDraft.mockReset()
    clearImageWorkbenchDraft.mockReset()
    getCachedImageWorkbenchBlob.mockResolvedValue(null)
    listCachedImageWorkbenchEntries.mockResolvedValue([])
    putCachedImageWorkbenchBlob.mockResolvedValue(undefined)
    deleteCachedImageWorkbenchEntry.mockResolvedValue(undefined)
    deleteJob.mockResolvedValue(undefined)
    loadImageWorkbenchDraft.mockResolvedValue(null)
    saveImageWorkbenchDraft.mockResolvedValue(undefined)
    clearImageWorkbenchDraft.mockResolvedValue(undefined)
  })

  it('restores the workbench draft, including uploaded reference images, for the current tab session', async () => {
    const referenceDataURL = 'data:image/png;base64,aW1hZ2U='
    loadImageWorkbenchDraft.mockResolvedValue({
      form: {
        apiKeyId: 7,
        model: 'gpt-image-2-2k',
        quality: 'high',
        size: '1536x1024',
        aspectRatio: '3:2',
        width: 1536,
        height: 1024,
        prompt: 'a red fox in snow',
        referenceUrls: 'https://example.com/reference.png'
      },
      referenceUrlInput: 'https://example.com/next.png',
      references: [{
        name: 'reference.png',
        type: 'image/png',
        lastModified: 123,
        dataURL: referenceDataURL,
        isFile: true
      }]
    })

    const wrapper = await mountWorkbench()
    const vm = wrapper.vm as any

    expect(loadImageWorkbenchDraft).toHaveBeenCalledWith(42)
    expect(vm.form.model).toBe('gpt-image-2-2k')
    expect(vm.form.quality).toBe('high')
    expect(vm.form.width).toBe(1536)
    expect(vm.form.height).toBe(1024)
    expect(vm.form.prompt).toBe('a red fox in snow')
    expect(vm.form.referenceUrls).toBe('https://example.com/reference.png')
    expect(vm.referenceUrlInput).toBe('https://example.com/next.png')
    expect(vm.referenceDataURLs).toEqual([referenceDataURL])
    expect(vm.referenceFiles[0].name).toBe('reference.png')
    wrapper.unmount()
  })

  it('estimates cost after a restored eligible key finishes loading without a model change', async () => {
    loadImageWorkbenchDraft.mockResolvedValue({
      form: {
        apiKeyId: 7,
        model: 'gpt-image-2-1k',
        quality: 'auto',
        size: '1024x1024',
        aspectRatio: '',
        width: 1024,
        height: 1024,
        prompt: '',
        referenceUrls: ''
      },
      referenceUrlInput: '',
      references: []
    })

    const wrapper = await mountWorkbench()

    expect(estimateCost).toHaveBeenCalledWith({ api_key_id: 7, model: 'gpt-image-2-1k' })
    wrapper.unmount()
  })

  it('debounces draft persistence while the user edits form fields and references', async () => {
    vi.useFakeTimers()
    const wrapper = await mountWorkbench()
    const vm = wrapper.vm as any
    const referenceDataURL = 'data:image/png;base64,aW1hZ2U='

    vm.form.prompt = 'a quiet lake'
    vm.form.width = 1536
    vm.referenceUrlInput = 'https://example.com/reference.png'
    vm.referenceDataURLs.push(referenceDataURL)
    vm.referenceFiles.push(new File(['image'], 'reference.png', { type: 'image/png' }))
    await vi.advanceTimersByTimeAsync(300)

    expect(saveImageWorkbenchDraft).toHaveBeenCalledWith(42, expect.objectContaining({
      form: expect.objectContaining({ prompt: 'a quiet lake', width: 1536 }),
      referenceUrlInput: 'https://example.com/reference.png',
      references: [expect.objectContaining({ name: 'reference.png', dataURL: referenceDataURL, isFile: true })]
    }))
    wrapper.unmount()
  })

  it('shows only the image-key empty state until an eligible key exists', async () => {
    const emptyWrapper = await mountWorkbench([], [])

    expect(emptyWrapper.find('[data-testid="no-image-api-key"]').text()).toBe('imageWorkbench.form.noApiKey')
    expect(emptyWrapper.find('select[required]').exists()).toBe(false)
    expect(emptyWrapper.text()).not.toContain('imageWorkbench.form.confirmSize')
    emptyWrapper.unmount()

    const keyedWrapper = await mountWorkbench()
    expect(keyedWrapper.find('[data-testid="no-image-api-key"]').exists()).toBe(false)
    expect(keyedWrapper.find('select[required]').text()).toContain('image-key')
    keyedWrapper.unmount()
  })

  it('shows a canvas before any task and switches to the submitted task dimensions', async () => {
    const queuedJob = {
      ...completedJob,
      id: 'imgjob_queued',
      status: 'queued',
      requested_size: '1024x768',
      actual_size: ''
    }
    const wrapper = await mountWorkbench([], [eligibleKey], queuedJob)

    expect(wrapper.find('[data-testid="preview-canvas"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="preview-canvas"]').text()).toContain('1024x1024')

    ;(wrapper.vm as any).form.width = 1024
    ;(wrapper.vm as any).form.height = 768
    ;(wrapper.vm as any).form.prompt = 'a bird on a branch'
    await (wrapper.vm as any).submitJob()
    await flushPromises()

    expect(createJob).toHaveBeenCalledWith(expect.objectContaining({ size: '1024x768' }))
    expect(wrapper.find('[data-testid="preview-canvas"]').text()).toContain('1024x768')
    expect(wrapper.find('[data-testid="preview-canvas"]').text()).toContain('imageWorkbench.status.queued')
    wrapper.unmount()
  })

  it('uses the prompt-backed artwork name and shows resolution and quality in both card types', async () => {
    const wrapper = await mountWorkbench([completedJob])

    expect(wrapper.text()).toContain('a mountain lake at sunset')
    expect(wrapper.find('[data-testid="batch-metadata-imgjob_completed"]').text()).toContain('1K')
    expect(wrapper.find('[data-testid="batch-metadata-imgjob_completed"]').text()).toContain('imageWorkbench.form.qualityOptions.high')
    expect(wrapper.find('[data-testid="library-metadata-imgjob_completed"]').text()).toContain('1K')
    expect(wrapper.find('[data-testid="library-metadata-imgjob_completed"]').text()).toContain('imageWorkbench.form.qualityOptions.high')
    wrapper.unmount()
  })

  it('does not use the resolution or model as the title of a legacy unnamed artwork', async () => {
    const unnamedJob = { ...completedJob, name: undefined, model: 'gpt-image-2-4k' as const }
    const wrapper = await mountWorkbench([unnamedJob])

    expect((wrapper.vm as any).jobDisplayName(unnamedJob)).toBe('imageWorkbench.library.untitled')
    expect((wrapper.vm as any).jobDisplayName(unnamedJob)).not.toContain('4K')
    wrapper.unmount()
  })

  it('submits from the prompt with Ctrl/⌘ + Enter and uses the eligible key', async () => {
    const wrapper = await mountWorkbench()
    const prompt = wrapper.find('textarea')

    await prompt.setValue('a dog under a blue sky')
    await prompt.trigger('keydown', { key: 'Enter', ctrlKey: true })
    await flushPromises()

    expect(createJob).toHaveBeenCalledWith(expect.objectContaining({
      api_key_id: 7,
      model: 'gpt-image-2-1k',
      prompt: 'a dog under a blue sky',
      operation: 'generation'
    }))
    expect(showSuccess).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('rejects non-image and excessive reference uploads before reading them', async () => {
    const wrapper = await mountWorkbench()
    const referenceInput = wrapper.findAll('input[type="file"]')[0]!
    const input = referenceInput.element as HTMLInputElement
    setInputFiles(input, [new File(['notes'], 'notes.txt', { type: 'text/plain' })])
    await referenceInput.trigger('change')
    expect(showError).toHaveBeenCalledWith('imageWorkbench.errors.invalidReferenceType')

    showError.mockReset()
    setInputFiles(input, Array.from({ length: 10 }, (_, index) => new File(['x'], `${index}.png`, { type: 'image/png' })))
    await referenceInput.trigger('change')
    expect(showError).toHaveBeenCalledWith('imageWorkbench.errors.tooManyReferences')
    wrapper.unmount()
  })

  it('keeps the responsive layout hooks and reports a preview failure', async () => {
    getContent.mockRejectedValue(new Error('not found'))
    const wrapper = await mountWorkbench([completedJob])

    expect(wrapper.find('.sm\\:flex-row').exists()).toBe(true)
    expect(wrapper.find('.lg\\:grid-cols-\\[340px_minmax\\(0\\,1fr\\)\\]').exists()).toBe(true)

    await flushPromises()
    expect(showError).toHaveBeenCalledWith('not found')
    wrapper.unmount()
  })

  it('shows a live sync wait time for pending jobs and freezes it after completion', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-04T00:00:05.000Z'))
    const pendingJob = {
      ...completedJob,
      id: 'imgjob_pending',
      status: 'in_progress',
      created_at: '2026-08-04T00:00:00.000Z',
      updated_at: '2026-08-04T00:00:00.000Z'
    } as const
    getJob.mockResolvedValue(pendingJob)
    const wrapper = await mountWorkbench([pendingJob])

    expect(wrapper.text()).toContain('imageWorkbench.preview.syncWait:00:05')
    await vi.advanceTimersByTimeAsync(2000)
    expect(wrapper.text()).toContain('imageWorkbench.preview.syncWait:00:07')

    const completed = { ...pendingJob, status: 'completed', updated_at: '2026-08-04T00:00:08.000Z' } as const
    getJob.mockResolvedValue(completed)
    await (wrapper.vm as any).refreshPendingJobs()
    expect(wrapper.text()).toContain('imageWorkbench.preview.syncWait:00:08')
    wrapper.unmount()
  })

  it('stores the first authenticated image response and reuses it for previews and downloads', async () => {
    const imageBlob = new Blob(['image-bytes'], { type: 'image/png' })
    getContent.mockResolvedValue(imageBlob)
    const createObjectURL = vi.fn(() => 'blob:preview')
    const revokeObjectURL = vi.fn()
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL })
    const wrapper = await mountWorkbench([completedJob])

    expect(getContent).toHaveBeenCalledTimes(1)
    expect(putCachedImageWorkbenchBlob).toHaveBeenCalledWith(42, completedJob, imageBlob)
    await (wrapper.vm as any).loadPreview(completedJob)
    await (wrapper.vm as any).downloadResult(completedJob)

    expect(getContent).toHaveBeenCalledTimes(1)
    expect(createObjectURL).toHaveBeenCalledTimes(2)
    expect(revokeObjectURL).toHaveBeenCalled()
    expect(click).toHaveBeenCalled()
    wrapper.unmount()
    click.mockRestore()
    delete (URL as typeof URL & { createObjectURL?: typeof createObjectURL }).createObjectURL
    delete (URL as typeof URL & { revokeObjectURL?: typeof revokeObjectURL }).revokeObjectURL
  })

  it('restores cached library images after reopening and downloads without fetching image content again', async () => {
    const cachedBlob = new Blob(['persisted-image'], { type: 'image/png' })
    listCachedImageWorkbenchEntries.mockResolvedValue([{ job: completedJob, blob: cachedBlob }])
    getCachedImageWorkbenchBlob.mockResolvedValue(cachedBlob)
    const createObjectURL = vi.fn(() => 'blob:cached-preview')
    const revokeObjectURL = vi.fn()
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL })

    const wrapper = await mountWorkbench([completedJob])
    await (wrapper.vm as any).downloadResult(completedJob)

    expect(listCachedImageWorkbenchEntries).toHaveBeenCalledWith(42)
    expect(getContent).not.toHaveBeenCalled()
    expect(putCachedImageWorkbenchBlob).not.toHaveBeenCalled()
    expect(click).toHaveBeenCalled()
    wrapper.unmount()
    click.mockRestore()
    delete (URL as typeof URL & { createObjectURL?: typeof createObjectURL }).createObjectURL
    delete (URL as typeof URL & { revokeObjectURL?: typeof revokeObjectURL }).revokeObjectURL
  })

  it('submits editor reference images and masks as PNG data URLs', async () => {
    const pngDataURL = 'data:image/png;base64,cG5n'
    const drawImage = vi.fn()
    const closeBitmap = vi.fn()
    const maskCompositeOperations: string[] = []
    const canvasContext = {
      drawImage,
      clearRect: vi.fn(),
      fillRect: vi.fn(),
      save: vi.fn(),
      restore: vi.fn(),
      fillStyle: '',
      get globalCompositeOperation() { return maskCompositeOperations.at(-1) || 'source-over' },
      set globalCompositeOperation(value: string) { maskCompositeOperations.push(value) }
    }
    const getContext = vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(canvasContext as unknown as CanvasRenderingContext2D)
    const toDataURL = vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL').mockReturnValue(pngDataURL)
    vi.stubGlobal('createImageBitmap', vi.fn().mockResolvedValue({ width: 1024, height: 1024, close: closeBitmap }))
    getContent.mockResolvedValue(new Blob(['image-bytes'], { type: 'image/jpeg' }))
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: vi.fn(() => 'blob:preview') })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() })

    const wrapper = await mountWorkbench([completedJob])
    await (wrapper.vm as any).openEditor(completedJob)
    ;(wrapper.vm as any).redrawBrushStrokes()
    ;(wrapper.vm as any).editor.prompt = 'add a flower'
    ;(wrapper.vm as any).editor.hasMarks = true
    await (wrapper.vm as any).submitEditFromEditor()

    expect(toDataURL).toHaveBeenCalledWith('image/png')
    expect(createJob).toHaveBeenCalledWith(expect.objectContaining({
      operation: 'edit',
      images: [pngDataURL],
      mask: pngDataURL
    }))
    expect(drawImage).toHaveBeenCalled()
    expect(closeBitmap).toHaveBeenCalled()
    expect(canvasContext.fillRect).toHaveBeenCalled()
    expect(maskCompositeOperations).toContain('destination-out')
    wrapper.unmount()
    getContext.mockRestore()
    toDataURL.mockRestore()
    vi.unstubAllGlobals()
    delete (URL as typeof URL & { createObjectURL?: () => string }).createObjectURL
    delete (URL as typeof URL & { revokeObjectURL?: () => void }).revokeObjectURL
  })

  it('renames a library artwork and immediately uses the same name in the task queue', async () => {
    const imageBlob = new Blob(['image-bytes'], { type: 'image/png' })
    getContent.mockResolvedValue(imageBlob)
    renameJob.mockResolvedValue({ ...completedJob, name: '蓝色知更鸟' })
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: vi.fn(() => 'blob:preview') })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() })
    const wrapper = await mountWorkbench([completedJob])

    await wrapper.find('[data-testid="rename-work-imgjob_completed"]').trigger('click')
    await wrapper.find('[data-testid="work-name-input"]').setValue('蓝色知更鸟')
    await wrapper.find('[data-testid="work-name-input"]').element.closest('form')?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()

    expect(renameJob).toHaveBeenCalledWith('imgjob_completed', '蓝色知更鸟')
    expect((wrapper.text().match(/蓝色知更鸟/g) || []).length).toBeGreaterThanOrEqual(2)
    wrapper.unmount()
    delete (URL as typeof URL & { createObjectURL?: () => string }).createObjectURL
    delete (URL as typeof URL & { revokeObjectURL?: () => void }).revokeObjectURL
  })

  it('opens a full artwork preview when the library image is clicked', async () => {
    getContent.mockResolvedValue(new Blob(['image-bytes'], { type: 'image/png' }))
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: vi.fn(() => 'blob:preview') })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() })
    const wrapper = await mountWorkbench([completedJob])

    await wrapper.find(`button[aria-label="imageWorkbench.actions.previewArtwork:"]`).trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="artwork-preview"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="artwork-preview"] img').attributes('src')).toBe('blob:preview')
    expect((wrapper.vm as any).selectedJobId).toBe(completedJob.id)
    wrapper.unmount()
    delete (URL as typeof URL & { createObjectURL?: () => string }).createObjectURL
    delete (URL as typeof URL & { revokeObjectURL?: () => void }).revokeObjectURL
  })

  it('deletes a completed artwork from both the library and generation batch and clears its cache', async () => {
    getContent.mockResolvedValue(new Blob(['image-bytes'], { type: 'image/png' }))
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: vi.fn(() => 'blob:preview') })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() })
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = await mountWorkbench([completedJob])

    expect(wrapper.find('[data-testid="delete-batch-imgjob_completed"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="delete-library-imgjob_completed"]').exists()).toBe(true)
    await wrapper.find('[data-testid="delete-library-imgjob_completed"]').trigger('click')
    await flushPromises()

    expect(deleteJob).toHaveBeenCalledWith('imgjob_completed')
    expect(deleteCachedImageWorkbenchEntry).toHaveBeenCalledWith(42, 'imgjob_completed')
    expect(wrapper.find('[data-testid="delete-batch-imgjob_completed"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="delete-library-imgjob_completed"]').exists()).toBe(false)
    expect(showSuccess).toHaveBeenCalledWith('imageWorkbench.messages.deleted')
    wrapper.unmount()
    confirm.mockRestore()
    delete (URL as typeof URL & { createObjectURL?: () => string }).createObjectURL
    delete (URL as typeof URL & { revokeObjectURL?: () => void }).revokeObjectURL
  })

  it('deletes failed batch jobs but keeps deletion disabled while a job is running', async () => {
    const failedJob = { ...completedJob, id: 'imgjob_failed', status: 'failed' as const }
    const runningJob = { ...completedJob, id: 'imgjob_running', status: 'in_progress' as const }
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = await mountWorkbench([failedJob, runningJob])

    expect(wrapper.find('[data-testid="delete-batch-imgjob_running"]').attributes('disabled')).toBeDefined()
    await wrapper.find('[data-testid="delete-batch-imgjob_failed"]').trigger('click')
    await flushPromises()

    expect(deleteJob).toHaveBeenCalledWith('imgjob_failed')
    expect(wrapper.find('[data-testid="delete-batch-imgjob_failed"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="delete-batch-imgjob_running"]').exists()).toBe(true)
    wrapper.unmount()
    confirm.mockRestore()
  })
})
