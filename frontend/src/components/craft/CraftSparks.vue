<template>
  <svg
    class="craft-sparks"
    :class="[`craft-sparks--${variant}`, `craft-sparks--${size}`]"
    viewBox="0 0 32 30"
    fill="none"
    aria-hidden="true"
  >
    <g
      v-if="variant === 'sticker'"
      stroke="#fffdf7"
      stroke-width="8"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M7.5 11.5 L12.5 4.8" />
      <path d="M11.5 17.2 L24.5 11.2" />
      <path d="M12.2 22.8 L22.2 21.6" />
    </g>
    <g
      :stroke="variant === 'sticker' ? '#ff9162' : 'var(--craft-peach, #ff9162)'"
      stroke-width="3.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M7.5 11.5 L12.5 4.8" />
      <path d="M11.5 17.2 L24.5 11.2" />
      <path d="M12.2 22.8 L22.2 21.6" />
    </g>
  </svg>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    variant?: 'right' | 'left' | 'sticker'
    size?: 'sm' | 'md' | 'lg'
  }>(),
  {
    variant: 'right',
    size: 'md',
  }
)
</script>

<style scoped>
.craft-sparks {
  display: inline-block;
  flex: none;
  vertical-align: middle;
  pointer-events: none;
  overflow: visible;
}

.craft-sparks--sm {
  width: 22px;
  height: 20px;
}

.craft-sparks--md {
  width: 28px;
  height: 26px;
}

.craft-sparks--lg {
  width: 36px;
  height: 34px;
}

.craft-sparks--left {
  transform: scaleX(-1);
}

.craft-sparks--sticker {
  filter: drop-shadow(0 2px 3px rgba(61, 44, 28, 0.16));
}
</style>
