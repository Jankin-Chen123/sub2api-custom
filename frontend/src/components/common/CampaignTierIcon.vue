<template>
  <svg
    :class="['drop-shadow-[0_2px_3px_rgba(0,0,0,0.18)]', sizeClass]"
    viewBox="0 0 48 48"
    fill="none"
    aria-hidden="true"
  >
    <!-- Premium: a restrained four-point star framed by concentric rings. -->
    <template v-if="props.tierKey === 'premium'">
      <circle cx="24" cy="24" r="16" stroke="currentColor" stroke-width="1.2" opacity="0.35" />
      <circle cx="24" cy="24" r="12" stroke="currentColor" stroke-width="1" opacity="0.2" />
      <path
        d="M24 10.5l2.6 9.1 8.9 4.4-8.9 4.4L24 37.5l-2.6-9.1-8.9-4.4 8.9-4.4L24 10.5z"
        fill="currentColor"
        opacity="0.94"
      />
      <circle cx="24" cy="24" r="2.2" fill="currentColor" />
      <circle cx="10.5" cy="24" r="1" fill="currentColor" opacity="0.55" />
      <circle cx="37.5" cy="24" r="1" fill="currentColor" opacity="0.55" />
    </template>

    <!-- Gold: a clean crown with a subtle jewel band. -->
    <template v-else-if="props.tierKey === 'gold'">
      <path
        d="M9.5 15.5l6.9 5.7L24 10l7.6 11.2 6.9-5.7-2.7 17.2H12.2L9.5 15.5z"
        fill="currentColor"
        opacity="0.95"
      />
      <path d="M11.5 29.5h25" stroke="currentColor" stroke-width="1.4" opacity="0.6" />
      <path d="M13 34.5h22" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" opacity="0.9" />
      <circle cx="9.5" cy="15.5" r="2" fill="currentColor" opacity="0.72" />
      <circle cx="24" cy="10" r="2" fill="currentColor" />
      <circle cx="38.5" cy="15.5" r="2" fill="currentColor" opacity="0.72" />
      <path d="M16 23h16" stroke="white" stroke-width="1" stroke-linecap="round" opacity="0.3" />
    </template>

    <!-- Diamond: faceted outline that remains legible at the compact menu size. -->
    <template v-else-if="props.tierKey === 'diamond'">
      <path d="M9 18.5L16 9h16l7 9.5L24 39 9 18.5z" fill="currentColor" opacity="0.9" />
      <path d="M9 18.5h30M16 9l8 30 8-30M9 18.5l15 20 15-20M16 9l8 9.5L32 9" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round" opacity="0.8" />
      <path d="M12 18.5h24M16 12.5h16" stroke="white" stroke-width="1" stroke-linecap="round" opacity="0.3" />
    </template>

    <!-- Safe fallback for a future configured tier. -->
    <path
      v-else
      d="M24 8l4.4 11.6L40 24l-11.6 4.4L24 40l-4.4-11.6L8 24l11.6-4.4L24 8z"
      fill="currentColor"
      opacity="0.92"
    />
  </svg>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
  tierKey: string
  size?: 'sm' | 'md'
}>(), {
  size: 'md'
})

const sizeClass = props.size === 'sm' ? 'h-6 w-6' : 'h-7 w-7'
</script>
