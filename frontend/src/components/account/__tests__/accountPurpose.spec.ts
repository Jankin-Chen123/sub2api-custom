import { describe, expect, it } from 'vitest'

import {
  CANGYUAN_BASE_URL,
  CANGYUAN_IMAGE_MODEL_MAPPINGS,
  applyAccountPurpose,
  applyCreatedAccountPurpose,
  resolveAccountPurpose
} from '../accountPurpose'

describe('accountPurpose', () => {
  it('defaults legacy and unknown values to general', () => {
    expect(resolveAccountPurpose()).toBe('general')
    expect(resolveAccountPurpose({})).toBe('general')
    expect(resolveAccountPurpose({ account_purpose: 'unknown' })).toBe('general')
  })

  it('recognizes image-only accounts', () => {
    expect(resolveAccountPurpose({ account_purpose: 'image_only' })).toBe('image_only')
  })

  it('writes image-only and removes the default general value', () => {
    expect(applyAccountPurpose({ quota_limit: 10 }, 'image_only')).toEqual({
      quota_limit: 10,
      account_purpose: 'image_only'
    })
    expect(applyAccountPurpose({ account_purpose: 'image_only' }, 'general')).toBeUndefined()
  })

  it('exposes only the three Cangyuan image tiers', () => {
    expect(CANGYUAN_BASE_URL).toBe('https://ai.cangyuansuanli.cn/v1')
    expect(CANGYUAN_IMAGE_MODEL_MAPPINGS.map(({ to }) => to)).toEqual([
      'gpt-image-2-1k',
      'gpt-image-2-2k',
      'gpt-image-2-4k'
    ])
  })

  it('only writes image-only purpose for OpenAI API-key creation', () => {
    expect(applyCreatedAccountPurpose({}, 'openai', 'image_only')).toEqual({
      account_purpose: 'image_only'
    })
    expect(applyCreatedAccountPurpose({}, 'gemini', 'image_only')).toBeUndefined()
  })
})
