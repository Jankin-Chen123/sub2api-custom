import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
import DocumentationEmbedView from '../DocumentationEmbedView.vue'

afterEach(() => vi.restoreAllMocks())

describe('embedded documentation management', () => {
  it('keeps navigation in the layout and only issues scoped tickets to its own same-origin frame', async () => {
    post.mockResolvedValue({ data: { ticket: 'docs-only-ticket' } })
    const wrapper = mount(DocumentationEmbedView, { attachTo: document.body })
    const frame = wrapper.get('iframe').element as HTMLIFrameElement
    expect(frame.getAttribute('src')).toBe('/docs/admin?embedded=1')
    const child = frame.contentWindow!
    const reply = vi.spyOn(child, 'postMessage')
    const send = (origin: string, source: MessageEventSource) => window.dispatchEvent(new MessageEvent('message', {
      origin, source, data: { type: 'aibaipiao-docs:ticket-request', id: 'request-1' }
    }))
    send('https://evil.example', child)
    send(window.location.origin, window)
    await flushPromises()
    expect(post).not.toHaveBeenCalled()
    send(window.location.origin, child)
    await flushPromises()
    expect(post).toHaveBeenCalledWith('/admin/integrations/docs-session')
    expect(reply).toHaveBeenCalledWith({ type: 'aibaipiao-docs:ticket-response', id: 'request-1', ticket: 'docs-only-ticket' }, window.location.origin)
    post.mockRejectedValue({ status: 403, message: 'Admin access required' })
    send(window.location.origin, child)
    await flushPromises()
    expect(reply).toHaveBeenLastCalledWith({ type: 'aibaipiao-docs:ticket-response', id: 'request-1', error: 'Admin access required', status: 403 }, window.location.origin)
    wrapper.unmount()
    post.mockClear()
    send(window.location.origin, child)
    await flushPromises()
    expect(post).not.toHaveBeenCalled()
  })
})
