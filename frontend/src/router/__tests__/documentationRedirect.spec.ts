import { afterEach, describe, expect, it, vi } from 'vitest'
import { redirectToDocumentation } from '../documentationRedirect'

afterEach(() => vi.unstubAllGlobals())

describe('independent documentation navigation', () => {
  it.each([
    ['Documentation', '/docs', '/docs?theme=dark#start', '/docs?theme=dark#start'],
    ['DocumentationSection', '/docs/代理节点', '/docs/代理节点?preview=1#配置', '/docs/代理节点?preview=1#配置'],
    ['DocumentationSection', '/docs/admin', '/docs/admin', '/docs/admin'],
    ['AdminDocumentation', '/admin/documentation', '/admin/documentation?tab=history#versions', '/docs/admin?tab=history#versions'],
  ])('leaves the SPA for %s and preserves query and fragment', (name, path, fullPath, target) => {
    const replace = vi.fn()
    vi.stubGlobal('window', { location: { replace } })

    expect(redirectToDocumentation({ name, path, fullPath })).toBe(false)
    expect(replace).toHaveBeenCalledTimes(1)
    expect(replace).toHaveBeenCalledWith(`https://aibaipiao.top${target}`)
  })
})
