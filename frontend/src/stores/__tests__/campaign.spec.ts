import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'
import { useCampaignStore } from '@/stores/campaign'
import type { NewcomerCampaignStatus } from '@/types/campaign'

const getStatus = vi.hoisted(() => vi.fn())
const reconcile = vi.hoisted(() => vi.fn())

vi.mock('@/api/campaign', () => ({
  default: { getStatus, reconcile },
}))

function statusFixture(overrides: Partial<NewcomerCampaignStatus> = {}): NewcomerCampaignStatus {
  return {
    campaign_key: 'newcomer_202609',
    name: '2026 年 9 月迎新活动',
    phase: 'active',
    starts_at: '2026-08-31T16:00:00.000Z',
    ends_at: '2026-10-01T16:00:00.000Z',
    first_recharge: {
      eligible: true,
      reward_status: 'pending',
      reward_amount: 2,
    },
    invite_link: 'https://example.test/register?aff=A',
    valid_invite_count: 2,
    next_tier_progress: 2,
    next_tier_remaining: 3,
    current_membership: {
      tier_key: 'premium',
      tier_name: '高级',
      factor: 0.98,
      starts_at: '2026-09-02T00:00:00.000Z',
      expires_at: '2099-01-01T00:00:00.000Z',
    },
    tiers: [],
    ...overrides,
  }
}

describe('campaign store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getStatus.mockReset()
    reconcile.mockReset()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('deduplicates requests and keeps status isolated by authenticated user', async () => {
    const authStore = useAuthStore()
    authStore.user = { id: 1 } as never
    const store = useCampaignStore()
    let resolveRequest!: (value: { data: NewcomerCampaignStatus }) => void
    getStatus.mockReturnValue(new Promise((resolve) => { resolveRequest = resolve }))

    const first = store.fetchStatus()
    const second = store.fetchStatus()
    expect(getStatus).toHaveBeenCalledTimes(1)

    resolveRequest({ data: statusFixture() })
    await expect(first).resolves.toMatchObject({ invite_link: expect.stringContaining('aff=A') })
    await expect(second).resolves.toMatchObject({ invite_link: expect.stringContaining('aff=A') })

    authStore.user = { id: 2 } as never
    expect(store.status).toBeNull()
    await nextTick()
    expect(store.loaded).toBe(false)
  })

  it('resets on logout and expires cached status after the TTL', async () => {
    const authStore = useAuthStore()
    authStore.user = { id: 1 } as never
    const store = useCampaignStore()
    getStatus.mockResolvedValue({ data: statusFixture() })

    await store.fetchStatus()
    expect(store.status).not.toBeNull()
    await store.fetchStatus()
    expect(getStatus).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(60_000)
    getStatus.mockResolvedValue({ data: statusFixture({ invite_link: 'https://example.test/register?aff=refreshed' }) })
    await store.fetchStatus()
    expect(getStatus).toHaveBeenCalledTimes(2)

    authStore.user = null
    await nextTick()
    expect(store.status).toBeNull()
    expect(store.loaded).toBe(false)
  })

  it('removes an expired membership from the rendered state automatically', async () => {
    const authStore = useAuthStore()
    authStore.user = { id: 1 } as never
    const store = useCampaignStore()
    getStatus.mockResolvedValue({
      data: statusFixture({
        current_membership: {
          tier_key: 'premium',
          tier_name: '高级',
          factor: 0.98,
          starts_at: '2026-09-01T00:00:00.000Z',
          expires_at: new Date(Date.now() + 1_000).toISOString(),
        },
      }),
    })

    await store.fetchStatus()
    expect(store.status?.current_membership).not.toBeUndefined()
    vi.advanceTimersByTime(30_000)
    expect(store.status?.current_membership).toBeUndefined()
  })

  it('deduplicates explicit reconcile calls and replaces the cached snapshot', async () => {
    const authStore = useAuthStore()
    authStore.user = { id: 1 } as never
    const store = useCampaignStore()
    const response = { data: statusFixture({ valid_invite_count: 5 }) }
    reconcile.mockResolvedValue(response)

    const first = store.reconcile()
    const second = store.reconcile()
    expect(reconcile).toHaveBeenCalledTimes(1)
    await expect(first).resolves.toMatchObject({ valid_invite_count: 5 })
    await expect(second).resolves.toMatchObject({ valid_invite_count: 5 })
    expect(store.status?.valid_invite_count).toBe(5)
  })
})
