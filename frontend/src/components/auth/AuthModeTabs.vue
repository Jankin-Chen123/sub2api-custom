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
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  width: min(220px, 100%);
  padding: 5px;
  border: 1px solid #dcebed;
  border-radius: 8px;
  background: #edf5f5;
}

.auth-mode-tab {
  display: inline-flex;
  min-height: 40px;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  color: #71868a;
  font-size: 0.95rem;
  font-weight: 700;
  text-decoration: none;
  transition: color 180ms, background-color 180ms, box-shadow 180ms;
}

.auth-mode-tab:hover {
  color: #0d8f89;
}

.auth-mode-tab--active {
  color: #087b76;
  background: #fff;
  box-shadow: 0 5px 16px rgba(24, 111, 108, 0.1);
}
</style>
