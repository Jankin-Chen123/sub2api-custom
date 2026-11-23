<template>
  <div class="empty-state rounded-[20px] border-[1.5px] border-dashed border-[var(--craft-border-strong)] bg-[var(--craft-surface-muted)]/60 px-6 py-12">
    <!-- Icon -->
    <div
      class="mb-4 flex h-16 w-16 -rotate-3 items-center justify-center rounded-2xl border border-[var(--craft-border-strong)] bg-[var(--craft-surface-warm)] shadow-sm dark:bg-[#232838]"
    >
      <slot name="icon">
        <component v-if="icon" :is="icon" class="empty-state-icon h-9 w-9" aria-hidden="true" />
        <CraftIcon v-else name="inbox" size="lg" />
      </slot>
    </div>

    <!-- Title -->
    <div class="mb-1 flex items-center justify-center gap-1">
      <h3 class="empty-state-title !mb-0 font-black">
        {{ displayTitle }}
      </h3>
      <CraftSparks variant="right" size="sm" />
    </div>

    <!-- Description -->
    <p class="empty-state-description">
      {{ description }}
    </p>

    <!-- Action -->
    <div v-if="actionText || $slots.action" class="mt-6">
      <slot name="action">
        <component
          :is="actionTo ? 'RouterLink' : 'button'"
          v-if="actionText"
          :to="actionTo"
          @click="!actionTo && $emit('action')"
          class="btn btn-primary"
        >
          <Icon v-if="actionIcon" name="plus" size="md" class="mr-2" />
          {{ actionText }}
        </component>
      </slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Component } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import CraftIcon from '@/components/craft/CraftIcon.vue'
import CraftSparks from '@/components/craft/CraftSparks.vue'

const { t } = useI18n()

interface Props {
  icon?: Component | string
  title?: string
  description?: string
  actionText?: string
  actionTo?: string | object
  actionIcon?: boolean
  message?: string
}

const props = withDefaults(defineProps<Props>(), {
  description: '',
  actionIcon: true
})

const displayTitle = computed(() => props.title || t('common.noData'))

defineEmits(['action'])
</script>
