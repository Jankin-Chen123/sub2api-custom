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

  it('registers documentation as a standalone public route', async () => {
    const { default: router, isBackendModePublicRouteAllowed } = await import('@/router')
    const route = router.getRoutes().find((record) => record.name === 'Documentation')
    const sectionRoute = router.getRoutes().find((record) => record.name === 'DocumentationSection')

    expect(route?.path).toBe('/docs')
    expect(sectionRoute?.path).toBe('/docs/:section')
    expect(route?.meta.requiresAuth).toBe(false)
    expect(isBackendModePublicRouteAllowed('/docs', false)).toBe(true)
    expect(isBackendModePublicRouteAllowed('/docs/代理节点', false)).toBe(true)
  })
})
