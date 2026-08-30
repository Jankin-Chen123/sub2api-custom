import { apiClient } from '../client'
import type {
  NewcomerCampaignAdminConfig,
  NewcomerCampaignAdminMembershipInput,
  NewcomerCampaignAdminUserMembership,
  NewcomerCampaignTier,
} from '@/types/campaign'

export const newcomerCampaignAdminAPI = {
  async getConfig(): Promise<NewcomerCampaignAdminConfig> {
    const { data } = await apiClient.get<NewcomerCampaignAdminConfig>('/admin/campaigns/newcomer/config')
    return data
  },

  async updateConfig(
    tiers: NewcomerCampaignTier[],
    startsAt?: string,
    endsAt?: string
  ): Promise<NewcomerCampaignAdminConfig> {
    const { data } = await apiClient.put<NewcomerCampaignAdminConfig>('/admin/campaigns/newcomer/config', {
      tiers,
      starts_at: startsAt,
      ends_at: endsAt,
    })
    return data
  },

  async getUserMembership(userId: number): Promise<NewcomerCampaignAdminUserMembership> {
    const { data } = await apiClient.get<NewcomerCampaignAdminUserMembership>(
      `/admin/campaigns/newcomer/users/${userId}/membership`
    )
    return data
  },

  async setUserMembership(
    userId: number,
    input: NewcomerCampaignAdminMembershipInput
  ): Promise<NewcomerCampaignAdminUserMembership> {
    const { data } = await apiClient.put<NewcomerCampaignAdminUserMembership>(
      `/admin/campaigns/newcomer/users/${userId}/membership`,
      input
    )
    return data
  },

  async clearUserMembership(userId: number): Promise<NewcomerCampaignAdminUserMembership> {
    const { data } = await apiClient.delete<NewcomerCampaignAdminUserMembership>(
      `/admin/campaigns/newcomer/users/${userId}/membership`
    )
    return data
  },

  async reconcile(): Promise<{ repaired_users: number }> {
    const { data } = await apiClient.post<{ repaired_users: number }>('/admin/campaigns/newcomer/reconcile')
    return data
  },
}

export default newcomerCampaignAdminAPI
