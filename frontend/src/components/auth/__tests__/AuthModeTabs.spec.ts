import { mount, RouterLinkStub } from '@vue/test-utils'
import { baseCompile } from '@intlify/message-compiler'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'
import AuthModeTabs from '@/components/auth/AuthModeTabs.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messageCompiler: (message: string) => new Function(`return ${baseCompile(message).code}`)(),
  messages: {
    zh: {
      auth: {
        modeTabsLabel: '登录与注册',
        signIn: '登录',
        signUp: '注册',
      },
    },
  },
})

describe('AuthModeTabs', () => {
  it('renders localized login and registration route links', () => {
    const wrapper = mount(AuthModeTabs, {
      props: { active: 'login' },
      global: {
        plugins: [i18n],
        stubs: { RouterLink: RouterLinkStub },
      },
    })

    expect(wrapper.get('nav').attributes('aria-label')).toBe('登录与注册')

    const links = wrapper.findAllComponents(RouterLinkStub)
    expect(links).toHaveLength(2)
    expect(links[0].props('to')).toBe('/login')
    expect(links[0].text()).toBe('登录')
    expect(links[1].props('to')).toBe('/register')
    expect(links[1].text()).toBe('注册')
  })

  it('marks only the active registration tab as the current page', () => {
    const wrapper = mount(AuthModeTabs, {
      props: { active: 'register' },
      global: {
        plugins: [i18n],
        stubs: { RouterLink: RouterLinkStub },
      },
    })

    const links = wrapper.findAllComponents(RouterLinkStub)
    expect(links[0].classes()).toContain('auth-mode-tab')
    expect(links[0].classes()).not.toContain('auth-mode-tab--active')
    expect(links[0].attributes('aria-current')).toBeUndefined()
    expect(links[1].classes()).toContain('auth-mode-tab')
    expect(links[1].classes()).toContain('auth-mode-tab--active')
    expect(links[1].attributes('aria-current')).toBe('page')
  })
})
