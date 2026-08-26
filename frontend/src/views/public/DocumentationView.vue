<template>
  <div class="docs-shell">
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
          @click="scrollToHeading(item.id)"
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
          <div class="docs-hero">
            <div class="docs-eyebrow"><span></span> KNOWLEDGE BASE</div>
            <h1>{{ manifest.title }}</h1>
            <div class="docs-meta">
              <span>{{ t('documentation.public.sections', { count: manifest.outline.length }) }}</span>
              <span>{{ t('documentation.public.images', { count: manifest.assets.length }) }}</span>
              <span>{{ formatDate(manifest.published_at || manifest.created_at) }}</span>
            </div>
          </div>
          <div ref="contentRef" class="docs-content" v-html="renderedHTML" @click="handleContentClick"></div>
          <footer class="docs-article-footer">
            <span>{{ t('documentation.public.endOfDocument') }}</span>
            <button @click="scrollToTop">{{ t('documentation.public.backToTop') }}</button>
          </footer>
        </article>

        <aside class="docs-page-outline">
          <div class="docs-outline-label">{{ t('documentation.public.onThisPage') }}</div>
          <button
            v-for="item in pageHeadings"
            :key="item.id"
            :class="{ active: activeHeading === item.id }"
            @click="scrollToHeading(item.id)"
          >{{ item.title }}</button>
        </aside>
      </template>
    </main>

    <div v-if="lightboxSource" class="docs-lightbox" @click="lightboxSource = ''">
      <button :aria-label="t('documentation.public.close')">×</button>
      <img :src="lightboxSource" alt="" @click.stop />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import {
  documentationAssetBase,
  getActiveDocumentation,
  getDocumentationContent,
  type DocumentationManifest
} from '@/api/documentation'
import { renderDocumentationMarkdown } from '@/utils/documentationMarkdown'

const { t, locale } = useI18n()
const appStore = useAppStore()
const manifest = ref<DocumentationManifest | null>(null)
const renderedHTML = ref('')
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

const filteredHeadings = computed(() => {
  const query = searchQuery.value.trim().toLocaleLowerCase(locale.value)
  if (!manifest.value) return []
  if (!query) return manifest.value.outline
  return manifest.value.outline.filter((item) => item.title.toLocaleLowerCase(locale.value).includes(query))
})

const sidebarHeadings = computed(() => filteredHeadings.value.filter((item) => item.level >= 2))
const pageHeadings = computed(() => (manifest.value?.outline || []).filter((item) => item.level === 2).slice(0, 12))

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'short', day: 'numeric' }).format(new Date(value))
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function scrollToHeading(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  activeHeading.value = id
  mobileOpen.value = false
  history.replaceState(null, '', `#${encodeURIComponent(id)}`)
}

function selectSearchResult(id: string) {
  searchFocused.value = false
  searchQuery.value = ''
  scrollToHeading(id)
}

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handleContentClick(event: MouseEvent) {
  const target = event.target as HTMLElement
  const image = target.closest<HTMLImageElement>('img.docs-zoomable-image')
  if (image) {
    lightboxSource.value = image.currentSrc || image.src
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
    const headings = manifest.value?.outline || []
    let current = headings[0]?.id || ''
    for (const item of headings) {
      const element = document.getElementById(item.id)
      if (element && element.getBoundingClientRect().top <= 130) current = item.id
    }
    activeHeading.value = current
  })
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
  contentRef.value?.querySelectorAll(':scope > *').forEach((element) => {
    element.classList.add('docs-reveal')
    revealObserver?.observe(element)
  })
}

async function loadDocumentation() {
  loading.value = true
  try {
    const active = await getActiveDocumentation()
    const markdown = await getDocumentationContent(active.id)
    manifest.value = active
    renderedHTML.value = renderDocumentationMarkdown(markdown, active.outline, documentationAssetBase(active.id), t('documentation.public.copy'))
    await nextTick()
    installRevealAnimations()
    if (window.location.hash) {
      const id = decodeURIComponent(window.location.hash.slice(1))
      window.setTimeout(() => scrollToHeading(id), 80)
    }
  } catch (error) {
    const detail = error as { status?: number; message?: string }
    errorStatus.value = detail.status || 0
    errorMessage.value = detail.message || t('documentation.public.loadFailed')
  } finally {
    loading.value = false
  }
}

function handleSearchShortcut(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    const input = document.querySelector<HTMLInputElement>('.docs-search-wrap input')
    input?.focus()
  }
  if (event.key === 'Escape' && lightboxSource.value) lightboxSource.value = ''
}

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
.docs-shell { --docs-accent: #6366f1; --docs-cyan: #06b6d4; min-height: 100vh; color: #172033; background: radial-gradient(circle at 78% -10%, rgba(99,102,241,.12), transparent 30rem), #fbfcff; }
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
.docs-main { min-height: 100vh; margin-left: 276px; padding: 68px 0 0; }
.docs-article { width: min(860px, calc(100vw - 600px)); margin: 0 auto; padding: 72px 34px 100px; }
.docs-hero { position: relative; padding-bottom: 42px; margin-bottom: 38px; border-bottom: 1px solid rgba(148,163,184,.2); }
.docs-hero::after { content: ''; position: absolute; bottom: -1px; left: 0; width: 88px; height: 2px; background: linear-gradient(90deg,var(--docs-accent),var(--docs-cyan)); }
.docs-eyebrow { display: flex; align-items: center; gap: 9px; color: #6366f1; font-size: 11px; font-weight: 800; letter-spacing: .16em; }
.docs-eyebrow span { width: 22px; height: 1px; background: currentColor; }
.docs-hero h1 { margin: 15px 0 16px; font-size: clamp(38px,5vw,62px); line-height: 1.08; letter-spacing: -.045em; font-weight: 850; color: #111827; }
.dark .docs-hero h1 { color: #f8fafc; }
.docs-meta { display: flex; flex-wrap: wrap; gap: 9px 18px; color: #94a3b8; font-size: 12px; }
.docs-meta span:not(:first-child)::before { content: '•'; margin-right: 18px; color: #cbd5e1; }
.docs-page-outline { position: fixed; top: 118px; right: 28px; display: flex; flex-direction: column; width: 190px; padding-left: 18px; border-left: 1px solid rgba(148,163,184,.22); }
.docs-outline-label { margin-bottom: 12px; color: #94a3b8; font-size: 10px; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
.docs-page-outline button { padding: 5px 0; overflow: hidden; color: #94a3b8; text-align: left; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; transition: .18s; }
.docs-page-outline button:hover,.docs-page-outline button.active { color: var(--docs-accent); transform: translateX(3px); }
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

.docs-content { font-size: 16px; line-height: 1.86; color: #334155; }
.dark .docs-content { color: #cbd5e1; }
.docs-content :deep(h1),.docs-content :deep(h2),.docs-content :deep(h3),.docs-content :deep(h4) { position: relative; scroll-margin-top: 100px; color: #172033; line-height: 1.28; letter-spacing: -.025em; font-weight: 780; }
.dark .docs-content :deep(h1),.dark .docs-content :deep(h2),.dark .docs-content :deep(h3),.dark .docs-content :deep(h4) { color: #f1f5f9; }
.docs-content :deep(h1) { display: none; }.docs-content :deep(h2) { margin: 64px 0 20px; padding-top: 10px; font-size: 30px; }.docs-content :deep(h3) { margin: 42px 0 14px; font-size: 22px; }.docs-content :deep(h4) { margin: 30px 0 10px; font-size: 17px; color: #475569; }
.docs-content :deep(.docs-heading-anchor) { position: absolute; left: -24px; opacity: 0; color: #818cf8; font-weight: 500; text-decoration: none; transition: .18s; }.docs-content :deep(h2:hover .docs-heading-anchor),.docs-content :deep(h3:hover .docs-heading-anchor),.docs-content :deep(h4:hover .docs-heading-anchor) { opacity: 1; }
.docs-content :deep(p) { margin: 14px 0; }.docs-content :deep(a) { color: #4f46e5; font-weight: 580; text-decoration: underline; text-decoration-color: rgba(99,102,241,.3); text-underline-offset: 4px; }.dark .docs-content :deep(a) { color: #a5b4fc; }
.docs-content :deep(strong) { color: #1e293b; font-weight: 720; }.dark .docs-content :deep(strong) { color: #f1f5f9; }
.docs-content :deep(ul),.docs-content :deep(ol) { margin: 16px 0; padding-left: 1.45rem; }.docs-content :deep(li) { margin: 7px 0; padding-left: 4px; }.docs-content :deep(li::marker) { color: #818cf8; font-weight: 700; }
.docs-content :deep(blockquote) { margin: 22px 0; padding: 3px 0 3px 20px; border-left: 3px solid #a5b4fc; color: #64748b; }
.docs-content :deep(.docs-callout) { position: relative; overflow: hidden; margin: 26px 0; padding: 22px 24px 20px; border: 1px solid rgba(99,102,241,.2); border-left: 3px solid #6366f1; border-radius: 14px; color: #475569; background: linear-gradient(135deg,rgba(99,102,241,.08),rgba(6,182,212,.045)); box-shadow: 0 12px 35px rgba(99,102,241,.06); }
.dark .docs-content :deep(.docs-callout) { color: #cbd5e1; background: linear-gradient(135deg,rgba(99,102,241,.15),rgba(6,182,212,.06)); }
.docs-content :deep(.docs-callout-label) { display: inline-flex; margin-bottom: 5px; padding: 3px 8px; border-radius: 99px; color: #4f46e5; background: rgba(99,102,241,.12); font-size: 9px; font-weight: 850; letter-spacing: .12em; }
.docs-content :deep(img) { display: block; max-width: 100%; height: auto; margin: 28px auto; border: 1px solid rgba(148,163,184,.2); border-radius: 14px; background: white; box-shadow: 0 18px 50px rgba(15,23,42,.12); cursor: zoom-in; transition: transform .25s ease,box-shadow .25s ease; }.docs-content :deep(img:hover) { transform: translateY(-3px); box-shadow: 0 24px 65px rgba(15,23,42,.18); }
.docs-content :deep(code) { padding: 2px 6px; border: 1px solid rgba(148,163,184,.2); border-radius: 6px; color: #be185d; background: rgba(241,245,249,.8); font-size: .88em; }.dark .docs-content :deep(code) { color: #f9a8d4; background: #182135; border-color: #334155; }
.docs-content :deep(pre) { position: relative; overflow: auto; margin: 24px 0; padding: 22px; border: 1px solid #273449; border-radius: 14px; background: #101827; box-shadow: 0 16px 36px rgba(15,23,42,.18); }.docs-content :deep(pre code) { padding: 0; border: 0; color: #dbeafe; background: transparent; }
.docs-content :deep(.docs-copy-button) { position: absolute; top: 9px; right: 9px; padding: 5px 9px; border: 1px solid #334155; border-radius: 7px; color: #94a3b8; background: #172033; font-size: 10px; }.docs-content :deep(.docs-copy-button:hover) { color: white; border-color: #64748b; }
.docs-content :deep(hr) { margin: 48px 0; border: 0; height: 1px; background: linear-gradient(90deg,transparent,#cbd5e1,transparent); }
.docs-content :deep(.docs-reveal) { opacity: 0; transform: translateY(14px); transition: opacity .5s ease,transform .5s cubic-bezier(.2,.8,.2,1); }.docs-content :deep(.docs-reveal.docs-visible) { opacity: 1; transform: none; }
@keyframes docs-shimmer { to { background-position: -200% 0; } } @keyframes docs-float { 50% { transform: translateY(-8px) rotate(3deg); } } @keyframes docs-fade { from { opacity: 0; } }

@media (max-width: 1180px) { .docs-page-outline { display: none; }.docs-article { width: min(860px, calc(100vw - 320px)); } }
@media (max-width: 760px) {
  .docs-mobile-only { display: grid; }.docs-header { height: 60px; }.docs-header-inner { padding: 0 14px; gap: 8px; }.docs-brand { gap: 7px; font-size: 14px; }.docs-logo { width: 31px; height: 31px; }.docs-brand-divider,.docs-brand-section,.docs-search-wrap,.docs-header-link { display: none; }
  .docs-sidebar { top: 0; z-index: 90; width: min(86vw,320px); transform: translateX(-105%); background: #fbfcff; box-shadow: 25px 0 70px rgba(15,23,42,.18); transition: transform .28s cubic-bezier(.2,.8,.2,1); }.dark .docs-sidebar { background: #0b1020; }.docs-sidebar-open { transform: none; }.docs-mobile-backdrop { display: block; position: fixed; z-index: 80; inset: 0; background: rgba(15,23,42,.42); backdrop-filter: blur(3px); }
  .docs-mobile-search { padding: 0 16px 12px; }.docs-mobile-search input { width: 100%; height: 40px; padding: 0 12px; border: 1px solid #dbe2eb; border-radius: 10px; background: transparent; }.dark .docs-mobile-search input { border-color: #334155; }
  .docs-main { margin-left: 0; padding-top: 60px; }.docs-article { width: 100%; padding: 48px 22px 72px; }.docs-hero { padding-bottom: 30px; margin-bottom: 24px; }.docs-hero h1 { font-size: 38px; }.docs-meta span:not(:first-child)::before { margin-right: 9px; }.docs-meta { gap: 8px 9px; }
  .docs-content { font-size: 15px; line-height: 1.78; }.docs-content :deep(h2) { margin-top: 48px; font-size: 26px; }.docs-content :deep(h3) { margin-top: 34px; font-size: 20px; }.docs-content :deep(.docs-heading-anchor) { display: none; }.docs-content :deep(.docs-callout) { padding: 18px; }.docs-content :deep(img) { margin: 22px auto; border-radius: 10px; }
}
@media (prefers-reduced-motion: reduce) { .docs-shell * { scroll-behavior: auto !important; animation: none !important; transition-duration: .01ms !important; }.docs-content :deep(.docs-reveal) { opacity: 1; transform: none; } }
</style>
