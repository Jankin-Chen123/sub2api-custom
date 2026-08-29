import { apiClient } from './client'
import type { NewcomerCampaignStatus } from '@/types/campaign'

export const newcomerCampaignAPI = {
  getStatus() {
    return apiClient.get<NewcomerCampaignStatus>('/user/campaigns/newcomer')
  },
  reconcile() {
    return apiClient.post<NewcomerCampaignStatus>('/user/campaigns/newcomer/reconcile')
  },
}

export default newcomerCampaignAPI
