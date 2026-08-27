import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ChillLionWidget from '@/components/ChillLionWidget.vue'

const mountChillLionWhenReady = vi.hoisted(() => vi.fn())

vi.mock('@/utils/chillLion', () => ({ mountChillLionWhenReady }))

describe('ChillLionWidget', () => {
  let originalRequestAnimationFrame: typeof window.requestAnimationFrame

  beforeEach(() => {
    mountChillLionWhenReady.mockReset()
    originalRequestAnimationFrame = window.requestAnimationFrame
    window.requestAnimationFrame = ((callback: FrameRequestCallback) => {
      return window.setTimeout(() => callback(performance.now()), 0)
    }) as typeof window.requestAnimationFrame
  })

  afterEach(() => {
    window.requestAnimationFrame = originalRequestAnimationFrame
  })

  it('mounts the canvas and releases it when the widget is unmounted', async () => {
    const cleanup = vi.fn()
    mountChillLionWhenReady.mockImplementation(async (element: HTMLElement) => {
      element.appendChild(document.createElement('canvas'))
      return cleanup
    })

    const wrapper = mount(ChillLionWidget)
    await vi.waitFor(() => expect(mountChillLionWhenReady).toHaveBeenCalledOnce())

    expect(wrapper.find('canvas').exists()).toBe(true)
    expect(wrapper.find('.chill-lion-fallback').exists()).toBe(false)
    wrapper.unmount()
    expect(cleanup).toHaveBeenCalledOnce()
  })

  it('shows an accessible fallback when the runtime cannot initialize', async () => {
    mountChillLionWhenReady.mockRejectedValueOnce(new Error('WebGL unavailable'))

    const wrapper = mount(ChillLionWidget)
    await vi.waitFor(() => expect(wrapper.find('.chill-lion-fallback').exists()).toBe(true))

    expect(wrapper.get('.chill-lion-fallback').text()).toContain('互动小狮子暂不可用')
  })

  it('keeps the widget hidden without disabling initialization on the docs home route', async () => {
    mountChillLionWhenReady.mockResolvedValueOnce(() => {})

    const wrapper = mount(ChillLionWidget, { props: { visible: false } })
    await vi.waitFor(() => expect(mountChillLionWhenReady).toHaveBeenCalledOnce())

    expect(wrapper.get('.docs-lion-floating').classes()).toContain('docs-lion-floating-hidden')
    expect(wrapper.get('.docs-lion-floating').attributes('aria-hidden')).toBe('true')
  })
})
