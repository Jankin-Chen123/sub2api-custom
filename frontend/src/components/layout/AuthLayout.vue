<template>
  <div class="auth-page">
    <div class="auth-shell">
      <AuthBrandPanel :site-name="siteName" :site-logo="siteLogo || '/brand-icon.png'" />

      <main class="auth-form-panel">
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
import { sanitizeUrl } from '@/utils/url'

useAuthLightTheme()

const { t } = useI18n()
const appStore = useAppStore()

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
  display: grid;
  min-height: 100vh;
  place-items: center;
  padding: 38px 5vw;
  background: linear-gradient(135deg, #f8ffff, #edf9f7 48%, #fff8ee);
}

.auth-shell {
  position: relative;
  display: grid;
  grid-template-columns: minmax(410px, 1fr) minmax(430px, 1fr);
  width: min(1120px, 100%);
  min-height: 610px;
  overflow: hidden;
  border: 1px solid #d9e9eb;
  border-radius: 34px;
  background: #fff;
  box-shadow: 0 26px 75px rgba(42, 100, 106, 0.15);
  animation: shell-enter 720ms cubic-bezier(0.22, 0.85, 0.32, 1) both;
}

.auth-shell :deep(.auth-brand-panel) {
  height: 100%;
}

.auth-form-panel {
  position: relative;
  display: flex;
  min-width: 0;
  flex-direction: column;
  padding: 34px 56px 32px;
  background: rgba(255, 255, 255, 0.98);
}

.auth-home-link {
  position: absolute;
  z-index: 3;
  top: 45px;
  right: 56px;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #729096;
  font-size: 11px;
  font-weight: 700;
  text-decoration: none;
  transition: color 220ms ease, transform 220ms ease;
}

.auth-home-link:hover {
  color: #087f83;
  transform: translateX(-2px);
}

.auth-home-link:focus-visible {
  border-radius: 4px;
  outline: 3px solid rgba(14, 139, 142, 0.2);
  outline-offset: 4px;
}

.auth-form-content {
  display: flex;
  width: 100%;
  min-height: 100%;
  flex: 1;
  flex-direction: column;
}

.auth-layout-footer {
  margin-top: auto;
  padding-top: 18px;
  text-align: center;
}

.auth-copyright {
  margin: 22px 0 0;
  color: #a1b3b6;
  font-size: 9px;
}

:deep(.auth-form-stack) {
  display: block;
}

:deep(.auth-view-heading) {
  margin: 31px 0 7px;
  color: #102d32;
  font-size: 30px;
  font-weight: 800;
  letter-spacing: 0;
  line-height: 1.2;
}

:deep(.auth-view-description) {
  margin: 0 0 25px;
  color: #789093;
  font-size: 13px;
}

:deep(.auth-form-stack form) {
  display: grid;
  gap: 16px;
}

:deep(.auth-register-stack form) {
  gap: 13px;
}

:deep(.auth-form-stack form > :not([hidden]) ~ :not([hidden])) {
  margin-top: 0 !important;
}

:deep(.input) {
  height: 49px;
  min-height: 49px;
  border: 1px solid #cee1e4;
  border-radius: 14px;
  color: #436269;
  background: #fcfefe;
  transition:
    border-color 220ms ease,
    box-shadow 220ms ease,
    transform 220ms ease;
}

:deep(.input:hover) {
  border-color: #8bcac6;
  box-shadow: 0 0 0 4px rgba(18, 164, 155, 0.07);
  transform: translateY(-1px);
}

:deep(.input:focus),
:deep(.input:focus-within) {
  border-color: #8bcac6;
  outline: none;
  background: #fff;
  box-shadow: 0 0 0 4px rgba(18, 164, 155, 0.09);
}

:deep(.input-label) {
  margin-bottom: 7px;
  color: #436269;
  font-size: 12px;
  font-weight: 750;
}

:deep(.auth-turnstile-slot) {
  width: 100%;
  overflow: hidden;
  border-radius: 11px;
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
  border-radius: 14px;
  color: #fff;
  text-align: center;
  box-shadow: 0 11px 24px rgba(19, 166, 156, 0.22);
  font-size: 14px;
  font-weight: 850;
  transition:
    box-shadow 230ms ease,
    transform 230ms ease;
}

:deep(.auth-primary-action::after) {
  position: absolute;
  inset: -50% auto -50% -35%;
  width: 26%;
  z-index: -1;
  pointer-events: none;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.42), transparent);
  content: '';
  transform: rotate(18deg);
  transition: left 550ms ease;
}

:deep(.auth-primary-action:hover:not(:disabled)) {
  transform: translateY(-2px);
  box-shadow: 0 16px 30px rgba(19, 166, 156, 0.3);
}

:deep(.auth-primary-action:hover:not(:disabled)::after) {
  left: 112%;
}

:deep(.auth-primary-action:disabled) {
  cursor: not-allowed;
  opacity: 0.62;
  transform: none;
  box-shadow: 0 8px 18px rgba(8, 127, 131, 0.12);
}

:deep(.auth-status-card) {
  border: 1px solid #d7e6e7;
  border-radius: 14px;
  padding: 16px;
  background: #f9fcfc;
}

:deep(.auth-status-card--success),
:deep(.auth-status-card.success) {
  border-color: #9ed9c7;
  background: #effaf6;
  color: #16634e;
}

:deep(.auth-status-card--warning),
:deep(.auth-status-card.warning) {
  border-color: #efd491;
  background: #fff9e9;
  color: #785b14;
}

:deep(.auth-inline-link),
:deep(.auth-footer-copy a) {
  color: #087f83;
  font-weight: 750;
  text-decoration: none;
}

:deep(.auth-inline-link:hover),
:deep(.auth-footer-copy a:hover) {
  color: #056568;
}

:deep(.auth-footer-copy) {
  margin: 0;
  color: #7c9499;
  font-size: 11px;
}

:deep(.auth-back-chip) {
  display: inline-flex;
  min-height: 32px;
  align-items: center;
  padding: 8px 12px;
  border-radius: 11px;
  color: #087f79;
  background: #edf8f7;
  font-size: 11px;
  font-weight: 800;
  text-decoration: none;
}

@keyframes shell-enter {
  from {
    opacity: 0;
    transform: translateY(22px) scale(0.985);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 900px) {
  .auth-page {
    display: block;
    padding: 20px 14px;
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
    padding: 28px 25px 25px;
  }

  .auth-home-link {
    top: 41px;
    right: 25px;
  }

  :deep(.auth-view-heading) {
    margin-top: 25px;
    font-size: 26px;
  }
}

@media (max-width: 520px) {
  .auth-home-link {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-shell {
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
