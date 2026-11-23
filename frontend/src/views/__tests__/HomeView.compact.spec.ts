import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

import HomeView from '../HomeView.vue'

const { appStore, authStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Fallback site',
    siteLogo: '',
    docUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  },
  authStore: {
    isAuthenticated: false,
    isAdmin: false,
    user: null as { email?: string } | null,
    checkAuth: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountHome(settings: Record<string, unknown> = {}) {
  appStore.cachedPublicSettings = {
    site_name: 'Test site',
    site_subtitle: 'Test subtitle',
    ...settings,
  }

  return mount(HomeView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        LocaleSwitcher: { template: '<div data-testid="locale-switcher" />' },
        Icon: { template: '<span data-testid="icon" />' },
      },
    },
  })
}

function compactDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper.get('[data-testid="compact-home"]').findComponent(RouterLinkStub).props('to')
}

function modelPlazaDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper
    .findAllComponents(RouterLinkStub)
    .find((link) => link.props('to') === '/model-plaza')
    ?.props('to')
}

describe('HomeView compact mode', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    authStore.user = null
    authStore.checkAuth.mockClear()
    appStore.fetchPublicSettings.mockClear()
    localStorage.clear()
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })

  it('renders custom HTML ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: '<section id="custom-home">Custom home</section>',
    })

    expect(wrapper.get('#custom-home').text()).toBe('Custom home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('keeps unrelated custom HTML with a similar class name', () => {
    const wrapper = mountHome({
      home_content: '<main class="ab-home"><h1>站点标题</h1><a href="/login">进入</a></main>',
    })

    expect(wrapper.get('.ab-home h1').text()).toBe('站点标题')
    expect(wrapper.get('.ab-home a').attributes('href')).toBe('/login')
  })

  it('upgrades the installed brand template to the Vue sticker home', () => {
    const wrapper = mountHome({
      home_content: `<div class="ab-home"><style></style><header class="ab-nav"></header>
        <main><section class="ab-hero"><div class="ab-emblem"><img alt=""></div></section>
        <section class="ab-benefits"><div class="ab-benefit-grid"></div></section></main>
        <footer class="ab-footer"></footer></div>`,
    })
    expect(wrapper.get('[data-sticker-home] h1').text()).toBe('Test site')
    expect(wrapper.find('.ab-home').exists()).toBe(false)
  })

  it('supports selecting the sticker homepage explicitly', () => {
    authStore.isAuthenticated = true
    const wrapper = mountHome({ home_content: 'sticker-home', compact_home_enabled: true })
    expect(wrapper.get('[data-sticker-home]').findAllComponents(RouterLinkStub).some(link => link.props('to') === '/dashboard')).toBe(true)
  })

  it('renders custom URL content ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: ' https://example.com/home ',
    })

    expect(wrapper.get('iframe').attributes('src')).toBe('https://example.com/home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('treats whitespace-only custom content as empty and selects compact mode', () => {
    const wrapper = mountHome({ compact_home_enabled: true, home_content: ' \n\t ' })

    expect(wrapper.get('[data-testid="compact-home"]').text()).toContain('Test site')
  })

  it.each([undefined, false])('selects the default home when compact mode is %s', (enabled) => {
    const settings = enabled === undefined ? {} : { compact_home_enabled: enabled }
    const wrapper = mountHome(settings)

    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
    expect(wrapper.find('.terminal-container').exists()).toBe(true)
  })

  it('links unauthenticated visitors to login', () => {
    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/login')
  })

  it('links authenticated users to their dashboard', () => {
    authStore.isAuthenticated = true

    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/dashboard')
  })

  it('links administrators to the admin dashboard', () => {
    authStore.isAuthenticated = true
    authStore.isAdmin = true

    const wrapper = mountHome({ compact_home_enabled: true })
    expect(compactDestination(wrapper)).toBe('/admin/dashboard')
    expect(authStore.checkAuth).toHaveBeenCalledOnce()
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
  })

  it('shows the model plaza link to anonymous visitors when public access is enabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link from anonymous visitors when sign-in is required', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it('shows the model plaza link to authenticated visitors when sign-in is required', () => {
    authStore.isAuthenticated = true

    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('shows the model plaza link in the default home header', () => {
    const wrapper = mountHome({
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link when the feature is disabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: false,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it('renders QQ group link and sound toggle in sticker navbar and moves docs button into hero actions', async () => {
    localStorage.setItem('sticker_paper_sound_enabled', 'true')
    const docsUrl = 'https://docs.example.com/guide'
    const wrapper = mountHome({
      home_content: 'sticker-home',
      doc_url: docsUrl,
    })

    const navActions = wrapper.get('[data-sticker-home] .sticker-nav-actions')
    const qqLink = navActions.get('a[href="https://qm.qq.com/q/DiCjrJU00i"]')
    expect(qqLink.attributes('target')).toBe('_blank')
    expect(navActions.find('.sticker-nav-doc').exists()).toBe(false)
    expect(
      navActions.findAllComponents(RouterLinkStub).some((link) => link.props('to') === '/login')
    ).toBe(false)

    const soundToggle = navActions.get('.sticker-sound-toggle')
    expect(soundToggle.attributes('aria-pressed')).toBe('true')
    await soundToggle.trigger('click')
    expect(soundToggle.attributes('aria-pressed')).toBe('false')
    expect(localStorage.getItem('sticker_paper_sound_enabled')).toBe('false')
    await soundToggle.trigger('click')
    expect(soundToggle.attributes('aria-pressed')).toBe('true')
    expect(localStorage.getItem('sticker_paper_sound_enabled')).toBe('true')

    const heroDocs = wrapper.get('[data-sticker-home] .sticker-actions a.sticker-button-docs')
    expect(heroDocs.attributes('href')).toBe(docsUrl)
    expect(heroDocs.text()).toContain('home.sticker.docs')

    const setupSection = wrapper.get('[data-sticker-home] .sticker-setup')
    expect(setupSection.find('.sticker-setup-cta').exists()).toBe(false)
    expect(setupSection.find(`a[href="${docsUrl}"]`).exists()).toBe(false)
  })

  it('animates the hero OK mascot with magnetic tilt hover and High-Five stamp bursts on click', async () => {
    const wrapper = mountHome({ home_content: 'sticker-home' })
    const mascotBtn = wrapper.get('[data-sticker-home] .sticker-mascot-trigger')

    vi.spyOn(mascotBtn.element, 'getBoundingClientRect').mockReturnValue({
      left: 0,
      top: 0,
      width: 400,
      height: 400,
      right: 400,
      bottom: 400,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    } as DOMRect)

    await mascotBtn.trigger('pointerenter', { clientX: 300, clientY: 100 })
    expect(mascotBtn.classes()).toContain('is-hovered')

    await mascotBtn.trigger('pointermove', { clientX: 300, clientY: 100 })
    const hoverStyle = mascotBtn.attributes('style') || ''
    expect(hoverStyle).toContain('--mascot-tx: 6.00px')
    expect(hoverStyle).toContain('--mascot-ty: -13.00px')
    expect(hoverStyle).toContain('--mascot-rx: 6.00deg')
    expect(hoverStyle).toContain('--mascot-ry: 7.00deg')

    await mascotBtn.trigger('pointerleave')
    expect(mascotBtn.classes()).not.toContain('is-hovered')
    expect(mascotBtn.attributes('style')).toContain('--mascot-tx: 0.00px')

    await mascotBtn.trigger('click', { clientX: 200, clientY: 200 })
    await nextTick()
    expect(mascotBtn.classes()).toContain('is-slapping')
    expect(mascotBtn.findAll('.sticker-mascot-stamp-text').map((el) => el.text())).toEqual(['OK ✓'])

    await mascotBtn.trigger('click', { clientX: 220, clientY: 180 })
    await nextTick()
    expect(mascotBtn.findAll('.sticker-mascot-stamp-text').map((el) => el.text())).toEqual([
      'OK ✓',
      'NICE!',
    ])

    wrapper.unmount()
  })

  it('assigns independent frame-by-frame sprite sheets to each benefit card and renders stamp impact ring on verification', () => {
    const wrapper = mountHome({ home_content: 'sticker-home' })

    const cards = wrapper.findAll('[data-sticker-home] .sticker-benefit')
    expect(cards).toHaveLength(3)

    expect(cards[0].classes()).toContain('sticker-benefit--pricing')
    expect(cards[0].find('.sticker-feature-sprite--pricing').exists()).toBe(true)
    expect(cards[0].find('.sticker-stamp-impact-ring').exists()).toBe(false)

    expect(cards[1].classes()).toContain('sticker-benefit--routing')
    expect(cards[1].find('.sticker-feature-sprite--routing').exists()).toBe(true)
    expect(cards[1].find('.sticker-stamp-impact-ring').exists()).toBe(false)

    expect(cards[2].classes()).toContain('sticker-benefit--verification')
    expect(cards[2].find('.sticker-feature-sprite--verification').exists()).toBe(true)
    expect(cards[2].find('.sticker-stamp-impact-ring').exists()).toBe(true)

    wrapper.unmount()
  })
})
