import { statSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { contactPageConfig } from '../contactPage'

function resolvePublicAsset(publicUrl: string) {
  return resolve(process.cwd(), 'public', publicUrl.replace(/^\/+/, ''))
}

function expectPublicAsset(publicUrl: string) {
  const stats = statSync(resolvePublicAsset(publicUrl))

  expect(stats.isFile()).toBe(true)
  expect(stats.size).toBeGreaterThan(0)
}

describe('contactPageConfig', () => {
  it('contains the published community details', () => {
    expect(contactPageConfig.qq.groupNumber).toBe('1097280864')
    expect(contactPageConfig.qq.groupNumber).toMatch(/^\d+$/)
    expect(contactPageConfig.qq.qrImageUrl).toBe('/contact/qq-group-qr.jpg')

    expect(contactPageConfig.telegram.channelName).toBe('@ai_baipiao')
    expect(contactPageConfig.telegram.channelUrl).toBe('https://t.me/ai_baipiao')
    expect(contactPageConfig.telegram.qrImageUrl).toBe('/contact/telegram-channel-qr.png')

    const telegramUrl = new URL(contactPageConfig.telegram.channelUrl)
    expect(telegramUrl.protocol).toBe('https:')
    expect(telegramUrl.hostname).toBe('t.me')
    expect(telegramUrl.pathname).toBe('/ai_baipiao')
  })

  it('ships the configured QQ QR asset', () => {
    expectPublicAsset(contactPageConfig.qq.qrImageUrl)
  })

  it('ships the configured Telegram QR asset', () => {
    expectPublicAsset(contactPageConfig.telegram.qrImageUrl)
  })
})
