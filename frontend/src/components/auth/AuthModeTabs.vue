<template>
  <nav
    class="auth-mode-tabs"
    :class="[
      `auth-mode-tabs--${props.active}`,
      `auth-mode-tabs--motion-${authRouteMotion}`,
    ]"
    :aria-label="t('auth.modeTabsLabel')"
  >
    <RouterLink
      to="/login"
      class="auth-mode-tab"
      :class="{ 'auth-mode-tab--active': props.active === 'login' }"
      :aria-current="props.active === 'login' ? 'page' : undefined"
    >
      {{ t('auth.signIn') }}
    </RouterLink>
    <RouterLink
      to="/register"
      class="auth-mode-tab"
      :class="{ 'auth-mode-tab--active': props.active === 'register' }"
      :aria-current="props.active === 'register' ? 'page' : undefined"
    >
      {{ t('auth.signUp') }}
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { getAuthRouteMotion } from '@/utils/authRouteMotion'

const props = defineProps<{
  active: 'login' | 'register'
}>()

const { t } = useI18n()
const authRouteMotion = getAuthRouteMotion()
</script>

<style scoped>
.auth-mode-tabs {
  position: relative;
  isolation: isolate;
  display: inline-grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px;
  width: max-content;
  max-width: 100%;
  padding: 4px;
  border-radius: 13px;
  background: #edf5f6;
}

.auth-mode-tabs::before {
  position: absolute;
  z-index: 0;
  width: calc((100% - 12px) / 2);
  top: 4px;
  bottom: 4px;
  left: 4px;
  border-radius: 10px;
  background: #fff;
  box-shadow: 0 3px 10px rgba(53, 115, 119, 0.1);
  content: '';
  transform: translate3d(0, 0, 0);
}

.auth-mode-tabs--register::before {
  transform: translate3d(calc(100% + 4px), 0, 0);
}

.auth-mode-tabs--register.auth-mode-tabs--motion-forward::before {
  animation: auth-tab-indicator-forward 340ms cubic-bezier(0.22, 1, 0.36, 1) both;
}

.auth-mode-tabs--login.auth-mode-tabs--motion-backward::before {
  animation: auth-tab-indicator-backward 340ms cubic-bezier(0.22, 1, 0.36, 1) both;
}

.auth-mode-tab {
  position: relative;
  z-index: 1;
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  justify-content: center;
  padding: 8px 17px;
  border-radius: 10px;
  color: #789095;
  font-size: 12px;
  font-weight: 750;
  text-decoration: none;
  transition: color 220ms ease, background-color 220ms ease, box-shadow 220ms ease,
    transform 220ms ease;
}

.auth-mode-tab:hover {
  color: #087f79;
  transform: translateY(-1px);
}

.auth-mode-tab--active {
  color: #087f79;
}

@keyframes auth-tab-indicator-forward {
  from {
    transform: translate3d(0, 0, 0);
  }

  to {
    transform: translate3d(calc(100% + 4px), 0, 0);
  }
}

@keyframes auth-tab-indicator-backward {
  from {
    transform: translate3d(calc(100% + 4px), 0, 0);
  }

  to {
    transform: translate3d(0, 0, 0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-mode-tabs::before {
    animation: none;
    transition: none;
  }

  .auth-mode-tab {
    transition: none;
  }
}
</style>
