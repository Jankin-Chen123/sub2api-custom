<template>
  <div class="grid gap-3" :class="canUseBatchImage ? 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-4' : 'grid-cols-1 sm:grid-cols-3'">
    <button
      type="button"
      class="quick-pill group"
      @click="router.push('/keys')"
    >
      <CraftIcon name="key" size="sm" />
      <span class="min-w-0 flex-1 truncate text-left">{{ t('dashboard.createApiKey') }}</span>
      <Icon
        name="chevronRight"
        size="sm"
        class="text-[#998977] transition-transform group-hover:translate-x-0.5 dark:text-[#8690a6]"
      />
    </button>

    <button
      type="button"
      class="quick-pill group"
      @click="router.push('/usage')"
    >
      <CraftIcon name="chart" size="sm" />
      <span class="min-w-0 flex-1 truncate text-left">{{ t('dashboard.viewUsage') }}</span>
      <Icon
        name="chevronRight"
        size="sm"
        class="text-[#998977] transition-transform group-hover:translate-x-0.5 dark:text-[#8690a6]"
      />
    </button>

    <button
      v-if="canUseBatchImage"
      type="button"
      class="quick-pill group"
      @click="router.push('/batch-image')"
    >
      <CraftIcon name="sparkles" size="sm" />
      <span class="min-w-0 flex-1 truncate text-left">{{ t('dashboard.batchImageAgent') }}</span>
      <Icon
        name="chevronRight"
        size="sm"
        class="text-[#998977] transition-transform group-hover:translate-x-0.5 dark:text-[#8690a6]"
      />
    </button>

    <button
      type="button"
      class="quick-pill group"
      @click="router.push('/redeem')"
    >
      <CraftIcon name="gift" size="sm" />
      <span class="min-w-0 flex-1 truncate text-left">{{ t('dashboard.redeemCode') }}</span>
      <Icon
        name="chevronRight"
        size="sm"
        class="text-[#998977] transition-transform group-hover:translate-x-0.5 dark:text-[#8690a6]"
      />
    </button>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import CraftIcon from '@/components/craft/CraftIcon.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

const router = useRouter()
const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

onMounted(() => {
  void refreshBatchImageAccess()
})
</script>

<style scoped>
.quick-pill {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 46px;
  padding: 10px 16px;
  border-radius: 15px;
  border: 1.25px solid var(--craft-border-strong, #d9c6a6);
  background-color: var(--craft-surface, #fffdf8);
  color: var(--craft-ink, #27221d);
  font-size: 0.875rem;
  font-weight: 750;
  box-shadow: 0 2.5px 0 rgba(92, 68, 38, 0.12), 0 4px 10px rgba(92, 68, 38, 0.05);
  transition: transform 0.16s ease, box-shadow 0.16s ease, background-color 0.16s ease;
}

.quick-pill:hover {
  background-color: var(--craft-surface-hover, #f9f0df);
  transform: translateY(-1.5px);
  box-shadow: 0 4px 0 rgba(92, 68, 38, 0.15), 0 8px 16px rgba(92, 68, 38, 0.08);
}

.quick-pill:active {
  transform: translateY(1px);
  box-shadow: 0 1px 0 rgba(92, 68, 38, 0.12);
}

@media (prefers-reduced-motion: reduce) {
  .quick-pill {
    transition: none;
  }
}
</style>
