import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ImageWorkbenchView from '../ImageWorkbenchView.vue'

const listKeys = vi.hoisted(() => vi.fn())
const listJobs = vi.hoisted(() => vi.fn())
const createJob = vi.hoisted(() => vi.fn())
const estimateCost = vi.hoisted(() => vi.fn())
const getJob = vi.hoisted(() => vi.fn())
const getContent = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/api', () => ({
  imageWorkbenchAPI: { createJob, estimateCost, listJobs, getJob, getContent },
  keysAPI: { list: listKeys }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, values?: Record<string, unknown>) => values ? `${key}:${String(values.time || '')}` : key,
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
  status: 'completed',
  operation: 'generation',
  model: 'gpt-image-2-1k',
  requested_size: '1024x1024',
  actual_size: '1024x1024',
  estimated_cost: 0.01,
  settled_cost: 0.01,
  created_at: '2026-08-04T00:00:00.000Z',
  updated_at: '2026-08-04T00:00:00.000Z'
} as const

async function mountWorkbench(jobs: unknown[] = []) {
  listKeys.mockResolvedValue({ items: [eligibleKey] })
  listJobs.mockResolvedValue({ data: jobs, limit: 30, offset: 0 })
  getJob.mockResolvedValue(jobs[0])
  estimateCost.mockResolvedValue({ model: 'gpt-image-2-1k', size_tier: '1K', base_cost: 0.01, rate_multiplier: 1, estimated_cost: 0.01 })
  createJob.mockResolvedValue({ ...completedJob, id: 'imgjob_created' })

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
    showError.mockReset()
    showSuccess.mockReset()
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

  it('refreshes previews through the authenticated content endpoint on every request', async () => {
    getContent.mockResolvedValue(new Blob(['image-bytes'], { type: 'image/png' }))
    const createObjectURL = vi.fn(() => 'blob:preview')
    const revokeObjectURL = vi.fn()
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL })
    const wrapper = await mountWorkbench([completedJob])
    getContent.mockClear()
    createObjectURL.mockClear()
    revokeObjectURL.mockClear()
    await (wrapper.vm as any).loadPreview(completedJob)
    await (wrapper.vm as any).loadPreview(completedJob)

    expect(getContent).toHaveBeenCalledTimes(2)
    expect(createObjectURL).toHaveBeenCalledTimes(2)
    expect(revokeObjectURL).toHaveBeenCalled()
    wrapper.unmount()
    delete (URL as typeof URL & { createObjectURL?: typeof createObjectURL }).createObjectURL
    delete (URL as typeof URL & { revokeObjectURL?: typeof revokeObjectURL }).revokeObjectURL
  })
})
