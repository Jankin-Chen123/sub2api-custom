export type NewcomerCampaignPhase = 'upcoming' | 'active' | 'ended'
export type NewcomerFirstRechargeRewardStatus = 'pending' | 'qualified' | 'granted' | 'revoked' | 'ineligible'

export interface NewcomerCampaignTier {
  key: 'premium' | 'gold' | 'diamond' | string
  name: string
  threshold: number
  factor: number
  duration_days: number
}

export interface NewcomerFirstRechargeStatus {
  eligible: boolean
  first_order_id?: number
  first_amount?: number
  first_completed_at?: string
  reward_status: NewcomerFirstRechargeRewardStatus | string
  reward_amount: number
}

export interface NewcomerMembershipStatus {
  tier_key: string
  tier_name: string
  factor: number
  starts_at: string
  expires_at: string
}

export interface NewcomerCampaignStatus {
  campaign_key: string
  name: string
  phase: NewcomerCampaignPhase | string
  starts_at: string
  ends_at: string
  first_recharge: NewcomerFirstRechargeStatus
  invite_link: string
  valid_invite_count: number
  next_tier?: NewcomerCampaignTier
  next_tier_progress: number
  next_tier_remaining: number
  current_membership?: NewcomerMembershipStatus
  tiers: NewcomerCampaignTier[]
}

export interface NewcomerCampaignAdminConfig {
  campaign_key: string
  name: string
  phase: NewcomerCampaignPhase | string
  starts_at: string
  ends_at: string
  tiers: NewcomerCampaignTier[]
}

export interface NewcomerCampaignAdminUserMembership {
  user_id: number
  email: string
  username: string
  valid_invite_count: number
  manual_membership?: NewcomerMembershipStatus
  current_membership?: NewcomerMembershipStatus
}

export interface NewcomerCampaignAdminMembershipInput {
  tier_key: string
  factor?: number
  starts_at?: string
  expires_at?: string
  duration_days?: number
  reason?: string
}
