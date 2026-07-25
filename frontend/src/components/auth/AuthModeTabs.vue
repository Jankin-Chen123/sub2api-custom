<template>
  <nav class="auth-mode-tabs" :aria-label="t('auth.modeTabsLabel')">
    <RouterLink
      to="/login"
      class="auth-mode-tab"
      :class="{ 'auth-mode-tab--active': active === 'login' }"
      :aria-current="active === 'login' ? 'page' : undefined"
    >
      {{ t('auth.signIn') }}
    </RouterLink>
    <RouterLink
      to="/register"
      class="auth-mode-tab"
      :class="{ 'auth-mode-tab--active': active === 'register' }"
      :aria-current="active === 'register' ? 'page' : undefined"
    >
      {{ t('auth.signUp') }}
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  active: 'login' | 'register'
}>()

const { t } = useI18n()
</script>

<style scoped>
.auth-mode-tabs {
  display: inline-flex;
  gap: 4px;
  width: max-content;
  max-width: 100%;
  padding: 4px;
  border-radius: 13px;
  background: #edf5f6;
}

.auth-mode-tab {
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
  background: #fff;
  box-shadow: 0 3px 10px rgba(53, 115, 119, 0.1);
}
</style>
