import { apiClient } from '../client'
import type { CheckinPrize } from '../checkin'

export interface CheckinConfig {
  streak_bonus_amount: number
  streak_target: number
}

export async function getConfig(): Promise<CheckinConfig> {
  const { data } = await apiClient.get<CheckinConfig>('/admin/check-in/config')
  return data
}

export async function updateConfig(streakBonusAmount: number): Promise<CheckinConfig> {
  const { data } = await apiClient.put<CheckinConfig>('/admin/check-in/config', {
    streak_bonus_amount: streakBonusAmount
  })
  return data
}

export async function listPrizes(): Promise<CheckinPrize[]> {
  const { data } = await apiClient.get<CheckinPrize[]>('/admin/check-in/prizes')
  return data
}

export async function replacePrizes(prizes: CheckinPrize[]): Promise<CheckinPrize[]> {
  const { data } = await apiClient.put<CheckinPrize[]>('/admin/check-in/prizes', { prizes })
  return data
}

export const adminCheckinAPI = { getConfig, updateConfig, listPrizes, replacePrizes }

export default adminCheckinAPI
