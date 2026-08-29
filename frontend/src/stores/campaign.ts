import { defineStore } from 'pinia'
import { ref } from 'vue'
import newcomerCampaignAPI from '@/api/campaign'
import type { NewcomerCampaignStatus } from '@/types/campaign'

export const useCampaignStore = defineStore('campaign', () => {
  const status = ref<NewcomerCampaignStatus | null>(null)
  const loading = ref(false)
  const loaded = ref(false)

  async function fetchStatus(force = false): Promise<NewcomerCampaignStatus | null> {
    if (loaded.value && !force) return status.value
    if (loading.value) return status.value

    loading.value = true
    try {
      const response = await newcomerCampaignAPI.getStatus()
      status.value = response.data
      loaded.value = true
      return status.value
    } catch (error) {
      // The campaign is time-boxed and optional for older deployments. Keep
      // the rest of the account UI usable when this endpoint is unavailable.
      console.warn('[campaign] Failed to fetch newcomer campaign:', error)
      return null
    } finally {
      loading.value = false
    }
  }

  async function reconcile(): Promise<NewcomerCampaignStatus | null> {
    loading.value = true
    try {
      const response = await newcomerCampaignAPI.reconcile()
      status.value = response.data
      loaded.value = true
      return status.value
    } catch (error) {
      console.warn('[campaign] Failed to reconcile newcomer campaign:', error)
      return null
    } finally {
      loading.value = false
    }
  }

  return { status, loading, loaded, fetchStatus, reconcile }
})
