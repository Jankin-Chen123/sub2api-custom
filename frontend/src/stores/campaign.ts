import { computed, reactive, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { useAuthStore } from '@/stores/auth'
import newcomerCampaignAPI from '@/api/campaign'
import type { NewcomerCampaignStatus } from '@/types/campaign'

const CAMPAIGN_STATUS_TTL_MS = 60_000
const CAMPAIGN_EXPIRY_TICK_MS = 30_000

interface CampaignCacheEntry {
  status: NewcomerCampaignStatus | null
  loadedAt: number
  request: Promise<NewcomerCampaignStatus | null> | null
  reconcileRequest: Promise<NewcomerCampaignStatus | null> | null
}

function hasExpiredMembership(status: NewcomerCampaignStatus): boolean {
  const expiresAt = status.current_membership?.expires_at
  if (!expiresAt) return false

  const expiresAtMs = new Date(expiresAt).getTime()
  return Number.isFinite(expiresAtMs) && expiresAtMs <= Date.now()
}

function withoutExpiredMembership(status: NewcomerCampaignStatus): NewcomerCampaignStatus {
  if (!hasExpiredMembership(status)) return status
  return { ...status, current_membership: undefined }
}

export const useCampaignStore = defineStore('campaign', () => {
  const authStore = useAuthStore()
  const cache = reactive(new Map<number, CampaignCacheEntry>())
  const expiryTick = ref(0)
  let expiryTimer: ReturnType<typeof setInterval> | null = null

  const userID = computed(() => {
    const id = Number(authStore.user?.id)
    return Number.isInteger(id) && id > 0 ? id : null
  })

  const activeEntry = computed(() => {
    // Make membership expiry reactive without mutating the server response.
    void expiryTick.value
    return userID.value === null ? undefined : cache.get(userID.value)
  })

  const status = computed(() => {
    void expiryTick.value
    const value = activeEntry.value?.status
    return value ? withoutExpiredMembership(value) : null
  })

  const loading = computed(() => Boolean(activeEntry.value?.request || activeEntry.value?.reconcileRequest))
  const loaded = computed(() => Boolean(activeEntry.value?.loadedAt))

  function startExpiryTicker(): void {
    if (expiryTimer) return
    expiryTimer = setInterval(() => {
      expiryTick.value += 1
    }, CAMPAIGN_EXPIRY_TICK_MS)
  }

  function reset(): void {
    cache.clear()
    expiryTick.value += 1
  }

  function getEntry(id: number): CampaignCacheEntry {
    const existing = cache.get(id)
    if (existing) return existing
    const entry: CampaignCacheEntry = {
      status: null,
      loadedAt: 0,
      request: null,
      reconcileRequest: null,
    }
    cache.set(id, entry)
    // reactive(Map) returns a proxy for stored objects. Use that same proxy
    // for identity checks so a late response cannot update a replaced entry.
    return cache.get(id)!
  }

  function visibleStatusFor(id: number, entry: CampaignCacheEntry): NewcomerCampaignStatus | null {
    if (userID.value !== id) return null
    return entry.status ? withoutExpiredMembership(entry.status) : null
  }

  async function fetchStatus(force = false): Promise<NewcomerCampaignStatus | null> {
    const id = userID.value
    if (id === null) {
      reset()
      return null
    }

    startExpiryTicker()
    const entry = getEntry(id)
    const now = Date.now()
    if (!force && entry.status && entry.loadedAt > 0 && now - entry.loadedAt < CAMPAIGN_STATUS_TTL_MS) {
      return visibleStatusFor(id, entry)
    }
    if (entry.request) return entry.request

    const request = newcomerCampaignAPI.getStatus()
      .then((response) => {
        if (userID.value === id && cache.get(id) === entry) {
          entry.status = response.data
          entry.loadedAt = Date.now()
        }
        return visibleStatusFor(id, entry)
      })
      .catch((error) => {
        // The campaign is time-boxed and optional for older deployments. Keep
        // the rest of the account UI usable when this endpoint is unavailable.
        console.warn('[campaign] Failed to fetch newcomer campaign:', error)
        return null
      })

    entry.request = request
    request.then(
      () => { if (cache.get(id) === entry) entry.request = null },
      () => { if (cache.get(id) === entry) entry.request = null },
    )
    return request
  }

  async function reconcile(): Promise<NewcomerCampaignStatus | null> {
    const id = userID.value
    if (id === null) {
      reset()
      return null
    }

    startExpiryTicker()
    const entry = getEntry(id)
    if (entry.reconcileRequest) return entry.reconcileRequest

    const request = (async () => {
      // Let an initial GET settle before the repair response replaces its
      // snapshot. The POST itself is still deduplicated below.
      if (entry.request) await entry.request.catch(() => null)
      const response = await newcomerCampaignAPI.reconcile()
      if (userID.value === id && cache.get(id) === entry) {
        entry.status = response.data
        entry.loadedAt = Date.now()
      }
      return visibleStatusFor(id, entry)
    })().catch((error) => {
      console.warn('[campaign] Failed to reconcile newcomer campaign:', error)
      return null
    })

    entry.reconcileRequest = request
    request.then(
      () => { if (cache.get(id) === entry) entry.reconcileRequest = null },
      () => { if (cache.get(id) === entry) entry.reconcileRequest = null },
    )
    return request
  }

  // A logout or account switch must invalidate the old user's snapshot before
  // another component can render it. The computed status is also null during
  // the transition, so a stale in-flight response cannot leak across users.
  watch(userID, (next, previous) => {
    if (next === null || (previous !== undefined && next !== previous)) reset()
  }, { immediate: true })

  return { userID, status, loading, loaded, fetchStatus, reconcile, reset }
})
