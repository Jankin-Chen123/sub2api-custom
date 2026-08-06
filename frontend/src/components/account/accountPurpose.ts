import type { AccountPurpose } from '@/types'

export const CANGYUAN_BASE_URL = 'https://ai.cangyuansuanli.cn/v1'

export const CANGYUAN_IMAGE_MODEL_MAPPINGS = [
  { from: 'gpt-image-2-1k', to: 'gpt-image-2-1k' },
  { from: 'gpt-image-2-2k', to: 'gpt-image-2-2k' },
  { from: 'gpt-image-2-4k', to: 'gpt-image-2-4k' }
] as const

export function resolveAccountPurpose(extra?: Record<string, unknown> | null): AccountPurpose {
  return extra?.account_purpose === 'image_only' ? 'image_only' : 'general'
}

export function applyAccountPurpose(
  extra: Record<string, unknown> | undefined,
  purpose: AccountPurpose
): Record<string, unknown> | undefined {
  const next = { ...(extra || {}) }
  if (purpose === 'image_only') {
    next.account_purpose = 'image_only'
  } else {
    delete next.account_purpose
  }
  return Object.keys(next).length > 0 ? next : undefined
}

export function applyCreatedAccountPurpose(
  extra: Record<string, unknown> | undefined,
  platform: string,
  purpose: AccountPurpose
): Record<string, unknown> | undefined {
  return applyAccountPurpose(extra, platform === 'openai' ? purpose : 'general')
}
