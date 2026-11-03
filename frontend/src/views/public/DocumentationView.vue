<template>
  <div
    class="tutorial-docs"
    :class="[
      currentSection ? 'tutorial-reader' : 'tutorial-home',
      { 'menu-open': mobileOpen, 'focus-mode': focusMode && currentSection }
    ]"
  >
    <div v-if="currentSection" class="reading-progress" aria-hidden="true">
      <i :style="{ width: `${scrollProgress * 100}%` }"></i>
    </div>

    <header class="site-header">
      <button
        class="icon-button mobile-menu menu"
        aria-label="打开章节目录"
        @click="mobileOpen = !mobileOpen"
      >
        <span v-html="icons.menu"></span>
      </button>
      <router-link to="/docs" class="brand"
        ><img :src="siteLogo" alt="" />{{ appStore.siteName || '爱白嫖公益站' }}
        <small>Docs</small></router-link
      >
      <button class="search-trigger" aria-label="搜索文档标题" @click="openSearch">
        <span v-html="icons.search"></span><span>搜索你想了解的内容…</span><kbd>Ctrl K</kbd>
      </button>
      <nav class="header-actions">
        <a
          class="qq-community"
          href="https://qm.qq.com/q/GKy4AywLaQ"
          target="_blank"
          rel="noopener noreferrer"
          aria-label="加入QQ交流群"
          ><span class="qq-full">加入QQ交流群</span
          ><span class="qq-short" aria-hidden="true">QQ群</span></a
        >
        <router-link to="/home">返回首页</router-link>
        <button
          class="icon-button theme"
          :aria-label="isDark ? '切换亮色模式' : '切换暗色模式'"
          :aria-pressed="isDark"
          @click="toggleTheme"
        >
          <span v-html="icons[isDark ? 'sun' : 'moon']"></span>
        </button>
      </nav>
    </header>
    <button
      v-if="mobileOpen"
      class="scrim"
      aria-label="关闭章节目录"
      @click="mobileOpen = false"
    ></button>
    <template v-if="currentSection"
      ><aside class="chapter-sidebar" aria-label="章节目录">
        <router-link class="overview" to="/docs"
          ><span v-html="icons['arrow-left']"></span>教程概览</router-link
        >
        <div class="sidebar-label">
          文档目录 <span id="chapter-count">{{ topLevelHeadings.length }} 个主题</span>
        </div>
        <nav id="chapters">
          <router-link
            v-for="(h, index) in topLevelHeadings"
            :key="h.id"
            :to="sectionRoute(h.id)"
            class="chapter-link"
            :class="{ active: currentSection?.id === h.id }"
            :aria-current="currentSection?.id === h.id ? 'page' : undefined"
            @click="mobileOpen = false"
            ><span class="index">{{ String(index + 1).padStart(2, '0') }}</span
            ><span>{{ h.title }}</span></router-link
          >
        </nav>
        <div class="sidebar-bottom">
          <span class="status-dot"></span
          ><span class="updated-date">{{
            manifest ? formatDate(manifest.published_at || manifest.created_at) : ''
          }}</span>
        </div>
      </aside>
      <main class="reader-layout">
        <div class="article-column">
          <nav class="breadcrumb" aria-label="当前位置">
            <router-link to="/docs">教程概览</router-link
            ><span v-html="icons['chevron-right']"></span
            ><span class="current-title">{{ currentSection?.title }}</span>
          </nav>
          <section class="article-header">
            <div class="article-kicker">
              <span class="chapter-index">{{ String(sectionIndex + 1).padStart(2, '0') }}</span
              ><span>使用指南</span>
            </div>
            <h1 class="current-title">{{ currentSection?.title }}</h1>
            <div class="article-meta">
              <span
                ><i v-html="icons['clock-3']"></i
                ><span id="reading-time">约 {{ readingTime }} 分钟</span></span
              ><span
                ><i v-html="icons['image']"></i
                ><span id="image-count">{{ imageCount }} 张图片</span></span
              ><span class="updated-date">{{
                manifest ? formatDate(manifest.published_at || manifest.created_at) : ''
              }}</span>
            </div>
            <div class="article-tools">
              <button id="fold-all" @click="foldAll">
                <i v-html="icons['chevrons-up-down']"></i
                ><span>{{ folded ? '展开全部' : '收起全部' }}</span></button
              ><button id="focus-mode" :aria-pressed="focusMode" @click="focusMode = !focusMode">
                <i v-html="icons['maximize-2']"></i
                ><span>{{ focusMode ? '退出专注' : '专注阅读' }}</span></button
              ><button id="copy-page" @click="copyPage">
                <i v-html="icons['link']"></i><span>复制链接</span>
              </button>
            </div>
          </section>
          <details ref="mobileToc" class="mobile-toc">
            <summary>
              <span v-html="icons['list']"></span>本页目录<span
                v-html="icons['chevron-down']"
              ></span>
            </summary>
            <nav>
              <button
                v-for="h in pageHeadings"
                :key="h.id"
                class="toc-link"
                :class="['level-' + h.level, { active: activeHeading === h.id }]"
                :aria-current="activeHeading === h.id ? 'location' : undefined"
                @click="navigateToHeading(h.id)"
              >
                {{ h.title }}
              </button>
              <p v-if="!pageHeadings.length" class="search-empty">本章暂无小节</p>
            </nav>
          </details>
          <article
            id="article"
            ref="contentRef"
            class="docs-content"
            aria-label="教程正文"
            v-html="renderedHTML"
            @click="handleContentClick"
            @toggle.capture="updateScrollState"
            @load.capture="updateScrollState"
          ></article>
          <nav class="chapter-pagination" aria-label="前后章节">
            <router-link class="pagination-link prev" :to="sectionRoute(previousChapter?.id)"
              ><span v-html="icons['arrow-left']"></span
              ><span
                ><small>{{ previousChapter ? '上一章' : '返回教程概览' }}</small
                ><strong>{{ previousChapter?.title || '挑一个主题，开始探索' }}</strong></span
              ></router-link
            ><router-link class="pagination-link next" :to="sectionRoute(nextChapter?.id)"
              ><span
                ><small>{{ nextChapter ? '下一章' : '返回教程概览' }}</small
                ><strong>{{ nextChapter?.title || '挑一个主题，开始探索' }}</strong></span
              ><span v-html="icons['arrow-right']"></span
            ></router-link>
          </nav>
          <footer class="article-footer">
            <span>慢慢探索，把 AI 用起来。</span
            ><button id="back-top" @click="scrollToTop">
              回到顶部 <span v-html="icons['arrow-up']"></span>
            </button>
          </footer>
        </div>
        <aside class="page-aside" aria-label="本页导航">
          <div class="toc-heading"><span v-html="icons['list']"></span>本页目录</div>
          <nav id="page-toc">
            <button
              v-for="h in pageHeadings"
              :key="h.id"
              class="toc-link"
              :class="['level-' + h.level, { active: activeHeading === h.id }]"
              :aria-current="activeHeading === h.id ? 'location' : undefined"
              @click="navigateToHeading(h.id)"
            >
              {{ h.title }}
            </button>
            <p v-if="!pageHeadings.length" class="search-empty">本章暂无小节</p>
          </nav>
          <div class="page-progress">
            <div>
              <span>阅读进度</span><strong>{{ Math.round(scrollProgress * 100) }}%</strong>
            </div>
            <span class="progress-track"
              ><i :style="{ width: `${scrollProgress * 100}%` }"></i
            ></span>
          </div>
          <div class="quick-links">
            <span>常用入口</span
            ><a href="https://aibaipiao.top" target="_blank" rel="noopener"
              ><b>👌</b>爱白嫖公益站<i v-html="icons['external-link']"></i></a
            ><a href="https://pay.ldxp.cn/shop/qiqiuxiaopu" target="_blank" rel="noopener"
              ><b>🎈</b>气球小铺<i v-html="icons['external-link']"></i
            ></a>
          </div>
        </aside>
      </main>
    </template>
    <template v-else>
      <div class="layout">
        <aside class="sidebar" :class="{ open: mobileOpen }" aria-label="章节目录">
          <div class="sidebar-label">文档目录</div>
          <router-link to="/docs" class="selected">▦ 教程概览</router-link>
          <nav id="side-links">
            <router-link
              v-for="h in topLevelHeadings"
              :key="h.id"
              :to="sectionRoute(h.id)"
              @click="mobileOpen = false"
              >{{ h.title }}</router-link
            >
          </nav>
          <div class="sidebar-footer">
            <span class="status-dot"></span
            ><span v-if="manifest"
              >更新于 {{ formatDate(manifest.published_at || manifest.created_at) }}</span
            >
          </div>
        </aside>
        <main class="main">
          <p v-if="loading" class="loading" role="status">正在加载教程…</p>
          <section v-else-if="errorMessage || !manifest" class="introduction">
            <h1>{{ errorStatus === 404 ? '教程尚未发布' : '教程加载失败' }}</h1>
            <p>{{ errorStatus === 404 ? '教程正在整理中，稍后再来看看。' : errorMessage }}</p>
            <button class="primary" @click="loadDocumentation">重新加载</button>
          </section>
          <template v-else>
            <section class="hero">
              <div class="hero-copy">
                <span class="eyebrow"><i class="status-dot"></i>你好，探索者！</span>
                <h1>把 AI 用起来，<br /><span>从这里开始。</span></h1>
                <p>一份持续更新的使用指南，陪你从好奇走向实践。</p>
                <div class="hero-actions">
                  <a class="primary" href="#chapters">开始探索 →</a
                  ><span class="meta"
                    >{{ manifest.outline.length }} 个章节 ·
                    {{ manifest.assets.length }} 张图片</span
                  >
                </div>
              </div>
              <img
                class="hero-art"
                src="/docs-learning-illustration.png"
                alt="机器人与打开的学习手册"
              />
            </section>
            <div class="content">
              <section id="chapters">
                <div class="section-head">
                  <h2 id="chapters-title">挑一个主题，开始探索</h2>
                  <small>{{ topLevelHeadings.length }} 个主题</small>
                </div>
                <div
                  ref="chapterScroll"
                  class="chapter-scroll"
                  role="region"
                  aria-labelledby="chapters-title"
                  tabindex="0"
                >
                  <div class="chapter-grid">
                    <router-link
                      v-for="h in filteredTopics"
                      :key="h.id"
                      :to="sectionRoute(h.id)"
                      class="chapter"
                      :data-section="h.id"
                      ><span
                        class="number"
                        :class="accents[topLevelHeadings.indexOf(h) % accents.length]"
                        >{{ String(topLevelHeadings.indexOf(h) + 1).padStart(2, '0') }}</span
                      ><span class="chapter-copy"
                        ><strong>{{ h.title }}</strong>
                        <p>{{ descriptionFor(h) }}</p></span
                      ><span class="arrow">→</span></router-link
                    >
                    <p v-if="!filteredTopics.length" class="no-results">
                      {{
                        searchQuery
                          ? '没有找到相关主题，换个关键词试试。'
                          : '教程正在整理中，稍后再来看看。'
                      }}
                    </p>
                  </div>
                </div>
              </section>
              <aside class="resources">
                <div class="section-head">
                  <h2>常用入口</h2>
                  <small>一键直达</small>
                </div>
                <a
                  class="resource resource-service"
                  href="https://aibaipiao.top"
                  target="_blank"
                  rel="noopener noreferrer"
                  ><div class="resource-topline">
                    <span class="resource-label">AI 模型服务</span
                    ><span class="resource-external" aria-hidden="true">↗</span>
                  </div>
                  <div class="resource-title">
                    <span class="resource-emoji" aria-hidden="true">👌</span>爱白嫖公益站
                  </div>
                  <p>一个 API，连接多种 AI 模型。</p>
                  <span class="resource-action">进入公益站<span aria-hidden="true">→</span></span
                  ><small>aibaipiao.top</small></a
                >
                <a
                  class="resource resource-shop"
                  href="https://pay.ldxp.cn/shop/qiqiuxiaopu"
                  target="_blank"
                  rel="noopener noreferrer"
                  ><div class="resource-topline">
                    <span class="resource-label">数字服务小铺</span
                    ><span class="resource-external" aria-hidden="true">↗</span>
                  </div>
                  <div class="resource-title">
                    <span class="resource-emoji" aria-hidden="true">🎈</span>气球小铺
                  </div>
                  <p>便捷、可靠的数字服务入口。</p>
                  <span class="resource-action">去小铺逛逛<span aria-hidden="true">→</span></span
                  ><small>pay.ldxp.cn/shop/qiqiuxiaopu</small></a
                >
                <div class="reading-note">
                  <strong>第一次来？</strong>
                  <p v-if="recommendations.length">
                    可以从<template v-for="(heading, index) in recommendations" :key="heading.id"
                      >{{ index ? '和' : ''
                      }}<router-link :to="sectionRoute(heading.id)"
                        >「{{ heading.title }}」</router-link
                      ></template
                    >开始，也可以在顶部搜索你感兴趣的内容。
                  </p>
                  <p v-else>教程正在整理中，发布后就能在这里查看。</p>
                </div>
              </aside>
            </div>
            <section v-if="introductionHTML" class="introduction">
              <h2>写在前面</h2>
              <div
                ref="contentRef"
                class="docs-content"
                v-html="introductionHTML"
                @click="handleContentClick"
              ></div>
            </section>
          </template>
        </main></div
    ></template>

    <div v-if="searchFocused" class="search-overlay" @click.self="searchFocused = false">
      <section id="search-dialog" role="dialog" aria-modal="true" aria-label="搜索文档">
        <div class="search-field">
          <span v-html="icons.search"></span
          ><input
            ref="searchInput"
            v-model="searchQuery"
            placeholder="搜索文档标题…"
            aria-label="搜索文档标题"
          /><button class="icon-button" aria-label="关闭搜索" @click="searchFocused = false">
            ×
          </button>
        </div>
        <div id="search-results">
          <button
            v-for="h in filteredHeadings.filter((h) => h.level >= 2)"
            :key="h.id"
            class="search-result"
            @click="selectSearchResult(h.id)"
          >
            <span v-html="icons['book-open']"></span
            ><span
              ><strong>{{ h.title }}</strong
              ><small>{{ owningSection(h.id)?.title }}</small></span
            >
          </button>
          <p v-if="!filteredHeadings.length" class="search-empty">
            没有找到相关标题，换个关键词试试。
          </p>
        </div>
        <div class="search-hint">搜索全部章节与小节 <kbd>Esc 关闭</kbd></div>
      </section>
    </div>
    <div
      v-if="lightboxSource"
      class="docs-lightbox"
      role="dialog"
      aria-modal="true"
      aria-label="查看大图"
      @click="lightboxSource = ''"
    >
      <button aria-label="关闭大图">×</button
      ><img :src="lightboxSource" alt="教程大图" @click.stop />
    </div>
    <div v-if="toastMessage" id="toast" role="status">{{ toastMessage }}</div>
  </div>
</template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore } from '@/stores'
import {
  documentationAssetBase,
  getActiveDocumentation,
  getDocumentationContent,
  type DocumentationHeading,
  type DocumentationManifest
} from '@/api/documentation'
import { sanitizeUrl } from '@/utils/url'
import { icons } from './documentationIcons'
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
const searchInput = ref<HTMLInputElement | null>(null)
const chapterScroll = ref<HTMLElement | null>(null)
const mobileToc = ref<HTMLDetailsElement | null>(null)
const focusMode = ref(false)
const folded = ref(false)
const toastMessage = ref('')
let toastTimer: ReturnType<typeof setTimeout> | undefined
let anchorTimer: ReturnType<typeof setTimeout> | undefined
let firstAnchorFrame = 0
let secondAnchorFrame = 0
let themeObserver: MutationObserver | undefined
const siteLogo = computed(
  () =>
    sanitizeUrl(appStore.siteLogo || '/brand-icon.png', {
      allowRelative: true,
      allowDataUrl: true
    }) || '/brand-icon.png'
)
const sectionIndex = computed(() =>
  topLevelHeadings.value.findIndex((h) => h.id === currentSection.value?.id)
)
const previousChapter = computed(() => topLevelHeadings.value[sectionIndex.value - 1])
const nextChapter = computed(() => topLevelHeadings.value[sectionIndex.value + 1])
const accents = [
  'mint',
  'yellow',
  'blue',
  'coral',
  'mint',
  'yellow',
  'coral',
  'blue',
  'yellow',
  'mint'
]
// Subtitles only decorate topics that exist in the active manifest.
const topicSubtitles = new Map([
  ['代理节点', '连接网络，准备开始'],
  ['防封秘籍：指纹浏览器+静态住宅ip', '了解浏览器环境与网络配置'],
  ['下载Agent产品', '找到适合你的 AI 工具'],
  ['中转站使用教程', '配置 API，连接你的应用'],
  ['反向代理软件的概念及使用', '理解原理与常见用法'],
  ['开通可订阅AI的国外卡', '了解支付准备与开通步骤'],
  ['获得用于接码的国外手机号esim（长期持有）', '了解号码获取与长期使用'],
  ['双席位Team空间开通教程（附优惠码申请教程）', '完成空间开通与优惠申请'],
  ['知识和概念的讲解', '补齐基础，理解常见术语'],
  ['玩法推荐', '探索 AI 的更多使用方式']
])
const recommendations = computed(() => {
  const preferred = ['下载Agent产品', '中转站使用教程']
    .map((title) => topLevelHeadings.value.find((h) => h.title === title))
    .filter((h): h is DocumentationHeading => !!h)
  return [...new Set([...preferred, ...topLevelHeadings.value])].slice(0, 2)
})
function descriptionFor(heading: DocumentationHeading) {
  const subtitle = topicSubtitles.get(heading.title)
  if (subtitle) return subtitle
  const outline = manifest.value?.outline || []
  const start = outline.findIndex((h) => h.id === heading.id)
  let end = outline.findIndex((h, i) => i > start && h.level === 2)
  if (end < 0) end = outline.length
  return (
    outline
      .slice(start + 1, end)
      .filter((h) => h.level === 3)
      .slice(0, 2)
      .map((h) => h.title)
      .join(' · ') || '打开教程，了解详细步骤'
  )
}
const imageCount = computed(
  () => new DOMParser().parseFromString(renderedHTML.value, 'text/html').images.length
)
const readingTime = computed(() => {
  const doc = new DOMParser().parseFromString(renderedHTML.value, 'text/html')
  return Math.max(
    1,
    Math.ceil((doc.body.textContent || '').replace(/\s/g, '').length / 350 + imageCount.value / 3)
  )
})
function sectionRoute(id?: string) {
  return id ? { name: 'DocumentationSection', params: { section: id } } : { name: 'Documentation' }
}
function notify(message: string) {
  toastMessage.value = message
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    toastMessage.value = ''
  }, 2200)
}
async function copyPage() {
  try {
    await navigator.clipboard.writeText(location.href)
    notify('阅读链接已复制')
  } catch {
    notify('未能复制，请复制地址栏链接')
  }
}
function foldAll() {
  folded.value = !folded.value
  contentRef.value
    ?.querySelectorAll<HTMLDetailsElement>('details:not(.docs-chapter-root)')
    .forEach((d) => {
      d.open = !folded.value
    })
  updateScrollState()
}
function openSearch() {
  searchFocused.value = true
  void nextTick(() => searchInput.value?.focus())
}
function syncTheme() {
  const saved = localStorage.getItem('theme')
  if (saved === 'dark' || saved === 'light')
    document.documentElement.classList.toggle('dark', saved === 'dark')
  isDark.value = document.documentElement.classList.contains('dark')
}
function storageTheme(event: StorageEvent) {
  if (event.key === 'theme' || event.key === null) syncTheme()
}
let scrollFrame = 0

const topLevelHeadings = computed(() =>
  (manifest.value?.outline || []).filter((item) => item.level === 2)
)
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
const introductionHTML = computed(() =>
  extractDocumentationIntroduction(fullRenderedHTML.value, topLevelHeadings.value[0]?.id || '')
)

const filteredHeadings = computed(() => {
  const query = searchQuery.value.trim().toLocaleLowerCase(locale.value)
  if (!manifest.value) return []
  if (!query) return manifest.value.outline
  return manifest.value.outline.filter((item) =>
    item.title.toLocaleLowerCase(locale.value).includes(query)
  )
})

const filteredTopics = computed(() =>
  topLevelHeadings.value.filter((item) =>
    `${item.title} ${descriptionFor(item)}`
      .toLocaleLowerCase(locale.value)
      .includes(searchQuery.value.trim().toLocaleLowerCase(locale.value))
  )
)
const pageHeadings = computed(() => currentSectionHeadings.value.filter((item) => item.level > 2))

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  }).format(new Date(value))
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function extractDocumentationSection(html: string, sectionID: string): string {
  if (!html || !sectionID) return ''
  const parsed = new DOMParser().parseFromString(
    `<div id="docs-section-root">${html}</div>`,
    'text/html'
  )
  const target = parsed.getElementById(sectionID)
  if (!target) return ''
  if (target instanceof HTMLDetailsElement) {
    target.open = true
    target.classList.add('docs-chapter-root')
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
    if (node === target) node.classList.add('docs-page-title')
    sectionNodes.push(node.outerHTML)
    node = node.nextElementSibling
  }
  return `<section class="notion-document docs-section-document">${sectionNodes.join('')}</section>`
}

function extractDocumentationIntroduction(html: string, firstSectionID: string): string {
  if (!html || !firstSectionID) return ''
  const parsed = new DOMParser().parseFromString(
    `<div id="docs-introduction-root">${html}</div>`,
    'text/html'
  )
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
  if (!element || !contentRef.value?.contains(element)) return
  let parent = element.closest('details')
  while (parent) {
    parent.open = true
    parent = parent.parentElement?.closest('details') || null
  }
  mobileOpen.value = false
  if (mobileToc.value) mobileToc.value.open = false
  if (updateLocation) {
    const hash = currentSection.value?.id === id ? '' : `#${id}`
    // Keep Vue Router's history bookkeeping for back/forward navigation.
    void router.push({ ...sectionRoute(currentSection.value?.id), hash })
  }
  cancelAnimationFrame(firstAnchorFrame)
  cancelAnimationFrame(secondAnchorFrame)
  firstAnchorFrame = requestAnimationFrame(() => {
    secondAnchorFrame = requestAnimationFrame(() => {
      const marker =
        element instanceof HTMLDetailsElement
          ? element.querySelector(':scope > summary') || element
          : element
      window.scrollTo({
        top: window.scrollY + marker.getBoundingClientRect().top - 96,
        behavior: window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
          ? 'auto'
          : 'smooth'
      })
      activeHeading.value = id
    })
  })
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
      hash: id === section.id ? '' : `#${id}`
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

async function handleContentClick(event: MouseEvent) {
  const target = event.target as HTMLElement
  const image = target.closest<HTMLImageElement>('img.docs-zoomable-image')
  if (image) {
    event.preventDefault()
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
    window.setTimeout(() => {
      button.textContent = t('documentation.public.copy')
    }, 1600)
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
      if (!element || !contentRef.value?.contains(element)) continue
      const marker =
        element instanceof HTMLDetailsElement ? element.querySelector(':scope > summary') : element
      if (!marker) continue
      let ancestor = marker.parentElement
      let hidden = false
      while (ancestor && ancestor !== contentRef.value) {
        if (ancestor instanceof HTMLDetailsElement && !ancestor.open && ancestor !== element) {
          hidden = true
          break
        }
        ancestor = ancestor.parentElement
      }
      if (!hidden && marker.getClientRects().length && marker.getBoundingClientRect().top <= 98)
        current = item.id
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
  folded.value = false
  contentRef.value?.querySelectorAll<HTMLImageElement>('img').forEach((image) => {
    const src = (image.getAttribute('src') || '').split(/[?#]/)[0]
    const asset = manifest.value?.assets.find((a) => src.endsWith('/' + a.path))
    if (asset?.width && asset?.height) {
      image.width = asset.width
      image.height = asset.height
    }
  })
  contentRef.value?.querySelectorAll('.docs-toggle-level-3 > summary').forEach((summary, index) => {
    if (summary.querySelector('.section-number')) return
    const number = document.createElement('span')
    number.className = 'section-number'
    number.textContent = String(index + 1).padStart(2, '0')
    summary.append(number)
  })
  clearTimeout(anchorTimer)
  const rawHash = route.hash.slice(1)
  const targetID = rawHash ? decodeURIComponent(rawHash) : ''
  if (targetID) {
    anchorTimer = setTimeout(() => scrollToHeading(targetID, false), 80)
  } else {
    window.scrollTo({ top: 0, behavior: 'auto' })
    activeHeading.value = currentSection.value?.id || ''
  }
}

async function loadDocumentation() {
  loading.value = true
  errorMessage.value = ''
  errorStatus.value = 0
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
    openSearch()
  }
  if (event.key === 'Escape') {
    lightboxSource.value = ''
    searchFocused.value = false
    mobileOpen.value = false
  }
}

watch([() => route.params.section, () => route.hash], () => {
  void refreshSectionView()
})

watch(searchQuery, () => {
  if (chapterScroll.value) chapterScroll.value.scrollTop = 0
})
onMounted(() => {
  syncTheme()
  themeObserver = new MutationObserver(() => {
    isDark.value = document.documentElement.classList.contains('dark')
  })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  window.addEventListener('storage', storageTheme)
  window.addEventListener('pageshow', syncTheme)
  window.addEventListener('resize', updateScrollState)
  void loadDocumentation()
  window.addEventListener('scroll', updateScrollState, { passive: true })
  window.addEventListener('keydown', handleSearchShortcut)
})

onBeforeUnmount(() => {
  themeObserver?.disconnect()
  clearTimeout(toastTimer)
  clearTimeout(anchorTimer)
  cancelAnimationFrame(firstAnchorFrame)
  cancelAnimationFrame(secondAnchorFrame)
  window.removeEventListener('storage', storageTheme)
  window.removeEventListener('pageshow', syncTheme)
  window.removeEventListener('resize', updateScrollState)
  window.removeEventListener('scroll', updateScrollState)
  window.removeEventListener('keydown', handleSearchShortcut)
  if (scrollFrame) cancelAnimationFrame(scrollFrame)
})
</script>
<style src="./documentation.css"></style>
