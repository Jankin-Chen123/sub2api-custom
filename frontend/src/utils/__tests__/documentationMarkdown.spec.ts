import { describe, expect, it } from 'vitest'
import { renderDocumentationMarkdown } from '../documentationMarkdown'

describe('renderDocumentationMarkdown', () => {
  it('pins assets, applies stable heading ids, styles callouts, and sanitizes HTML', () => {
    const html = renderDocumentationMarkdown(
      `# Guide

## Install

> [!TIP]
> Keep the ZIP flat.

![Screenshot](assets/0001.png)

<img src=x onerror="alert(1)"><script>alert(1)</script>
`,
      [
        { level: 1, title: 'Guide', id: 'guide' },
        { level: 2, title: 'Install', id: 'install' },
      ],
      '/api/v1/docs/versions/version-id',
    )

    expect(html).toContain('id="guide"')
    expect(html).toContain('id="install"')
    expect(html).toContain('class="docs-callout"')
    expect(html).toContain('/api/v1/docs/versions/version-id/assets/0001.png')
    expect(html).not.toContain('<script')
    expect(html).not.toContain('onerror')
  })

  it('does not rewrite external or traversal image URLs', () => {
    const html = renderDocumentationMarkdown(
      '![Remote](https://example.com/image.png)\n\n![Unsafe](../secret.png)',
      [],
      '/assets',
    )

    expect(html).toContain('https://example.com/image.png')
    expect(html).not.toContain('/assets/../secret.png')
  })
})
