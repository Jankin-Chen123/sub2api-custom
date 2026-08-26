import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { listAccountHealth } from '@/api/admin/channelMonitor'

describe('admin channel monitor account health API', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockResolvedValue({ data: { items: [] } })
  })

  it('requests health snapshots within the selected monitor scope', async () => {
    await listAccountHealth(42)

    expect(get).toHaveBeenCalledWith('/admin/channel-monitors/42/account-health', { params: {} })
  })

  it('passes the optional model filter', async () => {
    await listAccountHealth(42, { model: 'gpt-4o-mini' })

    expect(get).toHaveBeenCalledWith('/admin/channel-monitors/42/account-health', {
      params: { model: 'gpt-4o-mini' },
    })
  })
})
