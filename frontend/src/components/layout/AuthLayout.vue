<template>
  <div class="auth-page" :class="authMotionClass">
    <div class="auth-shell">
      <AuthBrandPanel :site-name="siteName" :site-logo="siteLogo || '/brand-icon.png'" />

      <main class="auth-form-panel">
        <div class="auth-binder-rings" aria-hidden="true">
          <span v-for="ring in 5" :key="ring" class="auth-ring-unit">
            <span class="auth-hole-l" />
            <span class="auth-ring-arc" />
            <span class="auth-hole-r" />
          </span>
        </div>

        <RouterLink to="/" class="auth-home-link">
          <span aria-hidden="true">&larr;</span>
          <span>{{ t('auth.backHome') }}</span>
        </RouterLink>

        <div class="auth-form-content">
          <slot />

          <footer class="auth-layout-footer">
            <slot name="footer" />
            <p class="auth-copyright">
              &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
            </p>
          </footer>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AuthBrandPanel from '@/components/auth/AuthBrandPanel.vue'
import { useAuthLightTheme } from '@/composables/useAuthLightTheme'
import { useAppStore } from '@/stores'
import { getAuthRouteMotion } from '@/utils/authRouteMotion'
import { sanitizeUrl } from '@/utils/url'

useAuthLightTheme()

const { t } = useI18n()
const appStore = useAppStore()
const authMotionClass = `auth-motion--${getAuthRouteMotion()}`

const siteName = computed(
  () =>
    appStore.cachedPublicSettings?.site_name ||
    (appStore.siteName !== 'Sub2API' ? appStore.siteName : '爱白嫖公益站')
)
const siteLogo = computed(() =>
  sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true })
)
const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-page {
  position: relative;
  display: grid;
  min-height: 100vh;
  place-items: center;
  padding: 36px 5vw;
  overflow: hidden;
  font-family: 'PingFang SC', 'Hiragino Maru Gothic ProN', 'Microsoft YaHei UI', 'Microsoft YaHei', system-ui, sans-serif;
  background-color: #f6ede0;
  background-image:
    radial-gradient(circle at 14% 18%, rgba(146, 212, 251, 0.28) 0%, transparent 42%),
    radial-gradient(circle at 88% 82%, rgba(255, 166, 126, 0.24) 0%, transparent 42%),
    url('/sticker/cream.webp');
  background-size: 100% 100%, 100% 100%, 160px;
}

.auth-page::before,
.auth-page::after {
  position: absolute;
  pointer-events: none;
  border: 2px dashed rgba(212, 169, 122, 0.35);
  border-radius: 50%;
  content: '';
}

.auth-page::before {
  width: 420px;
  height: 200px;
  left: -120px;
  bottom: 6%;
  transform: rotate(-14deg);
}

.auth-page::after {
  width: 320px;
  height: 320px;
  top: 5%;
  right: -110px;
  border-style: solid;
  border-width: 1px;
  opacity: 0.65;
}

.auth-shell {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: minmax(480px, 1fr) minmax(500px, 1fr);
  width: min(1280px, 100%);
  min-height: 750px;
  overflow: hidden;
  border: 1.5px solid #e4d4ba;
  border-radius: 34px;
  background: #fffdf8;
  box-shadow:
    0 28px 72px rgba(96, 69, 36, 0.14),
    0 6px 18px rgba(96, 69, 36, 0.08);
  animation: shell-enter 640ms cubic-bezier(0.22, 0.85, 0.32, 1) both;
}

.auth-shell :deep(.auth-brand-panel) {
  height: 100%;
}

.auth-form-panel {
  position: relative;
  display: flex;
  min-width: 0;
  flex-direction: column;
  padding: 42px 64px 38px 72px;
  border-left: 1px solid #eadbc3;
  background-color: #fffcf5;
  background-image: url('/sticker/cream.webp');
  background-size: 160px;
  box-shadow: inset 10px 0 22px -14px rgba(112, 84, 48, 0.14);
}

/* Open journal binder rings straddling the center spine */
.auth-binder-rings {
  position: absolute;
  top: 76px;
  left: -16px;
  z-index: 6;
  display: grid;
  gap: 104px;
  pointer-events: none;
}

.auth-ring-unit {
  position: relative;
  width: 31px;
  height: 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.auth-hole-l,
.auth-hole-r {
  width: 9.5px;
  height: 9.5px;
  border-radius: 999px;
  background: #e9dac1;
  border: 1px solid #cbb38e;
  box-shadow: inset 0 1.5px 2px rgba(82, 58, 30, 0.38);
  flex: none;
}

.auth-ring-arc {
  position: absolute;
  left: 3.5px;
  right: 3.5px;
  top: 3px;
  height: 8px;
  border-radius: 999px;
  background: linear-gradient(180deg, #f3be7a 0%, #ffe6b3 34%, #d38b47 72%, #a8642d 100%);
  border: 1px solid #b37138;
  box-shadow: 0 2px 4px rgba(72, 48, 22, 0.25);
}

.auth-home-link {
  position: absolute;
  z-index: 3;
  top: 51px;
  right: 64px;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  border-radius: 999px;
  border: 1px solid #e4d4ba;
  background: rgba(255, 250, 240, 0.9);
  color: #5e5042;
  font-size: 11px;
  font-weight: 750;
  text-decoration: none;
  transition: color 200ms ease, border-color 200ms ease, transform 200ms ease, background-color 200ms ease;
}

.auth-home-link:hover {
  color: #27221d;
  border-color: #92d4fb;
  background: #e8f6fe;
  transform: translateX(-2px);
}

.auth-home-link:focus-visible {
  outline: 3px solid rgba(59, 157, 248, 0.28);
  outline-offset: 3px;
}

.auth-form-content {
  display: flex;
  width: 100%;
  min-height: 100%;
  flex: 1;
  flex-direction: column;
}

.auth-motion--forward .auth-shell,
.auth-motion--backward .auth-shell,
.auth-motion--neutral .auth-shell {
  animation: none;
}

.auth-motion--forward .auth-form-content {
  animation: auth-content-enter-forward 320ms cubic-bezier(0.22, 1, 0.36, 1) both;
}

.auth-motion--backward .auth-form-content {
  animation: auth-content-enter-backward 320ms cubic-bezier(0.22, 1, 0.36, 1) both;
}

.auth-motion--neutral .auth-form-content {
  animation: auth-content-enter-neutral 220ms ease-out both;
}

.auth-layout-footer {
  margin-top: auto;
  padding-top: 18px;
  text-align: center;
}

.auth-copyright {
  margin: 20px 0 0;
  color: #9a8b78;
  font-size: 10px;
}

:deep(.auth-form-stack) {
  display: block;
}

:deep(.auth-view-heading) {
  margin: 28px 0 7px;
  color: #27221d;
  font-size: 30px;
  font-weight: 900;
  letter-spacing: -0.02em;
  line-height: 1.2;
}

:deep(.auth-view-description) {
  margin: 0 0 24px;
  color: #756758;
  font-size: 13px;
}

:deep(.auth-form-stack form) {
  display: grid;
  gap: 15px;
}

:deep(.auth-register-stack form) {
  gap: 13px;
}

:deep(.auth-form-stack form > :not([hidden]) ~ :not([hidden])) {
  margin-top: 0 !important;
}

:deep(.input) {
  height: 48px;
  min-height: 48px;
  border: 1.25px solid #e2d2b8;
  border-radius: 15px;
  color: #27221d;
  background: #fffaf0;
  transition:
    border-color 200ms ease,
    box-shadow 200ms ease,
    background-color 200ms ease,
    transform 200ms ease;
}

:deep(.input:hover) {
  border-color: #7ec7f7;
  box-shadow: 0 0 0 4px rgba(146, 212, 251, 0.22);
}

:deep(.input:focus),
:deep(.input:focus-within) {
  border-color: #3b9df8;
  outline: none;
  background: #ffffff;
  box-shadow: 0 0 0 4px rgba(59, 157, 248, 0.16);
}

:deep(.input-label) {
  margin-bottom: 6px;
  color: #4c4237;
  font-size: 12px;
  font-weight: 750;
}

:deep(.auth-turnstile-slot) {
  width: 100%;
  overflow: hidden;
  border-radius: 14px;
}

:deep(.auth-primary-action) {
  position: relative;
  isolation: isolate;
  display: inline-flex;
  height: 50px;
  min-height: 50px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 2px solid #27221d;
  border-radius: 999px;
  background: #27221d;
  background-image: none;
  color: #fffaf2;
  text-align: center;
  box-shadow:
    0 4px 0 rgba(39, 34, 29, 0.24),
    0 10px 22px rgba(39, 34, 29, 0.14);
  font-size: 14px;
  font-weight: 850;
  transition:
    box-shadow 200ms ease,
    transform 200ms ease,
    background-color 200ms ease;
}

:deep(.auth-primary-action::after) {
  position: absolute;
  inset: -50% auto -50% -35%;
  width: 26%;
  z-index: -1;
  pointer-events: none;
  background: linear-gradient(90deg, transparent, rgba(255, 244, 224, 0.24), transparent);
  content: '';
  transform: rotate(18deg);
  transition: left 550ms ease;
}

:deep(.auth-primary-action:hover:not(:disabled)) {
  background: #352e27;
  background-image: none;
  transform: translateY(-2px);
  box-shadow:
    0 6px 0 rgba(39, 34, 29, 0.26),
    0 14px 26px rgba(39, 34, 29, 0.18);
}

:deep(.auth-primary-action:hover:not(:disabled)::after) {
  left: 112%;
}

:deep(.auth-primary-action:active:not(:disabled)) {
  transform: translateY(2px);
  box-shadow: 0 1px 0 rgba(39, 34, 29, 0.2);
}

:deep(.auth-primary-action:disabled) {
  cursor: not-allowed;
  opacity: 0.58;
  transform: none;
  box-shadow: 0 2px 0 rgba(39, 34, 29, 0.16);
}

:deep(.auth-status-card) {
  border: 1px solid #e4d4ba;
  border-radius: 16px;
  padding: 16px;
  background: #fffaf0;
}

:deep(.auth-status-card--success),
:deep(.auth-status-card.success) {
  border-color: #9ad8bc;
  background: #eefaf4;
  color: #16634e;
}

:deep(.auth-status-card--warning),
:deep(.auth-status-card.warning) {
  border-color: #f0cf88;
  background: #fff7e2;
  color: #785b14;
}

:deep(.auth-inline-link),
:deep(.auth-footer-copy a) {
  color: #2684ce;
  font-weight: 750;
  text-decoration: none;
}

:deep(.auth-inline-link:hover),
:deep(.auth-footer-copy a:hover) {
  color: #1968a6;
  text-decoration: underline;
}

:deep(.auth-footer-copy) {
  margin: 0;
  color: #847664;
  font-size: 12px;
}

:deep(.auth-back-chip) {
  display: inline-flex;
  min-height: 32px;
  align-items: center;
  padding: 6px 13px;
  border-radius: 999px;
  border: 1px solid #badffb;
  color: #1e6299;
  background: #e8f5fe;
  font-size: 11px;
  font-weight: 800;
  text-decoration: none;
}

@keyframes shell-enter {
  from {
    opacity: 0;
    transform: translateY(18px) scale(0.988);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes auth-content-enter-forward {
  from {
    opacity: 0;
    transform: translate3d(16px, 0, 0);
  }

  to {
    opacity: 1;
    transform: translate3d(0, 0, 0);
  }
}

@keyframes auth-content-enter-backward {
  from {
    opacity: 0;
    transform: translate3d(-16px, 0, 0);
  }

  to {
    opacity: 1;
    transform: translate3d(0, 0, 0);
  }
}

@keyframes auth-content-enter-neutral {
  from {
    opacity: 0;
  }

  to {
    opacity: 1;
  }
}

@media (max-width: 900px) {
  .auth-page {
    display: block;
    padding: 18px 14px;
    overflow: visible;
  }

  .auth-page::before,
  .auth-page::after,
  .auth-binder-rings {
    display: none;
  }

  .auth-shell {
    min-height: 0;
    grid-template-columns: minmax(0, 1fr);
    border-radius: 26px;
  }

  .auth-shell :deep(.auth-brand-panel) {
    height: auto;
    min-height: 230px;
  }

  .auth-form-panel {
    padding: 28px 24px 24px;
    border-left: 0;
    border-top: 1px solid #eadbc3;
    box-shadow: none;
  }

  .auth-home-link {
    top: 39px;
    right: 24px;
  }

  :deep(.auth-view-heading) {
    margin-top: 24px;
    font-size: 26px;
  }

  .auth-motion--forward .auth-form-content {
    animation-name: auth-content-enter-forward-mobile;
  }

  .auth-motion--backward .auth-form-content {
    animation-name: auth-content-enter-backward-mobile;
  }
}

@keyframes auth-content-enter-forward-mobile {
  from {
    opacity: 0;
    transform: translate3d(12px, 0, 0);
  }

  to {
    opacity: 1;
    transform: translate3d(0, 0, 0);
  }
}

@keyframes auth-content-enter-backward-mobile {
  from {
    opacity: 0;
    transform: translate3d(-12px, 0, 0);
  }

  to {
    opacity: 1;
    transform: translate3d(0, 0, 0);
  }
}

@media (max-width: 520px) {
  .auth-home-link {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-shell,
  .auth-form-content {
    animation: none;
  }

  .auth-home-link,
  :deep(.auth-primary-action),
  :deep(.auth-primary-action::after) {
    transition: none;
  }

  .auth-home-link:hover,
  :deep(.auth-primary-action:hover:not(:disabled)),
  :deep(.auth-primary-action:hover:not(:disabled)::after) {
    transform: none;
  }
}
</style>
