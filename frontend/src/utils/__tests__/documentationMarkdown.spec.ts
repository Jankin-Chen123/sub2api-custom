import { describe, expect, it } from 'vitest'
import { renderDocumentationContent, renderDocumentationHTML, renderDocumentationMarkdown } from '../documentationMarkdown'

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

describe('renderDocumentationHTML', () => {
  it('preserves Notion structure while decorating safe interactive blocks', () => {
    const html = renderDocumentationHTML(
      `<section class="notion-document">
        <h1 class="page-title docs-page-title" id="guide">Guide</h1>
        <aside class="callout" data-notion-callout-icon="💡"><p>Remember this.</p></aside>
        <details id="install" data-docs-section data-docs-level="2" open>
          <summary>Install</summary>
          <figure><a href="assets/0001.png"><img src="assets/0001.png" data-docs-width="480"></a></figure>
          <p><a href="https://example.com/setup">Open setup</a></p>
          <p><mark class="highlight-default">Default text</mark><mark class="highlight-default_background">Default background</mark><mark class="highlight-yellow_background">Intentional highlight</mark></p>
          <pre><code>npm install</code></pre>
          <table><tbody><tr><td>Value</td></tr></tbody></table>
        </details>
        <script>alert(1)</script>
      </section>`,
      [
        { level: 1, title: 'Guide', id: 'guide' },
        { level: 2, title: 'Install', id: 'install' },
      ],
      '/api/v1/docs/versions/version-id',
    )
    const document = new DOMParser().parseFromString(html, 'text/html')

    expect(document.querySelector('#install.docs-toggle')).not.toBeNull()
    expect(document.querySelector('#install > summary.docs-toggle-summary')).not.toBeNull()
    expect(document.querySelector('.docs-callout-notion .docs-callout-icon')?.textContent).toBe('💡')
    expect(document.querySelector('img')?.src).toContain('/api/v1/docs/versions/version-id/assets/0001.png')
    expect(document.querySelector('img')?.style.getPropertyValue('--docs-image-width')).toBe('480px')
    expect(document.querySelector('a[href="#install"]')).toBeNull()
    expect(document.querySelector('a[target="_blank"]')?.classList.contains('docs-link-card')).toBe(true)
    expect(document.querySelector('.docs-copy-button')).not.toBeNull()
    expect(document.querySelector('.docs-table-scroll > table')).not.toBeNull()
    expect(document.querySelector('.highlight-default')).not.toBeNull()
    expect(document.querySelector('.highlight-default_background')).not.toBeNull()
    expect(document.querySelector('.highlight-yellow_background')).not.toBeNull()
    expect(html).not.toContain('<script')
  })

  it('dispatches HTML content without parsing it as Markdown', () => {
    const html = renderDocumentationContent(
      '<details id="chapter" data-docs-section><summary>Chapter</summary><p>Body</p></details>',
      'html',
      [{ level: 2, title: 'Chapter', id: 'chapter' }],
      '/assets',
    )

    expect(html).toContain('<details')
    expect(html).toContain('docs-toggle-summary')
    expect(html).not.toContain('&lt;details')
  })
})
