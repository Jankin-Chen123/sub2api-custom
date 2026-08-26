<template>
  <div class="docs-shell" :class="{ 'docs-home-mode': !currentSection }">
    <div class="docs-progress" :style="{ transform: `scaleX(${scrollProgress})` }"></div>

    <header class="docs-header">
      <div class="docs-header-inner">
        <div class="docs-brand-wrap">
          <button class="docs-icon-button docs-mobile-only" :aria-label="t('documentation.public.openMenu')" @click="mobileOpen = true">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg>
          </button>
          <router-link to="/home" class="docs-brand">
            <span class="docs-logo"><img :src="appStore.siteLogo || '/logo.svg'" alt="" /></span>
            <span>{{ appStore.siteName }}</span>
            <span class="docs-brand-divider"></span>
            <span class="docs-brand-section">Docs</span>
          </router-link>
        </div>

        <div class="docs-search-wrap">
          <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-4-4" /></svg>
          <input v-model="searchQuery" :placeholder="t('documentation.public.search')" @focus="searchFocused = true" @keydown.escape="searchFocused = false" />
          <kbd>⌘ K</kbd>
          <div v-if="searchFocused && searchQuery" class="docs-search-results">
            <button v-for="item in filteredHeadings.slice(0, 8)" :key="item.id" @click="selectSearchResult(item.id)">
              <span>{{ item.title }}</span><small>H{{ item.level }}</small>
            </button>
            <div v-if="filteredHeadings.length === 0" class="docs-search-empty">{{ t('documentation.public.noResults') }}</div>
          </div>
        </div>

        <nav class="docs-header-actions">
          <router-link to="/home" class="docs-header-link">{{ t('documentation.public.backHome') }}</router-link>
          <button class="docs-icon-button" :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')" @click="toggleTheme">
            <svg v-if="isDark" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42M17.65 17.65l1.42 1.42M2 12h2M20 12h2M4.93 19.07l1.42-1.42M17.65 6.35l1.42-1.42" /></svg>
            <svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="M20.5 15.5A8.5 8.5 0 118.5 3.5a7 7 0 0012 12z" /></svg>
          </button>
        </nav>
      </div>
    </header>

    <div v-if="mobileOpen" class="docs-mobile-backdrop" @click="mobileOpen = false"></div>

    <aside class="docs-sidebar" :class="{ 'docs-sidebar-open': mobileOpen }">
      <div class="docs-sidebar-head">
        <span>{{ t('documentation.public.contents') }}</span>
        <button class="docs-icon-button docs-mobile-only" :aria-label="t('documentation.public.close')" @click="mobileOpen = false">×</button>
      </div>
      <div class="docs-mobile-search docs-mobile-only">
        <input v-model="searchQuery" :placeholder="t('documentation.public.search')" />
      </div>
      <nav v-if="manifest" class="docs-tree">
        <button
          v-for="item in sidebarHeadings"
          :key="item.id"
          :class="[`docs-level-${item.level}`, { active: activeHeading === item.id }]"
          @click="navigateToHeading(item.id)"
        >
          <span class="docs-tree-dot"></span><span>{{ item.title }}</span>
        </button>
      </nav>
      <div class="docs-sidebar-footer">
        <span class="docs-status-dot"></span>
        <span v-if="manifest">{{ t('documentation.public.updated', { date: formatDate(manifest.published_at || manifest.created_at) }) }}</span>
        <span v-else>{{ t('documentation.public.notPublished') }}</span>
      </div>
    </aside>

    <main class="docs-main">
      <div v-if="loading" class="docs-loading">
        <div class="docs-skeleton docs-skeleton-kicker"></div>
        <div class="docs-skeleton docs-skeleton-title"></div>
        <div v-for="index in 8" :key="index" class="docs-skeleton" :style="{ width: `${92 - (index % 3) * 12}%` }"></div>
      </div>

      <section v-else-if="errorMessage || !manifest" class="docs-empty-state">
        <div class="docs-empty-orbit"><span>✦</span></div>
        <h1>{{ errorStatus === 404 ? t('documentation.public.emptyTitle') : t('documentation.public.loadFailed') }}</h1>
        <p>{{ errorStatus === 404 ? t('documentation.public.emptyDescription') : errorMessage }}</p>
        <router-link to="/home">{{ t('documentation.public.backHome') }}</router-link>
      </section>

      <template v-else>
        <article class="docs-article">
          <div v-if="!currentSection" class="docs-hero">
            <div class="docs-hero-glow docs-hero-glow-one"></div>
            <div class="docs-hero-glow docs-hero-glow-two"></div>
            <div class="docs-eyebrow"><span></span> KNOWLEDGE BASE</div>
            <h1>{{ manifest.title }}</h1>
            <div class="docs-meta">
              <span>{{ t('documentation.public.sections', { count: manifest.outline.length }) }}</span>
              <span>{{ t('documentation.public.images', { count: manifest.assets.length }) }}</span>
              <span>{{ formatDate(manifest.published_at || manifest.created_at) }}</span>
            </div>
            <div class="docs-hero-actions">
              <button class="docs-primary-action" @click="startReading">
                {{ t('documentation.public.startReading') }}
              </button>
              <router-link to="/home" class="docs-secondary-action">
                {{ t('documentation.public.visitWebsite') }}
              </router-link>
            </div>
          </div>
          <section v-if="!currentSection" class="docs-home-panels">
            <section class="docs-resource-navigation">
              <div class="docs-resource-heading">
                <div>
                  <span class="docs-resource-kicker">EXPLORE MORE</span>
                  <h2>网址导航</h2>
                  <p>常用入口集中在这里，点击卡片即可访问。</p>
                </div>
                <span class="docs-resource-status"><i></i> 在线入口</span>
              </div>
              <div class="docs-resource-grid">
                <div v-for="resource in resourceLinks" :key="resource.url" class="docs-resource-card" :class="resource.theme">
                  <a class="docs-resource-main" :href="resource.url" target="_blank" rel="noopener noreferrer">
                    <span class="docs-resource-icon">{{ resource.icon }}</span>
                    <span class="docs-resource-copy">
                      <strong>{{ resource.name }}</strong>
                      <small>{{ resource.host }}</small>
                      <span>{{ resource.description }}</span>
                    </span>
                    <span class="docs-resource-arrow" aria-hidden="true">↗</span>
                  </a>
                </div>
              </div>
            </section>
            <section class="docs-introduction-panel">
              <div v-if="introductionHTML" ref="contentRef" class="docs-content docs-introduction" v-html="introductionHTML" @click="handleContentClick"></div>
            </section>
          </section>
          <router-link v-if="currentSection" to="/docs" class="docs-back-navigation">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m15 18-6-6 6-6" /></svg>
            {{ t('documentation.public.allSections') }}
          </router-link>
          <div v-if="currentSection" ref="contentRef" class="docs-content docs-section-page" v-html="renderedHTML" @click="handleContentClick"></div>
          <footer v-if="currentSection" class="docs-article-footer">
            <span>{{ t('documentation.public.endOfDocument') }}</span>
            <button @click="scrollToTop">{{ t('documentation.public.backToTop') }}</button>
          </footer>
        </article>

        <aside v-if="currentSection" class="docs-page-outline">
          <template v-if="pageHeadings.length">
            <div class="docs-outline-label">{{ t('documentation.public.onThisPage') }}</div>
            <button
              v-for="item in pageHeadings"
              :key="item.id"
              :class="{ active: activeHeading === item.id }"
              @click="navigateToHeading(item.id)"
            >{{ item.title }}</button>
          </template>
        </aside>
      </template>
    </main>

    <div v-if="currentSection" class="docs-lion-floating" aria-label="互动小狮子">
      <ChillLionWidget />
    </div>

    <div v-if="lightboxSource" class="docs-lightbox" @click="lightboxSource = ''">
      <button :aria-label="t('documentation.public.close')">×</button>
      <img :src="lightboxSource" alt="" @click.stop />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import ChillLionWidget from '@/components/ChillLionWidget.vue'
import { useAppStore } from '@/stores'
import {
  documentationAssetBase,
  getActiveDocumentation,
  getDocumentationContent,
  type DocumentationHeading,
  type DocumentationManifest
} from '@/api/documentation'
import { renderDocumentationContent } from '@/utils/documentationMarkdown'

const { t, locale } = useI18n()
const appStore = useAppStore()
const route = useRoute()
const router = useRouter()
const manifest = ref<DocumentationManifest | null>(null)
const fullRenderedHTML = ref('')
const loading = ref(true)
const errorMessage = ref('')
const errorStatus = ref(0)
const searchQuery = ref('')
const searchFocused = ref(false)
const mobileOpen = ref(false)
const activeHeading = ref('')
const scrollProgress = ref(0)
const lightboxSource = ref('')
const contentRef = ref<HTMLElement | null>(null)
const isDark = ref(document.documentElement.classList.contains('dark'))
let revealObserver: IntersectionObserver | null = null
let scrollFrame = 0

const resourceLinks = [
  {
    name: '爱白嫖公益站',
    host: 'aibaipiao.top',
    url: 'https://aibaipiao.top',
    description: '周末开启超低倍率，一个API连接所有模型',
    icon: '👌',
    theme: 'docs-resource-card-indigo',
  },
  {
    name: '气球小铺',
    host: 'pay.ldxp.cn/shop/qiqiuxiaopu',
    url: 'https://pay.ldxp.cn/shop/qiqiuxiaopu',
    description: '便捷、可靠的数字服务入口',
    icon: '🎈',
    theme: 'docs-resource-card-cyan',
  },
] as const

const topLevelHeadings = computed(() => (manifest.value?.outline || []).filter((item) => item.level === 2))
const currentSection = computed<DocumentationHeading | null>(() => {
  const rawSection = route.params.section
  const sectionID = Array.isArray(rawSection) ? rawSection[0] : rawSection
  if (!sectionID) return null
  return topLevelHeadings.value.find((item) => item.id === sectionID) || null
})
const currentSectionHeadings = computed(() => {
  if (!manifest.value || !currentSection.value) return []
  const start = manifest.value.outline.findIndex((item) => item.id === currentSection.value?.id)
  if (start < 0) return []
  let end = manifest.value.outline.length
  for (let index = start + 1; index < manifest.value.outline.length; index += 1) {
    if (manifest.value.outline[index].level === 2) {
      end = index
      break
    }
  }
  return manifest.value.outline.slice(start, end)
})
const renderedHTML = computed(() => {
  if (!currentSection.value) return ''
  return extractDocumentationSection(fullRenderedHTML.value, currentSection.value.id)
})
const introductionHTML = computed(() => extractDocumentationIntroduction(fullRenderedHTML.value, topLevelHeadings.value[0]?.id || ''))

const filteredHeadings = computed(() => {
  const query = searchQuery.value.trim().toLocaleLowerCase(locale.value)
  if (!manifest.value) return []
  if (!query) return manifest.value.outline
  return manifest.value.outline.filter((item) => item.title.toLocaleLowerCase(locale.value).includes(query))
})

const sidebarHeadings = computed(() => filteredHeadings.value.filter((item) => item.level === 2))
const pageHeadings = computed(() => currentSectionHeadings.value.filter((item) => item.level > 2).slice(0, 16))

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'short', day: 'numeric' }).format(new Date(value))
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function extractDocumentationSection(html: string, sectionID: string): string {
  if (!html || !sectionID) return ''
  const parsed = new DOMParser().parseFromString(`<div id="docs-section-root">${html}</div>`, 'text/html')
  const target = parsed.getElementById(sectionID)
  if (!target) return ''
  if (target instanceof HTMLDetailsElement) {
    target.open = true
    return `<section class="notion-document docs-section-document">${target.outerHTML}</section>`
  }

  const headingMatch = target.tagName.match(/^H([1-6])$/)
  if (!headingMatch) return target.outerHTML
  const level = Number(headingMatch[1])
  const sectionNodes: string[] = []
  let node: Element | null = target
  while (node) {
    if (node !== target) {
      const nextHeading = node.tagName.match(/^H([1-6])$/)
      if (nextHeading && Number(nextHeading[1]) <= level) break
    }
    sectionNodes.push(node.outerHTML)
    node = node.nextElementSibling
  }
  return `<section class="notion-document docs-section-document">${sectionNodes.join('')}</section>`
}

function extractDocumentationIntroduction(html: string, firstSectionID: string): string {
  if (!html || !firstSectionID) return ''
  const parsed = new DOMParser().parseFromString(`<div id="docs-introduction-root">${html}</div>`, 'text/html')
  const firstSection = parsed.getElementById(firstSectionID)
  const parent = firstSection?.parentElement
  if (!firstSection || !parent) return ''
  const introductionNodes: string[] = []
  for (const child of Array.from(parent.children)) {
    if (child === firstSection) break
    if (child.matches('.docs-page-title,.page-cover-image,.page-header-icon')) continue
    if (child.matches('aside.docs-callout-notion, aside[data-notion-callout]')) {
      const calloutBody = child.querySelector<HTMLElement>(':scope > div:last-child')
      if (calloutBody) {
        const bodyChildren = Array.from(calloutBody.children)
        const firstParagraph = bodyChildren[0]
        if (firstParagraph?.tagName === 'P' && firstParagraph.textContent?.trim() === '前言') {
          firstParagraph.remove()
        }
        introductionNodes.push(...Array.from(calloutBody.children).map((node) => node.outerHTML))
        continue
      }
    }
    introductionNodes.push(child.outerHTML)
  }
  if (introductionNodes.length === 0) return ''
  return `<section class="notion-document docs-introduction-document">${introductionNodes.join('')}</section>`
}

function owningSection(id: string): DocumentationHeading | null {
  if (!manifest.value) return null
  let owner: DocumentationHeading | null = null
  for (const item of manifest.value.outline) {
    if (item.level === 2) owner = item
    if (item.id === id) return owner
  }
  return null
}

function scrollToHeading(id: string, updateLocation = true) {
  const element = document.getElementById(id)
  if (!element) return
  if (element instanceof HTMLDetailsElement) element.open = true
  let parent = element.parentElement?.closest('details')
  while (parent) {
    parent.open = true
    parent = parent.parentElement?.closest('details')
  }
  element.scrollIntoView({ behavior: 'smooth', block: 'start' })
  activeHeading.value = id
  mobileOpen.value = false
  if (updateLocation) {
    const hash = currentSection.value?.id === id ? '' : `#${encodeURIComponent(id)}`
    history.replaceState(null, '', `${window.location.pathname}${hash}`)
  }
}

async function navigateToHeading(id: string) {
  const section = owningSection(id)
  if (!section) return
  searchFocused.value = false
  mobileOpen.value = false
  if (currentSection.value?.id !== section.id) {
    await router.push({
      name: 'DocumentationSection',
      params: { section: section.id },
      hash: id === section.id ? '' : `#${encodeURIComponent(id)}`
    })
    return
  }
  scrollToHeading(id)
}

function selectSearchResult(id: string) {
  searchQuery.value = ''
  void navigateToHeading(id)
}

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function startReading() {
  const first = topLevelHeadings.value[0]
  if (first) void navigateToHeading(first.id)
}

async function handleContentClick(event: MouseEvent) {
  const target = event.target as HTMLElement
  const image = target.closest<HTMLImageElement>('img.docs-zoomable-image')
  if (image) {
    lightboxSource.value = image.currentSrc || image.src
    return
  }
  const link = target.closest<HTMLAnchorElement>('a[href^="#"]')
  if (link) {
    event.preventDefault()
    const id = decodeURIComponent((link.getAttribute('href') || '').slice(1))
    if (id) void navigateToHeading(id)
    return
  }
  const button = target.closest<HTMLButtonElement>('.docs-copy-button')
  if (!button) return
  const code = button.closest('pre')?.querySelector('code')?.textContent || ''
  try {
    await navigator.clipboard.writeText(code)
    button.textContent = t('documentation.public.copied')
    window.setTimeout(() => { button.textContent = t('documentation.public.copy') }, 1600)
  } catch {
    button.textContent = t('documentation.public.copy')
  }
}

function updateScrollState() {
  if (scrollFrame) return
  scrollFrame = requestAnimationFrame(() => {
    scrollFrame = 0
    const max = document.documentElement.scrollHeight - window.innerHeight
    scrollProgress.value = max > 0 ? Math.min(1, window.scrollY / max) : 0
    const headings = currentSectionHeadings.value
    let current = currentSection.value?.id || ''
    for (const item of headings) {
      const element = document.getElementById(item.id)
      if (element && element.getBoundingClientRect().top <= 130) current = item.id
    }
    activeHeading.value = current
  })
}

async function refreshSectionView() {
  if (loading.value || !manifest.value) return
  if (route.params.section && !currentSection.value) {
    await router.replace({ name: 'Documentation' })
    return
  }
  await nextTick()
  installRevealAnimations()
  const rawHash = route.hash.slice(1)
  const targetID = rawHash ? decodeURIComponent(rawHash) : ''
  if (targetID) {
    window.setTimeout(() => scrollToHeading(targetID, false), 80)
  } else {
    window.scrollTo({ top: 0, behavior: 'auto' })
    activeHeading.value = currentSection.value?.id || ''
  }
}

function installRevealAnimations() {
  revealObserver?.disconnect()
  revealObserver = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (entry.isIntersecting) {
        entry.target.classList.add('docs-visible')
        revealObserver?.unobserve(entry.target)
      }
    })
  }, { rootMargin: '0px 0px -8% 0px', threshold: 0.04 })
  contentRef.value?.querySelectorAll(':scope > *, .notion-document > *').forEach((element) => {
    element.classList.add('docs-reveal')
    revealObserver?.observe(element)
  })
}

async function loadDocumentation() {
  loading.value = true
  try {
    const active = await getActiveDocumentation()
    const content = await getDocumentationContent(active.id)
    manifest.value = active
    fullRenderedHTML.value = renderDocumentationContent(
      content,
      active.content_format || 'markdown',
      active.outline,
      documentationAssetBase(active.id),
      t('documentation.public.copy')
    )
  } catch (error) {
    const detail = error as { status?: number; message?: string }
    errorStatus.value = detail.status || 0
    errorMessage.value = detail.message || t('documentation.public.loadFailed')
  } finally {
    loading.value = false
  }
  await refreshSectionView()
}

function handleSearchShortcut(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    const input = document.querySelector<HTMLInputElement>('.docs-search-wrap input')
    input?.focus()
  }
  if (event.key === 'Escape' && lightboxSource.value) lightboxSource.value = ''
}

watch([() => route.params.section, () => route.hash], () => {
  void refreshSectionView()
})

onMounted(() => {
  void loadDocumentation()
  window.addEventListener('scroll', updateScrollState, { passive: true })
  window.addEventListener('keydown', handleSearchShortcut)
})

onBeforeUnmount(() => {
  revealObserver?.disconnect()
  window.removeEventListener('scroll', updateScrollState)
  window.removeEventListener('keydown', handleSearchShortcut)
  if (scrollFrame) cancelAnimationFrame(scrollFrame)
})
</script>

<style scoped>
.docs-shell { --docs-accent: #6366f1; --docs-cyan: #06b6d4; min-height: 100vh; overflow-x: clip; color: #172033; background: radial-gradient(circle at 78% -10%, rgba(99,102,241,.12), transparent 30rem), #fbfcff; }
.dark .docs-shell { color: #dbe5f5; background: radial-gradient(circle at 78% -10%, rgba(99,102,241,.18), transparent 34rem), #0b1020; }
.docs-shell::before { content: ''; position: fixed; inset: 0; pointer-events: none; opacity: .35; background-image: linear-gradient(rgba(99,102,241,.035) 1px, transparent 1px), linear-gradient(90deg, rgba(99,102,241,.035) 1px, transparent 1px); background-size: 32px 32px; mask-image: linear-gradient(to bottom, black, transparent 45%); }
.docs-progress { position: fixed; z-index: 100; top: 0; left: 0; width: 100%; height: 3px; transform-origin: left; background: linear-gradient(90deg, var(--docs-accent), var(--docs-cyan)); box-shadow: 0 0 14px rgba(6,182,212,.55); }
.docs-header { position: fixed; z-index: 60; inset: 0 0 auto; height: 68px; border-bottom: 1px solid rgba(148,163,184,.18); background: rgba(251,252,255,.82); backdrop-filter: blur(18px) saturate(150%); }
.dark .docs-header { background: rgba(11,16,32,.82); border-color: rgba(71,85,105,.35); }
.docs-header-inner { height: 100%; display: flex; align-items: center; justify-content: space-between; gap: 24px; padding: 0 26px; }
.docs-brand-wrap,.docs-brand,.docs-header-actions { display: flex; align-items: center; }
.docs-brand { gap: 10px; color: inherit; font-weight: 750; letter-spacing: -.02em; text-decoration: none; white-space: nowrap; }
.docs-logo { display: grid; place-items: center; width: 34px; height: 34px; overflow: hidden; border-radius: 11px; background: linear-gradient(135deg,#eef2ff,#cffafe); box-shadow: 0 7px 20px rgba(99,102,241,.18); }
.docs-logo img { width: 100%; height: 100%; object-fit: contain; }
.docs-brand-divider { width: 1px; height: 20px; margin-left: 3px; background: #cbd5e1; }
.dark .docs-brand-divider { background: #334155; }
.docs-brand-section { color: #64748b; font-weight: 600; }
.docs-search-wrap { position: relative; display: flex; align-items: center; width: min(430px, 36vw); }
.docs-search-wrap > svg { position: absolute; left: 14px; width: 17px; fill: none; stroke: #94a3b8; stroke-width: 1.8; }
.docs-search-wrap input { width: 100%; height: 40px; padding: 0 62px 0 42px; border: 1px solid rgba(148,163,184,.28); border-radius: 12px; outline: none; color: inherit; background: rgba(255,255,255,.68); transition: .2s ease; }
.dark .docs-search-wrap input { background: rgba(30,41,59,.62); border-color: rgba(100,116,139,.35); }
.docs-search-wrap input:focus { border-color: rgba(99,102,241,.6); box-shadow: 0 0 0 4px rgba(99,102,241,.1); }
.docs-search-wrap kbd { position: absolute; right: 11px; padding: 2px 7px; border: 1px solid #dbe2eb; border-radius: 6px; color: #94a3b8; font-size: 11px; background: rgba(248,250,252,.85); }
.dark .docs-search-wrap kbd { border-color: #475569; background: #1e293b; }
.docs-search-results { position: absolute; top: 48px; left: 0; right: 0; padding: 7px; border: 1px solid rgba(148,163,184,.25); border-radius: 14px; background: rgba(255,255,255,.97); box-shadow: 0 24px 60px rgba(15,23,42,.16); }
.dark .docs-search-results { background: #151d30; border-color: #334155; }
.docs-search-results button { display: flex; width: 100%; justify-content: space-between; padding: 9px 10px; border-radius: 9px; text-align: left; font-size: 13px; }
.docs-search-results button:hover { background: rgba(99,102,241,.09); color: var(--docs-accent); }
.docs-search-results small { color: #94a3b8; }
.docs-search-empty { padding: 16px; color: #94a3b8; text-align: center; font-size: 13px; }
.docs-header-actions { gap: 8px; }
.docs-header-link { padding: 7px 10px; border-radius: 9px; color: #64748b; font-size: 13px; text-decoration: none; }
.docs-header-link:hover { color: var(--docs-accent); background: rgba(99,102,241,.08); }
.docs-icon-button { display: grid; place-items: center; width: 38px; height: 38px; border-radius: 10px; color: #64748b; transition: .2s; }
.docs-icon-button:hover { color: var(--docs-accent); background: rgba(99,102,241,.09); transform: translateY(-1px); }
.docs-icon-button svg { width: 19px; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; }
.docs-sidebar { position: fixed; z-index: 50; top: 68px; bottom: 0; left: 0; display: flex; flex-direction: column; width: 276px; border-right: 1px solid rgba(148,163,184,.17); background: rgba(249,250,253,.7); backdrop-filter: blur(12px); }
.dark .docs-sidebar { background: rgba(11,16,32,.68); border-color: rgba(71,85,105,.3); }
.docs-sidebar-head { display: flex; align-items: center; justify-content: space-between; padding: 25px 24px 12px; color: #94a3b8; font-size: 11px; font-weight: 800; letter-spacing: .13em; text-transform: uppercase; }
.docs-tree { flex: 1; overflow-y: auto; padding: 4px 14px 24px; }
.docs-tree button { position: relative; display: flex; align-items: center; gap: 10px; width: 100%; min-height: 36px; padding: 7px 12px; border-radius: 9px; color: #64748b; text-align: left; font-size: 13px; transition: .18s; }
.dark .docs-tree button { color: #94a3b8; }
.docs-tree button:hover { color: #334155; background: rgba(148,163,184,.09); }
.dark .docs-tree button:hover { color: #e2e8f0; }
.docs-tree button.active { color: #4f46e5; background: linear-gradient(90deg,rgba(99,102,241,.12),rgba(6,182,212,.04)); font-weight: 650; }
.dark .docs-tree button.active { color: #a5b4fc; }
.docs-tree-dot { width: 5px; height: 5px; flex: 0 0 auto; border-radius: 99px; background: #cbd5e1; }
.docs-tree button.active .docs-tree-dot { background: var(--docs-accent); box-shadow: 0 0 0 4px rgba(99,102,241,.12); }
.docs-tree .docs-level-3 { padding-left: 28px; font-size: 12.5px; }
.docs-tree .docs-level-4 { padding-left: 43px; font-size: 12px; }
.docs-sidebar-footer { display: flex; align-items: center; gap: 8px; padding: 16px 22px; border-top: 1px solid rgba(148,163,184,.14); color: #94a3b8; font-size: 11px; }
.docs-status-dot { width: 7px; height: 7px; border-radius: 50%; background: #22c55e; box-shadow: 0 0 0 4px rgba(34,197,94,.12); }
.docs-main { min-height: 100vh; margin-left: 276px; padding: 68px 336px 0 0; }
.docs-article { width: min(860px, calc(100vw - 600px)); margin: 0 auto; padding: 72px 34px 100px; }
.docs-hero { position: relative; isolation: isolate; overflow: hidden; padding: 38px 40px 42px; margin: -18px -40px 38px; border: 1px solid rgba(99,102,241,.13); border-radius: 26px; background: linear-gradient(145deg,rgba(255,255,255,.78),rgba(238,242,255,.62)); box-shadow: 0 24px 70px rgba(64,74,145,.09); }
.dark .docs-hero { border-color: rgba(129,140,248,.18); background: linear-gradient(145deg,rgba(23,30,54,.86),rgba(15,23,42,.7)); box-shadow: 0 28px 80px rgba(0,0,0,.22); }
.docs-hero::after { content: ''; position: absolute; bottom: -1px; left: 0; width: 88px; height: 2px; background: linear-gradient(90deg,var(--docs-accent),var(--docs-cyan)); }
.docs-hero-glow { position: absolute; z-index: -1; border-radius: 50%; filter: blur(2px); pointer-events: none; }
.docs-hero-glow-one { top: -110px; right: -60px; width: 260px; height: 260px; background: radial-gradient(circle,rgba(99,102,241,.23),transparent 68%); animation: docs-orbit 9s ease-in-out infinite; }
.docs-hero-glow-two { right: 170px; bottom: -120px; width: 210px; height: 210px; background: radial-gradient(circle,rgba(6,182,212,.17),transparent 70%); animation: docs-orbit 11s ease-in-out -3s infinite reverse; }
.docs-eyebrow { display: flex; align-items: center; gap: 9px; color: #6366f1; font-size: 11px; font-weight: 800; letter-spacing: .16em; }
.docs-eyebrow span { width: 22px; height: 1px; background: currentColor; }
.docs-hero h1 { margin: 15px 0 16px; font-size: clamp(38px,5vw,62px); line-height: 1.08; letter-spacing: -.045em; font-weight: 850; color: #111827; }
.dark .docs-hero h1 { color: #f8fafc; }
.docs-meta { display: flex; flex-wrap: wrap; gap: 9px 18px; color: #94a3b8; font-size: 12px; }
.docs-meta span:not(:first-child)::before { content: '•'; margin-right: 18px; color: #cbd5e1; }
.docs-hero-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 26px; }
.docs-primary-action,.docs-secondary-action { display: inline-flex; align-items: center; justify-content: center; gap: 9px; min-height: 42px; padding: 0 17px; border-radius: 12px; font-size: 13px; font-weight: 700; text-decoration: none; transition: transform .22s ease,box-shadow .22s ease,border-color .22s ease; }
.docs-primary-action { color: white; background: linear-gradient(135deg,#6366f1,#4f46e5 52%,#0891b2); box-shadow: 0 12px 28px rgba(79,70,229,.25); }
.docs-secondary-action { border: 1px solid rgba(99,102,241,.2); color: #4f46e5; background: rgba(255,255,255,.7); }
.dark .docs-secondary-action { color: #c7d2fe; background: rgba(30,41,59,.68); border-color: rgba(129,140,248,.28); }
.docs-primary-action:hover,.docs-secondary-action:hover { transform: translateY(-2px); box-shadow: 0 16px 36px rgba(79,70,229,.2); }
.docs-home-mode { height: 100dvh; overflow: hidden; }
.docs-home-mode .docs-main { height: 100dvh; min-height: 0; overflow: hidden; padding-right: 0; }
.docs-home-mode .docs-article { display: flex; width: min(1000px, calc(100vw - 390px)); height: 100%; min-height: 0; flex-direction: column; padding: 42px 34px 28px; }
.docs-home-mode .docs-hero { flex: 0 0 auto; margin: -14px -40px 20px; padding: 28px 34px 30px; }
.docs-home-mode .docs-hero h1 { margin-top: 11px; margin-bottom: 12px; font-size: clamp(34px, 4.2vw, 52px); }
.docs-home-mode .docs-hero-actions { margin-top: 18px; }
.docs-home-panels { display: flex; min-height: 0; flex: 1 1 auto; flex-direction: column; gap: 14px; margin: 0 -40px; }
.docs-home-panels .docs-resource-navigation { flex: 0 0 auto; margin: 0; padding: 23px 26px 24px; }
.docs-home-panels .docs-resource-heading { margin-bottom: 16px; }
.docs-introduction-panel { position: relative; display: flex; min-height: 0; flex: 1 1 auto; flex-direction: column; overflow: hidden; padding: 23px 28px 25px; border: 1px solid rgba(6,182,212,.16); border-radius: 22px 22px 14px 22px; background: linear-gradient(145deg,rgba(236,254,255,.7),rgba(255,255,255,.84) 56%,rgba(238,242,255,.68)); box-shadow: 0 20px 60px rgba(6,182,212,.07); }
.dark .docs-introduction-panel { border-color: rgba(103,232,249,.2); background: linear-gradient(145deg,rgba(8,47,73,.42),rgba(15,23,42,.8) 56%,rgba(30,27,75,.45)); box-shadow: 0 24px 70px rgba(0,0,0,.2); }
.docs-introduction-panel::before { content: ''; position: absolute; z-index: 0; inset: -35%; pointer-events: none; background: radial-gradient(circle at 10% 70%,rgba(6,182,212,.11),transparent 24%),radial-gradient(circle at 85% 15%,rgba(129,140,248,.12),transparent 23%); animation: docs-introduction-drift 18s ease-in-out infinite alternate; }
.docs-introduction-panel .docs-introduction { position: relative; z-index: 1; min-height: 0; overflow: visible; padding: 0; font-size: 15px; line-height: 1.65; }
.docs-introduction-panel .docs-introduction > :first-child { margin-top: 0; }
.docs-introduction-panel .docs-introduction p { margin: 10px 0; }
.docs-resource-navigation { position: relative; isolation: isolate; overflow: hidden; margin: 0 -12px 46px; padding: 28px 30px 30px; border: 1px solid rgba(99,102,241,.16); border-radius: 22px; background: linear-gradient(135deg,rgba(255,255,255,.82),rgba(238,242,255,.62) 58%,rgba(236,254,255,.56)); box-shadow: 0 20px 60px rgba(64,74,145,.08); }
.dark .docs-resource-navigation { border-color: rgba(129,140,248,.2); background: linear-gradient(135deg,rgba(23,30,54,.9),rgba(15,23,42,.76) 58%,rgba(8,47,73,.38)); box-shadow: 0 24px 70px rgba(0,0,0,.2); }
.docs-resource-navigation::before { content: ''; position: absolute; z-index: -1; inset: -55% -20%; pointer-events: none; background: radial-gradient(circle at 25% 45%,rgba(99,102,241,.16),transparent 24%),radial-gradient(circle at 75% 65%,rgba(6,182,212,.14),transparent 25%); animation: docs-resource-drift 16s ease-in-out infinite alternate; }
.docs-resource-heading { position: relative; display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; margin-bottom: 20px; }
.docs-resource-kicker { display: inline-flex; align-items: center; gap: 8px; color: #6366f1; font-size: 10px; font-weight: 850; letter-spacing: .16em; }
.docs-resource-kicker::before { content: ''; width: 20px; height: 1px; background: currentColor; }
.docs-resource-heading h2 { margin: 8px 0 4px; color: #172033; font-size: 25px; line-height: 1.2; letter-spacing: -.03em; }
.dark .docs-resource-heading h2 { color: #f8fafc; }
.docs-resource-heading p { margin: 0; color: #64748b; font-size: 12px; }.dark .docs-resource-heading p { color: #94a3b8; }
.docs-resource-status { display: inline-flex; align-items: center; gap: 7px; padding: 6px 10px; border: 1px solid rgba(34,197,94,.18); border-radius: 99px; color: #15803d; background: rgba(240,253,244,.7); font-size: 10px; font-weight: 750; white-space: nowrap; }.dark .docs-resource-status { color: #86efac; border-color: rgba(74,222,128,.2); background: rgba(20,83,45,.2); }
.docs-resource-status i { width: 6px; height: 6px; border-radius: 50%; background: #22c55e; box-shadow: 0 0 0 4px rgba(34,197,94,.14); animation: docs-resource-pulse 2.2s ease-in-out infinite; }
.docs-resource-grid { position: relative; display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 12px; }
.docs-resource-card { position: relative; display: flex; flex-direction: column; min-height: 142px; overflow: hidden; border: 1px solid rgba(148,163,184,.2); border-radius: 16px; background: rgba(255,255,255,.72); box-shadow: 0 12px 30px rgba(15,23,42,.05); transition: transform .24s cubic-bezier(.2,.8,.2,1),border-color .24s,box-shadow .24s; }
.dark .docs-resource-card { border-color: rgba(71,85,105,.48); background: rgba(15,23,42,.56); box-shadow: 0 15px 38px rgba(0,0,0,.14); }
.docs-resource-card::after { content: ''; position: absolute; right: -34px; bottom: -48px; width: 130px; height: 130px; border: 18px solid rgba(99,102,241,.06); border-radius: 50%; pointer-events: none; transition: transform .3s ease; }
.docs-resource-card-cyan::after { border-color: rgba(6,182,212,.07); }
.docs-resource-card:hover { transform: translateY(-5px); border-color: rgba(99,102,241,.38); box-shadow: 0 22px 42px rgba(79,70,229,.13); }.dark .docs-resource-card:hover { border-color: rgba(129,140,248,.55); box-shadow: 0 24px 48px rgba(0,0,0,.26); }.docs-resource-card:hover::after { transform: scale(1.12) rotate(12deg); }
.docs-resource-main { position: relative; z-index: 1; display: grid; grid-template-columns: 44px minmax(0,1fr) 20px; align-items: start; gap: 14px; flex: 1; padding: 21px 20px 22px; color: inherit; text-decoration: none; }
.docs-resource-icon { display: grid; place-items: center; width: 44px; height: 44px; border-radius: 14px; color: #4f46e5; background: linear-gradient(135deg,#eef2ff,#e0e7ff); box-shadow: 0 9px 22px rgba(99,102,241,.15); font-size: 22px; font-weight: 800; }.docs-resource-card-cyan .docs-resource-icon { color: #0891b2; background: linear-gradient(135deg,#cffafe,#e0f2fe); box-shadow: 0 9px 22px rgba(6,182,212,.14); }.dark .docs-resource-icon { color: #c7d2fe; background: rgba(79,70,229,.22); }.dark .docs-resource-card-cyan .docs-resource-icon { color: #a5f3fc; background: rgba(6,182,212,.18); }
.docs-resource-copy { display: flex; min-width: 0; flex-direction: column; gap: 5px; }.docs-resource-copy strong { color: #1e293b; font-size: 18px; line-height: 1.25; font-weight: 780; letter-spacing: -.015em; }.dark .docs-resource-copy strong { color: #f8fafc; }.docs-resource-copy small { overflow-wrap: anywhere; color: #6366f1; font-family: ui-monospace,SFMono-Regular,Menlo,monospace; font-size: 11px; line-height: 1.45; }.docs-resource-card-cyan .docs-resource-copy small { color: #0891b2; }.dark .docs-resource-card-cyan .docs-resource-copy small { color: #67e8f9; }.docs-resource-copy > span { color: #64748b; font-size: 14px; line-height: 1.65; }.dark .docs-resource-copy > span { color: #a8b4c7; }
.docs-resource-arrow { color: #a5b4fc; font-size: 18px; line-height: 1; transition: transform .24s ease,color .24s; }.docs-resource-card:hover .docs-resource-arrow { color: #6366f1; transform: translate(3px,-3px); }.docs-resource-card-cyan:hover .docs-resource-arrow { color: #06b6d4; }
.docs-page-outline { position: fixed; top: 118px; right: 28px; display: flex; flex-direction: column; width: 300px; padding-left: 24px; border-left: 1px solid rgba(148,163,184,.22); }
.docs-outline-label { margin-bottom: 12px; color: #94a3b8; font-size: 10px; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
.docs-page-outline button { padding: 5px 0; overflow: hidden; color: #94a3b8; text-align: left; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; transition: .18s; }
.docs-page-outline button:hover,.docs-page-outline button.active { color: var(--docs-accent); transform: translateX(3px); }
.docs-lion-floating { position: fixed; z-index: 45; right: 28px; bottom: 24px; width: 300px; height: clamp(220px,30vh,360px); overflow: hidden; border: 1px solid rgba(99,102,241,.18); border-radius: 18px; background: #ebe5e7; box-shadow: 0 18px 48px rgba(64,74,145,.16); }
.dark .docs-lion-floating { border-color: rgba(129,140,248,.26); box-shadow: 0 20px 54px rgba(0,0,0,.3); }
.docs-lion-floating :deep(.chill-lion-canvas) { width: 100%; height: 100%; }
.docs-article-footer { display: flex; justify-content: space-between; margin-top: 80px; padding-top: 24px; border-top: 1px solid rgba(148,163,184,.2); color: #94a3b8; font-size: 12px; }
.docs-article-footer button:hover { color: var(--docs-accent); }
.docs-loading { width: min(780px, calc(100% - 60px)); margin: 0 auto; padding-top: 140px; }
.docs-skeleton { height: 14px; margin: 18px 0; border-radius: 8px; background: linear-gradient(90deg,rgba(148,163,184,.12),rgba(148,163,184,.26),rgba(148,163,184,.12)); background-size: 200% 100%; animation: docs-shimmer 1.4s infinite; }
.docs-skeleton-kicker { width: 130px; height: 10px; }.docs-skeleton-title { width: 58%; height: 54px; margin: 22px 0 58px; }
.docs-empty-state { display: grid; place-items: center; width: min(520px,90%); margin: 0 auto; padding-top: 22vh; text-align: center; }
.docs-empty-orbit { display: grid; place-items: center; width: 78px; height: 78px; border: 1px solid rgba(99,102,241,.25); border-radius: 50%; animation: docs-float 3s ease-in-out infinite; }
.docs-empty-orbit span { display: grid; place-items: center; width: 48px; height: 48px; border-radius: 16px; color: white; font-size: 22px; background: linear-gradient(135deg,#6366f1,#06b6d4); box-shadow: 0 16px 38px rgba(99,102,241,.28); }
.docs-empty-state h1 { margin: 26px 0 10px; font-size: 28px; font-weight: 800; }.docs-empty-state p { color: #64748b; }.docs-empty-state a { margin-top: 22px; color: var(--docs-accent); font-weight: 650; }
.docs-lightbox { position: fixed; z-index: 120; inset: 0; display: grid; place-items: center; padding: 4vw; background: rgba(3,7,18,.86); backdrop-filter: blur(12px); animation: docs-fade .18s ease; }
.docs-lightbox img { max-width: 94vw; max-height: 90vh; border-radius: 14px; box-shadow: 0 30px 100px rgba(0,0,0,.55); }.docs-lightbox button { position: absolute; top: 24px; right: 30px; color: white; font-size: 34px; }
.docs-mobile-only,.docs-mobile-backdrop { display: none; }

.docs-back-navigation { display: inline-flex; align-items: center; gap: 6px; margin-bottom: 8px; color: #64748b; font-size: 12px; font-weight: 650; text-decoration: none; transition: color .18s ease,transform .18s ease; }.docs-back-navigation:hover { color: var(--docs-accent); transform: translateX(-2px); }.docs-back-navigation svg { width: 15px; fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
.docs-content { font-size: 16px; line-height: 1.86; color: #334155; }
.dark .docs-content { color: #cbd5e1; }
.docs-content :deep(h1),.docs-content :deep(h2),.docs-content :deep(h3),.docs-content :deep(h4) { position: relative; scroll-margin-top: 100px; color: #172033; line-height: 1.28; letter-spacing: -.025em; font-weight: 780; }
.dark .docs-content :deep(h1),.dark .docs-content :deep(h2),.dark .docs-content :deep(h3),.dark .docs-content :deep(h4) { color: #f1f5f9; }
.docs-content :deep(h1) { display: none; }.docs-content :deep(h2) { margin: 64px 0 20px; padding-top: 10px; font-size: 30px; }.docs-content :deep(h3) { margin: 42px 0 14px; font-size: 22px; }.docs-content :deep(h4) { margin: 30px 0 10px; font-size: 17px; color: #475569; }
.docs-content :deep(.docs-heading-anchor) { position: absolute; left: -24px; opacity: 0; color: #818cf8; font-weight: 500; text-decoration: none; transition: .18s; }.docs-content :deep(h2:hover .docs-heading-anchor),.docs-content :deep(h3:hover .docs-heading-anchor),.docs-content :deep(h4:hover .docs-heading-anchor) { opacity: 1; }
.docs-content :deep(p) { margin: 14px 0; }.docs-content :deep(a) { color: #4f46e5; font-weight: 580; text-decoration: underline; text-decoration-color: rgba(99,102,241,.3); text-underline-offset: 4px; }.dark .docs-content :deep(a) { color: #a5b4fc; }
.docs-content :deep(strong),.docs-content :deep(b) { color: #1e293b; font-weight: 720; }.dark .docs-content :deep(strong),.dark .docs-content :deep(b) { color: #f8fafc; }
.docs-content :deep(ul),.docs-content :deep(ol) { margin: 16px 0; padding-left: 1.45rem; list-style-position: outside; }.docs-content :deep(ul) { list-style-type: disc; }.docs-content :deep(ol) { list-style-type: decimal; }.docs-content :deep(li) { margin: 7px 0; padding-left: 4px; }.docs-content :deep(li::marker) { color: #818cf8; font-weight: 700; }
.docs-content :deep(blockquote) { margin: 22px 0; padding: 3px 0 3px 20px; border-left: 3px solid #a5b4fc; color: #64748b; }
.docs-content :deep(.docs-callout) { position: relative; overflow: hidden; margin: 26px 0; padding: 22px 24px 20px; border: 1px solid rgba(99,102,241,.2); border-left: 3px solid #6366f1; border-radius: 14px; color: #475569; background: linear-gradient(135deg,rgba(99,102,241,.08),rgba(6,182,212,.045)); box-shadow: 0 12px 35px rgba(99,102,241,.06); }
.dark .docs-content :deep(.docs-callout) { color: #cbd5e1; background: linear-gradient(135deg,rgba(99,102,241,.15),rgba(6,182,212,.06)); }
.docs-content :deep(.docs-callout-label) { display: inline-flex; margin-bottom: 5px; padding: 3px 8px; border-radius: 99px; color: #4f46e5; background: rgba(99,102,241,.12); font-size: 9px; font-weight: 850; letter-spacing: .12em; }
.docs-content :deep(img) { display: block; max-width: 100%; height: auto; margin: 28px auto; border: 1px solid rgba(148,163,184,.2); border-radius: 14px; background: white; box-shadow: 0 18px 50px rgba(15,23,42,.12); cursor: zoom-in; transition: transform .25s ease,box-shadow .25s ease; }.docs-content :deep(img:hover) { transform: translateY(-3px); box-shadow: 0 24px 65px rgba(15,23,42,.18); }
.docs-content :deep(code) { padding: 2px 6px; border: 1px solid rgba(148,163,184,.2); border-radius: 6px; color: #be185d; background: rgba(241,245,249,.8); font-size: .88em; }.dark .docs-content :deep(code) { color: #f9a8d4; background: #182135; border-color: #334155; }
.docs-content :deep(pre) { position: relative; overflow: auto; margin: 24px 0; padding: 22px; border: 1px solid #273449; border-radius: 14px; background: #101827; box-shadow: 0 16px 36px rgba(15,23,42,.18); }.docs-content :deep(pre code) { padding: 0; border: 0; color: #dbeafe; background: transparent; }
.docs-content :deep(.docs-copy-button) { position: absolute; top: 9px; right: 9px; padding: 5px 9px; border: 1px solid #334155; border-radius: 7px; color: #94a3b8; background: #172033; font-size: 10px; }.docs-content :deep(.docs-copy-button:hover) { color: white; border-color: #64748b; }
.docs-content :deep(hr) { margin: 48px 0; border: 0; height: 1px; background: linear-gradient(90deg,transparent,#cbd5e1,transparent); }
.docs-content :deep(.notion-document) { min-width: 0; }
.docs-content :deep(.docs-page-title) { display: none; }
.docs-content :deep(.page-cover-image),.docs-content :deep(.page-header-icon) { display: none; }
.docs-content :deep(.icon[data-emoji]:empty::before) { content: attr(data-emoji); }
.docs-content :deep(.docs-toggle) { position: relative; scroll-margin-top: 92px; }
.docs-content :deep(.docs-toggle > .docs-toggle-summary) { position: relative; display: flex; align-items: center; justify-content: flex-start; gap: 10px; list-style: none; cursor: pointer; transition: color .2s ease,background .2s ease,transform .2s ease; }
.docs-content :deep(.docs-toggle > .docs-toggle-summary::-webkit-details-marker) { display: none; }
.docs-content :deep(.docs-toggle > .docs-toggle-summary::after) { content: ''; order: -1; width: 0; height: 0; flex: 0 0 auto; border-top: 5px solid transparent; border-bottom: 5px solid transparent; border-left: 7px solid #94a3b8; transform-origin: 3px 5px; transition: transform .2s cubic-bezier(.2,.8,.2,1),border-left-color .2s; }
.docs-content :deep(.docs-toggle[open] > .docs-toggle-summary::after) { border-left-color: #6366f1; transform: rotate(90deg); }
.docs-content :deep(.docs-toggle > :not(summary)) { animation: docs-content-enter .32s cubic-bezier(.2,.8,.2,1); }
.docs-content :deep(.docs-toggle-level-2) { margin: 54px 0 24px; padding-top: 9px; border-top: 1px solid rgba(148,163,184,.18); }
.docs-content :deep(.docs-toggle-level-2 > .docs-toggle-summary) { margin: 0 0 18px; padding: 18px 2px 7px; color: #172033; font-size: 29px; line-height: 1.3; font-weight: 800; letter-spacing: -.028em; }
.dark .docs-content :deep(.docs-toggle-level-2 > .docs-toggle-summary) { color: #f1f5f9; }
.docs-content :deep(.docs-toggle-level-2 > .docs-toggle-summary::before) { content: ''; position: absolute; top: -10px; left: 0; width: 62px; height: 2px; border-radius: 99px; background: linear-gradient(90deg,#6366f1,#06b6d4); box-shadow: 0 0 14px rgba(99,102,241,.3); }
.docs-content :deep(.docs-toggle-level-3) { margin: 30px 0 18px; }
.docs-content :deep(.docs-toggle-level-3 > .docs-toggle-summary) { padding: 10px 13px; border-radius: 12px; color: #334155; background: linear-gradient(90deg,rgba(99,102,241,.075),transparent 75%); font-size: 20px; line-height: 1.4; font-weight: 740; letter-spacing: -.018em; }
.dark .docs-content :deep(.docs-toggle-level-3 > .docs-toggle-summary) { color: #e2e8f0; background: linear-gradient(90deg,rgba(99,102,241,.15),transparent 75%); }
.docs-content :deep(.docs-toggle-level-4) { margin: 15px 0; padding-left: 14px; border-left: 1px solid rgba(129,140,248,.18); }
.docs-content :deep(.docs-toggle-level-4 > .docs-toggle-summary) { min-height: 38px; padding: 7px 11px; border-radius: 10px; color: #475569; font-size: 15px; line-height: 1.5; font-weight: 680; }
.dark .docs-content :deep(.docs-toggle-level-4 > .docs-toggle-summary) { color: #cbd5e1; }
.docs-content :deep(.docs-toggle-level-4 > .docs-toggle-summary:hover) { color: #4f46e5; background: rgba(99,102,241,.065); transform: translateX(2px); }.dark .docs-content :deep(.docs-toggle-level-4 > .docs-toggle-summary:hover) { color: #c7d2fe; }
.docs-content :deep(figure.image) { margin: 24px 0 32px; text-align: center; }
.docs-content :deep(figure.image > a) { display: inline-block; max-width: 100%; }
.docs-content :deep(figure.image img) { width: min(100%,var(--docs-image-width,100%)); margin: 0 auto; }
.docs-content :deep(figcaption) { margin-top: 9px; color: #94a3b8; font-size: 12px; text-align: center; }
.docs-content :deep(.docs-callout-notion) { display: grid !important; grid-template-columns: auto minmax(0,1fr); align-items: start; gap: 13px; }
.docs-content :deep(.docs-callout-icon) { display: grid; place-items: center; width: 31px; height: 31px; border-radius: 10px; background: rgba(255,255,255,.72); box-shadow: 0 7px 20px rgba(99,102,241,.1); font-size: 16px; }.dark .docs-content :deep(.docs-callout-icon) { background: rgba(15,23,42,.58); }
.docs-content :deep(.docs-callout-notion p:first-child) { margin-top: 1px; }
.docs-content :deep(.docs-link-card) { display: inline-flex; align-items: center; gap: 8px; max-width: 100%; padding: 10px 14px; border: 1px solid rgba(99,102,241,.2); border-radius: 11px; text-decoration: none; background: linear-gradient(135deg,rgba(99,102,241,.08),rgba(6,182,212,.045)); box-shadow: 0 9px 24px rgba(99,102,241,.06); transition: .2s ease; }
.docs-content :deep(.docs-link-card::after) { content: '↗'; font-size: 12px; }.docs-content :deep(.docs-link-card:hover) { border-color: rgba(99,102,241,.38); transform: translateY(-2px); box-shadow: 0 14px 32px rgba(99,102,241,.12); }
.docs-content :deep(.docs-anchor-link) { text-decoration-style: dashed; }
.docs-content :deep(.docs-table-scroll),.docs-content :deep(.collection-content-wrapper) { max-width: 100%; overflow-x: auto; margin: 24px 0; border: 1px solid rgba(148,163,184,.2); border-radius: 13px; }
.docs-content :deep(table) { width: 100%; min-width: 520px; border-collapse: collapse; font-size: 13px; }.docs-content :deep(th),.docs-content :deep(td) { padding: 10px 12px; border: 1px solid rgba(148,163,184,.18); text-align: left; }.docs-content :deep(th) { color: #64748b; background: rgba(241,245,249,.72); font-weight: 700; }.dark .docs-content :deep(th) { background: rgba(30,41,59,.72); }
.docs-content :deep(.column-list) { display: flex; gap: 28px; }.docs-content :deep(.column) { min-width: 0; flex: 1; }
.docs-content :deep(.bookmark) { display: flex; overflow: hidden; margin: 20px 0; border: 1px solid rgba(148,163,184,.2); border-radius: 13px; text-decoration: none; }.docs-content :deep(.bookmark-info) { flex: 1; padding: 14px; }.docs-content :deep(.bookmark-image) { width: 34%; object-fit: cover; }
.docs-content :deep(mark) { color: inherit; background-color: transparent; }
.docs-content :deep(.highlight-default),.docs-content :deep(.highlight-default_background) { color: inherit; background-color: transparent; }
.docs-content :deep(.highlight-gray) { color: #78716c; }.docs-content :deep(.highlight-blue) { color: #2563eb; }.docs-content :deep(.highlight-purple) { color: #7c3aed; }.docs-content :deep(.highlight-pink) { color: #db2777; }.docs-content :deep(.highlight-red) { color: #dc2626; }.docs-content :deep(.highlight-orange) { color: #ea580c; }.docs-content :deep(.highlight-yellow) { color: #ca8a04; }.docs-content :deep(.highlight-teal) { color: #0f766e; }
.dark .docs-content :deep(.highlight-gray) { color: #c4bdb7; }.dark .docs-content :deep(.highlight-blue) { color: #93c5fd; }.dark .docs-content :deep(.highlight-purple) { color: #c4b5fd; }.dark .docs-content :deep(.highlight-pink) { color: #f9a8d4; }.dark .docs-content :deep(.highlight-red) { color: #fca5a5; }.dark .docs-content :deep(.highlight-orange) { color: #fdba74; }.dark .docs-content :deep(.highlight-yellow) { color: #fde68a; }.dark .docs-content :deep(.highlight-teal) { color: #5eead4; }
.docs-content :deep(.block-color-gray_background),.docs-content :deep(.highlight-gray_background) { background-color: rgba(120,113,108,.08); }.docs-content :deep(.block-color-blue_background),.docs-content :deep(.highlight-blue_background) { background-color: rgba(59,130,246,.09); }.docs-content :deep(.block-color-purple_background),.docs-content :deep(.highlight-purple_background) { background-color: rgba(139,92,246,.09); }.docs-content :deep(.block-color-yellow_background),.docs-content :deep(.highlight-yellow_background) { background-color: rgba(234,179,8,.1); }.docs-content :deep(.block-color-red_background),.docs-content :deep(.highlight-red_background) { background-color: rgba(239,68,68,.08); }
.docs-content :deep(.docs-reveal) { opacity: 0; transform: translateY(14px); transition: opacity .5s ease,transform .5s cubic-bezier(.2,.8,.2,1); }.docs-content :deep(.docs-reveal.docs-visible) { opacity: 1; transform: none; }
@keyframes docs-shimmer { to { background-position: -200% 0; } } @keyframes docs-float { 50% { transform: translateY(-8px) rotate(3deg); } } @keyframes docs-fade { from { opacity: 0; } } @keyframes docs-orbit { 50% { transform: translate3d(-16px,12px,0) scale(1.08); } } @keyframes docs-content-enter { from { opacity: 0; transform: translateY(-5px); } } @keyframes docs-resource-drift { 50% { transform: translate3d(3%,2%,0) scale(1.08); } } @keyframes docs-resource-pulse { 50% { opacity: .45; transform: scale(.72); } } @keyframes docs-introduction-drift { 50% { transform: translate3d(4%,3%,0) scale(1.08); } }

@media (max-width: 1180px) { .docs-main { padding-right: 0; }.docs-page-outline,.docs-lion-floating { display: none; }.docs-article { width: min(860px, calc(100vw - 320px)); } }
@media (max-width: 760px) {
  .docs-mobile-only { display: grid; }.docs-header { height: 60px; }.docs-header-inner { padding: 0 14px; gap: 8px; }.docs-brand { gap: 7px; font-size: 14px; }.docs-logo { width: 31px; height: 31px; }.docs-brand-divider,.docs-brand-section,.docs-search-wrap,.docs-header-link { display: none; }
  .docs-sidebar { top: 0; z-index: 90; width: min(86vw,320px); transform: translateX(-105%); background: #fbfcff; box-shadow: 25px 0 70px rgba(15,23,42,.18); transition: transform .28s cubic-bezier(.2,.8,.2,1); }.dark .docs-sidebar { background: #0b1020; }.docs-sidebar-open { transform: none; }.docs-mobile-backdrop { display: block; position: fixed; z-index: 80; inset: 0; background: rgba(15,23,42,.42); backdrop-filter: blur(3px); }
  .docs-mobile-search { padding: 0 16px 12px; }.docs-mobile-search input { width: 100%; height: 40px; padding: 0 12px; border: 1px solid #dbe2eb; border-radius: 10px; background: transparent; }.dark .docs-mobile-search input { border-color: #334155; }
  .docs-main { margin-left: 0; padding: 60px 0 0; }.docs-article { width: 100%; padding: 36px 16px 72px; }.docs-hero { margin: 0 0 24px; padding: 28px 22px 30px; border-radius: 20px; }.docs-hero h1 { font-size: 38px; }.docs-meta span:not(:first-child)::before { margin-right: 9px; }.docs-meta { gap: 8px 9px; }.docs-hero-actions { align-items: stretch; }.docs-primary-action,.docs-secondary-action { flex: 1; padding: 0 12px; white-space: nowrap; }
  .docs-resource-navigation { margin: 0 0 34px; padding: 22px 17px 18px; border-radius: 17px; }.docs-resource-heading { align-items: flex-start; flex-direction: column; gap: 12px; }.docs-resource-heading h2 { font-size: 23px; }.docs-resource-status { align-self: flex-start; }.docs-resource-grid { grid-template-columns: 1fr; }.docs-resource-main { padding: 18px 15px 21px; }
  .docs-content { font-size: 15px; line-height: 1.78; }.docs-content :deep(h2) { margin-top: 48px; font-size: 26px; }.docs-content :deep(h3) { margin-top: 34px; font-size: 20px; }.docs-content :deep(.docs-heading-anchor) { display: none; }.docs-content :deep(.docs-callout) { padding: 18px; }.docs-content :deep(img) { margin: 22px auto; border-radius: 10px; }
  .docs-content :deep(.page-cover-image) { height: 190px; margin-bottom: 32px; border-radius: 15px; }.docs-content :deep(.docs-toggle-level-2 > .docs-toggle-summary) { font-size: 24px; }.docs-content :deep(.docs-toggle-level-3 > .docs-toggle-summary) { font-size: 18px; }.docs-content :deep(.docs-toggle-level-4) { padding-left: 9px; }.docs-content :deep(.column-list) { flex-direction: column; gap: 12px; }
}
.docs-home-mode .docs-article { width: min(1000px, calc(100vw - 390px)); }
@media (max-width: 760px) {
  .docs-home-mode { height: 100dvh; }
  .docs-home-mode .docs-main { height: 100dvh; overflow: hidden; }
  .docs-home-mode .docs-article { width: 100%; height: 100%; padding: 24px 16px 20px; }
  .docs-home-mode .docs-hero { margin: 0 0 16px; padding: 24px 21px 25px; border-radius: 19px; }
  .docs-home-mode .docs-hero h1 { font-size: clamp(32px, 10vw, 42px); }
  .docs-home-mode .docs-hero-actions { margin-top: 16px; }
  .docs-home-panels { margin: 0; gap: 12px; }
  .docs-home-panels .docs-resource-navigation { padding: 18px 17px 16px; border-radius: 17px; }
  .docs-home-panels .docs-resource-heading { align-items: flex-start; flex-direction: column; gap: 10px; margin-bottom: 13px; }
  .docs-home-panels .docs-resource-heading h2 { font-size: 23px; }
  .docs-home-panels .docs-resource-status { align-self: flex-start; }
  .docs-home-panels .docs-resource-grid { grid-template-columns: 1fr; }
  .docs-home-panels .docs-resource-main { padding: 18px 15px 21px; }
  .docs-introduction-panel { padding: 19px 17px 20px; border-radius: 17px; }
  .docs-introduction-panel .docs-introduction { padding: 0; font-size: 14px; line-height: 1.55; }
  .docs-introduction-panel .docs-introduction p { margin: 7px 0; }
}
@media (prefers-reduced-motion: reduce) { .docs-shell * { scroll-behavior: auto !important; animation: none !important; transition-duration: .01ms !important; }.docs-content :deep(.docs-reveal) { opacity: 1; transform: none; } }
</style>
