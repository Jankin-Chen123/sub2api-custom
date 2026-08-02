<template>
  <div :class="{ dark: isDarkTheme }" data-testid="contact-theme-root">
    <main class="contact-page">
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
              <Icon name="chatBubble" size="xl" :stroke-width="1.8" />
            </div>
            <p class="contact-kicker">{{ t('contactPage.kicker') }}</p>
            <h1>{{ t('contactPage.title') }}</h1>
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
import { computed, defineComponent, h, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getContactPageSettings } from '@/api/auth'
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

const requestedTheme = searchParams.get('theme')
const isDarkTheme = requestedTheme === 'dark'
  || (requestedTheme !== 'light' && typeof document !== 'undefined' && document.documentElement.classList.contains('dark'))
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
.contact-page {
  --contact-bg: #f6fbfa;
  --contact-surface: rgba(255, 255, 255, 0.97);
  --contact-panel: #f9fcfc;
  --contact-panel-strong: #f2faf9;
  --contact-ink: #102d32;
  --contact-ink-soft: #355a60;
  --contact-muted: #6d898e;
  --contact-faint: #94aaae;
  --contact-border: #d9e9eb;
  --contact-divider: #e4eeee;
  --contact-brand: #12a49b;
  --contact-brand-strong: #087f83;
  --contact-brand-soft: #dff5f6;
  --contact-peach: #ffb49f;
  --contact-yellow: #ffd45a;
  --contact-telegram: #229ed9;
  --contact-grid: rgba(18, 164, 155, 0.045);
  --contact-shadow: 0 26px 75px rgba(42, 100, 106, 0.15);
  position: relative;
  display: grid;
  min-height: 100svh;
  place-items: center;
  overflow: hidden;
  color: var(--contact-ink);
  background-color: var(--contact-bg);
  background-image:
    linear-gradient(var(--contact-grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--contact-grid) 1px, transparent 1px),
    linear-gradient(135deg, rgba(248, 255, 255, 0.92), rgba(237, 249, 247, 0.9) 48%, rgba(255, 248, 238, 0.88));
  background-size: 72px 72px, 72px 72px, 100% 100%;
}

.dark .contact-page {
  --contact-bg: #07181b;
  --contact-surface: rgba(10, 31, 35, 0.97);
  --contact-panel: #10292e;
  --contact-panel-strong: #123338;
  --contact-ink: #effffd;
  --contact-ink-soft: #c6e2e1;
  --contact-muted: #98b7b8;
  --contact-faint: #789597;
  --contact-border: rgba(125, 217, 213, 0.2);
  --contact-divider: rgba(125, 217, 213, 0.13);
  --contact-brand: #55d7cc;
  --contact-brand-strong: #78e2da;
  --contact-brand-soft: #103a3d;
  --contact-grid: rgba(85, 215, 204, 0.045);
  --contact-shadow: 0 30px 85px rgba(0, 0, 0, 0.3);
  background-image:
    linear-gradient(var(--contact-grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--contact-grid) 1px, transparent 1px),
    linear-gradient(135deg, #07181b, #0a2326 48%, #151e1e);
}

.contact-page-inner {
  position: relative;
  z-index: 2;
  width: 100%;
  padding: 38px 5vw;
}

.contact-page-inner--embedded {
  padding-top: 24px;
  padding-bottom: 24px;
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
  background: rgba(255, 180, 159, 0.2);
}

.contact-page-shape--aqua {
  top: -155px;
  right: -105px;
  width: 390px;
  height: 390px;
  border: 1px solid rgba(18, 164, 155, 0.12);
  border-radius: 50%;
}

.contact-page-route {
  right: 3.5vw;
  bottom: 4vh;
  width: 170px;
  height: 110px;
  opacity: 0.72;
}

.contact-page-route-line {
  position: absolute;
  inset: 18px 12px 12px 8px;
  border: 2px dashed rgba(17, 150, 142, 0.28);
  border-radius: 50%;
  transform: rotate(-14deg);
}

.contact-page-route-dot {
  position: absolute;
  z-index: 1;
  display: block;
  border: 5px solid rgba(255, 255, 255, 0.78);
  border-radius: 50%;
  box-shadow: 0 8px 18px rgba(24, 120, 119, 0.13);
}

.contact-page-route-dot--teal {
  top: 36px;
  left: 2px;
  width: 38px;
  height: 38px;
  background: #7bd9d5;
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
  padding: 30px 54px 38px;
  overflow: hidden;
  border: 1px solid var(--contact-border);
  border-radius: 34px;
  background: var(--contact-surface);
  box-shadow: var(--contact-shadow);
}

.contact-shell--embedded {
  box-shadow: 0 18px 52px rgba(42, 100, 106, 0.11);
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
  background: rgba(111, 215, 217, 0.16);
  transform: rotate(16deg);
}

.contact-shell-blob--peach {
  bottom: -122px;
  left: -96px;
  width: 230px;
  height: 230px;
  border-radius: 50%;
  background: rgba(255, 180, 159, 0.13);
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
  width: 46px;
  height: 46px;
  flex: 0 0 46px;
  border-radius: 15px;
  object-fit: cover;
  box-shadow: 0 9px 22px rgba(18, 139, 135, 0.2);
  animation: contact-logo-float 4.8s ease-in-out infinite;
}

.contact-brand-name {
  overflow: hidden;
  color: var(--contact-ink);
  font-size: 18px;
  font-weight: 850;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.contact-status {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 7px;
  padding: 7px 12px;
  border: 1px solid rgba(18, 164, 155, 0.2);
  border-radius: 999px;
  color: var(--contact-brand-strong);
  background: rgba(223, 245, 246, 0.72);
  font-size: 12px;
  font-weight: 800;
}

.dark .contact-status {
  background: rgba(18, 164, 155, 0.12);
}

.contact-status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #25b99f;
  box-shadow: 0 0 0 4px rgba(37, 185, 159, 0.12);
}

.contact-hero {
  max-width: 680px;
  margin: 38px auto 30px;
  text-align: center;
}

.contact-hero-mark {
  display: grid;
  width: 58px;
  height: 58px;
  margin: 0 auto 15px;
  place-items: center;
  border: 1px solid rgba(18, 164, 155, 0.14);
  border-radius: 19px;
  color: var(--contact-brand-strong);
  background: var(--contact-brand-soft);
  box-shadow: 0 10px 24px rgba(18, 139, 135, 0.1);
}

.contact-kicker {
  margin: 0;
  color: var(--contact-brand-strong);
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.contact-hero h1 {
  margin: 9px 0 8px;
  color: var(--contact-ink);
  font-size: 42px;
  font-weight: 850;
  letter-spacing: -0.03em;
  line-height: 1.12;
}

.contact-subtitle {
  max-width: 620px;
  margin: 0 auto;
  color: var(--contact-muted);
  font-size: 15px;
  line-height: 1.75;
}

.contact-action-grid,
.contact-qr-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.contact-action-card {
  --card-accent: var(--contact-brand);
  display: flex;
  min-width: 0;
  min-height: 166px;
  flex-direction: column;
  justify-content: space-between;
  padding: 23px 24px 19px;
  overflow: hidden;
  border: 1px solid var(--contact-border);
  border-top: 3px solid var(--card-accent);
  border-radius: 19px;
  color: inherit;
  text-align: left;
  text-decoration: none;
  appearance: none;
  background: var(--contact-panel);
  box-shadow: 0 10px 26px rgba(42, 100, 106, 0.055);
  font: inherit;
  transition: transform 220ms ease, border-color 220ms ease, box-shadow 220ms ease;
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

.contact-action-card:not(.contact-action-card--disabled):not(:disabled):hover {
  border-right-color: rgba(18, 164, 155, 0.32);
  border-bottom-color: rgba(18, 164, 155, 0.32);
  border-left-color: rgba(18, 164, 155, 0.32);
  box-shadow: 0 16px 34px rgba(42, 100, 106, 0.11);
  animation: contact-card-hover-float 1.8s ease-in-out infinite;
}

.contact-action-card:focus-visible {
  outline: 3px solid rgba(18, 164, 155, 0.2);
  outline-offset: 3px;
}

.contact-card-content {
  display: contents;
}

.contact-card-main {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 14px;
}

.contact-platform-icon,
.contact-placeholder-icon {
  display: grid;
  width: 48px;
  height: 48px;
  flex: 0 0 48px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 16px;
}

.contact-platform-icon--qq,
.contact-placeholder-icon--qq {
  border-color: rgba(18, 164, 155, 0.12);
  color: var(--contact-brand-strong);
  background: var(--contact-brand-soft);
}

.contact-platform-icon--telegram,
.contact-placeholder-icon--telegram {
  border-color: rgba(34, 158, 217, 0.13);
  color: var(--contact-telegram);
  background: rgba(34, 158, 217, 0.09);
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
  font-size: 13px;
  font-weight: 780;
}

.contact-card-value {
  display: block;
  margin-top: 6px;
  overflow: hidden;
  color: var(--contact-ink);
  font-size: 21px;
  font-weight: 850;
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
  padding-top: 15px;
  margin-top: 18px;
  border-top: 1px solid var(--contact-divider);
  color: var(--contact-muted);
  font-size: 12px;
  font-weight: 700;
}

.contact-card-action-icon {
  display: grid;
  width: 31px;
  height: 31px;
  flex: 0 0 31px;
  place-items: center;
  border-radius: 10px;
  color: var(--card-accent);
  background: var(--contact-panel-strong);
  transition: transform 220ms ease, background-color 220ms ease;
}

.contact-action-card:hover .contact-card-action-icon {
  transform: translateX(2px);
}

.contact-scan-divider {
  display: flex;
  align-items: center;
  gap: 14px;
  margin: 31px 0 18px;
}

.contact-scan-line {
  height: 1px;
  flex: 1;
  background: var(--contact-divider);
}

.contact-scan-label {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  color: var(--contact-muted);
  font-size: 13px;
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
  background: #7bd9d5;
}

.contact-scan-dot--yellow {
  background: var(--contact-yellow);
}

.contact-qr-card {
  padding: 19px 19px 17px;
  border: 1px solid var(--contact-border);
  border-radius: 22px;
  background: var(--contact-panel);
  box-shadow: 0 10px 26px rgba(42, 100, 106, 0.045);
  transition: border-color 220ms ease, box-shadow 220ms ease;
}

.contact-qr-card:hover {
  border-color: rgba(18, 164, 155, 0.32);
  box-shadow: 0 16px 34px rgba(42, 100, 106, 0.11);
  animation: contact-card-hover-float 1.8s ease-in-out infinite;
}

.contact-qr-platform {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 15px;
  color: var(--contact-ink-soft);
  font-size: 13px;
  font-weight: 800;
}

.contact-qr-frame {
  display: flex;
  width: min(100%, 360px);
  height: clamp(390px, 36vw, 450px);
  align-items: center;
  justify-content: center;
  padding: 10px;
  margin-inline: auto;
  overflow: hidden;
  border: 1px solid var(--contact-border);
  border-radius: 22px;
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
  border: 1px dashed rgba(18, 164, 155, 0.22);
  border-radius: 15px;
  text-align: center;
  background: var(--contact-panel-strong);
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
  margin: 13px 0 0;
  color: var(--contact-muted);
  text-align: center;
  font-size: 12px;
  line-height: 1.6;
}

.contact-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin-top: 24px;
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
  background: #7bd9d5;
}

.contact-footer-dots span:nth-child(2) {
  background: var(--contact-yellow);
}

.contact-footer-dots span:nth-child(3) {
  background: var(--contact-peach);
}

@keyframes contact-logo-float {
  50% {
    transform: translateY(-5px);
  }
}

@keyframes contact-card-hover-float {
  0%,
  100% {
    transform: translateY(0);
  }

  50% {
    transform: translateY(-6px);
  }
}

@media (max-width: 760px) {
  .contact-page {
    display: block;
    overflow: visible;
  }

  .contact-page-inner,
  .contact-page-inner--embedded {
    padding: 16px 12px;
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

  .contact-hero-mark {
    width: 54px;
    height: 54px;
    border-radius: 18px;
  }

  .contact-kicker {
    font-size: 10px;
  }

  .contact-hero h1 {
    font-size: 34px;
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
  .contact-brand-logo {
    animation: none;
  }

  .contact-action-card:not(.contact-action-card--disabled):not(:disabled):hover {
    animation: none;
  }

  .contact-qr-card:hover {
    animation: none;
  }

  .contact-action-card,
  .contact-action-card *,
  .contact-qr-card,
  .contact-qr-card * {
    transition: none !important;
  }
}
</style>
