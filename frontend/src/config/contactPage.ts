export interface ContactPageConfig {
  qq: {
    groupNumber: string
    qrImageUrl: string
  }
  telegram: {
    channelName: string
    channelUrl: string
    qrImageUrl: string
  }
}

/**
 * Contact page content lives here so the real community details can be
 * replaced without touching the page layout.
 *
 * QR assets should be copied to frontend/public/contact/ and referenced with
 * an absolute public path, for example: /contact/qq-group-qr.png.
 */
export const contactPageConfig: ContactPageConfig = {
  qq: {
    groupNumber: '1097280864',
    qrImageUrl: '/contact/qq-group-qr.jpg',
  },
  telegram: {
    channelName: '@ai_baipiao',
    channelUrl: 'https://t.me/ai_baipiao',
    qrImageUrl: '/contact/telegram-channel-qr.png',
  },
}
