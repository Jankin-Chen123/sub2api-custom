import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LoginView from '@/views/auth/LoginView.vue'

const { getPublicSettingsMock, pushMock, loginMock } = vi.hoisted(() => ({
  getPublicSettingsMock: vi.fn(),
  pushMock: vi.fn(),
  loginMock: vi.fn()
}))

const publicSettings = {
  registration_enabled: true,
  turnstile_enabled: false,
  turnstile_site_key: '',
  tencent_captcha_enabled: false,
  tencent_captcha_app_id: '',
  aliyun_captcha_enabled: false,
  aliyun_captcha_scene_id: '',
  aliyun_captcha_prefix: '',
  linuxdo_oauth_enabled: false,
  dingtalk_oauth_enabled: false,
  wechat_oauth_enabled: false,
  backend_mode_enabled: false,
  oidc_oauth_enabled: false,
  oidc_oauth_provider_name: 'OIDC',
  github_oauth_enabled: false,
  google_oauth_enabled: false,
  password_reset_enabled: false,
  passkey_enabled: false,
  login_agreement_enabled: false,
  login_agreement_documents: []
}

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: pushMock,
    currentRoute: { value: { query: {} } }
  })
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key
    }
  }),
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    login: (...args: unknown[]) => loginMock(...args),
    loginWithPasskey: vi.fn(),
    login2FA: vi.fn()
  }),
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn()
  })
}))

vi.mock('@/api/auth', () => ({
  buildOAuthLoginStartURL: vi.fn(),
  getPublicSettings: (...args: unknown[]) => getPublicSettingsMock(...args),
  isTotp2FARequired: vi.fn(() => false),
  isWeChatWebOAuthEnabled: vi.fn(() => false),
  startOAuthLogin: vi.fn()
}))

function mountLogin() {
  return mount(LoginView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        DingTalkOAuthSection: true,
        EmailOAuthButtons: true,
        Icon: true,
        LinuxDoOAuthSection: true,
        LoginAgreementPrompt: true,
        OidcOAuthSection: true,
        RouterLink: { template: '<a><slot /></a>' },
        TotpLoginModal: true,
        TurnstileWidget: true,
        WechatOAuthSection: true,
        transition: false
      }
    }
  })
}

describe('LoginView registration entry', () => {
  beforeEach(() => {
    getPublicSettingsMock.mockReset()
    pushMock.mockReset()
    loginMock.mockReset()
    loginMock.mockResolvedValue({})
    localStorage.removeItem('sub2api_login_agreement_consent')
    getPublicSettingsMock.mockResolvedValue(publicSettings)
  })

  it('shows the registration entry when registration is enabled', async () => {
    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).toContain('auth.signUp')
  })

  it('hides the registration entry when registration is disabled', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      registration_enabled: false
    })

    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).not.toContain('auth.signUp')
  })

  it('allows editing before policy consent but gates submission until accepted', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      login_agreement_enabled: true,
      login_agreement_mode: 'checkbox',
      login_agreement_revision: 'test-login-policy',
      login_agreement_documents: [{ id: 'terms', title: 'Terms' }]
    })
    const wrapper = mountLogin()
    await flushPromises()

    expect((wrapper.get('#email').element as HTMLInputElement).disabled).toBe(false)
    expect((wrapper.get('#password').element as HTMLInputElement).disabled).toBe(false)
    await wrapper.get('#email').setValue('user@example.com')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#password').element.parentElement!.querySelector('button')!.click()
    await flushPromises()
    expect(wrapper.get('#password').attributes('type')).toBe('text')
    expect((wrapper.get('button[type="submit"]').element as HTMLButtonElement).disabled).toBe(true)

    // A form submit (including Enter) must not bypass the disabled button.
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(loginMock).not.toHaveBeenCalled()
    expect(pushMock).not.toHaveBeenCalled()

    const agreement = wrapper.findComponent({ name: 'LoginAgreementPrompt' })
    agreement.vm.$emit('accept')
    await flushPromises()
    expect((wrapper.get('button[type="submit"]').element as HTMLButtonElement).disabled).toBe(false)

    agreement.vm.$emit('reject')
    await flushPromises()
    expect((wrapper.get('button[type="submit"]').element as HTMLButtonElement).disabled).toBe(true)
    expect((wrapper.get('#email').element as HTMLInputElement).value).toBe('user@example.com')
    expect((wrapper.get('#password').element as HTMLInputElement).disabled).toBe(false)

    agreement.vm.$emit('accept')
    await flushPromises()
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(loginMock).toHaveBeenCalledWith(expect.objectContaining({
      email: 'user@example.com', password: 'secret-123'
    }))
    expect(pushMock).toHaveBeenCalledWith('/dashboard')
    wrapper.unmount()
  })
})
