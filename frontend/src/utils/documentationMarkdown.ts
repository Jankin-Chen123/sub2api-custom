import DOMPurify from 'dompurify'
import { marked } from 'marked'
import type { DocumentationContentFormat, DocumentationHeading } from '@/api/documentation'

function isRelativeAsset(src: string): boolean {
  const value = src.trim()
  if (!value || value.startsWith('/') || value.startsWith('//') || /^[a-z][a-z\d+.-]*:/i.test(value)) {
    return false
  }
  const pathname = value.split(/[?#]/, 1)[0]
  if (!pathname) return false
  return pathname
    .split('/')
    .filter((part) => part && part !== '.')
    .every((part) => part !== '..' && !part.includes('\\'))
}

function joinAssetURL(baseURL: string, source: string): string {
  const match = source.trim().match(/^([^?#]*)(.*)$/)
  const pathname = match?.[1] || source.trim()
  const suffix = match?.[2] || ''
  const encoded = pathname
    .split('/')
    .filter((part) => part && part !== '.')
    .map((part) => encodeURIComponent(part))
    .join('/')
  return `${baseURL.replace(/\/+$/, '')}/${encoded}${suffix}`
}

function rewriteAssets(markdown: string, assetBaseURL: string): string {
  return markdown.replace(/!\[([^\]]*)\]\(([^)\r\n]+)\)/g, (match, alt, source) => {
    const value = String(source).trim()
    return isRelativeAsset(value) ? `![${alt}](${joinAssetURL(assetBaseURL, value)})` : match
  })
}

function rewriteHTMLAssetURL(value: string, assetBaseURL: string): string {
  return isRelativeAsset(value) ? joinAssetURL(assetBaseURL, value) : value
}

function decorateDocumentationHTML(
  document: Document,
  outline: DocumentationHeading[],
  assetBaseURL: string,
  copyLabel: string
): string {
  const sectionElements = Array.from(document.querySelectorAll<HTMLElement>('[data-docs-section]'))
  const headingElements = Array.from(document.querySelectorAll<HTMLElement>('h1, h2, h3, h4'))

  if (sectionElements.length > 0) {
    const sectionOutline = outline.filter((item) => item.level > 1)
    sectionElements.forEach((section, index) => {
      const item = sectionOutline[index]
      section.id = section.id || item?.id || `section-${index + 1}`
      section.dataset.docsLevel = section.dataset.docsLevel || String(item?.level || 2)
      section.classList.add('docs-toggle')
      section.querySelector(':scope > summary')?.classList.add('docs-toggle-summary')
    })
  }

  headingElements.forEach((heading, index) => {
    if (!heading.id) {
      const item = outline[index]
      heading.id = item?.id || `heading-${index + 1}`
    }
    if (heading.classList.contains('docs-page-title')) return
    const anchor = document.createElement('a')
    anchor.className = 'docs-heading-anchor'
    anchor.href = `#${heading.id}`
    anchor.setAttribute('aria-label', 'Link to this section')
    anchor.textContent = '#'
    heading.appendChild(anchor)
  })

  document.querySelectorAll<HTMLElement>('aside.callout').forEach((callout) => {
    callout.classList.add('docs-callout', 'docs-callout-notion')
    const icon = callout.querySelector<HTMLElement>('.icon[data-emoji]')
    const emoji = icon?.dataset.emoji || callout.dataset.notionCalloutIcon || '💡'
    if (icon) {
      icon.textContent = emoji
      icon.classList.add('docs-callout-icon')
    } else {
      const label = document.createElement('span')
      label.className = 'docs-callout-icon'
      label.textContent = emoji
      callout.insertBefore(label, callout.firstChild)
    }
  })

  document.querySelectorAll('blockquote').forEach((blockquote) => {
    const firstParagraph = blockquote.querySelector(':scope > p:first-child')
    if (!firstParagraph || !firstParagraph.textContent?.trim().startsWith('[!TIP]')) return
    firstParagraph.innerHTML = firstParagraph.innerHTML.replace('[!TIP]', '').trim()
    const label = document.createElement('span')
    label.className = 'docs-callout-label'
    label.textContent = 'TIP'
    blockquote.classList.add('docs-callout')
    blockquote.insertBefore(label, blockquote.firstChild)
    if (!firstParagraph.textContent?.trim()) firstParagraph.remove()
  })

  document.querySelectorAll<HTMLImageElement>('img').forEach((image) => {
    const source = image.getAttribute('src') || ''
    image.src = rewriteHTMLAssetURL(source, assetBaseURL)
    image.loading = image.classList.contains('page-cover-image') ? 'eager' : 'lazy'
    image.decoding = 'async'
    image.classList.add('docs-zoomable-image')
    const width = Number(image.dataset.docsWidth || 0)
    if (Number.isFinite(width) && width > 0) image.style.setProperty('--docs-image-width', `${width}px`)
  })

  document.querySelectorAll<HTMLAnchorElement>('a').forEach((link) => {
    const href = link.getAttribute('href') || ''
    const resolvedHref = isRelativeAsset(href) ? joinAssetURL(assetBaseURL, href) : href
    if (resolvedHref !== href) link.href = resolvedHref
    if (/^https?:\/\//i.test(resolvedHref)) {
      link.target = '_blank'
      link.rel = 'noopener noreferrer'
    }
    if (link.querySelector('img')) return
    link.classList.add(href.startsWith('#') ? 'docs-anchor-link' : 'docs-text-link')
    const parent = link.parentElement
    if (parent?.tagName === 'P' && parent.textContent?.trim() === link.textContent?.trim()) {
      link.classList.add('docs-link-card')
    }
  })

  document.querySelectorAll('pre').forEach((pre) => {
    const button = document.createElement('button')
    button.type = 'button'
    button.className = 'docs-copy-button'
    button.textContent = copyLabel
    pre.appendChild(button)
  })

  document.querySelectorAll('table').forEach((table) => {
    if (table.parentElement?.classList.contains('docs-table-scroll')) return
    const wrapper = document.createElement('div')
    wrapper.className = 'docs-table-scroll'
    table.parentNode?.insertBefore(wrapper, table)
    wrapper.appendChild(table)
  })

  return document.body.innerHTML
}

export function renderDocumentationMarkdown(
  markdown: string,
  outline: DocumentationHeading[],
  assetBaseURL: string,
  copyLabel = 'Copy'
): string {
  const html = marked.parse(rewriteAssets(markdown, assetBaseURL), { gfm: true }) as string
  const sanitized = DOMPurify.sanitize(html)
  const document = new DOMParser().parseFromString(sanitized, 'text/html')
  return decorateDocumentationHTML(document, outline, assetBaseURL, copyLabel)
}

export function renderDocumentationHTML(
  html: string,
  outline: DocumentationHeading[],
  assetBaseURL: string,
  copyLabel = 'Copy'
): string {
  const sanitized = DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    ADD_TAGS: ['details', 'summary'],
    ADD_ATTR: [
      'open',
      'data-docs-section',
      'data-docs-level',
      'data-docs-width',
      'data-docs-document',
      'data-notion-callout-icon',
      'data-emoji'
    ]
  })
  const document = new DOMParser().parseFromString(sanitized, 'text/html')
  return decorateDocumentationHTML(document, outline, assetBaseURL, copyLabel)
}

export function renderDocumentationContent(
  content: string,
  format: DocumentationContentFormat,
  outline: DocumentationHeading[],
  assetBaseURL: string,
  copyLabel = 'Copy'
): string {
  return format === 'html'
    ? renderDocumentationHTML(content, outline, assetBaseURL, copyLabel)
    : renderDocumentationMarkdown(content, outline, assetBaseURL, copyLabel)
}
