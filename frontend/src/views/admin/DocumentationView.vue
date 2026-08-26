<template>
  <AppLayout>
    <div class="docs-admin space-y-6">
      <div class="flex justify-end">
        <a href="/docs" target="_blank" rel="noopener noreferrer" class="btn btn-secondary">
          {{ t('documentation.admin.viewSite') }} ↗
        </a>
      </div>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 p-5 dark:border-dark-700">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('documentation.admin.uploadTitle') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('documentation.admin.uploadRule') }}</p>
        </div>
        <div class="p-5">
          <input ref="fileInput" class="hidden" type="file" accept=".zip,application/zip" @change="handleFileInput" />
          <button
            type="button"
            class="docs-dropzone"
            :class="{ 'docs-dropzone-active': dragActive, 'docs-dropzone-busy': importing }"
            :disabled="importing"
            @click="fileInput?.click()"
            @dragenter.prevent="dragActive = true"
            @dragover.prevent="dragActive = true"
            @dragleave.prevent="dragActive = false"
            @drop.prevent="handleDrop"
          >
            <span class="docs-upload-icon">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16V4m0 0L7.5 8.5M12 4l4.5 4.5M5 13.5v4.25A2.25 2.25 0 007.25 20h9.5A2.25 2.25 0 0019 17.75V13.5" /></svg>
            </span>
            <strong>{{ importing ? t('documentation.admin.importing', { progress: uploadProgress }) : t('documentation.admin.uploadHint') }}</strong>
            <span v-if="selectedFile">{{ selectedFile.name }} · {{ formatBytes(selectedFile.size) }}</span>
            <span v-else>Markdown + PNG / JPG / GIF / WebP · ZIP ≤ 64 MB</span>
            <span v-if="importing" class="docs-upload-progress"><i :style="{ width: `${uploadProgress}%` }"></i></span>
            <span v-else class="docs-choose-button">{{ t('documentation.admin.chooseFile') }}</span>
          </button>
        </div>
      </section>

      <section v-if="preview" class="card overflow-hidden">
        <div class="docs-preview-header">
          <div>
            <div class="flex items-center gap-2">
              <span class="docs-ready-dot"></span>
              <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('documentation.admin.previewTitle') }}</h2>
            </div>
            <p class="mt-1 text-sm text-gray-500">{{ t('documentation.admin.source', { name: preview.manifest.source_file }) }}</p>
          </div>
          <button class="btn btn-primary" :disabled="publishing" @click="publishPreview">
            {{ publishing ? t('documentation.admin.publishing') : t('documentation.admin.publish') }}
          </button>
        </div>

        <div class="docs-summary-grid">
          <div><span>§</span><strong>{{ preview.manifest.outline.length }}</strong><small>{{ t('documentation.admin.chapterLabel') }}</small></div>
          <div><span>▧</span><strong>{{ preview.manifest.assets.length }}</strong><small>{{ t('documentation.admin.imageLabel') }}</small></div>
          <div><span>Δ</span><strong>{{ preview.changes.content_changed ? t('documentation.admin.yes') : t('documentation.admin.no') }}</strong><small>{{ preview.changes.content_changed ? t('documentation.admin.contentChanged') : t('documentation.admin.contentUnchanged') }}</small></div>
          <div><span>MB</span><strong>{{ formatBytes(preview.manifest.content_bytes) }}</strong><small>Markdown</small></div>
        </div>

        <div class="grid gap-5 p-5 xl:grid-cols-[260px_minmax(0,1fr)]">
          <aside class="space-y-4">
            <div class="docs-side-panel">
              <h3>{{ t('documentation.admin.changeSummary') }}</h3>
              <div class="docs-change-row"><span class="docs-change-add">+</span>{{ t('documentation.admin.assetsAdded', { count: preview.changes.assets_added }) }}</div>
              <div class="docs-change-row"><span class="docs-change-edit">~</span>{{ t('documentation.admin.assetsChanged', { count: preview.changes.assets_changed }) }}</div>
              <div class="docs-change-row"><span class="docs-change-remove">−</span>{{ t('documentation.admin.assetsRemoved', { count: preview.changes.assets_removed }) }}</div>
            </div>
            <div class="docs-side-panel">
              <h3>{{ t('documentation.admin.previewContents') }}</h3>
              <div class="docs-preview-outline">
                <span v-for="item in preview.manifest.outline" :key="item.id" :style="{ paddingLeft: `${Math.max(0, item.level - 1) * 10}px` }">{{ item.title }}</span>
              </div>
            </div>
            <div v-if="preview.manifest.warnings.length" class="docs-warning-panel">
              <h3>⚠ {{ t('documentation.admin.warnings') }}</h3>
              <p v-for="warning in preview.manifest.warnings" :key="warning">{{ warning }}</p>
            </div>
          </aside>

          <div class="docs-preview-frame">
            <div class="docs-preview-toolbar"><i></i><i></i><i></i><span>{{ preview.manifest.title }}</span></div>
            <article class="docs-preview-content" v-html="renderedPreview"></article>
          </div>
        </div>
      </section>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(360px,.7fr)]">
        <section class="card p-5">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('documentation.admin.activeVersion') }}</h2>
          <div v-if="loadingState" class="mt-5 h-24 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-700"></div>
          <div v-else-if="state?.active" class="docs-active-card">
            <span class="docs-live-badge"><i></i> LIVE</span>
            <h3>{{ state.active.title }}</h3>
            <p>{{ formatDate(state.active.published_at || state.active.created_at) }} · {{ t('documentation.public.images', { count: state.active.assets.length }) }} · {{ t('documentation.public.sections', { count: state.active.outline.length }) }}</p>
            <code>{{ shortID(state.active.id) }}</code>
          </div>
          <div v-else class="docs-no-active">{{ t('documentation.admin.noActiveVersion') }}</div>
        </section>

        <section class="card p-5">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('documentation.admin.versionHistory') }}</h2>
          <div class="mt-4 max-h-80 space-y-2 overflow-y-auto pr-1">
            <div v-for="version in state?.versions || []" :key="version.manifest.id" class="docs-version-row">
              <div class="min-w-0">
                <div class="flex items-center gap-2"><strong class="truncate">{{ version.manifest.title }}</strong><span v-if="version.active" class="docs-active-pill">{{ t('documentation.admin.active') }}</span></div>
                <small>{{ formatDate(version.manifest.published_at || version.manifest.created_at) }} · {{ shortID(version.manifest.id) }}</small>
              </div>
              <button v-if="!version.active" class="btn btn-secondary btn-sm flex-shrink-0" :disabled="activatingID === version.manifest.id" @click="activateVersion(version)">{{ t('documentation.admin.rollback') }}</button>
            </div>
            <div v-if="!state?.versions.length" class="py-8 text-center text-sm text-gray-400">{{ t('documentation.admin.neverPublished') }}</div>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores'
import {
  activateDocumentationVersion,
  documentationPreviewAssetBase,
  getDocumentationState,
  importDocumentation,
  publishDocumentationDraft,
  type DocumentationPreview,
  type DocumentationState,
  type DocumentationVersion
} from '@/api/documentation'
import { renderDocumentationMarkdown } from '@/utils/documentationMarkdown'

const { t, locale } = useI18n()
const appStore = useAppStore()
const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const preview = ref<DocumentationPreview | null>(null)
const state = ref<DocumentationState | null>(null)
const importing = ref(false)
const publishing = ref(false)
const loadingState = ref(true)
const dragActive = ref(false)
const uploadProgress = ref(0)
const activatingID = ref('')

const renderedPreview = computed(() => {
  if (!preview.value) return ''
  return renderDocumentationMarkdown(
    preview.value.markdown,
    preview.value.manifest.outline,
    documentationPreviewAssetBase(preview.value.draft_id),
    t('documentation.public.copy')
  )
})

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function shortID(id: string): string {
  return id.slice(0, 8)
}

function extractMessage(error: unknown, fallback: string): string {
  return (error as { message?: string })?.message || fallback
}

async function loadState() {
  loadingState.value = true
  try {
    state.value = await getDocumentationState()
  } catch (error) {
    appStore.showError(extractMessage(error, t('errors.networkError')))
  } finally {
    loadingState.value = false
  }
}

async function processFile(file: File) {
  if (!file.name.toLowerCase().endsWith('.zip')) {
    appStore.showError(t('documentation.admin.importFailed'))
    return
  }
  selectedFile.value = file
  importing.value = true
  uploadProgress.value = 0
  try {
    preview.value = await importDocumentation(file, (value) => { uploadProgress.value = value })
    uploadProgress.value = 100
  } catch (error) {
    appStore.showError(extractMessage(error, t('documentation.admin.importFailed')))
  } finally {
    importing.value = false
    if (fileInput.value) fileInput.value.value = ''
  }
}

function handleFileInput(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (file) void processFile(file)
}

function handleDrop(event: DragEvent) {
  dragActive.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file) void processFile(file)
}

async function publishPreview() {
  if (!preview.value) return
  publishing.value = true
  try {
    await publishDocumentationDraft(preview.value.draft_id)
    preview.value = null
    selectedFile.value = null
    appStore.showSuccess(t('documentation.admin.publishSuccess'))
    await loadState()
  } catch (error) {
    appStore.showError(extractMessage(error, t('documentation.admin.publishFailed')))
  } finally {
    publishing.value = false
  }
}

async function activateVersion(version: DocumentationVersion) {
  if (!window.confirm(t('documentation.admin.rollbackConfirm', { title: version.manifest.title }))) return
  activatingID.value = version.manifest.id
  try {
    await activateDocumentationVersion(version.manifest.id)
    appStore.showSuccess(t('documentation.admin.rollbackSuccess'))
    await loadState()
  } catch (error) {
    appStore.showError(extractMessage(error, t('documentation.admin.rollbackFailed')))
  } finally {
    activatingID.value = ''
  }
}

onMounted(() => { void loadState() })
</script>

<style scoped>
.docs-dropzone { display: flex; min-height: 230px; width: 100%; flex-direction: column; align-items: center; justify-content: center; gap: 10px; border: 1.5px dashed #cbd5e1; border-radius: 18px; color: #64748b; background: radial-gradient(circle at 50% 0,rgba(99,102,241,.08),transparent 45%),rgba(248,250,252,.7); transition: .22s ease; }
.dark .docs-dropzone { border-color: #475569; background: radial-gradient(circle at 50% 0,rgba(99,102,241,.14),transparent 45%),rgba(15,23,42,.4); }
.docs-dropzone:hover,.docs-dropzone-active { border-color: #818cf8; color: #4f46e5; background-color: rgba(238,242,255,.8); transform: translateY(-2px); box-shadow: 0 18px 45px rgba(99,102,241,.1); }.docs-dropzone-busy { pointer-events: none; }
.docs-upload-icon { display: grid; place-items: center; width: 54px; height: 54px; border-radius: 17px; color: #6366f1; background: linear-gradient(135deg,#eef2ff,#cffafe); box-shadow: 0 13px 30px rgba(99,102,241,.16); }.docs-upload-icon svg { width: 26px; fill: none; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
.docs-dropzone strong { color: #1e293b; font-size: 15px; }.dark .docs-dropzone strong { color: #f1f5f9; }.docs-dropzone > span { font-size: 12px; }
.docs-choose-button { margin-top: 5px; padding: 7px 14px; border: 1px solid rgba(99,102,241,.25); border-radius: 9px; color: #4f46e5; background: rgba(255,255,255,.75); font-weight: 650; }.dark .docs-choose-button { background: rgba(30,41,59,.8); color: #a5b4fc; }
.docs-upload-progress { width: min(360px,75%); height: 5px; overflow: hidden; border-radius: 99px; background: #e2e8f0; }.docs-upload-progress i { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg,#6366f1,#06b6d4); transition: width .2s; }
.docs-preview-header { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 15px; padding: 18px 20px; border-bottom: 1px solid #f1f5f9; background: linear-gradient(90deg,rgba(99,102,241,.055),transparent); }.dark .docs-preview-header { border-color: #293548; }
.docs-ready-dot { width: 8px; height: 8px; border-radius: 50%; background: #22c55e; box-shadow: 0 0 0 5px rgba(34,197,94,.12); }
.docs-summary-grid { display: grid; grid-template-columns: repeat(4,minmax(0,1fr)); border-bottom: 1px solid #f1f5f9; }.dark .docs-summary-grid { border-color: #293548; }.docs-summary-grid > div { display: grid; grid-template-columns: auto 1fr; align-items: center; column-gap: 10px; padding: 16px 20px; border-right: 1px solid #f1f5f9; }.dark .docs-summary-grid > div { border-color: #293548; }.docs-summary-grid > div:last-child { border: 0; }.docs-summary-grid span { grid-row: span 2; display: grid; place-items: center; width: 34px; height: 34px; border-radius: 10px; color: #6366f1; background: #eef2ff; font-size: 11px; font-weight: 800; }.dark .docs-summary-grid span { background: rgba(99,102,241,.16); color: #a5b4fc; }.docs-summary-grid strong { color: #172033; font-size: 17px; }.dark .docs-summary-grid strong { color: #f8fafc; }.docs-summary-grid small { color: #94a3b8; font-size: 10px; }
.docs-side-panel,.docs-warning-panel { padding: 15px; border: 1px solid #e8edf4; border-radius: 13px; background: #fbfcfe; }.dark .docs-side-panel { border-color: #334155; background: rgba(15,23,42,.35); }.docs-side-panel h3,.docs-warning-panel h3 { margin-bottom: 11px; color: #64748b; font-size: 10px; font-weight: 800; letter-spacing: .09em; text-transform: uppercase; }
.docs-change-row { display: flex; align-items: center; gap: 8px; margin: 7px 0; color: #64748b; font-size: 12px; }.docs-change-row span { display: grid; place-items: center; width: 19px; height: 19px; border-radius: 6px; font-weight: 800; }.docs-change-add { color: #16a34a; background: #dcfce7; }.docs-change-edit { color: #d97706; background: #fef3c7; }.docs-change-remove { color: #dc2626; background: #fee2e2; }
.docs-preview-outline { max-height: 260px; overflow-y: auto; }.docs-preview-outline span { display: block; overflow: hidden; padding-top: 4px; padding-bottom: 4px; color: #64748b; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; }
.docs-warning-panel { border-color: #fde68a; background: #fffbeb; }.dark .docs-warning-panel { border-color: rgba(245,158,11,.3); background: rgba(120,53,15,.13); }.docs-warning-panel h3 { color: #b45309; }.docs-warning-panel p { margin-top: 7px; color: #92400e; font-size: 11px; line-height: 1.55; }.dark .docs-warning-panel p { color: #fcd34d; }
.docs-preview-frame { min-width: 0; overflow: hidden; border: 1px solid #dfe6ef; border-radius: 15px; background: white; box-shadow: 0 18px 45px rgba(15,23,42,.08); }.dark .docs-preview-frame { border-color: #334155; background: #0f172a; }.docs-preview-toolbar { display: flex; align-items: center; gap: 6px; height: 38px; padding: 0 13px; border-bottom: 1px solid #e8edf4; color: #94a3b8; background: #f8fafc; font-size: 10px; }.dark .docs-preview-toolbar { border-color: #334155; background: #182135; }.docs-preview-toolbar i { width: 8px; height: 8px; border-radius: 50%; background: #cbd5e1; }.docs-preview-toolbar i:first-child { background: #f87171; }.docs-preview-toolbar i:nth-child(2) { background: #fbbf24; }.docs-preview-toolbar i:nth-child(3) { background: #4ade80; }.docs-preview-toolbar span { margin-left: 6px; }
.docs-preview-content { max-height: 720px; overflow: auto; padding: 32px clamp(22px,5vw,64px); color: #475569; font-size: 13px; line-height: 1.75; }.dark .docs-preview-content { color: #cbd5e1; }.docs-preview-content :deep(h1) { margin: 0 0 26px; color: #111827; font-size: 32px; font-weight: 800; }.docs-preview-content :deep(h2) { margin: 36px 0 12px; color: #172033; font-size: 23px; font-weight: 760; }.docs-preview-content :deep(h3) { margin: 28px 0 10px; color: #334155; font-size: 18px; font-weight: 720; }.dark .docs-preview-content :deep(h1),.dark .docs-preview-content :deep(h2),.dark .docs-preview-content :deep(h3) { color: #f1f5f9; }.docs-preview-content :deep(.docs-heading-anchor),.docs-preview-content :deep(.docs-copy-button) { display: none; }.docs-preview-content :deep(img) { max-width: 100%; margin: 18px auto; border-radius: 9px; box-shadow: 0 10px 28px rgba(15,23,42,.12); }.docs-preview-content :deep(.docs-callout) { margin: 16px 0; padding: 14px 16px; border: 1px solid rgba(99,102,241,.2); border-left: 3px solid #6366f1; border-radius: 10px; background: rgba(99,102,241,.06); }.docs-preview-content :deep(.docs-callout-label) { display: block; color: #6366f1; font-size: 9px; font-weight: 800; }.docs-preview-content :deep(li) { margin: 5px 0; }.docs-preview-content :deep(ul),.docs-preview-content :deep(ol) { padding-left: 20px; }
.docs-active-card { position: relative; overflow: hidden; margin-top: 16px; padding: 20px; border: 1px solid rgba(34,197,94,.18); border-radius: 15px; background: linear-gradient(135deg,rgba(34,197,94,.08),rgba(6,182,212,.04)); }.docs-active-card::after { content: ''; position: absolute; right: -35px; top: -40px; width: 120px; height: 120px; border: 20px solid rgba(34,197,94,.05); border-radius: 50%; }.docs-live-badge { display: inline-flex; align-items: center; gap: 6px; color: #16a34a; font-size: 9px; font-weight: 850; letter-spacing: .13em; }.docs-live-badge i { width: 7px; height: 7px; border-radius: 50%; background: #22c55e; box-shadow: 0 0 0 4px rgba(34,197,94,.12); }.docs-active-card h3 { margin-top: 12px; color: #172033; font-size: 20px; font-weight: 760; }.dark .docs-active-card h3 { color: #f8fafc; }.docs-active-card p { margin-top: 5px; color: #64748b; font-size: 12px; }.docs-active-card code { display: inline-block; margin-top: 13px; padding: 3px 7px; border-radius: 6px; color: #64748b; background: rgba(255,255,255,.7); font-size: 10px; }.dark .docs-active-card code { background: #1e293b; }
.docs-no-active { display: grid; min-height: 128px; place-items: center; margin-top: 16px; border: 1px dashed #dbe2eb; border-radius: 15px; color: #94a3b8; font-size: 13px; }.dark .docs-no-active { border-color: #334155; }
.docs-version-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 11px 12px; border: 1px solid #eef1f5; border-radius: 11px; }.dark .docs-version-row { border-color: #293548; }.docs-version-row strong { color: #334155; font-size: 12px; }.dark .docs-version-row strong { color: #e2e8f0; }.docs-version-row small { display: block; margin-top: 3px; color: #94a3b8; font-size: 10px; }.docs-active-pill { padding: 2px 6px; border-radius: 99px; color: #15803d; background: #dcfce7; font-size: 8px; font-weight: 800; text-transform: uppercase; }
@media (max-width: 760px) { .docs-summary-grid { grid-template-columns: repeat(2,minmax(0,1fr)); }.docs-summary-grid > div:nth-child(2) { border-right: 0; }.docs-summary-grid > div:nth-child(-n+2) { border-bottom: 1px solid #f1f5f9; } }
</style>
