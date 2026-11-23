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

<style>
html.contact-embedded-host,
body.contact-embedded-host {
  margin: 0;
  background: transparent !important;
  min-height: 0 !important;
}

.contact-page {
  --contact-bg: #fdf6e9;
  --contact-surface: #fffdf8;
  --contact-panel: #fff9ee;
  --contact-panel-strong: #fff2da;
  --contact-ink: #27221d;
  --contact-ink-soft: #4e4336;
  --contact-muted: #7b6b57;
  --contact-faint: #9a8872;
  --contact-border: #e3d2b6;
  --contact-divider: #e8dac0;
  --contact-brand: #40a7ef;
  --contact-brand-strong: #2682c8;
  --contact-brand-soft: #e6f5fe;
  --contact-peach: #ffad87;
  --contact-yellow: #ffdf76;
  --contact-telegram: #2a9be0;
  --contact-grid: rgba(143, 108, 65, 0.055);
  --contact-shadow: 0 10px 0 rgba(143, 108, 65, 0.08), 0 24px 56px rgba(108, 78, 42, 0.14);
  position: relative;
  display: grid;
  height: 100svh;
  min-height: 100svh;
  place-items: center;
  overflow: hidden;
  color: var(--contact-ink);
  font-family: 'PingFang SC', 'Hiragino Maru Gothic ProN', 'Microsoft YaHei UI', 'Microsoft YaHei', system-ui, sans-serif;
  background-color: var(--contact-bg);
  background-image:
    linear-gradient(var(--contact-grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--contact-grid) 1px, transparent 1px),
    linear-gradient(135deg, rgba(255, 250, 240, 0.88), rgba(253, 244, 226, 0.82) 52%, rgba(233, 246, 255, 0.78)),
    url('/sticker/cream.webp');
  background-size: 72px 72px, 72px 72px, 100% 100%, 160px 160px;
}

.contact-page--embedded {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  justify-content: stretch;
  background: transparent;
  overflow-x: hidden;
  overflow-y: auto;
}

.contact-page--embedded .contact-page-shape,
.contact-page--embedded .contact-page-route {
  display: none;
}

.contact-page-inner {
  position: relative;
  z-index: 2;
  width: 100%;
  padding: 16px clamp(18px, 3vw, 44px);
}

.contact-page-inner--embedded {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 2px 6px 12px;
}

.contact-page-shape,
.contact-page-route {
  position: absolute;
  pointer-events: none;
}

.contact-page-shape--peach {
  bottom: -118px;
  left: -78px;
  width: 300px;
  height: 300px;
  border-radius: 50%;
  background: rgba(255, 173, 135, 0.2);
}

.contact-page-shape--aqua {
  top: -155px;
  right: -105px;
  width: 390px;
  height: 390px;
  border: 1.5px dashed rgba(124, 199, 249, 0.45);
  border-radius: 50%;
}

.contact-page-route {
  right: 3.5vw;
  bottom: 4vh;
  width: 170px;
  height: 110px;
  opacity: 0.78;
}

.contact-page-route-line {
  position: absolute;
  inset: 18px 12px 12px 8px;
  border: 2px dashed rgba(143, 108, 65, 0.25);
  border-radius: 50%;
  transform: rotate(-14deg);
}

.contact-page-route-dot {
  position: absolute;
  z-index: 1;
  display: block;
  border: 5px solid rgba(255, 253, 248, 0.9);
  border-radius: 50%;
  box-shadow: 0 6px 14px rgba(108, 78, 42, 0.15);
}

.contact-page-route-dot--teal {
  top: 36px;
  left: 2px;
  width: 38px;
  height: 38px;
  background: #7cc7f9;
}

.contact-page-route-dot--yellow {
  top: 4px;
  right: 13px;
  width: 28px;
  height: 28px;
  background: var(--contact-yellow);
}

.contact-page-route-dot--peach {
  right: 2px;
  bottom: 4px;
  width: 25px;
  height: 25px;
  background: var(--contact-peach);
}

.contact-shell {
  position: relative;
  isolation: isolate;
  width: min(1040px, 100%);
  margin-inline: auto;
  padding: 20px 36px 22px;
  overflow: hidden;
  border: 1.5px solid var(--contact-border);
  border-radius: 28px;
  outline: 1.5px dashed rgba(213, 190, 154, 0.65);
  outline-offset: -8px;
  background: var(--contact-surface);
  box-shadow: var(--contact-shadow);
}

.contact-shell--embedded {
  width: 100%;
  max-width: none;
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 24px clamp(28px, 3.5vw, 52px) 22px;
  box-shadow: 0 6px 0 rgba(143, 108, 65, 0.08), 0 10px 24px rgba(108, 78, 42, 0.1);
}

.contact-shell--embedded .contact-hero {
  max-width: 760px;
  margin: 12px auto 18px;
}

.contact-shell--embedded .contact-hero h1 {
  font-size: 36px;
}

.contact-shell--embedded .contact-subtitle {
  font-size: 14px;
}

.contact-shell--embedded .contact-action-grid,
.contact-shell--embedded .contact-qr-grid {
  gap: 18px;
}

.contact-shell--embedded .contact-action-card {
  min-height: 122px;
  padding: 18px 22px 15px;
}

.contact-shell--embedded .contact-card-value {
  font-size: 20px;
}

.contact-shell--embedded .contact-qr-card {
  padding: 16px;
}

.contact-shell--embedded .contact-qr-frame {
  width: min(100%, 340px);
  height: clamp(250px, 34vh, 330px);
}

.contact-shell-blob {
  position: absolute;
  z-index: -1;
  pointer-events: none;
}

.contact-shell-blob--aqua {
  top: -165px;
  right: -96px;
  width: 410px;
  height: 340px;
  border-radius: 46% 54% 64% 36%;
  background: rgba(148, 212, 252, 0.22);
  transform: rotate(16deg);
}

.contact-shell-blob--peach {
  bottom: -122px;
  left: -96px;
  width: 230px;
  height: 230px;
  border-radius: 50%;
  background: rgba(255, 173, 135, 0.18);
}

.contact-topbar,
.contact-hero,
.contact-action-grid,
.contact-scan-divider,
.contact-qr-grid,
.contact-footer {
  position: relative;
  z-index: 1;
}

.contact-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.contact-brand {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 11px;
}

.contact-brand-logo {
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  border: 2px solid #fffdf8;
  border-radius: 13px;
  object-fit: cover;
  box-shadow: 0 4px 10px rgba(108, 78, 42, 0.18);
  transform: rotate(-3deg);
}

.contact-brand-name {
  overflow: hidden;
  color: var(--contact-ink);
  font-size: 16px;
  font-weight: 850;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.contact-status {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 7px;
  padding: 6px 11px;
  border: 1px solid #dcc39b;
  border-radius: 999px;
  color: #7a4e1e;
  background: #fff6e2;
  font-size: 11px;
  font-weight: 800;
}

.contact-status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #30b57a;
  box-shadow: 0 0 0 4px rgba(48, 181, 122, 0.16);
}

.contact-hero {
  max-width: 680px;
  margin: 10px auto 14px;
  text-align: center;
}

.contact-hero-mark {
  display: inline-flex;
  width: auto;
  height: auto;
  margin: 0 auto 4px;
  align-items: center;
  justify-content: center;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.contact-kicker {
  margin: 0;
  color: var(--contact-brand-strong);
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.contact-hero-title-row {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.contact-hero h1 {
  margin: 4px 0 4px;
  color: var(--contact-ink);
  font-size: 32px;
  font-weight: 900;
  letter-spacing: -0.03em;
  line-height: 1.12;
}

.contact-subtitle {
  max-width: 620px;
  margin: 0 auto;
  color: var(--contact-muted);
  font-size: 13px;
  line-height: 1.45;
}

.contact-action-grid,
.contact-qr-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.contact-action-card {
  --card-accent: var(--contact-brand);
  display: flex;
  min-width: 0;
  min-height: 110px;
  flex-direction: column;
  justify-content: space-between;
  padding: 15px 18px 13px;
  overflow: hidden;
  border: 1.5px solid var(--contact-border);
  border-top: 4px solid var(--card-accent);
  border-radius: 19px;
  color: inherit;
  text-align: left;
  text-decoration: none;
  appearance: none;
  background: #fffaf0;
  box-shadow: 0 5px 0 rgba(143, 108, 65, 0.08);
  font: inherit;
  transition: transform 220ms cubic-bezier(0.23, 1, 0.32, 1), border-color 220ms ease, box-shadow 220ms ease;
}

button.contact-action-card:not(:disabled),
a.contact-action-card {
  cursor: pointer;
}

.contact-action-card--telegram {
  --card-accent: var(--contact-telegram);
}

.contact-action-card--disabled,
.contact-action-card:disabled {
  cursor: not-allowed;
  opacity: 0.72;
}

@media (hover: hover) and (pointer: fine) {
  .contact-action-card:not(.contact-action-card--disabled):not(:disabled):hover {
    transform: translateY(-3px);
    border-right-color: #d5b98e;
    border-bottom-color: #d5b98e;
    border-left-color: #d5b98e;
    box-shadow: 0 8px 0 rgba(143, 108, 65, 0.12);
  }
}

.contact-action-card:focus-visible {
  outline: 3px solid rgba(64, 167, 239, 0.28);
  outline-offset: 3px;
}

.contact-card-content {
  display: contents;
}

.contact-card-main {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 11px;
}

.contact-platform-icon,
.contact-placeholder-icon {
  display: grid;
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  place-items: center;
  border: 1.5px solid transparent;
  border-radius: 13px;
}

.contact-platform-icon--qq,
.contact-placeholder-icon--qq {
  border-color: #cde3f3;
  color: var(--contact-brand-strong);
  background: var(--contact-brand-soft);
}

.contact-platform-icon--telegram,
.contact-placeholder-icon--telegram {
  border-color: rgba(42, 155, 224, 0.22);
  color: var(--contact-telegram);
  background: rgba(148, 212, 252, 0.2);
}

.contact-platform-icon--small {
  width: 32px;
  height: 32px;
  flex-basis: 32px;
  border-radius: 11px;
}

.contact-telegram-logo {
  width: 24px;
  height: 24px;
}

.contact-platform-icon--small .contact-telegram-logo {
  width: 17px;
  height: 17px;
}

.contact-card-copy {
  display: block;
  min-width: 0;
  flex: 1;
}

.contact-card-label {
  display: block;
  color: var(--contact-ink-soft);
  font-size: 12px;
  font-weight: 800;
}

.contact-card-value {
  display: block;
  margin-top: 4px;
  overflow: hidden;
  color: var(--contact-ink);
  font-size: 18px;
  font-weight: 900;
  letter-spacing: -0.01em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.contact-card-value--mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  letter-spacing: 0.03em;
}

.contact-card-action {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 9px;
  margin-top: 9px;
  border-top: 1.5px dashed var(--contact-divider);
  color: var(--contact-muted);
  font-size: 11px;
  font-weight: 700;
}

.contact-card-action-icon {
  display: grid;
  width: 27px;
  height: 27px;
  flex: 0 0 27px;
  place-items: center;
  border: 1px solid var(--contact-border);
  border-radius: 10px;
  color: var(--card-accent);
  background: var(--contact-panel-strong);
  transition: transform 220ms cubic-bezier(0.23, 1, 0.32, 1), background-color 220ms ease;
}

.contact-action-card:hover .contact-card-action-icon {
  transform: translateX(2px);
}

.contact-scan-divider {
  display: flex;
  align-items: center;
  gap: 14px;
  margin: 14px 0 10px;
}

.contact-scan-line {
  height: 0;
  flex: 1;
  border-top: 1.5px dashed #dec8a4;
  background: transparent;
}

.contact-scan-label {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  color: var(--contact-muted);
  font-size: 12px;
  font-weight: 800;
}

.contact-scan-dot {
  width: 8px;
  height: 8px;
  border: 2px solid var(--contact-surface);
  border-radius: 50%;
  box-shadow: 0 0 0 1px var(--contact-border);
}

.contact-scan-dot--teal {
  background: #7cc7f9;
}

.contact-scan-dot--yellow {
  background: var(--contact-yellow);
}

.contact-qr-card {
  padding: 12px;
  border: 1.5px solid var(--contact-border);
  border-radius: 18px;
  background: #fffaf0;
  box-shadow: 0 5px 0 rgba(143, 108, 65, 0.08);
  transition: transform 220ms cubic-bezier(0.23, 1, 0.32, 1), border-color 220ms ease, box-shadow 220ms ease;
}

@media (hover: hover) and (pointer: fine) {
  .contact-qr-card:hover {
    transform: translateY(-3px);
    border-color: #d5b98e;
    box-shadow: 0 8px 0 rgba(143, 108, 65, 0.12);
  }
}

.contact-qr-platform {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
  color: var(--contact-ink-soft);
  font-size: 12px;
  font-weight: 800;
}

.contact-qr-frame {
  display: flex;
  width: min(100%, 310px);
  height: clamp(240px, 30vh, 300px);
  align-items: center;
  justify-content: center;
  padding: 8px;
  margin-inline: auto;
  overflow: hidden;
  border: 1.5px dashed #dcc6a4;
  border-radius: 18px;
  background: #fff;
}

.contact-qr-image {
  display: block;
  width: 100%;
  height: 100%;
  border-radius: 14px;
  background: #fff;
  object-fit: contain;
  object-position: center;
}

.contact-qr-placeholder {
  display: flex;
  width: 100%;
  height: 100%;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  padding: 20px;
  border: 1.5px dashed #d5bc93;
  border-radius: 15px;
  text-align: center;
  background: #fff4de;
}

.contact-placeholder-icon {
  width: 45px;
  height: 45px;
  flex-basis: 45px;
  border-radius: 15px;
}

.contact-placeholder-title {
  margin-top: 12px;
  color: var(--contact-ink-soft);
  font-size: 13px;
  font-weight: 800;
}

.contact-placeholder-description {
  margin-top: 5px;
  color: var(--contact-faint);
  font-size: 11px;
  line-height: 1.55;
}

.contact-qr-description {
  margin: 8px 0 0;
  color: var(--contact-muted);
  text-align: center;
  font-size: 11px;
  line-height: 1.35;
}

.contact-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin-top: 12px;
  color: var(--contact-faint);
  font-size: 11px;
}

.contact-footer p {
  margin: 0;
}

.contact-footer-dots {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.contact-footer-dots span {
  display: block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.contact-footer-dots span:nth-child(1) {
  background: #7cc7f9;
}

.contact-footer-dots span:nth-child(2) {
  background: var(--contact-yellow);
}

.contact-footer-dots span:nth-child(3) {
  background: var(--contact-peach);
}

@media (min-width: 761px) and (max-height: 820px) {
  .contact-page-inner {
    padding-top: 8px;
    padding-bottom: 8px;
  }

  .contact-page-inner--embedded {
    padding: 2px 6px 10px;
  }

  .contact-shell {
    padding-top: 14px;
    padding-bottom: 14px;
  }

  .contact-shell--embedded {
    padding: 14px clamp(20px, 3vw, 36px) 14px;
  }

  .contact-hero,
  .contact-shell--embedded .contact-hero {
    margin-top: 6px;
    margin-bottom: 10px;
  }

  .contact-shell--embedded .contact-hero h1 {
    font-size: 30px;
  }

  .contact-hero-mark {
    margin-bottom: 2px;
    transform: scale(0.88);
  }

  .contact-action-card {
    min-height: 100px;
    padding-top: 12px;
    padding-bottom: 11px;
  }

  .contact-shell--embedded .contact-action-card {
    min-height: 102px;
    padding: 13px 18px 11px;
  }

  .contact-scan-divider {
    margin-top: 10px;
    margin-bottom: 8px;
  }

  .contact-qr-frame,
  .contact-shell--embedded .contact-qr-frame {
    height: clamp(205px, 28vh, 220px);
  }

  .contact-footer {
    margin-top: 8px;
  }
}

@media (max-width: 760px) {
  .contact-page,
  .contact-page--embedded {
    display: block;
    height: auto;
    overflow: visible;
  }

  .contact-page-inner {
    padding: 16px 12px;
  }

  .contact-page-inner--embedded {
    padding: 4px 4px 12px;
  }

  .contact-page-shape,
  .contact-page-route {
    display: none;
  }

  .contact-shell {
    padding: 22px 17px 26px;
    border-radius: 27px;
  }

  .contact-shell-blob--aqua {
    top: -205px;
    right: -190px;
  }

  .contact-brand-logo {
    width: 42px;
    height: 42px;
    flex-basis: 42px;
    border-radius: 14px;
  }

  .contact-brand-name {
    max-width: 170px;
    font-size: 15px;
  }

  .contact-status {
    padding: 6px 9px;
    font-size: 10px;
  }

  .contact-hero {
    margin: 32px auto 26px;
  }

  .contact-kicker {
    font-size: 10px;
  }

  .contact-hero h1 {
    font-size: 32px;
  }

  .contact-subtitle {
    max-width: 330px;
    font-size: 13px;
    line-height: 1.7;
  }

  .contact-action-grid,
  .contact-qr-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .contact-action-card {
    min-height: 158px;
    padding: 21px 20px 17px;
  }

  .contact-scan-divider {
    margin-top: 27px;
  }

  .contact-qr-card {
    padding: 17px;
  }
}

@media (max-width: 390px) {
  .contact-brand-name {
    max-width: 145px;
  }

  .contact-status {
    gap: 5px;
  }

  .contact-status-dot {
    width: 6px;
    height: 6px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .contact-action-card,
  .contact-action-card *,
  .contact-qr-card,
  .contact-qr-card * {
    transition: none !important;
  }
}
</style>
