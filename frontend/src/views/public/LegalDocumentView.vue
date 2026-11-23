<template>
  <div class="journal-standalone min-h-screen text-[var(--craft-ink)]">
    <header class="journal-topbar sticky top-0 z-20">
      <div class="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-4 sm:px-6">
        <RouterLink to="/home" class="flex min-w-0 items-center gap-3">
          <template v-if="settings">
            <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center overflow-hidden rounded-xl border border-[var(--craft-border)] bg-[var(--craft-surface)] shadow-sm">
              <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
            </span>
            <span class="truncate text-base font-bold text-[var(--craft-ink)]">
              {{ siteName }}
            </span>
          </template>
          <template v-else>
            <span class="skeleton h-10 w-10 flex-shrink-0 rounded-xl" aria-hidden="true"></span>
            <span class="skeleton h-5 w-28 rounded" aria-hidden="true"></span>
          </template>
        </RouterLink>
        <RouterLink
          to="/login"
          class="btn btn-primary flex-shrink-0 rounded-full px-4 py-2 text-sm font-semibold"
        >
          {{ t('home.login') }}
        </RouterLink>
      </div>
    </header>

    <main class="mx-auto max-w-4xl px-4 py-8 sm:px-6 lg:py-10">
      <div v-if="loading" class="flex min-h-[320px] items-center justify-center">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-[var(--craft-sky-deep)]"></div>
      </div>

      <section
        v-else-if="loadError"
        class="card border-red-200 bg-red-50 p-6 text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200"
      >
        <h1 class="text-lg font-semibold">{{ t('legal.loadFailed') }}</h1>
        <p class="mt-2 text-sm">{{ t('legal.retryLater') }}</p>
      </section>

      <section
        v-else-if="!currentDocument"
        class="card p-6"
      >
        <div class="flex items-start gap-3">
          <span class="journal-icon-well--sm text-[var(--craft-muted)]">
            <Icon name="document" size="sm" />
          </span>
          <div>
            <h1 class="text-lg font-semibold text-[var(--craft-ink)]">{{ t('legal.notFound') }}</h1>
            <p class="mt-2 text-sm leading-6 text-[var(--craft-ink-soft)]">
              {{ t('legal.notFoundDescription') }}
            </p>
          </div>
        </div>
      </section>

      <article v-else class="card p-6 sm:p-10">
        <div class="mb-8 border-b border-[var(--craft-border)] pb-6">
          <div class="flex items-start gap-4">
            <span class="journal-icon-well text-[var(--craft-sky-deep)]">
              <Icon :name="documentIcon" size="md" />
            </span>
            <div class="min-w-0">
              <p class="text-sm font-semibold text-[var(--craft-sky-deep)]">{{ documentTypeLabel }}</p>
              <h1 class="mt-2 break-words text-2xl font-bold tracking-normal text-[var(--craft-ink)] sm:text-3xl">
                {{ currentDocument.title }}
              </h1>
              <p v-if="updatedAt" class="mt-3 text-sm text-[var(--craft-muted)]">
                {{ t('legal.updatedAt', { date: updatedAt }) }}
              </p>
            </div>
          </div>
        </div>

        <div
          v-if="hasContent"
          class="legal-document-content"
          v-html="renderedHtml"
        ></div>
        <div
          v-else
          class="rounded-2xl border border-dashed border-[var(--craft-border-strong)] bg-[var(--craft-surface-muted)] px-6 py-14 text-center text-sm text-[var(--craft-muted)]"
        >
          {{ t('legal.empty') }}
        </div>
      </article>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { getLocale } from '@/i18n'
import { sanitizeUrl } from '@/utils/url'
import { useAppStore } from '@/stores/app'
import type { LoginAgreementDocument } from '@/types'
import zhAdminCompliance from '../../../../docs/legal/admin-compliance.zh.md?raw'
import enAdminCompliance from '../../../../docs/legal/admin-compliance.en.md?raw'

type LegalDocumentIcon = 'document' | 'shield' | 'globe' | 'cog'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const settings = computed(() => appStore.cachedPublicSettings)
const loading = ref(!settings.value)
const loadError = ref(false)

marked.setOptions({
  breaks: true,
  gfm: true,
})

const documentId = computed(() => String(route.params.documentId || ''))
const isAdminComplianceDocument = computed(() => documentId.value === 'admin-compliance')
const documents = computed(() => settings.value?.login_agreement_documents ?? [])
const siteName = computed(() => settings.value?.site_name || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(settings.value?.site_logo || '', {
  allowRelative: true,
  allowDataUrl: true,
}))
const updatedAt = computed(() =>
  isAdminComplianceDocument.value ? '' : settings.value?.login_agreement_updated_at || ''
)
const documentTypeLabel = computed(() =>
  isAdminComplianceDocument.value ? t('legal.adminCompliance') : t('legal.loginAgreement')
)

const currentDocument = computed<LoginAgreementDocument | null>(() => {
  if (isAdminComplianceDocument.value) {
    return {
      id: 'admin-compliance',
      title: t('adminCompliance.title'),
      content_md: getLocale() === 'zh' ? zhAdminCompliance : enAdminCompliance
    }
  }
  const id = documentId.value
  if (!id) {
    return null
  }
  return documents.value.find((doc) => doc.id === id) ?? null
})

const hasContent = computed(() => Boolean(currentDocument.value?.content_md?.trim()))

const renderedHtml = computed(() => {
  const content = currentDocument.value?.content_md?.trim() || ''
  if (!content) {
    return ''
  }
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
})

const documentIcon = computed<LegalDocumentIcon>(() => {
  const title = currentDocument.value?.title || ''
  if (title.includes('政策') || title.includes('隐私')) {
    return 'shield'
  }
  if (title.includes('国家') || title.includes('地区')) {
    return 'globe'
  }
  if (title.includes('特定')) {
    return 'cog'
  }
  return 'document'
})

onMounted(async () => {
  loadError.value = false
  const loadedSettings = await appStore.fetchPublicSettings()
  if (!loadedSettings) {
    loadError.value = true
  }
  loading.value = false
})
</script>

<style scoped>
.legal-document-content {
  line-height: 1.75;
  overflow-wrap: anywhere;
  color: inherit;
}

.legal-document-content :deep(h1) {
  @apply mb-4 mt-8 pb-3 text-3xl font-bold;
  border-bottom: 1px solid var(--craft-border, #e6d7be);
}

.legal-document-content :deep(h2) {
  @apply mb-3 mt-7 text-2xl font-bold;
}

.legal-document-content :deep(h3) {
  @apply mb-2 mt-6 text-xl font-semibold;
}

.legal-document-content :deep(h4) {
  @apply mb-2 mt-5 text-lg font-semibold;
}

.legal-document-content :deep(p) {
  @apply mb-4;
  color: var(--craft-ink-soft, #4c4237);
}

.legal-document-content :deep(a) {
  color: var(--craft-sky-deep, #2a8bd4);
  @apply underline underline-offset-4;
}

.legal-document-content :deep(ul) {
  @apply mb-4 list-disc pl-6;
}

.legal-document-content :deep(ol) {
  @apply mb-4 list-decimal pl-6;
}

.legal-document-content :deep(li) {
  @apply mb-1;
  color: var(--craft-ink-soft, #4c4237);
}

.legal-document-content :deep(blockquote) {
  @apply my-5 border-l-4 pl-4;
  border-color: var(--craft-border-strong, #d5c09e);
  color: var(--craft-muted, #756758);
}

.legal-document-content :deep(code) {
  @apply rounded-md px-1.5 py-0.5 font-mono text-sm;
  background-color: var(--craft-surface-muted, #faf2e2);
  border: 1px solid var(--craft-border, #e6d7be);
}

.legal-document-content :deep(pre) {
  @apply my-5 overflow-x-auto rounded-xl bg-[#221e1a] p-4 text-[#f5ede0];
}

.legal-document-content :deep(pre code) {
  @apply border-0 bg-transparent p-0 text-inherit;
}

.legal-document-content :deep(table) {
  @apply my-5 block w-full overflow-x-auto border-collapse;
}

.legal-document-content :deep(th) {
  @apply px-3 py-2 text-left font-semibold;
  border: 1px solid var(--craft-border-strong, #d5c09e);
  background-color: var(--craft-surface-header, #f7ead2);
}

.legal-document-content :deep(td) {
  @apply px-3 py-2;
  border: 1px solid var(--craft-border, #e6d7be);
}

.legal-document-content :deep(img) {
  @apply my-5 h-auto max-w-full rounded-xl;
}

.legal-document-content :deep(hr) {
  @apply my-7;
  border-color: var(--craft-border, #e6d7be);
}
</style>
