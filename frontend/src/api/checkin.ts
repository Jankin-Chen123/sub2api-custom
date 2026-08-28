import { apiClient } from './client'

export interface CheckinPrize {
  id: number
  name: string
  amount: number
  probability: number
  color: string
  sort_order: number
}

export interface CheckinResult {
  id: number
  prize_id?: number
  prize_name: string
  amount: number
  bonus_amount: number
  total_amount: number
  probability: number
  new_balance: number
  streak_days: number
  checked_at: string
}

export interface CheckinStatus {
  date: string
  checked_today: boolean
  can_checkin: boolean
  streak_days: number
  streak_target: number
  streak_bonus_amount: number
  days_until_bonus: number
  prizes: CheckinPrize[]
  today_result?: CheckinResult
}

export interface CheckinHistoryItem {
  id: number
  prize_id?: number
  prize_name: string
  amount: number
  bonus_amount: number
  total_amount: number
  streak_days: number
  probability: number
  checked_at: string
}

export async function getStatus(): Promise<CheckinStatus> {
  const { data } = await apiClient.get<CheckinStatus>('/check-in')
  return data
}

export async function draw(): Promise<CheckinResult> {
  const { data } = await apiClient.post<CheckinResult>('/check-in/draw')
  return data
}

export async function getHistory(): Promise<CheckinHistoryItem[]> {
  const { data } = await apiClient.get<CheckinHistoryItem[]>('/check-in/history')
  return data
}

export const checkinAPI = { getStatus, draw, getHistory }

export default checkinAPI
