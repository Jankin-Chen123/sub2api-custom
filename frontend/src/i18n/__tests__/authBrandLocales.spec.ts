import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

const brandKeys = [
  'kicker',
  'headline',
  'description',
  'billingTitle',
  'billingDescription',
  'routingTitle',
  'routingDescription'
] as const

describe('auth brand locales', () => {
  it.each([
    ['zh', zh],
    ['en', en]
  ] as const)('%s provides non-empty brand copy', (_locale, messages) => {
    for (const key of brandKeys) {
      expect(messages.auth.brand[key]).not.toHaveLength(0)
    }
  })

  it('provides Chinese auth navigation copy', () => {
    expect(zh.auth.modeTabsLabel).toBe('登录与注册')
    expect(zh.auth.backHome).toBe('返回首页')
  })

  it('provides English auth navigation copy', () => {
    expect(en.auth.modeTabsLabel).toBe('Sign in or register')
    expect(en.auth.backHome).toBe('Back home')
  })
})
