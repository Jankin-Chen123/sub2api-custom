<template>
  <div class="sticker-home" data-sticker-home>
    <!-- Hidden SVG defs for smooth vinyl sticker die-cut filter -->
    <svg class="sticker-svg-defs" width="0" height="0" aria-hidden="true" focusable="false">
      <defs>
        <filter id="sticker-diecut" x="-24%" y="-24%" width="148%" height="148%" color-interpolation-filters="sRGB">
          <feMorphology in="SourceAlpha" operator="dilate" radius="5.2" result="dilated" />
          <feGaussianBlur in="dilated" stdDeviation="1.35" result="smoothed" />
          <feComponentTransfer in="smoothed" result="solid">
            <feFuncA type="table" tableValues="0 0 0.85 1 1 1" />
          </feComponentTransfer>
          <feFlood flood-color="#fffdf7" result="white" />
          <feComposite in="white" in2="solid" operator="in" result="stickerOutline" />
          <feDropShadow
            in="stickerOutline"
            dx="0"
            dy="4"
            stdDeviation="2.4"
            flood-color="#3c2f23"
            flood-opacity="0.22"
            result="outlinedWithShadow"
          />
          <feMerge>
            <feMergeNode in="outlinedWithShadow" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>
    </svg>

    <div class="sticker-cover">
      <div class="sticker-nav-wrap sticker-shell">
        <nav class="sticker-nav" :aria-label="t('home.sticker.navigation')">
          <RouterLink to="/home" class="sticker-brand">
            <img :src="logo" alt="" width="40" height="40" />
            <span>{{ siteName }}</span>
          </RouterLink>
          <div class="sticker-nav-actions">
            <RouterLink v-if="showModelPlaza" to="/model-plaza" class="sticker-nav-plaza" :aria-label="t('nav.modelPlaza')">
              <Icon name="grid" size="sm" />
              <span>{{ t('nav.modelPlaza') }}</span>
            </RouterLink>
            <a
              href="https://qm.qq.com/q/DiCjrJU00i"
              target="_blank"
              rel="noopener noreferrer"
              class="sticker-button sticker-button-qq sticker-button-small"
            >
              <svg class="sticker-nav-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M17.5 11.2c0 3.3-3 6-6.8 6-.9 0-1.8-.15-2.6-.44L4.5 18l1-2.85C4.55 14.1 4 12.7 4 11.2c0-3.3 3-6 6.75-6s6.75 2.7 6.75 6Z"
                  fill="#fffaf0"
                  stroke="currentColor"
                  stroke-width="1.9"
                  stroke-linejoin="round"
                />
                <path
                  d="M14.8 16.65c.85.55 1.9.85 3.05.85.65 0 1.28-.12 1.85-.35l2.3.85-.65-1.95c.65-.75 1.05-1.68 1.05-2.7 0-1.92-1.4-3.56-3.35-4.1"
                  stroke="currentColor"
                  stroke-width="1.9"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              <span>{{ t('home.sticker.joinQqGroup') }}</span>
            </a>
            <button
              type="button"
              class="sticker-sound-toggle"
              :class="{ 'is-muted': !soundEnabled }"
              :aria-pressed="soundEnabled"
              :aria-label="soundEnabled ? t('home.sticker.soundDisable') : t('home.sticker.soundEnable')"
              :title="soundEnabled ? t('home.sticker.soundDisable') : t('home.sticker.soundEnable')"
              @click="toggleSound"
            >
              <svg v-if="soundEnabled" class="sticker-nav-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M11 5.5 6.8 9H4a1 1 0 0 0-1 1v4a1 1 0 0 0 1 1h2.8L11 18.5V5.5Z"
                  fill="#ffe7da"
                  stroke="currentColor"
                  stroke-width="1.9"
                  stroke-linejoin="round"
                />
                <path
                  d="M15.2 8.8a4.5 4.5 0 0 1 0 6.4M18 6a8.4 8.4 0 0 1 0 12"
                  stroke="currentColor"
                  stroke-width="1.9"
                  stroke-linecap="round"
                />
              </svg>
              <svg v-else class="sticker-nav-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M11 5.5 6.8 9H4a1 1 0 0 0-1 1v4a1 1 0 0 0 1 1h2.8L11 18.5V5.5Z"
                  fill="#f3e9d8"
                  stroke="currentColor"
                  stroke-width="1.9"
                  stroke-linejoin="round"
                />
                <path
                  d="m15.5 9.5 5 5m0-5-5 5"
                  stroke="currentColor"
                  stroke-width="1.9"
                  stroke-linecap="round"
                />
              </svg>
              <span>{{ soundEnabled ? t('home.sticker.soundOn') : t('home.sticker.soundOff') }}</span>
            </button>
          </div>
        </nav>
      </div>

      <main class="sticker-hero sticker-shell">
        <div class="sticker-title-wrap">
          <h1 class="sticker-title" :class="{ 'sticker-title-brand': isBrandTitle }">
            <template v-if="isBrandTitle"><span>爱白嫖</span><span>公益站</span></template>
            <template v-else>{{ siteName }}</template>
          </h1>
          <CraftSparks variant="sticker" size="lg" class="sticker-title-sparks" />
          <svg class="sticker-title-underline" viewBox="0 0 260 18" fill="none" aria-hidden="true">
            <path d="M6 12C76 6 178 5 254 11" stroke="#fffdf7" stroke-width="11" stroke-linecap="round" />
            <path d="M6 12C76 6 178 5 254 11" stroke="#ff9466" stroke-width="5.5" stroke-linecap="round" />
          </svg>
        </div>

        <div class="sticker-mascot-wrap">
          <button
            ref="mascotButtonRef"
            type="button"
            class="sticker-mascot-trigger"
            :class="{
              'is-hovered': mascotHovered,
              'is-wobbling': mascotWobbling,
              'is-slapping': mascotSlapping,
            }"
            :style="mascotMotionStyle"
            :aria-label="t('home.sticker.mascot')"
            @pointerenter="handleMascotPointerEnter"
            @pointermove="handleMascotPointerMove"
            @pointerleave="handleMascotPointerLeave"
            @click="handleMascotClick"
          >
            <span class="sticker-mascot-float">
              <span class="sticker-mascot-tilt">
                <span class="sticker-mascot-slap">
                  <img
                    class="sticker-mascot"
                    src="/sticker/mascot.webp"
                    width="560"
                    height="560"
                    :alt="t('home.sticker.mascot')"
                    draggable="false"
                    fetchpriority="high"
                  />
                </span>
              </span>
            </span>

            <span class="sticker-mascot-fx" aria-hidden="true">
              <span
                v-for="burst in mascotBursts"
                :key="burst.id"
                class="sticker-mascot-burst"
                :style="{ left: `${burst.x}%`, top: `${burst.y}%` }"
              >
                <svg class="sticker-mascot-ripple" viewBox="0 0 180 180" fill="none">
                  <circle
                    cx="90"
                    cy="90"
                    r="68"
                    stroke="#fffdf7"
                    stroke-width="11"
                    stroke-linecap="round"
                    stroke-dasharray="28 16 44 18 22 14"
                  />
                  <circle
                    cx="90"
                    cy="90"
                    r="68"
                    :stroke="burst.accent"
                    stroke-width="5.5"
                    stroke-linecap="round"
                    stroke-dasharray="28 16 44 18 22 14"
                  />
                </svg>
                <svg class="sticker-mascot-rays" viewBox="0 0 180 180" fill="none">
                  <g stroke="#fffdf7" stroke-width="8" stroke-linecap="round">
                    <line x1="90" y1="14" x2="90" y2="30" />
                    <line x1="154" y1="50" x2="140" y2="58" />
                    <line x1="156" y1="126" x2="142" y2="118" />
                    <line x1="90" y1="166" x2="90" y2="150" />
                    <line x1="26" y1="128" x2="40" y2="120" />
                    <line x1="24" y1="52" x2="38" y2="60" />
                  </g>
                  <g :stroke="burst.accent" stroke-width="4" stroke-linecap="round">
                    <line x1="90" y1="14" x2="90" y2="30" />
                    <line x1="154" y1="50" x2="140" y2="58" />
                    <line x1="156" y1="126" x2="142" y2="118" />
                    <line x1="90" y1="166" x2="90" y2="150" />
                    <line x1="26" y1="128" x2="40" y2="120" />
                    <line x1="24" y1="52" x2="38" y2="60" />
                  </g>
                </svg>
                <span
                  class="sticker-mascot-stamp"
                  :class="`is-tone-${burst.tone}`"
                  :style="{ '--stamp-rot': `${burst.rot}deg` }"
                >
                  <span class="sticker-mascot-stamp-inner">
                    <span class="sticker-mascot-stamp-text">{{ burst.label }}</span>
                  </span>
                </span>
              </span>
            </span>
          </button>
        </div>

        <div class="sticker-intro">
          <p>{{ t('home.sticker.tagline') }}</p>
          <div class="sticker-actions">
            <RouterLink :to="entryPath" class="sticker-button sticker-button-ink">
              {{ signedIn ? t('home.goToDashboard') : t('home.sticker.start') }}
              <Icon name="arrowRight" size="md" aria-hidden="true" />
            </RouterLink>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="sticker-button sticker-button-docs"
            >
              <CraftIcon name="book" size="sm" />
              <span>{{ t('home.sticker.docs') }}</span>
              <Icon name="chevronRight" size="sm" aria-hidden="true" />
            </a>
          </div>
        </div>
      </main>

      <!-- Real generated layered torn cotton-paper divider between sky-blue cover, apricot strip and cream notebook paper -->
      <img class="sticker-cover-divider" src="/sticker/torn-hero-divider.webp" alt="" aria-hidden="true" />
    </div>

    <section class="sticker-benefits sticker-shell" aria-labelledby="sticker-benefits-title">
      <div class="sticker-section-head sticker-reveal">
        <CraftSparks variant="left" size="md" class="sticker-head-spark" />
        <h2 id="sticker-benefits-title" class="sticker-section-title">{{ t('home.sticker.benefits') }}</h2>
        <CraftSparks variant="right" size="md" class="sticker-head-spark" />
        <svg class="sticker-section-underline" viewBox="0 0 180 14" fill="none" aria-hidden="true">
          <path d="M6 9C58 4 122 4 174 9" stroke="#ff9466" stroke-width="5" stroke-linecap="round" />
        </svg>
      </div>

      <div class="sticker-benefit-grid">
        <article
          v-for="(feature, index) in features"
          :key="feature"
          class="sticker-benefit sticker-reveal"
          :class="`sticker-benefit--${feature}`"
          :style="{ '--reveal-delay': `${index * 80}ms` }"
        >
          <div class="sticker-feature-art" aria-hidden="true">
            <img class="sticker-cloud-img" :class="`sticker-cloud-img--${index}`" src="/sticker/watercolor-cloud.webp" alt="" />
            <span v-if="feature === 'verification'" class="sticker-stamp-impact-ring" />
            <div class="sticker-feature-stage" :class="`sticker-feature-stage--${feature}`">
              <div class="sticker-feature-sprite" :class="`sticker-feature-sprite--${feature}`" />
            </div>
          </div>
          <div class="sticker-benefit-copy">
            <h3>{{ t(`home.sticker.${feature}.title`) }}</h3>
            <p>{{ t(`home.sticker.${feature}.description`) }}</p>
          </div>
        </article>
      </div>
    </section>

    <section class="sticker-setup-section sticker-shell sticker-reveal" aria-labelledby="sticker-setup-title">
      <div class="sticker-setup">
        <div class="sticker-setup-head">
          <div class="sticker-setup-title-wrap">
            <h2 id="sticker-setup-title" class="sticker-setup-title">{{ t('home.sticker.setup') }}</h2>
            <CraftSparks variant="sticker" size="sm" class="sticker-setup-sparks" />
          </div>
          <span class="sticker-setup-divider" aria-hidden="true" />
        </div>

        <ol class="sticker-setup-steps">
          <li>
            <CraftIcon name="user" size="lg" />
            <span>{{ t('home.sticker.signIn') }}</span>
          </li>
          <li>
            <CraftIcon name="key" size="lg" />
            <span>{{ t('dashboard.createApiKey') }}</span>
          </li>
          <li>
            <CraftIcon name="code" size="lg" />
            <span>{{ t('home.sticker.request') }}</span>
          </li>
        </ol>
      </div>
    </section>

    <footer class="sticker-footer sticker-shell">
      <RouterLink to="/home" class="sticker-brand"><img :src="logo" alt="" width="32" height="32" />{{ siteName }}</RouterLink>
      <span>{{ t('home.sticker.footer') }}</span>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import CraftSparks from '@/components/craft/CraftSparks.vue'
import CraftIcon from '@/components/craft/CraftIcon.vue'
import {
  isPaperSoundEnabled,
  playMascotHighFiveSound,
  playPaperSoundToggleCue,
  setPaperSoundEnabled,
} from '@/features/paper-transition/paperSound'

interface MascotStampBurst {
  id: number
  x: number
  y: number
  label: string
  tone: 'coral' | 'teal' | 'gold'
  accent: string
  rot: number
}

const MASCOT_STAMPS: ReadonlyArray<{
  label: string
  tone: 'coral' | 'teal' | 'gold'
  accent: string
}> = [
  { label: 'OK ✓', tone: 'coral', accent: '#ff7d4d' },
  { label: 'NICE!', tone: 'teal', accent: '#12a48a' },
  { label: 'FREE ★', tone: 'gold', accent: '#ef9f26' },
  { label: 'HIGH-5!', tone: 'coral', accent: '#ff7d4d' },
]

const STAMP_ANGLES = [-13, 9, -8, 12] as const

const props = defineProps<{
  siteName: string
  siteLogo: string
  docUrl: string
  signedIn: boolean
  dashboardPath: string
  showModelPlaza: boolean
}>()

const { t } = useI18n()
const logo = computed(() => props.siteLogo || '/brand-icon.png')
const entryPath = computed(() => (props.signedIn ? props.dashboardPath : '/login'))
const isBrandTitle = computed(() => props.siteName === '爱白嫖公益站')
const features = ['pricing', 'routing', 'verification'] as const
const soundEnabled = ref(isPaperSoundEnabled())

const mascotButtonRef = ref<HTMLButtonElement | null>(null)
const mascotHovered = ref(false)
const mascotWobbling = ref(false)
const mascotSlapping = ref(false)
const mascotTilt = ref({ tx: 0, ty: 0, rx: 0, ry: 0, sx: 0, sy: 18 })
const mascotBursts = ref<MascotStampBurst[]>([])

const mascotMotionStyle = computed<Record<string, string>>(() => ({
  '--mascot-tx': `${mascotTilt.value.tx.toFixed(2)}px`,
  '--mascot-ty': `${mascotTilt.value.ty.toFixed(2)}px`,
  '--mascot-rx': `${mascotTilt.value.rx.toFixed(2)}deg`,
  '--mascot-ry': `${mascotTilt.value.ry.toFixed(2)}deg`,
  '--mascot-sx': `${mascotTilt.value.sx.toFixed(2)}px`,
  '--mascot-sy': `${mascotTilt.value.sy.toFixed(2)}px`,
}))

let burstSeq = 0
let slapSeq = 0
let lastClickAt = 0
let comboCount = 0
let wobbleTimer: ReturnType<typeof setTimeout> | undefined
let slapTimer: ReturnType<typeof setTimeout> | undefined
const burstTimers = new Set<ReturnType<typeof setTimeout>>()

function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )
}

function updateMascotTilt(event: PointerEvent) {
  if (prefersReducedMotion()) return
  const el = mascotButtonRef.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) {
    mascotTilt.value = { tx: 0, ty: -8, rx: 0, ry: 0, sx: 0, sy: 24 }
    return
  }
  const rawNx = ((event.clientX - rect.left) / rect.width) * 2 - 1
  const rawNy = ((event.clientY - rect.top) / rect.height) * 2 - 1
  const nx = Math.max(-1, Math.min(1, rawNx))
  const ny = Math.max(-1, Math.min(1, rawNy))
  mascotTilt.value = {
    tx: nx * 12,
    ty: ny * 10 - 8,
    rx: -ny * 12,
    ry: nx * 14,
    sx: -nx * 14,
    sy: 24 - ny * 10,
  }
}

function handleMascotPointerEnter(event: PointerEvent) {
  if (event.pointerType === 'touch') return
  mascotHovered.value = true
  updateMascotTilt(event)
  if (prefersReducedMotion() || mascotSlapping.value) return
  if (wobbleTimer !== undefined) {
    clearTimeout(wobbleTimer)
  }
  mascotWobbling.value = true
  wobbleTimer = setTimeout(() => {
    wobbleTimer = undefined
    mascotWobbling.value = false
  }, 440)
}

function handleMascotPointerMove(event: PointerEvent) {
  if (event.pointerType === 'touch') return
  mascotHovered.value = true
  updateMascotTilt(event)
}

function handleMascotPointerLeave() {
  mascotHovered.value = false
  mascotWobbling.value = false
  if (wobbleTimer !== undefined) {
    clearTimeout(wobbleTimer)
    wobbleTimer = undefined
  }
  mascotTilt.value = { tx: 0, ty: 0, rx: 0, ry: 0, sx: 0, sy: 18 }
}

async function handleMascotClick(event: MouseEvent) {
  let x = 52
  let y = 46
  const el = mascotButtonRef.value
  if (el && (event.clientX !== 0 || event.clientY !== 0)) {
    const rect = el.getBoundingClientRect()
    if (rect.width > 0 && rect.height > 0) {
      const px = ((event.clientX - rect.left) / rect.width) * 100
      const py = ((event.clientY - rect.top) / rect.height) * 100
      x = Math.max(26, Math.min(74, px))
      y = Math.max(24, Math.min(68, py))
    }
  }

  const idx = burstSeq++
  const preset = MASCOT_STAMPS[idx % MASCOT_STAMPS.length]
  const rot = STAMP_ANGLES[idx % STAMP_ANGLES.length]
  const burst: MascotStampBurst = {
    id: idx + 1,
    x: Number(x.toFixed(1)),
    y: Number(y.toFixed(1)),
    label: preset.label,
    tone: preset.tone,
    accent: preset.accent,
    rot,
  }

  const nextBursts = [...mascotBursts.value, burst]
  mascotBursts.value = nextBursts.length > 5 ? nextBursts.slice(-5) : nextBursts

  const removeTimer = setTimeout(() => {
    burstTimers.delete(removeTimer)
    mascotBursts.value = mascotBursts.value.filter((item) => item.id !== burst.id)
  }, 1200)
  burstTimers.add(removeTimer)

  const now = Date.now()
  comboCount = now - lastClickAt < 1400 ? comboCount + 1 : 0
  lastClickAt = now
  playMascotHighFiveSound(comboCount)

  if (wobbleTimer !== undefined) {
    clearTimeout(wobbleTimer)
    wobbleTimer = undefined
  }
  if (slapTimer !== undefined) {
    clearTimeout(slapTimer)
    slapTimer = undefined
  }

  mascotWobbling.value = false
  mascotSlapping.value = false
  if (!prefersReducedMotion()) {
    const token = ++slapSeq
    await nextTick()
    if (token !== slapSeq) return
    void mascotButtonRef.value?.offsetWidth
    mascotSlapping.value = true
    slapTimer = setTimeout(() => {
      slapTimer = undefined
      mascotSlapping.value = false
    }, 600)
  }
}

function toggleSound() {
  const next = !soundEnabled.value
  soundEnabled.value = next
  setPaperSoundEnabled(next)
  if (next) {
    playPaperSoundToggleCue()
  }
}

let observer: IntersectionObserver | undefined

onMounted(() => {
  const nodes = Array.from(document.querySelectorAll<HTMLElement>('.sticker-reveal'))
  if (!nodes.length) return

  if (prefersReducedMotion() || typeof IntersectionObserver === 'undefined') {
    nodes.forEach((node) => node.classList.add('is-visible'))
    return
  }

  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-visible')
          observer?.unobserve(entry.target)
        }
      }
    },
    { threshold: 0.12, rootMargin: '0px 0px -24px 0px' }
  )

  nodes.forEach((node) => observer?.observe(node))
})

onBeforeUnmount(() => {
  observer?.disconnect()
  if (wobbleTimer !== undefined) {
    clearTimeout(wobbleTimer)
    wobbleTimer = undefined
  }
  if (slapTimer !== undefined) {
    clearTimeout(slapTimer)
    slapTimer = undefined
  }
  for (const timer of burstTimers) {
    clearTimeout(timer)
  }
  burstTimers.clear()
})
</script>

<style scoped src="./sticker-home.css"></style>
