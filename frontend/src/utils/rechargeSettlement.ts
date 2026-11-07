import type { RechargeBonusTier } from '@/types/payment'
import { quoteRechargeBonus, roundRechargeAmount, type RechargeBonusMode } from './rechargeBonus'
import { rechargeDecimalProduct } from './rechargeDecimal'

export function calculateRechargeFee(principal: number, feeRate: number, digits = 2): number {
  if (!Number.isFinite(principal) || !Number.isFinite(feeRate) || principal <= 0 || feeRate <= 0) return 0
  return rechargeDecimalProduct([principal, feeRate], 100, digits, true)
}

export function quoteRechargeSettlement(tiers: RechargeBonusTier[], principal: number, options: {
  multiplier?: number
  mode?: RechargeBonusMode
  currencyDigits?: number
  feeRate?: number
  firstRechargeReward?: number
} = {}) {
  const quote = quoteRechargeBonus(tiers, principal, options)
  const digits = options.currencyDigits ?? 2
  const fee = calculateRechargeFee(principal, options.feeRate ?? 0, digits)
  const reward = options.firstRechargeReward ?? 0
  // The campaign threshold is based on principal, excluding fees and tier bonuses.
  const firstBonus = principal >= 10 && Number.isFinite(reward) && reward > 0 ? roundRechargeAmount(reward) : 0
  return {
    ...quote,
    fee,
    firstBonus,
    discount: roundRechargeAmount(principal - quote.payBase, digits),
    payTotal: roundRechargeAmount(quote.payBase + fee, digits),
    creditedTotal: roundRechargeAmount(quote.credited + firstBonus),
  }
}
