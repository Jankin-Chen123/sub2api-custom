// Match the backend's decimal multiplication and monetary rounding without binary float drift.
function decimalRatio(value: number): [bigint, bigint] {
  const [coefficient, exponent = '0'] = value.toString().split('e')
  const [whole, fraction = ''] = coefficient.split('.')
  const scale = fraction.length - Number(exponent)
  const numerator = BigInt(whole + fraction)
  return scale >= 0
    ? [numerator, 10n ** BigInt(scale)]
    : [numerator * 10n ** BigInt(-scale), 1n]
}

export function rechargeDecimalProduct(factors: number[], divisor = 1, digits = 2, roundUp = false): number {
  if (!factors.every(Number.isFinite) || !Number.isFinite(divisor) || divisor <= 0) return 0
  let numerator = 1n
  let denominator = 1n
  for (const factor of factors) {
    const [value, scale] = decimalRatio(factor)
    numerator *= value
    denominator *= scale
  }
  const [divisorValue, divisorScale] = decimalRatio(divisor)
  const units = 10n ** BigInt(digits)
  numerator *= divisorScale * units
  denominator *= divisorValue
  const sign = numerator < 0 ? -1n : 1n
  const absolute = numerator * sign
  const rounded = roundUp
    ? (absolute + denominator - 1n) / denominator
    : (absolute * 2n + denominator) / (denominator * 2n)
  return Number(sign * rounded) / Number(units)
}
