import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'

const componentSource = readFileSync(resolve(process.cwd(), 'src/views/public/ContactPageView.vue'), 'utf8')

const { appStore, contactConfig, copyToClipboard, getContactPageSettings } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: null as null | { site_name?: string },
    siteName: 'Sub2API',
    siteLogo: '',
  },
  contactConfig: {
    qq: {
      groupNumber: '123456789',
      qrImageUrl: '/contact/qq-group-qr.png',
    },
    telegram: {
      channelName: '@example_channel',
      channelUrl: 'https://t.me/example_channel',
      qrImageUrl: '/contact/telegram-channel-qr.png',
    },
  },
  copyToClipboard: vi.fn().mockResolvedValue(true),
  getContactPageSettings: vi.fn(),
}))

vi.mock('@/api/auth', () => ({
  getContactPageSettings,
}))

vi.mock('@/config/contactPage', () => ({
  contactPageConfig: contactConfig,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copied: ref(false),
    copyToClipboard,
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import ContactPageView from '../ContactPageView.vue'

function mountView() {
  return mount(ContactPageView)
}

describe('ContactPageView', () => {
  beforeEach(() => {
    contactConfig.qq.groupNumber = '123456789'
    contactConfig.qq.qrImageUrl = '/contact/qq-group-qr.png'
    contactConfig.telegram.channelName = '@example_channel'
    contactConfig.telegram.channelUrl = 'https://t.me/example_channel'
    contactConfig.telegram.qrImageUrl = '/contact/telegram-channel-qr.png'
    copyToClipboard.mockClear()
    getContactPageSettings.mockReset()
    getContactPageSettings.mockRejectedValue(new Error('backend unavailable'))
    appStore.cachedPublicSettings = null
    appStore.siteName = 'Sub2API'
    appStore.siteLogo = ''
    window.history.replaceState({}, '', '/contact')
  })

  it('uses the Aibaipiao brand identity when no runtime branding is configured', () => {
    const wrapper = mountView()

    expect(wrapper.get('[data-testid="contact-brand-name"]').text()).toBe('爱白嫖公益站')
    expect(wrapper.get('[data-testid="contact-brand-logo"]').attributes('src')).toBe('/brand-icon.png')
  })

  it('copies the QQ group number when the whole contact card is clicked', async () => {
    const wrapper = mountView()

    await wrapper.get('[data-testid="qq-contact-card"]').trigger('click')

    expect(copyToClipboard).toHaveBeenCalledWith('123456789', 'contactPage.qq.copied')
  })

  it('opens the configured Telegram channel in a new tab', () => {
    const wrapper = mountView()
    const telegramCard = wrapper.get('[data-testid="telegram-contact-card"]')

    expect(telegramCard.element.tagName).toBe('A')
    expect(telegramCard.attributes('href')).toBe('https://t.me/example_channel')
    expect(telegramCard.attributes('target')).toBe('_blank')
    expect(telegramCard.attributes('rel')).toBe('noopener noreferrer')
  })

  it('renders both configured QR images with accessible alt text', () => {
    const wrapper = mountView()

    expect(wrapper.get('[data-testid="qq-qr-image"]').attributes('src')).toBe('/contact/qq-group-qr.png')
    expect(wrapper.get('[data-testid="qq-qr-image"]').attributes('alt')).toBe('contactPage.qq.qrAlt')
    expect(wrapper.get('[data-testid="telegram-qr-image"]').attributes('src')).toBe('/contact/telegram-channel-qr.png')
    expect(wrapper.get('[data-testid="telegram-qr-image"]').attributes('alt')).toBe('contactPage.telegram.qrAlt')
  })

  it('replaces packaged defaults with runtime settings from the public endpoint', async () => {
    getContactPageSettings.mockResolvedValueOnce({
      qq_group_number: '987654321',
      qq_qr_image: 'data:image/png;base64,cXE=',
      telegram_name: '@runtime_channel',
      telegram_url: 'https://t.me/runtime_channel',
      telegram_qr_image: '/runtime/telegram.png',
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="qq-contact-card"]').text()).toContain('987654321')
    expect(wrapper.get('[data-testid="qq-qr-image"]').attributes('src')).toBe('data:image/png;base64,cXE=')
    expect(wrapper.get('[data-testid="telegram-contact-card"]').attributes('href')).toBe('https://t.me/runtime_channel')
    expect(wrapper.get('[data-testid="telegram-contact-card"]').text()).toContain('@runtime_channel')
    expect(wrapper.get('[data-testid="telegram-qr-image"]').attributes('src')).toBe('/runtime/telegram.png')
  })

  it('shows intentional placeholders while real contact details are pending', () => {
    contactConfig.qq.groupNumber = ''
    contactConfig.qq.qrImageUrl = ''
    contactConfig.telegram.channelName = ''
    contactConfig.telegram.channelUrl = ''
    contactConfig.telegram.qrImageUrl = ''

    const wrapper = mountView()

    expect(wrapper.get('[data-testid="qq-contact-card"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="telegram-contact-card"]').element.tagName).toBe('DIV')
    expect(wrapper.get('[data-testid="qq-qr-placeholder"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="telegram-qr-placeholder"]').exists()).toBe(true)
  })

  it('rejects a non-Telegram URL instead of rendering an unsafe outbound link', () => {
    contactConfig.telegram.channelUrl = 'https://example.com/not-telegram'

    const wrapper = mountView()

    expect(wrapper.get('[data-testid="telegram-contact-card"]').element.tagName).toBe('DIV')
    expect(wrapper.get('[data-testid="telegram-contact-card"]').attributes('aria-disabled')).toBe('true')
  })

  it('honors the embedded page theme supplied by the custom-page iframe', () => {
    window.history.replaceState({}, '', '/contact?ui_mode=embedded&theme=dark')

    const wrapper = mountView()

    expect(wrapper.get('[data-testid="contact-theme-root"]').classes()).toContain('dark')
  })

  it('keeps the branded light palette even when the host requests dark mode', () => {
    expect(componentSource).not.toContain('.dark .contact-page {')
    expect(componentSource).not.toContain('.dark .contact-status {')
  })

  it('uses viewport-aware compact sizing on desktop to avoid page scrolling', () => {
    expect(componentSource).toContain('height: 100svh;')
    expect(componentSource).toContain('height: clamp(240px, 30vh, 300px);')
    expect(componentSource).toContain('min-height: 110px;')
    expect(componentSource).toContain('@media (min-width: 761px) and (max-height: 820px)')
    expect(componentSource).toContain('height: clamp(205px, 28vh, 220px);')
  })
})
