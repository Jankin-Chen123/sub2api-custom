<template>
  <div
    class="docs-lion-floating"
    :class="{ 'docs-lion-floating-hidden': !visible }"
    :aria-hidden="!visible"
    aria-label="互动小狮子"
  >
    <div ref="canvasRef" class="chill-lion-canvas" role="img" aria-label="互动小狮子"></div>
    <div v-if="unavailable" class="chill-lion-fallback" role="status">
      <span aria-hidden="true">🦁</span>
      <span>互动小狮子暂不可用</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { mountChillLionWhenReady } from '@/utils/chillLion'

withDefaults(defineProps<{ visible?: boolean }>(), { visible: true })

const canvasRef = ref<HTMLElement | null>(null)
const unavailable = ref(false)
let cleanup: (() => void) | null = null
let disposed = false
let startFrame = 0

async function initialize() {
  if (!canvasRef.value) return
  try {
    const destroy = await mountChillLionWhenReady(canvasRef.value)
    if (disposed) {
      destroy()
    } else {
      cleanup = destroy
    }
  } catch {
    // Keep a visible, accessible fallback if WebGL or the runtime is unavailable.
    unavailable.value = true
  }
}

onMounted(() => {
  // Let the documentation view start its text/API work and paint the first
  // frame before evaluating the Three.js chunk.
  startFrame = requestAnimationFrame(() => {
    startFrame = 0
    void initialize()
  })
})

onBeforeUnmount(() => {
  disposed = true
  if (startFrame) cancelAnimationFrame(startFrame)
  cleanup?.()
  cleanup = null
})
</script>

<style scoped>
.docs-lion-floating { position: fixed; z-index: 45; right: 28px; bottom: 24px; width: 300px; height: clamp(220px,30vh,360px); overflow: hidden; border: 1px solid rgba(99,102,241,.18); border-radius: 18px; background: #ebe5e7; box-shadow: 0 18px 48px rgba(64,74,145,.16); transition: opacity .2s ease,visibility .2s ease; }
.dark .docs-lion-floating { border-color: rgba(129,140,248,.26); box-shadow: 0 20px 54px rgba(0,0,0,.3); }
.docs-lion-floating-hidden { visibility: hidden; opacity: 0; pointer-events: none; }
.chill-lion-canvas { position: relative; width: 100%; height: 100%; overflow: hidden; touch-action: none; background: #ebe5e7; }
.chill-lion-canvas :deep(canvas) { display: block; width: 100%; height: 100%; }
.chill-lion-fallback { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; gap: 8px; color: #653f4c; background: #ebe5e7; font-size: 13px; font-weight: 650; }
.chill-lion-fallback span:first-child { font-size: 32px; }
.dark .chill-lion-fallback { color: #fdd276; background: #2d2630; }
@media (max-width: 1180px) { .docs-lion-floating { display: none; } }
</style>
