import { describe, expect, it } from 'vitest'
import { calculateRechargeFee, quoteRechargeSettlement } from '../rechargeSettlement'

describe('recharge settlement', () => {
  const tiers = [{ min_amount: 20, bonus_percent: 3 }, { min_amount: 50, bonus_percent: 8 }, { min_amount: 100, bonus_percent: 10 }]

  it.each([
    [10, 0, 10, 12], [19.99, 0, 19.99, 21.99], [20, 0.60, 19.40, 22],
    [50, 4, 46, 52], [100, 10, 90, 102],
  ])('discount and fixed reward use principal %s', (principal, discount, pay, credit) => {
    expect(quoteRechargeSettlement(tiers, principal, { mode: 'discount', firstRechargeReward: 2 }))
      .toMatchObject({ discount, payTotal: pay, creditedTotal: credit })
  })

  it('charges fees on principal without discounting the fee or charging gifts', () => {
    expect(quoteRechargeSettlement(tiers, 20, { mode: 'discount', feeRate: 5, firstRechargeReward: 2 }))
      .toMatchObject({ discount: 0.60, fee: 1, payTotal: 20.40, creditedTotal: 22 })
    expect(quoteRechargeSettlement(tiers, 20, { mode: 'bonus', feeRate: 5, firstRechargeReward: 2 }))
      .toMatchObject({ bonus: 0.60, firstBonus: 2, fee: 1, payTotal: 21, creditedTotal: 22.60 })
  })

  it('does not multiply the fixed reward or qualify with tier bonuses or fees', () => {
    expect(quoteRechargeSettlement(tiers, 20, { mode: 'bonus', multiplier: 0.14, firstRechargeReward: 2 }))
      .toMatchObject({ creditedTotal: 4.88, firstBonus: 2 })
    expect(quoteRechargeSettlement([{ min_amount: 0, bonus_percent: 100 }], 9.99, { feeRate: 20, firstRechargeReward: 2 }).firstBonus).toBe(0)
  })

  it('matches decimal half-up rounding for discount, multiplier and bonus', () => {
    expect(quoteRechargeSettlement(tiers, 33.50, { mode: 'discount' }).payTotal).toBe(32.50)
    expect(quoteRechargeSettlement([], 1.01, { multiplier: 0.5 }).creditedTotal).toBe(0.51)
    expect(quoteRechargeSettlement([{ min_amount: 0, bonus_percent: 50 }], 1.01).bonus).toBe(0.51)
  })

  it.each([[0.07, 100, 2, 0.07], [20.01, 5, 2, 1.01], [101, 2.5, 0, 3], [20, 5, 3, 1], [1, 1e-7, 2, 0.01]])(
    'rounds fee upwards in currency units: %s at %s%%', (principal, rate, digits, fee) => {
      expect(calculateRechargeFee(principal, rate, digits)).toBe(fee)
    },
  )
})
