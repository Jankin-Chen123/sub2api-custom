import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import zh from '@/i18n/locales/zh'
import type { CustomMenuItem, PublicSettings } from '@/types'
import AppSidebar from '../AppSidebar.vue'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('maps uploaded SVG paint colors to the current menu color', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: inherit;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).toContain("[fill]:not([fill='none'])")
    expect(componentSource).toContain('fill: currentColor !important;')
    expect(componentSource).toContain("[stroke]:not([stroke='none'])")
    expect(componentSource).toContain('stroke: currentColor !important;')
  })

  it('adds the external tutorial documentation entry with the supplied SVG', () => {
    expect(componentSource).toContain("tutorialDocsIconSvg from '@/assets/tutorial-docs.svg?raw'")
    expect(componentSource).toContain("t('nav.tutorialDocs')")
    expect(componentSource).toContain("externalUrl: 'https://aibaipiao.top/docs'")
    expect(componentSource).toContain(":is=\"item.externalUrl ? 'a' : 'router-link'\"")
    expect(componentSource).toContain("target: '_blank'")
  })

  it('opens documentation management inside the admin layout', () => {
    expect(componentSource).toMatch(/path: '\/admin\/documentation', label: t\('nav.documentation'\), icon: BookIcon \}/)
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar collapsible groups', () => {
  it('lets the user collapse a group even while a child route is active', () => {
    // The expand state must come from the user's override first, falling back
    // to the active-route heuristic only when the user has not clicked yet.
    expect(componentSource).toContain('const groupExpandOverrides = ref<Map<string, boolean>>(new Map())')
    expect(componentSource).not.toContain('expandedGroups.value.has(item.path) || isGroupActive(item)')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar subscription feature flag', () => {
  it('gates the My Subscriptions entry behind the subscription public-settings flag', () => {
    expect(componentSource).toContain('const flagSubscription = makeSidebarFlag(FeatureFlags.subscription)')
    expect(componentSource).toMatch(/path: '\/subscriptions'[^\n]*featureFlag: flagSubscription/)
  })

  it('also hides the admin Subscription Management entry on recharge-only sites', () => {
    expect(componentSource).toMatch(/path: '\/admin\/subscriptions'[^\n]*featureFlag: flagSubscription/)
  })

  it('derives the purchase entry label from the site billing mode', () => {
    expect(componentSource).toContain("import { resolveSiteBillingMode } from '@/utils/siteBillingMode'")
    expect(componentSource).toMatch(/case 'recharge_only':\s*return t\('nav\.recharge'\)/)
    expect(componentSource).toMatch(/case 'subscription_only':\s*return t\('nav\.subscribe'\)/)
    expect(componentSource).toMatch(/path: '\/purchase'[^\n]*label: purchaseNavLabel\.value/)
  })
})

describe('AppSidebar MCP entry', () => {
  it('uses the MCP logo and shows the entry only when the deployment enables MCP', () => {
    expect(componentSource).toContain("render: () => h(Icon, { name: 'mcp' })")
    expect(componentSource).toContain('const flagMCP = makeSidebarFlag(FeatureFlags.mcp)')
    expect(componentSource).toMatch(/path: '\/settings\/mcp'[^\n]*icon: MCPIcon, featureFlag: flagMCP/)
    expect(componentSource).toMatch(/if \(flagMCP\(\)\) filtered\.push\(\{ path: '\/settings\/mcp'[^\n]*icon: MCPIcon/)
    expect(componentSource).not.toMatch(/path: '\/settings\/mcp'[^\n]*PluginIcon/)
  })
})

describe('account sidebar directory', () => {
  async function render(settings: Partial<PublicSettings>) {
    const pinia = createPinia()
    const app = useAppStore(pinia)
    app.$patch({ cachedPublicSettings: settings })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
    })
    await router.push('/dashboard')
    const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
    return mount(AppSidebar, { global: { plugins: [pinia, router, i18n], stubs: { VersionBadge: true } } })
  }

  function menu(id: string, label: string, sort_order: number): CustomMenuItem {
    return { id, label, sort_order, url: 'about:blank', icon_svg: '', visibility: 'user' }
  }

  it('places the configured shop and contact once, keeps their routes and respects feature flags', async () => {
    const wrapper = await render({
      dedicated_image_enabled: true,
      affiliate_enabled: true,
      available_channels_enabled: false,
      custom_menu_items: [menu('more', '资源', 0), menu('contact', '联系我们', 2), menu('shop', '气球小铺', 1)],
    })
    const groups = wrapper.findAll('nav section')
    expect(groups.map(group => group.get('h2').text())).toEqual(['常规', '充值与订阅', '个人', '其他'])
    const links = groups.map(group => group.findAll('a').map(link => link.attributes('href')))
    expect(links).toEqual([
      ['/dashboard', '/keys', '/image-workbench', '/usage', '/monitor'],
      ['/subscriptions', '/purchase', '/orders', '/redeem', '/custom/shop'],
      ['/affiliate', '/profile'],
      ['https://aibaipiao.top/docs', '/custom/contact', '/custom/more'],
    ])
    wrapper.unmount()
  })

  it('uses the built-in contact page when no custom contact is configured', async () => {
    const wrapper = await render({ dedicated_image_enabled: false, subscription_enabled: false })
    expect(wrapper.find('nav a[href="/contact"]').exists()).toBe(true)
    expect(wrapper.find('nav a[href="/image-workbench"]').exists()).toBe(false)
    expect(wrapper.find('nav a[href="/subscriptions"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('opens the mobile directory with labels while retaining the compact desktop preference', async () => {
    const wrapper = await render({})
    const app = useAppStore()
    app.setSidebarCollapsed(true)
    await wrapper.vm.$nextTick()
    expect(wrapper.find('nav h2').exists()).toBe(false)
    expect(wrapper.get('nav a[href="/keys"]').attributes('aria-label')).toBe('API 密钥')
    app.setMobileOpen(true)
    await wrapper.vm.$nextTick()
    expect(wrapper.get('nav h2').text()).toBe('常规')
    expect(app.sidebarCollapsed).toBe(true)
    wrapper.unmount()
  })

  it('shows only documentation in backend mode', async () => {
    const wrapper = await render({ backend_mode_enabled: true })
    expect(wrapper.findAll('nav a').map(a => a.attributes('href'))).toEqual(['https://aibaipiao.top/docs'])
    expect(wrapper.find('nav h2').exists()).toBe(false)
    wrapper.unmount()
  })
})
