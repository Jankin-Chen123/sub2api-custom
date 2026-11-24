<template>
  <div data-testid="contact-theme-root">
    <main class="contact-page" :class="{ 'contact-page--embedded': isEmbedded }">
      <div class="contact-page-shape contact-page-shape--peach" aria-hidden="true"></div>
      <div class="contact-page-shape contact-page-shape--aqua" aria-hidden="true"></div>
      <div class="contact-page-route" aria-hidden="true">
        <span class="contact-page-route-line"></span>
        <span class="contact-page-route-dot contact-page-route-dot--teal"></span>
        <span class="contact-page-route-dot contact-page-route-dot--yellow"></span>
        <span class="contact-page-route-dot contact-page-route-dot--peach"></span>
      </div>

      <div
        class="contact-page-inner"
        :class="{ 'contact-page-inner--embedded': isEmbedded }"
      >
        <section
          class="contact-shell"
          :class="{ 'contact-shell--embedded': isEmbedded }"
          data-testid="contact-shell"
        >
          <div class="contact-shell-blob contact-shell-blob--aqua" aria-hidden="true"></div>
          <div class="contact-shell-blob contact-shell-blob--peach" aria-hidden="true"></div>

          <div class="contact-topbar">
            <div class="contact-brand">
              <img
                :src="siteLogo"
                :alt="siteName"
                class="contact-brand-logo"
                data-testid="contact-brand-logo"
              />
              <span class="contact-brand-name" data-testid="contact-brand-name">{{ siteName }}</span>
            </div>
            <span class="contact-status">
              <span class="contact-status-dot" aria-hidden="true"></span>
              {{ t('contactPage.online') }}
            </span>
          </div>

          <header class="contact-hero">
            <div class="contact-hero-mark" aria-hidden="true">
              <JournalBannerSticker name="contact" size="sm" />
            </div>
            <p class="contact-kicker">{{ t('contactPage.kicker') }}</p>
            <div class="contact-hero-title-row">
              <h1>{{ t('contactPage.title') }}</h1>
              <CraftSparks variant="right" size="sm" />
            </div>
            <p class="contact-subtitle">{{ t('contactPage.subtitle') }}</p>
          </header>

          <section
            class="contact-action-grid"
            :aria-label="t('contactPage.optionsAria')"
          >
            <button
              type="button"
              data-testid="qq-contact-card"
              class="contact-action-card contact-action-card--qq"
              :disabled="!hasQqGroup"
              :aria-label="hasQqGroup ? t('contactPage.qq.copy') : t('contactPage.pending')"
              @click="copyQqGroupNumber"
            >
              <span class="contact-card-main">
                <span class="contact-platform-icon contact-platform-icon--qq">
                  <Icon name="users" size="lg" :stroke-width="1.8" />
                </span>
                <span class="contact-card-copy">
                  <span class="contact-card-label">{{ t('contactPage.qq.title') }}</span>
                  <span class="contact-card-value contact-card-value--mono">
                    {{ hasQqGroup ? contactSettings.qq.groupNumber : t('contactPage.pending') }}
                  </span>
                </span>
              </span>
              <span class="contact-card-action">
                <span>{{ t('contactPage.qq.copy') }}</span>
                <span class="contact-card-action-icon">
                  <Icon :name="copied ? 'check' : 'copy'" size="sm" :stroke-width="2" />
                </span>
              </span>
            </button>

            <a
              v-if="telegramUrl"
              data-testid="telegram-contact-card"
              :href="telegramUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="contact-action-card contact-action-card--telegram"
              :aria-label="t('contactPage.telegram.open')"
            >
              <TelegramCardContent />
            </a>
            <div
              v-else
              data-testid="telegram-contact-card"
              class="contact-action-card contact-action-card--telegram contact-action-card--disabled"
              aria-disabled="true"
            >
              <TelegramCardContent />
            </div>
          </section>

          <div class="contact-scan-divider">
            <span class="contact-scan-line"></span>
            <span class="contact-scan-label">
              <span class="contact-scan-dot contact-scan-dot--teal" aria-hidden="true"></span>
              {{ t('contactPage.scanToJoin') }}
              <span class="contact-scan-dot contact-scan-dot--yellow" aria-hidden="true"></span>
            </span>
            <span class="contact-scan-line"></span>
          </div>

          <section
            class="contact-qr-grid"
            :aria-label="t('contactPage.qrAria')"
          >
            <article class="contact-qr-card contact-qr-card--qq">
              <div class="contact-qr-platform">
                <span class="contact-platform-icon contact-platform-icon--qq contact-platform-icon--small">
                  <Icon name="users" size="sm" :stroke-width="1.9" />
                </span>
                <span>{{ t('contactPage.qq.title') }}</span>
              </div>
              <div class="contact-qr-frame">
                <img
                  v-if="hasQqQrCode"
                  :src="qqQrImageUrl"
                  :alt="t('contactPage.qq.qrAlt')"
                  class="contact-qr-image"
                  data-testid="qq-qr-image"
                />
                <QrPlaceholder v-else platform="qq" />
              </div>
              <p class="contact-qr-description">{{ t('contactPage.qq.scan') }}</p>
            </article>

            <article class="contact-qr-card contact-qr-card--telegram">
              <div class="contact-qr-platform">
                <span class="contact-platform-icon contact-platform-icon--telegram contact-platform-icon--small">
                  <TelegramLogo />
                </span>
                <span>{{ t('contactPage.telegram.title') }}</span>
              </div>
              <div class="contact-qr-frame">
                <img
                  v-if="hasTelegramQrCode"
                  :src="telegramQrImageUrl"
                  :alt="t('contactPage.telegram.qrAlt')"
                  class="contact-qr-image"
                  data-testid="telegram-qr-image"
                />
                <QrPlaceholder v-else platform="telegram" />
              </div>
              <p class="contact-qr-description">{{ t('contactPage.telegram.scan') }}</p>
            </article>
          </section>

          <footer class="contact-footer">
            <span class="contact-footer-dots" aria-hidden="true">
              <span></span><span></span><span></span>
            </span>
            <p>{{ t('contactPage.footer') }}</p>
          </footer>
        </section>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getContactPageSettings } from '@/api/auth'
import CraftSparks from '@/components/craft/CraftSparks.vue'
import JournalBannerSticker from '@/components/craft/JournalBannerSticker.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { contactPageConfig, type ContactPageConfig } from '@/config/contactPage'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import './contact-page.css'

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()
const appStore = useAppStore()
const contactSettings = ref<ContactPageConfig>({
  qq: { ...contactPageConfig.qq },
  telegram: { ...contactPageConfig.telegram },
})

const searchParams = typeof window === 'undefined'
  ? new URLSearchParams()
  : new URLSearchParams(window.location.search)

const isEmbedded = searchParams.get('ui_mode') === 'embedded'

const siteName = computed(
  () => appStore.cachedPublicSettings?.site_name
    || (appStore.siteName !== 'Sub2API' ? appStore.siteName : '爱白嫖公益站')
)
const siteLogo = computed(
  () => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }) || '/brand-icon.png'
)

const hasQqGroup = computed(() => Boolean(contactSettings.value.qq.groupNumber.trim()))
const qqQrImageUrl = computed(() => sanitizeUrl(contactSettings.value.qq.qrImageUrl, {
  allowRelative: true,
  allowDataUrl: true,
}))
const telegramQrImageUrl = computed(() => sanitizeUrl(contactSettings.value.telegram.qrImageUrl, {
  allowRelative: true,
  allowDataUrl: true,
}))
const hasQqQrCode = computed(() => Boolean(qqQrImageUrl.value))
const hasTelegramQrCode = computed(() => Boolean(telegramQrImageUrl.value))

const telegramUrl = computed(() => {
  const value = contactSettings.value.telegram.channelUrl.trim()
  if (!value) return ''

  try {
    const parsed = new URL(value)
    const hostname = parsed.hostname.toLowerCase().replace(/^www\./, '')
    if (parsed.protocol !== 'https:' || !['t.me', 'telegram.me'].includes(hostname)) {
      return ''
    }
    return parsed.toString()
  } catch {
    return ''
  }
})

const telegramLabel = computed(() => {
  const configuredName = contactSettings.value.telegram.channelName.trim()
  if (configuredName) return configuredName
  if (!telegramUrl.value) return t('contactPage.pending')

  try {
    const path = new URL(telegramUrl.value).pathname.replace(/^\/+|\/+$/g, '')
    return path ? `@${path.split('/')[0]}` : t('contactPage.telegram.title')
  } catch {
    return t('contactPage.telegram.title')
  }
})

async function copyQqGroupNumber() {
  if (!hasQqGroup.value) return
  await copyToClipboard(contactSettings.value.qq.groupNumber.trim(), t('contactPage.qq.copied'))
}

onMounted(async () => {
  if (isEmbedded && typeof document !== 'undefined') {
    document.documentElement.classList.add('contact-embedded-host')
    document.body.classList.add('contact-embedded-host')
  }

  try {
    const runtimeSettings = await getContactPageSettings()
    contactSettings.value = {
      qq: {
        groupNumber: runtimeSettings.qq_group_number ?? '',
        qrImageUrl: runtimeSettings.qq_qr_image ?? '',
      },
      telegram: {
        channelName: runtimeSettings.telegram_name ?? '',
        channelUrl: runtimeSettings.telegram_url ?? '',
        qrImageUrl: runtimeSettings.telegram_qr_image ?? '',
      },
    }
  } catch {
    // Keep the packaged defaults when the backend is unavailable during preview.
  }
})

onUnmounted(() => {
  if (isEmbedded && typeof document !== 'undefined') {
    document.documentElement.classList.remove('contact-embedded-host')
    document.body.classList.remove('contact-embedded-host')
  }
})

const TelegramLogo = defineComponent({
  name: 'TelegramLogo',
  setup() {
    return () => h('svg', {
      viewBox: '0 0 24 24',
      fill: 'currentColor',
      class: 'contact-telegram-logo',
      'aria-hidden': 'true',
    }, [
      h('path', {
        d: 'M21.8 4.6 18.9 19c-.2 1-1 1.3-1.8.8l-4.4-3.2-2.1 2c-.2.2-.4.4-.9.4l.3-4.5 8.2-7.4c.4-.3-.1-.5-.5-.2L7.6 13.3l-4.4-1.4c-1-.3-1-1 .2-1.5L20.5 3.8c.8-.3 1.5.2 1.3.8Z',
      }),
    ])
  },
})

const TelegramCardContent = defineComponent({
  name: 'TelegramCardContent',
  setup() {
    return () => h('div', { class: 'contact-card-content' }, [
      h('span', { class: 'contact-card-main' }, [
        h('span', { class: 'contact-platform-icon contact-platform-icon--telegram' }, [h(TelegramLogo)]),
        h('span', { class: 'contact-card-copy' }, [
          h('span', { class: 'contact-card-label' }, t('contactPage.telegram.title')),
          h('span', { class: 'contact-card-value' }, telegramLabel.value),
        ]),
      ]),
      h('span', { class: 'contact-card-action' }, [
        h('span', t('contactPage.telegram.open')),
        h('span', { class: 'contact-card-action-icon' }, [
          h(Icon, { name: 'externalLink', size: 'sm', strokeWidth: 2 }),
        ]),
      ]),
    ])
  },
})

const QrPlaceholder = defineComponent({
  name: 'QrPlaceholder',
  props: {
    platform: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    return () => h('div', {
      class: 'contact-qr-placeholder',
      'data-testid': `${props.platform}-qr-placeholder`,
    }, [
      h('span', {
        class: props.platform === 'telegram'
          ? 'contact-placeholder-icon contact-placeholder-icon--telegram'
          : 'contact-placeholder-icon contact-placeholder-icon--qq',
      }, [props.platform === 'telegram'
        ? h(TelegramLogo)
        : h(Icon, { name: 'users', size: 'lg', strokeWidth: 1.8 })]),
      h('span', { class: 'contact-placeholder-title' }, t('contactPage.qrPending')),
      h('span', { class: 'contact-placeholder-description' }, t('contactPage.qrPendingHint')),
    ])
  },
})
</script>
