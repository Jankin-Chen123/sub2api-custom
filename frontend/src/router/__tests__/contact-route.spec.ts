import { describe, expect, it, vi } from 'vitest'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: false,
  isAdmin: false,
  isSimpleMode: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  cachedPublicSettings: null as null | Record<string, unknown>,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({
    customMenuItems: [],
  }),
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

describe('contact page route', () => {
  it('registers a standalone public URL for the custom menu iframe', async () => {
    const { default: router } = await import('@/router')
    const route = router.getRoutes().find((record) => record.name === 'ContactPage')

    expect(route?.path).toBe('/contact')
    expect(route?.meta.requiresAuth).toBe(false)
    expect(route?.meta.titleKey).toBe('contactPage.title')
  })

  it('remains public for anonymous visitors when backend mode is enabled', async () => {
    const { isBackendModePublicRouteAllowed } = await import('@/router')

    expect(isBackendModePublicRouteAllowed('/contact', false)).toBe(true)
    expect(isBackendModePublicRouteAllowed('/contact/', false)).toBe(true)
  })

  it('hands documentation routes to the independent service without loading old pages', async () => {
    const { default: router, isBackendModePublicRouteAllowed } = await import('@/router')
    const { redirectToDocumentation } = await import('../documentationRedirect')
    const route = router.getRoutes().find((record) => record.name === 'Documentation')
    const sectionRoute = router.getRoutes().find((record) => record.name === 'DocumentationSection')
    const adminRoute = router.getRoutes().find((record) => record.name === 'AdminDocumentation')

    expect(route?.path).toBe('/docs')
    expect(sectionRoute?.path).toBe('/docs/:section')
    expect(route?.meta.requiresAuth).toBe(false)
    expect(isBackendModePublicRouteAllowed('/docs', false)).toBe(true)
    expect(isBackendModePublicRouteAllowed('/docs/代理节点', false)).toBe(true)
    expect(isBackendModePublicRouteAllowed('/admin/documentation', false)).toBe(false)
    for (const record of [route, sectionRoute]) {
      expect(record?.beforeEnter).toBe(redirectToDocumentation)
      expect(record?.components).toBeFalsy()
      expect(record?.meta.requiresAuth).toBe(false)
    }
    expect(adminRoute?.beforeEnter).toBeUndefined()
    expect(adminRoute?.components?.default).toBeTypeOf('function')
    expect(adminRoute?.meta.requiresAuth).toBe(true)
    expect(adminRoute?.meta.requiresAdmin).toBe(true)
    // Keep this unrelated, more specific legacy alias in the main application.
    expect(router.resolve('/docs/batch-image').name).toBe('BatchImageGuide')
  })
})
