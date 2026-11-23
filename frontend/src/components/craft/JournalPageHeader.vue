<template>
  <section :class="[bannerClass, 'group relative overflow-visible rounded-[22px] p-2.5 sm:p-3']">
    <div
      :class="[
        'journal-banner-stitch flex flex-col gap-3 rounded-[16px] sm:flex-row sm:items-center sm:justify-between',
        compact ? 'px-4 py-3 sm:px-5' : 'px-4 py-4 sm:px-6 sm:py-5'
      ]"
    >
      <div class="flex min-w-0 items-center gap-3.5 sm:gap-4">
        <div
          v-if="stickerSrc"
          class="journal-banner-sticker shrink-0 select-none"
        >
          <img
            :src="stickerSrc"
            alt=""
            class="h-12 w-auto object-contain sm:h-14"
          />
        </div>
        <JournalBannerSticker v-else :name="icon" :size="compact ? 'sm' : 'md'" />

        <div class="min-w-0">
          <div class="flex items-center gap-1.5">
            <h1
              :class="[
                'truncate font-black tracking-tight',
                headingToneClass,
                compact ? 'text-lg sm:text-xl' : 'text-xl sm:text-2xl'
              ]"
            >
              {{ title }}
            </h1>
            <CraftSparks variant="right" size="sm" />
          </div>
          <p
            v-if="description"
            :class="[
              'mt-0.5 line-clamp-2 text-xs font-medium sm:text-sm',
              subtitleToneClass
            ]"
          >
            {{ description }}
          </p>
        </div>
      </div>

      <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2 sm:shrink-0 sm:justify-end">
        <slot name="actions" />
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import CraftSparks from './CraftSparks.vue'
import JournalBannerSticker from './JournalBannerSticker.vue'

type BannerVariant = 'sky' | 'apricot' | 'peach'

const props = withDefaults(
  defineProps<{
    icon?: string
    stickerSrc?: string
    title: string
    description?: string
    variant?: BannerVariant
    compact?: boolean
  }>(),
  {
    icon: 'sparkles',
    variant: 'sky',
    compact: false,
  }
)

const bannerClass = computed(() => {
  if (props.variant === 'apricot') return 'journal-banner-apricot'
  if (props.variant === 'peach') return 'journal-banner-peach'
  return 'journal-banner-sky'
})

const headingToneClass = computed(() => {
  if (props.variant === 'sky') {
    return 'text-[#201c18] dark:text-[#f5ede0]'
  }
  return 'text-[#27221d] dark:text-[#f5ede0]'
})

const subtitleToneClass = computed(() => {
  if (props.variant === 'sky') {
    return 'text-[#24435c] dark:text-[#b6defa]'
  }
  return 'text-[#6e5d4b] dark:text-[#c7bfa8]'
})
</script>
