import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, reactive, ref } from 'vue'

const { appStore, getActiveDocumentation, getDocumentationContent, routeState, routerPush } =
  vi.hoisted(() => ({
    appStore: {
      siteName: 'Sub2API',
      siteLogo: ''
    },
    getActiveDocumentation: vi.fn(),
    getDocumentationContent: vi.fn(),
    routeState: {
      params: { section: 'proxy' as string | undefined },
      hash: ''
    },
    routerPush: vi.fn()
  }))

vi.mock('@/api/documentation', () => ({
  documentationAssetBase: (versionID: string) => `/api/v1/docs/versions/${versionID}`,
  getActiveDocumentation,
  getDocumentationContent
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
    locale: ref('zh-CN')
  })
}))

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push: routerPush, replace: routerPush })
}))

import DocumentationView from '../DocumentationView.vue'

const route = reactive(routeState)
const mounted: ReturnType<typeof mount>[] = []
let frames: FrameRequestCallback[] = []
function runFrames() {
  for (let i = 0; frames.length && i < 5; i++) frames.splice(0).forEach((fn) => fn(0))
}

const documentation = `<details id="proxy" data-docs-section data-docs-level="2" open>
  <summary>代理节点</summary>
  <figure><a href="assets/0001.png"><img src="assets/0001.png"></a></figure>
</details>`

function mountView() {
  const wrapper = mount(DocumentationView, {
    attachTo: document.body,
    global: {
      stubs: {
        RouterLink: { template: '<a><slot /></a>' }
      }
    }
  })
  mounted.push(wrapper)
  return wrapper
}

describe('DocumentationView', () => {
  beforeEach(() => {
    route.params.section = 'proxy'
    route.hash = ''
    localStorage.removeItem('theme')
    document.documentElement.classList.remove('dark')
    frames = []
    getActiveDocumentation.mockReset()
    getDocumentationContent.mockReset()
    getActiveDocumentation.mockResolvedValue({
      id: 'version-id',
      title: '使用教程',
      created_at: '2026-09-12T00:00:00Z',
      assets: [],
      outline: [{ level: 2, title: '代理节点', id: 'proxy' }]
    })
    getDocumentationContent.mockResolvedValue(documentation)
    routerPush.mockReset()
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      }
    )
    vi.stubGlobal('scrollTo', vi.fn())
    vi.stubGlobal('requestAnimationFrame', (fn: FrameRequestCallback) => {
      frames.push(fn)
      return frames.length
    })
    vi.stubGlobal('cancelAnimationFrame', vi.fn())
  })

  afterEach(() => {
    mounted.splice(0).forEach((wrapper) => wrapper.unmount())
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('opens an image in the closable lightbox without following its asset link', async () => {
    const wrapper = mountView()
    await flushPromises()

    const image = wrapper.get('img.docs-zoomable-image')
    const click = new MouseEvent('click', { bubbles: true, cancelable: true })
    image.element.dispatchEvent(click)
    await nextTick()

    expect(click.defaultPrevented).toBe(true)
    expect(wrapper.get('.docs-lightbox img').attributes('src')).toContain(
      '/api/v1/docs/versions/version-id/assets/0001.png'
    )

    await wrapper.get('.docs-lightbox button').trigger('click')
    expect(wrapper.find('.docs-lightbox').exists()).toBe(false)
  })

  it('uses the published version for both content and image URLs', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(getDocumentationContent).toHaveBeenCalledWith('version-id')
    expect(wrapper.get('#article img').attributes('src')).toContain(
      '/versions/version-id/assets/0001.png'
    )
  })

  it('rebuilds topics, numbering and recommendations after published deletions, renames and reordering', async () => {
    route.params.section = undefined
    const headings = [
      { level: 2, title: '新主题 B', id: 'b' },
      { level: 3, title: 'B 的步骤', id: 'b-step' },
      { level: 2, title: '重命名后的 A', id: 'a' }
    ]
    getActiveDocumentation.mockResolvedValueOnce({
      id: 'updated',
      created_at: '2026-11-03T00:00:00Z',
      assets: [],
      outline: headings
    })
    getDocumentationContent.mockResolvedValueOnce(
      '<h2 id="b">新主题 B</h2><h3 id="b-step">B 的步骤</h3><h2 id="a">重命名后的 A</h2>'
    )
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findAll('.chapter strong').map((el) => el.text())).toEqual([
      '新主题 B',
      '重命名后的 A'
    ])
    expect(wrapper.findAll('.number').map((el) => el.text())).toEqual(['01', '02'])
    expect(wrapper.get('.reading-note').text()).toContain('新主题 B')
    expect(wrapper.get('.reading-note').text()).not.toContain('下载Agent产品')
    expect(wrapper.get('.chapter p').text()).toBe('B 的步骤')
    expect(wrapper.get('.chapter-scroll').attributes('tabindex')).toBe('0')
  })

  it('keeps imported introduction content on the overview', async () => {
    route.params.section = undefined
    getActiveDocumentation.mockResolvedValueOnce({
      id: 'version-id',
      content_format: 'html',
      created_at: '2026-11-03T00:00:00Z',
      assets: [],
      outline: [{ level: 2, title: '代理节点', id: 'proxy' }]
    })
    getDocumentationContent.mockResolvedValueOnce(
      '<article><h1 id="page-title" class="docs-page-title">使用教程</h1><p>本次更新的真实前言</p><h2 id="proxy">代理节点</h2><p>正文</p></article>'
    )
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('.introduction').text()).toContain('本次更新的真实前言')
  })

  it('preserves dark mode across overview and reader navigation, reload and storage events', async () => {
    route.params.section = undefined
    localStorage.setItem('theme', 'dark')
    const wrapper = mountView()
    await flushPromises()
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    route.params.section = 'proxy'
    await nextTick()
    expect(wrapper.find('.tutorial-reader').exists()).toBe(true)
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    await wrapper.get('.theme').trigger('click')
    expect(localStorage.getItem('theme')).toBe('light')
    localStorage.setItem('theme', 'dark')
    window.dispatchEvent(new StorageEvent('storage', { key: 'theme', newValue: 'dark' }))
    await nextTick()
    expect(wrapper.get('.theme').attributes('aria-pressed')).toBe('true')
    route.params.section = undefined
    await nextTick()
    window.dispatchEvent(new PageTransitionEvent('pageshow'))
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('highlights the selected section at the anchor offset and ignores children of collapsed sections', async () => {
    getActiveDocumentation.mockResolvedValueOnce({
      id: 'version-id',
      content_format: 'html',
      created_at: '2026-11-03T00:00:00Z',
      assets: [],
      outline: [
        { level: 2, title: '代理节点', id: 'proxy' },
        { level: 3, title: '3. 获取支付长链接', id: 'step-3' },
        { level: 3, title: '4. 填写支付信息', id: 'step-4' },
        { level: 4, title: '隐藏的子标题', id: 'hidden-child' }
      ]
    })
    getDocumentationContent.mockResolvedValueOnce(
      '<details id="proxy" open><summary>代理节点</summary><h3 id="step-3">3. 获取支付长链接</h3><details id="step-4"><summary>4. 填写支付信息</summary><h4 id="hidden-child">隐藏的子标题</h4></details></details>'
    )
    const wrapper = mountView()
    await flushPromises()
    vi.spyOn(Element.prototype, 'getClientRects').mockReturnValue([{}] as unknown as DOMRectList)
    vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (
      this: Element
    ) {
      const id = this.tagName === 'SUMMARY' ? this.parentElement?.id : this.id
      return { top: id === 'step-3' ? -200 : id === 'step-4' ? 97 : 0 } as DOMRect
    })
    window.dispatchEvent(new Event('scroll'))
    runFrames()
    await nextTick()
    expect(wrapper.get('#page-toc .active').text()).toBe('4. 填写支付信息')
    await wrapper.get('#page-toc .active').trigger('click')
    runFrames()
    expect((wrapper.get('#step-4').element as HTMLDetailsElement).open).toBe(true)
    expect(routerPush).toHaveBeenCalledWith(expect.objectContaining({ hash: '#step-4' }))
    expect(window.scrollTo).toHaveBeenLastCalledWith(expect.objectContaining({ top: 1 }))
  })

  it('passes Chinese hashes to Vue Router without pre-encoding them', async () => {
    getActiveDocumentation.mockResolvedValueOnce({
      id: 'version-id',
      content_format: 'html',
      created_at: '2026-11-03T00:00:00Z',
      assets: [],
      outline: [
        { level: 2, title: '代理节点', id: 'proxy' },
        { level: 3, title: '填写支付信息', id: '填写支付信息' }
      ]
    })
    getDocumentationContent.mockResolvedValueOnce(
      '<h2 id="proxy">代理节点</h2><h3 id="填写支付信息">填写支付信息</h3>'
    )
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('#page-toc button').trigger('click')
    expect(routerPush).toHaveBeenCalledWith(expect.objectContaining({ hash: '#填写支付信息' }))
  })

  it('shows retryable failures and can recover without stale errors', async () => {
    route.params.section = undefined
    getActiveDocumentation.mockRejectedValueOnce({ status: 503, message: '服务暂时不可用' })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('服务暂时不可用')
    await wrapper.get('.introduction .primary').trigger('click')
    await flushPromises()
    expect(wrapper.find('.hero').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('服务暂时不可用')
  })

  it('keeps the community and resource URLs available', async () => {
    route.params.section = undefined
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('a.qq-community').attributes('href')).toBe('https://qm.qq.com/q/GKy4AywLaQ')
    expect(wrapper.get('.resource-service').text()).toContain('👌爱白嫖公益站')
    expect(wrapper.get('.resource-shop').text()).toContain('🎈气球小铺')
  })
})
