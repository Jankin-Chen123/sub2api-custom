import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { createJob, estimateCost } from '../imageWorkbench'

vi.mock('../client', () => ({
  apiClient: { post: vi.fn() }
}))

describe('image workbench API', () => {
  beforeEach(() => {
    vi.mocked(apiClient.post).mockReset()
  })

  it('reuses the caller-provided idempotency key', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({ data: { id: 'imgjob_1' } })

    await createJob({
      api_key_id: 7,
      operation: 'generation',
      model: 'gpt-image-2-2k',
      prompt: 'a dog'
    }, 'stable-submit-key')

    expect(apiClient.post).toHaveBeenCalledWith(
      '/user/image-workbench/jobs',
      expect.objectContaining({ model: 'gpt-image-2-2k' }),
      { headers: { 'Idempotency-Key': 'stable-submit-key' } }
    )
  })

  it('posts a read-only cost estimate without a submission key', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({ data: { estimated_cost: 0.08 } })

    await estimateCost({ api_key_id: 7, model: 'gpt-image-2-4k' })

    expect(apiClient.post).toHaveBeenCalledWith('/user/image-workbench/estimate', {
      api_key_id: 7,
      model: 'gpt-image-2-4k'
    })
  })
})
