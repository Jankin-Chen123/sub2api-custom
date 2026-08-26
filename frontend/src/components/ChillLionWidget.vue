<template>
  <div ref="canvasRef" class="chill-lion-canvas" role="img" aria-label="互动小狮子"></div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { mountChillLionWhenReady } from '@/utils/chillLion'

const canvasRef = ref<HTMLElement | null>(null)
let cleanup: (() => void) | null = null
let disposed = false

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
    // Keep the reserved area quiet if WebGL or the runtime is unavailable.
  }
}

onMounted(() => {
  void initialize()
})

onBeforeUnmount(() => {
  disposed = true
  cleanup?.()
  cleanup = null
})
</script>

<style scoped>
.chill-lion-canvas { position: relative; width: 100%; height: 100%; overflow: hidden; touch-action: none; background: #ebe5e7; }
.chill-lion-canvas :deep(canvas) { display: block; width: 100%; height: 100%; }
</style>
