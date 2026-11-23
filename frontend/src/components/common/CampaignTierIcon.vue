<template>
  <svg
    :class="['shrink-0 overflow-visible', sizeClass]"
    viewBox="0 0 64 64"
    fill="none"
    aria-hidden="true"
  >
    <defs>
      <filter
        :id="filterId"
        filterUnits="userSpaceOnUse"
        x="-10"
        y="-10"
        width="84"
        height="84"
        color-interpolation-filters="sRGB"
      >
        <feMorphology in="SourceAlpha" operator="dilate" radius="2.2" result="dilated" />
        <feGaussianBlur in="dilated" stdDeviation="0.4" result="smooth" />
        <feComponentTransfer in="smooth" result="solid">
          <feFuncA type="table" tableValues="0 0 0.45 1 1" />
        </feComponentTransfer>
        <feFlood flood-color="#ffffff" result="white" />
        <feComposite in="white" in2="solid" operator="in" result="stickerEdge" />
        <feDropShadow
          in="stickerEdge"
          dx="0"
          dy="2.4"
          stdDeviation="1.6"
          flood-color="#27221d"
          flood-opacity="0.22"
          result="shadowed"
        />
        <feMerge>
          <feMergeNode in="shadowed" />
          <feMergeNode in="SourceGraphic" />
        </feMerge>
      </filter>

      <linearGradient :id="grad.sky" x1="12" y1="8" x2="52" y2="48" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="#a9e0ff" />
        <stop offset="100%" stop-color="#4ca8f0" />
      </linearGradient>
      <linearGradient :id="grad.mint" x1="18" y1="36" x2="46" y2="56" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="#c2f5e4" />
        <stop offset="100%" stop-color="#7cd9bd" />
      </linearGradient>
      <linearGradient :id="grad.gold" x1="12" y1="14" x2="52" y2="48" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="#ffef9e" />
        <stop offset="100%" stop-color="#f4b235" />
      </linearGradient>
      <linearGradient :id="grad.apricot" x1="14" y1="40" x2="50" y2="52" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="#ffbe85" />
        <stop offset="100%" stop-color="#f58f56" />
      </linearGradient>
      <linearGradient :id="grad.lavender" x1="10" y1="14" x2="54" y2="54" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="#f6f0ff" />
        <stop offset="55%" stop-color="#ccb8ff" />
        <stop offset="100%" stop-color="#8e73f2" />
      </linearGradient>
    </defs>

    <g :filter="`url(#${filterId})`">
      <!-- Premium: handcrafted sky-blue rosette star medallion with mint ribbons -->
      <template v-if="props.tierKey === 'premium'">
        <path
          d="M22 39L17 55L25.5 50.5L30.5 56L32 41.5"
          :fill="`url(#${grad.mint})`"
          stroke="#27221d"
          stroke-width="2.6"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
        <path
          d="M42 39L47 55L38.5 50.5L33.5 56L32 41.5"
          :fill="`url(#${grad.mint})`"
          stroke="#27221d"
          stroke-width="2.6"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
        <path
          d="M32 7.5C35.2 7.5 37.5 9.8 39.2 11.4C41.4 10.8 44.5 11.9 46.1 14.3C47.7 16.7 47.4 19.8 46.6 21.8C48.5 23.6 50.5 26 50.5 29C50.5 32 48.5 34.4 46.6 36.2C47.4 38.2 47.7 41.3 46.1 43.7C44.5 46.1 41.4 47.2 39.2 46.6C37.5 48.2 35.2 50.5 32 50.5C28.8 50.5 26.5 48.2 24.8 46.6C22.6 47.2 19.5 46.1 17.9 43.7C16.3 41.3 16.6 38.2 17.4 36.2C15.5 34.4 13.5 32 13.5 29C13.5 26 15.5 23.6 17.4 21.8C16.6 19.8 16.3 16.7 17.9 14.3C19.5 11.9 22.6 10.8 24.8 11.4C26.5 9.8 28.8 7.5 32 7.5Z"
          :fill="`url(#${grad.sky})`"
          stroke="#27221d"
          stroke-width="2.8"
          stroke-linejoin="round"
        />
        <circle
          cx="32"
          cy="29"
          r="11.5"
          fill="#fffaf0"
          stroke="#e6c88a"
          stroke-width="1.8"
          stroke-dasharray="3 2.5"
        />
        <path
          d="M32 20.2C33.4 25.4 35.4 27.4 40.6 29C35.4 30.6 33.4 32.6 32 37.8C30.6 32.6 28.6 30.6 23.4 29C28.6 27.4 30.6 25.4 32 20.2Z"
          :fill="`url(#${grad.gold})`"
          stroke="#27221d"
          stroke-width="2.2"
          stroke-linejoin="round"
        />
        <circle cx="48.5" cy="12" r="2.2" fill="#ffe27a" stroke="#27221d" stroke-width="1.6" />
      </template>

      <!-- Gold: chunky honey-gold storybook crown with cabochon gems -->
      <template v-else-if="props.tierKey === 'gold'">
        <path
          d="M12 24L20.5 31L32 17L43.5 31L52 24L48.5 46H15.5L12 24Z"
          :fill="`url(#${grad.gold})`"
          stroke="#27221d"
          stroke-width="2.8"
          stroke-linejoin="round"
        />
        <path
          d="M17 28L20.5 31L26 24.5"
          stroke="#ffffff"
          stroke-opacity="0.7"
          stroke-width="2.2"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
        <rect
          x="14.5"
          y="42"
          width="35"
          height="8.5"
          rx="4.25"
          :fill="`url(#${grad.apricot})`"
          stroke="#27221d"
          stroke-width="2.6"
        />
        <circle cx="12" cy="22" r="3.4" fill="#fff6c8" stroke="#27221d" stroke-width="2.4" />
        <circle cx="32" cy="15" r="3.8" fill="#fff6c8" stroke="#27221d" stroke-width="2.4" />
        <circle cx="52" cy="22" r="3.4" fill="#fff6c8" stroke="#27221d" stroke-width="2.4" />
        <circle cx="22" cy="36.5" r="2.6" fill="#ff886b" stroke="#27221d" stroke-width="1.9" />
        <circle cx="32" cy="34" r="3.1" fill="#7cc7fb" stroke="#27221d" stroke-width="2" />
        <circle cx="42" cy="36.5" r="2.6" fill="#ff886b" stroke="#27221d" stroke-width="1.9" />
        <path
          d="M49 11L50.3 13.7L53 15L50.3 16.3L49 19L47.7 16.3L45 15L47.7 13.7L49 11Z"
          fill="#ffe27a"
          stroke="#27221d"
          stroke-width="1.5"
          stroke-linejoin="round"
        />
      </template>

      <!-- Diamond: pastel lavender & ice-sky faceted crystal sticker -->
      <template v-else-if="props.tierKey === 'diamond'">
        <path
          d="M18 17H46L55 29L32 53L9 29L18 17Z"
          :fill="`url(#${grad.lavender})`"
          stroke="#27221d"
          stroke-width="2.8"
          stroke-linejoin="round"
        />
        <path d="M18 17H27L22 29H9L18 17Z" fill="#f6f0ff" />
        <path d="M27 17H37L42 29H22L27 17Z" fill="#d8efff" />
        <path d="M37 17H46L55 29H42L37 17Z" fill="#dfceff" />
        <path d="M9 29H22L32 53L9 29Z" fill="#b6d8ff" />
        <path d="M22 29H42L32 53L22 29Z" fill="#a8b4ff" />
        <path d="M42 29H55L32 53L42 29Z" fill="#8e73f2" fill-opacity="0.72" />
        <path
          d="M9 29H55M18 17L22 29L32 53L42 29L46 17M27 17L22 29M37 17L42 29"
          stroke="#27221d"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
        <path
          d="M18 17H46L55 29L32 53L9 29L18 17Z"
          stroke="#27221d"
          stroke-width="2.8"
          stroke-linejoin="round"
        />
        <path
          d="M19 21H25"
          stroke="#ffffff"
          stroke-opacity="0.88"
          stroke-width="2.2"
          stroke-linecap="round"
        />
        <path
          d="M52 10L53.4 13.1L56.5 14.5L53.4 15.9L52 19L50.6 15.9L47.5 14.5L50.6 13.1L52 10Z"
          fill="#ffe27a"
          stroke="#27221d"
          stroke-width="1.5"
          stroke-linejoin="round"
        />
        <circle cx="11" cy="14" r="2.2" fill="#9fe0ff" stroke="#27221d" stroke-width="1.5" />
      </template>

      <!-- Safe fallback for a future configured tier -->
      <g v-else>
        <circle
          cx="32"
          cy="32"
          r="20"
          :fill="`url(#${grad.apricot})`"
          stroke="#27221d"
          stroke-width="2.8"
        />
        <path
          d="M32 19L35.2 27.8L44 31L35.2 34.2L32 43L28.8 34.2L20 31L28.8 27.8L32 19Z"
          fill="#fffaf0"
          stroke="#27221d"
          stroke-width="2.2"
          stroke-linejoin="round"
        />
      </g>
    </g>
  </svg>
</template>

<script lang="ts">
let tierUidSeed = 0
</script>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  tierKey: string
  size?: 'xs' | 'sm' | 'md'
}>(), {
  size: 'md',
})

const instanceUid = `cti-${++tierUidSeed}`
const filterId = `${instanceUid}-sticker`
const grad = {
  sky: `${instanceUid}-sky`,
  mint: `${instanceUid}-mint`,
  gold: `${instanceUid}-gold`,
  apricot: `${instanceUid}-apricot`,
  lavender: `${instanceUid}-lavender`,
}

const sizeClass = computed(() => {
  if (props.size === 'xs') return 'h-5 w-5'
  if (props.size === 'sm') return 'h-9 w-9'
  return 'h-10 w-10'
})
</script>
