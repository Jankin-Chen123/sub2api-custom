import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'

const { appStore, getActiveDocumentation, getDocumentationContent, routeState, routerPush } = vi.hoisted(() => ({
  appStore: {
    siteName: 'Sub2API',
    siteLogo: '',
  },
  getActiveDocumentation: vi.fn(),
  getDocumentationContent: vi.fn(),
  routeState: {
    params: { section: 'proxy' as string | undefined },
    hash: '',
  },
  routerPush: vi.fn(),
}))

vi.mock('@/api/documentation', () => ({
  documentationAssetBase: (versionID: string) => `/api/v1/docs/versions/${versionID}`,
  getActiveDocumentation,
  getDocumentationContent,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
    locale: ref('zh-CN'),
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ push: routerPush }),
}))

import DocumentationView from '../DocumentationView.vue'

const documentation = `<details id="proxy" data-docs-section data-docs-level="2" open>
  <summary>代理节点</summary>
  <figure><a href="assets/0001.png"><img src="assets/0001.png"></a></figure>
</details>`

function mountView() {
  return mount(DocumentationView, {
    global: {
      stubs: {
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('DocumentationView', () => {
  beforeEach(() => {
    routeState.params.section = 'proxy'
    routeState.hash = ''
    getActiveDocumentation.mockReset()
    getDocumentationContent.mockReset()
    getActiveDocumentation.mockResolvedValue({
      id: 'version-id',
      title: '使用教程',
      created_at: '2026-09-12T00:00:00Z',
      assets: [],
      outline: [{ level: 2, title: '代理节点', id: 'proxy' }],
    })
    getDocumentationContent.mockResolvedValue(documentation)
    routerPush.mockReset()
    vi.stubGlobal('IntersectionObserver', class {
      observe() {}
      unobserve() {}
      disconnect() {}
    })
    vi.stubGlobal('scrollTo', vi.fn())
  })

  it('opens an image in the closable lightbox without following its asset link', async () => {
    const wrapper = mountView()
    await flushPromises()

    const image = wrapper.get('img.docs-zoomable-image')
    const click = new MouseEvent('click', { bubbles: true, cancelable: true })
    image.element.dispatchEvent(click)
    await nextTick()

    expect(click.defaultPrevented).toBe(true)
    expect(wrapper.get('.docs-lightbox img').attributes('src')).toContain('/api/v1/docs/versions/version-id/assets/0001.png')

    await wrapper.get('.docs-lightbox button').trigger('click')
    expect(wrapper.find('.docs-lightbox').exists()).toBe(false)
  })
})
