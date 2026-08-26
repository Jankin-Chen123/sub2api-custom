import DOMPurify from 'dompurify'
import { marked } from 'marked'
import type { DocumentationHeading } from '@/api/documentation'

function isRelativeAsset(src: string): boolean {
  const value = src.trim()
  if (!value || value.startsWith('/') || value.startsWith('//') || /^[a-z][a-z\d+.-]*:/i.test(value)) {
    return false
  }
  return value
    .split(/[?#]/, 1)[0]
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

export function renderDocumentationMarkdown(
  markdown: string,
  outline: DocumentationHeading[],
  assetBaseURL: string,
  copyLabel = 'Copy'
): string {
  const html = marked.parse(rewriteAssets(markdown, assetBaseURL), { gfm: true }) as string
  const sanitized = DOMPurify.sanitize(html)
  const document = new DOMParser().parseFromString(sanitized, 'text/html')

  document.querySelectorAll('h1, h2, h3, h4').forEach((heading, index) => {
    const item = outline[index]
    heading.id = item?.id || `section-${index + 1}`
    const anchor = document.createElement('a')
    anchor.className = 'docs-heading-anchor'
    anchor.href = `#${heading.id}`
    anchor.setAttribute('aria-label', 'Link to this section')
    anchor.textContent = '#'
    heading.appendChild(anchor)
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

  document.querySelectorAll('a').forEach((link) => {
    const href = link.getAttribute('href') || ''
    if (/^https?:\/\//i.test(href)) {
      link.target = '_blank'
      link.rel = 'noopener noreferrer'
    }
  })

  document.querySelectorAll('img').forEach((image) => {
    image.loading = 'lazy'
    image.decoding = 'async'
    image.classList.add('docs-zoomable-image')
  })

  document.querySelectorAll('pre').forEach((pre) => {
    const button = document.createElement('button')
    button.type = 'button'
    button.className = 'docs-copy-button'
    button.textContent = copyLabel
    pre.appendChild(button)
  })

  return document.body.innerHTML
}
