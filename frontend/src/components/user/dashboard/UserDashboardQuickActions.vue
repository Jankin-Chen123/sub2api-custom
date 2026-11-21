<template>
  <nav class="action-strip" :aria-label="t('dashboard.quickActions')">
    <RouterLink v-for="action in actions" :key="action.to" :to="action.to" class="action">
      <Icon :name="action.icon" size="md" aria-hidden="true" />
      <span>{{ t(action.label) }}</span>
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()
const actions = computed(() => [
  { to: '/keys', icon: 'key', label: 'dashboard.createApiKey' },
  { to: '/usage', icon: 'chart', label: 'dashboard.viewUsage' },
  { to: '/redeem', icon: 'gift', label: 'dashboard.redeemCode' },
  ...(canUseBatchImage.value ? [{ to: '/batch-image', icon: 'sparkles' as const, label: 'dashboard.batchImageAgent' }] : []),
] as const)

onMounted(() => { void refreshBatchImageAccess() })
</script>

<style scoped>
.action-strip { display: grid; grid-template-columns: repeat(3, minmax(0,1fr)); gap: 9px; }
.action {
  display: flex; align-items: center; justify-content: center; gap: 7px;
  min-height: 44px; padding: 6px; border: 1px solid var(--notebook-ring);
  border-radius: 15px; background: var(--notebook-surface); color: var(--notebook-ink);
  box-shadow: 0 2px 2px var(--notebook-shadow); font-size: .9rem; font-weight: 730;
  text-align: center;
}
.action svg { flex: none; }
.action:hover { background: var(--notebook-selected); }
.action:nth-child(4) { grid-column: 1 / -1; }
@media (max-width: 520px) {
  .action { font-size: .76rem; gap: 4px; }
  .action svg { width: 15px; height: 15px; }
}
</style>
