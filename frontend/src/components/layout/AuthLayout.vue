<template>
  <div class="auth-page">
    <div class="auth-shell">
      <AuthBrandPanel :site-name="siteName" :site-logo="siteLogo || '/logo.svg'" />

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

const siteName = computed(() => appStore.siteName || 'Sub2API')
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
  overflow: hidden;
  padding: clamp(16px, 3vw, 48px);
  background-color: #f6fbfa;
  background-image:
    linear-gradient(rgba(28, 145, 143, 0.07) 1px, transparent 1px),
    linear-gradient(90deg, rgba(28, 145, 143, 0.07) 1px, transparent 1px);
  background-attachment: fixed;
  background-size: 72px 72px;
  letter-spacing: 0;
}

.auth-shell {
  position: relative;
  z-index: 1;
  display: grid;
  width: min(1760px, 100%);
  min-height: min(820px, calc(100vh - 48px));
  grid-template-columns: minmax(0, 1.06fr) minmax(480px, 0.94fr);
  overflow: hidden;
  border: 1px solid #dbe9e8;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 28px 80px rgba(36, 83, 82, 0.14);
  animation: shell-enter 420ms ease-out both;
}

.auth-shell :deep(.auth-brand-panel) {
  height: 100%;
  min-height: 0;
}

.auth-form-panel {
  position: relative;
  min-width: 0;
  max-height: calc(100vh - 48px);
  overflow-y: auto;
  padding: clamp(72px, 7vh, 104px) clamp(40px, 5vw, 92px) 42px;
  background: #fff;
}

.auth-home-link {
  position: absolute;
  top: 32px;
  right: 36px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #527174;
  font-size: 0.9rem;
  font-weight: 700;
  line-height: 1.4;
  text-decoration: none;
  transition:
    color 160ms ease,
    transform 160ms ease;
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
  width: min(590px, 100%);
  margin-inline: auto;
}

.auth-layout-footer {
  display: grid;
  gap: 22px;
  margin-top: 30px;
  text-align: center;
}

.auth-copyright {
  margin: 0;
  color: #91a4a6;
  font-size: 0.78rem;
  line-height: 1.6;
}

:deep(.auth-form-stack) {
  display: grid;
  gap: 24px;
}

:deep(.auth-view-heading) {
  margin: 0;
  color: #12373a;
  font-size: 2.5rem;
  font-weight: 800;
  letter-spacing: 0;
  line-height: 1.12;
}

:deep(.auth-view-description) {
  margin: 10px 0 0;
  color: #789093;
  font-size: 1rem;
  line-height: 1.65;
}

:deep(.input) {
  min-height: 56px;
  border: 1px solid #d7e6e7;
  border-radius: 8px;
  background: #f9fcfc;
  color: #163d40;
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease,
    background-color 160ms ease;
}

:deep(.input:hover) {
  border-color: #9ccfd0;
  background: #fff;
}

:deep(.input:focus),
:deep(.input:focus-within) {
  border-color: #14999c;
  outline: none;
  background: #fff;
  box-shadow: 0 0 0 4px rgba(20, 153, 156, 0.12);
}

:deep(.input-label) {
  color: #315659;
  font-weight: 700;
  letter-spacing: 0;
}

:deep(.auth-turnstile-slot) {
  width: 100%;
  overflow: hidden;
  border-radius: 8px;
}

:deep(.auth-primary-action) {
  position: relative;
  isolation: isolate;
  display: inline-flex;
  min-height: 56px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 8px;
  color: #fff;
  text-align: center;
  box-shadow: 0 12px 28px rgba(8, 127, 131, 0.2);
  transition:
    box-shadow 180ms ease,
    transform 180ms ease;
}

:deep(.auth-primary-action::after) {
  position: absolute;
  inset: 0;
  z-index: -1;
  pointer-events: none;
  background: linear-gradient(
    105deg,
    transparent 25%,
    rgba(255, 255, 255, 0.2) 45%,
    transparent 65%
  );
  content: '';
  transform: translateX(-120%);
  transition: transform 520ms ease;
}

:deep(.auth-primary-action:hover:not(:disabled)) {
  transform: translateY(-2px);
  box-shadow: 0 16px 34px rgba(8, 127, 131, 0.26);
}

:deep(.auth-primary-action:hover:not(:disabled)::after) {
  transform: translateX(120%);
}

:deep(.auth-primary-action:disabled) {
  cursor: not-allowed;
  opacity: 0.62;
  transform: none;
  box-shadow: 0 8px 18px rgba(8, 127, 131, 0.12);
}

:deep(.auth-status-card) {
  border: 1px solid #d7e6e7;
  border-radius: 8px;
  padding: 24px;
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
  font-weight: 700;
  text-decoration-thickness: 1px;
  text-underline-offset: 3px;
}

:deep(.auth-inline-link:hover),
:deep(.auth-footer-copy a:hover) {
  color: #056568;
}

:deep(.auth-footer-copy) {
  margin: 0;
  color: #789093;
  font-size: 0.9rem;
  line-height: 1.6;
}

@keyframes shell-enter {
  from {
    opacity: 0;
    transform: translateY(16px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 1100px) {
  .auth-shell {
    grid-template-columns: minmax(0, 0.94fr) minmax(420px, 1.06fr);
  }

  .auth-form-panel {
    padding-inline: clamp(36px, 5vw, 64px);
  }
}

@media (max-width: 900px) {
  .auth-page {
    place-items: start center;
    overflow-y: auto;
  }

  .auth-shell {
    min-height: 0;
    grid-template-columns: minmax(0, 1fr);
  }

  .auth-shell :deep(.auth-brand-panel) {
    height: auto;
    min-height: 280px;
  }

  .auth-form-panel {
    max-height: none;
    overflow-y: visible;
    padding: 82px clamp(32px, 8vw, 72px) 38px;
  }
}

@media (max-width: 560px) {
  .auth-page {
    padding: 0;
  }

  .auth-shell {
    border: 0;
    border-radius: 0;
    box-shadow: none;
  }

  .auth-form-panel {
    padding: 76px 22px 32px;
  }

  .auth-home-link {
    top: 24px;
    right: 22px;
  }

  :deep(.auth-view-heading) {
    font-size: 2rem;
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
