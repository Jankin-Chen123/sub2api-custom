import { describe, expect, it } from 'vitest'
import {
  buildPaymentErrorToastMessage,
  describePaymentScenarioError,
  isEasyPayChannelUnavailable,
  normalizePaymentMethodForDisplay,
} from '../paymentUx'

describe('EasyPay channel availability', () => {
  it.each([
    ['payment gateway error: easypay error: 当前付款环境没有已启用的思云支付产品，请检查通道支付接口配置', true],
    ['payment gateway error: easypay error: 请用手机系统浏览器打开当前付款链接（PAYMENT_SCENE_MISMATCH）', false],
    ['payment gateway error: easypay create: context deadline exceeded', false],
    ['payment gateway error: other provider: 当前付款环境没有已启用的思云支付产品', false],
  ])('classifies %s', (message, expected) => {
    expect(isEasyPayChannelUnavailable({ reason: 'PAYMENT_GATEWAY_ERROR', message })).toBe(expected)
  })
})

describe('normalizePaymentMethodForDisplay', () => {
  it('collapses visible payment aliases to canonical method ids', () => {
    expect(normalizePaymentMethodForDisplay(' alipay_direct ')).toBe('alipay')
    expect(normalizePaymentMethodForDisplay('wxpay_direct')).toBe('wxpay')
    expect(normalizePaymentMethodForDisplay('wechat_pay')).toBe('wxpay')
  })

  it('leaves non-aliased methods untouched', () => {
    expect(normalizePaymentMethodForDisplay('stripe')).toBe('stripe')
  })
})

describe('describePaymentScenarioError', () => {
  it('maps WeChat H5 authorization errors to explicit in-app guidance', () => {
    expect(describePaymentScenarioError(
      { reason: 'WECHAT_H5_NOT_AUTHORIZED' },
      { paymentMethod: 'wxpay', isMobile: true, isWechatBrowser: false },
    )).toEqual({
      messageKey: 'payment.errors.wechatH5NotAuthorized',
      hintKey: 'payment.errors.wechatOpenInWeChatHint',
    })
  })

  it('maps WeChat H5 authorization errors when provider aliases use wxpay_direct', () => {
    expect(describePaymentScenarioError(
      { reason: 'WECHAT_H5_NOT_AUTHORIZED' },
      { paymentMethod: 'wxpay_direct', isMobile: true, isWechatBrowser: false },
    )).toEqual({
      messageKey: 'payment.errors.wechatH5NotAuthorized',
      hintKey: 'payment.errors.wechatOpenInWeChatHint',
    })
  })

  it('maps missing WeixinJSBridge to a JSAPI-specific prompt', () => {
    expect(describePaymentScenarioError(
      new Error('WeixinJSBridge is unavailable'),
      { paymentMethod: 'wxpay', isMobile: true, isWechatBrowser: true },
    )).toEqual({
      messageKey: 'payment.errors.wechatJsapiUnavailable',
      hintKey: 'payment.errors.wechatOpenInWeChatHint',
    })
  })

  it('maps the internal JSAPI unavailable marker to the same prompt', () => {
    expect(describePaymentScenarioError(
      new Error('WECHAT_JSAPI_UNAVAILABLE'),
      { paymentMethod: 'wxpay', isMobile: true, isWechatBrowser: true },
    )).toEqual({
      messageKey: 'payment.errors.wechatJsapiUnavailable',
      hintKey: 'payment.errors.wechatOpenInWeChatHint',
    })
  })

  it('maps generic desktop Alipay failures to QR guidance', () => {
    expect(describePaymentScenarioError(
      { reason: 'PAYMENT_GATEWAY_ERROR' },
      { paymentMethod: 'alipay', isMobile: false, isWechatBrowser: false },
    )).toEqual({
      messageKey: 'payment.errors.alipayDesktopUnavailable',
      hintKey: 'payment.errors.alipayDesktopQrHint',
    })
  })
})

describe('buildPaymentErrorToastMessage', () => {
  it('returns the main message when no hint is present', () => {
    expect(buildPaymentErrorToastMessage('Payment failed')).toBe('Payment failed')
  })

  it('appends the hint to the toast body when present', () => {
    expect(buildPaymentErrorToastMessage('Payment failed', 'Open WeChat to continue.')).toBe(
      'Payment failed Open WeChat to continue.'
    )
  })
})
