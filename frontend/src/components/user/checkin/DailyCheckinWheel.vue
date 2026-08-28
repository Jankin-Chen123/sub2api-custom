<template>
  <section class="card flex h-full flex-col overflow-hidden">
    <div class="checkin-surface flex min-h-0 flex-1 flex-col p-5 sm:p-6">
      <div class="flex items-start justify-between gap-3">
        <div class="flex items-center gap-3">
          <div class="checkin-icon flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl">
            <Icon name="sparkles" size="md" />
          </div>
          <div>
            <h2 class="text-xl font-black tracking-tight text-gray-900 dark:text-white">
              {{ t('redeem.checkin.title') }}
            </h2>
          </div>
        </div>
        <span class="today-pill rounded-full px-3 py-1.5 text-xs font-bold">
          {{ status?.checked_today ? t('redeem.checkin.checkedToday') : t('redeem.checkin.checkinButton') }}
        </span>
      </div>

      <div v-if="loading" class="flex flex-1 items-center justify-center py-16">
        <span class="h-9 w-9 animate-spin rounded-full border-4 border-primary-100 border-t-primary-500" />
      </div>

      <template v-else-if="status && status.prizes.length">
        <div class="streak-card mt-5 rounded-2xl p-4">
          <div class="flex items-end justify-between gap-3">
            <div>
              <p class="text-xs font-semibold text-gray-500 dark:text-dark-400">
                {{ t('redeem.checkin.streakLabel') }}
              </p>
              <p class="mt-1 text-2xl font-black tracking-tight text-gray-900 dark:text-white">
                {{ t('redeem.checkin.streakDays', { days: streakDays }) }}
              </p>
            </div>
            <p class="streak-hint text-right text-xs font-bold">
              {{ streakHint }}
            </p>
          </div>

          <div class="mt-4 grid gap-1.5" :style="{ gridTemplateColumns: `repeat(${streakTarget}, minmax(0, 1fr))` }">
            <div
              v-for="day in streakTarget"
              :key="day"
              :class="[
                'streak-day relative flex flex-col items-center gap-1.5 text-[10px] font-semibold',
                day <= streakDays ? 'is-done' : '',
                day === Math.min(streakDays + 1, streakTarget) ? 'is-current' : '',
                day === streakTarget ? 'is-bonus' : ''
              ]"
            >
              <span class="day-dot flex h-7 w-7 items-center justify-center rounded-full text-xs font-black">
                {{ day <= streakDays ? '✓' : day }}
              </span>
              <span>{{ t('redeem.checkin.streakDay', { day }) }}</span>
            </div>
          </div>
        </div>

        <div class="checkin-content mt-5 flex min-h-0 flex-1 items-center justify-center gap-5 xl:gap-8">
          <div class="wheel-stage relative h-[218px] w-[218px] flex-shrink-0 sm:h-[230px] sm:w-[230px]">
            <svg class="wheel-pointer z-20" viewBox="0 0 32 46" aria-hidden="true">
              <path d="M 1.5 2.5 Q 16 0 30.5 2.5 L 18.4 40.1 Q 16 46 13.6 40.1 Z" />
            </svg>
            <div class="wheel-shell absolute inset-1 rounded-full p-2">
              <svg
                viewBox="0 0 300 300"
                class="h-full w-full rounded-full"
                :style="wheelStyle"
                :aria-label="t('redeem.checkin.title')"
              >
                <g v-for="(prize, index) in status.prizes" :key="prize.id">
                  <title>{{ prize.name }}</title>
                  <path
                    :d="segmentPath(index, status.prizes.length)"
                    :fill="prize.color"
                    stroke="rgba(255,255,255,.86)"
                    stroke-width="2"
                  />
                  <text
                    :x="labelPoint(index, status.prizes.length).x"
                    :y="labelPoint(index, status.prizes.length).y"
                    text-anchor="middle"
                    dominant-baseline="middle"
                    fill="white"
                    class="wheel-label"
                  >
                    {{ shortLabel(prize.name) }}
                  </text>
                </g>
              </svg>
            </div>
            <div class="wheel-center absolute left-1/2 top-1/2 z-10 flex h-[60px] w-[60px] -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border-4 border-white text-center text-xs font-black text-white">
              {{ spinning ? t('redeem.checkin.spinning') : t('redeem.checkin.lucky') }}
            </div>
          </div>

          <div class="draw-copy min-w-0 max-w-[185px]">
            <p class="text-[10px] font-black tracking-[0.12em] text-gray-400 dark:text-dark-500">
              TODAY'S LUCKY DRAW
            </p>
            <h3 class="mt-1.5 text-xl font-black tracking-tight text-gray-900 dark:text-white">
              {{ t('redeem.checkin.drawTitle') }}
            </h3>
            <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">
              {{ t('redeem.checkin.drawHint') }}
            </p>
          </div>
        </div>

        <button
          type="button"
          class="checkin-button mt-5 w-full rounded-xl px-5 py-3 text-sm font-black text-white transition disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="!status.can_checkin || spinning"
          @click="startDraw"
        >
          {{ buttonLabel }}
        </button>
      </template>

      <div v-else class="flex flex-1 items-center justify-center py-12 text-center text-sm text-gray-500 dark:text-dark-400">
        {{ t('redeem.checkin.unavailable') }}
      </div>
    </div>
  </section>

  <Teleport to="body">
    <div
      v-if="successResult"
      class="success-modal fixed inset-0 z-[100] grid place-items-center p-5"
      role="presentation"
      @click.self="closeSuccess"
    >
      <div class="confetti-layer pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
        <span
          v-for="index in 40"
          :key="index"
          class="confetti-piece"
          :style="confettiStyle(index)"
        />
      </div>

      <section
        class="success-dialog relative z-10 w-full max-w-sm rounded-3xl p-7 text-center"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="successTitleId"
      >
        <button
          type="button"
          class="success-close absolute right-3 top-3 flex h-8 w-8 items-center justify-center rounded-full text-lg"
          :aria-label="t('redeem.checkin.successClose')"
          @click="closeSuccess"
        >
          ×
        </button>
        <div class="celebration-mark mx-auto flex h-[68px] w-[68px] items-center justify-center rounded-full">
          <span>✓</span>
        </div>
        <p class="mt-4 text-xs font-black tracking-[0.14em] text-primary-600 dark:text-primary-300">
          {{ t('redeem.checkin.successTitle') }}
        </p>
        <h2 :id="successTitleId" class="mt-2 text-2xl font-black tracking-tight text-gray-900 dark:text-white">
          {{ t('redeem.checkin.successHeading') }}
        </h2>
        <p class="success-amount mt-3 text-4xl font-black tracking-tight">
          +${{ totalReward.toFixed(2) }}
        </p>
        <p class="mt-1 text-sm font-bold text-gray-600 dark:text-dark-300">
          {{ successResult.prize_name }}
        </p>
        <p
          v-if="successResult.bonus_amount > 0"
          class="mt-2 text-xs font-semibold text-amber-600 dark:text-amber-300"
        >
          {{ t('redeem.checkin.successBonus', { amount: successResult.bonus_amount.toFixed(2) }) }}
        </p>
        <div class="success-balance mt-5 rounded-xl px-3 py-2.5 text-xs font-bold">
          {{ t('redeem.checkin.successBalance') }}
        </div>
        <button
          type="button"
          class="success-confirm mt-4 w-full rounded-xl px-4 py-3 text-sm font-black text-white"
          @click="closeSuccess"
        >
          {{ t('redeem.checkin.successClose') }}
        </button>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { checkinAPI, type CheckinResult, type CheckinStatus } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const emit = defineEmits<{
  rewardAdded: []
}>()

const status = ref<CheckinStatus | null>(null)
const loading = ref(true)
const spinning = ref(false)
const rotation = ref(0)
const successResult = ref<CheckinResult | null>(null)
const successTitleId = 'daily-checkin-success-title'

const streakTarget = computed(() => Math.max(2, status.value?.streak_target || 7))
const streakDays = computed(() => Math.max(0, status.value?.streak_days || 0))
const streakHint = computed(() => {
  if (status.value?.checked_today && (status.value.today_result?.bonus_amount ?? 0) > 0) {
    return t('redeem.checkin.streakBonusUnlocked')
  }
  if (streakDays.value === 0) return t('redeem.checkin.streakStartHint')
  const days = status.value?.days_until_bonus || streakTarget.value - (streakDays.value % streakTarget.value)
  return t('redeem.checkin.streakDaysUntilBonus', { days: days || streakTarget.value })
})
const totalReward = computed(() => {
  const result = successResult.value
  if (!result) return 0
  return result.total_amount ?? result.amount + (result.bonus_amount || 0)
})
const wheelStyle = computed(() => ({
  transform: `rotate(${rotation.value}deg)`,
  transition: spinning.value ? 'transform 4.6s cubic-bezier(0.12, 0.72, 0.18, 1)' : 'none'
}))
const buttonLabel = computed(() => {
  if (spinning.value) return t('redeem.checkin.drawing')
  if (status.value?.checked_today) return t('redeem.checkin.checkedToday')
  return t('redeem.checkin.checkinButton')
})

const polarPoint = (angle: number, radius: number) => {
  const radians = ((angle - 90) * Math.PI) / 180
  return { x: 150 + radius * Math.cos(radians), y: 150 + radius * Math.sin(radians) }
}

const segmentPath = (index: number, count: number) => {
  const start = (index * 360) / count
  const end = ((index + 1) * 360) / count
  const a = polarPoint(start, 142)
  const b = polarPoint(end, 142)
  const largeArc = end - start > 180 ? 1 : 0
  return `M 150 150 L ${a.x} ${a.y} A 142 142 0 ${largeArc} 1 ${b.x} ${b.y} Z`
}

const labelPoint = (index: number, count: number) =>
  polarPoint(((index + 0.5) * 360) / count, count > 10 ? 103 : 96)

const shortLabel = (value: string) => {
  const chars = Array.from(value)
  return chars.length > 10 ? `${chars.slice(0, 9).join('')}…` : value
}

const loadStatus = async () => {
  loading.value = true
  try {
    status.value = await checkinAPI.getStatus()
  } catch (error) {
    console.error('Failed to load check-in status:', error)
  } finally {
    loading.value = false
  }
}

const confettiColors = ['#f77b86', '#f6bd4e', '#5ea9ec', '#41c99f', '#a978e3']
const confettiStyle = (index: number) => ({
  '--left': `${(index * 37) % 101}%`,
  '--duration': `${3.8 + ((index * 13) % 14) / 10}s`,
  '--delay': `${-((index * 7) % 42) / 10}s`,
  '--drift': `${((index * 29) % 111) - 55}px`,
  '--color': confettiColors[index % confettiColors.length],
  '--width': `${6 + (index % 5)}px`,
  '--height': `${15 + ((index * 11) % 12)}px`
})

const closeSuccess = () => {
  successResult.value = null
}

const startDraw = async () => {
  if (!status.value?.can_checkin || spinning.value) return
  spinning.value = true
  successResult.value = null
  try {
    const result = await checkinAPI.draw()
    const count = status.value.prizes.length
    const winnerIndex = Math.max(0, status.value.prizes.findIndex((prize) => prize.id === result.prize_id))
    const winnerCenter = ((winnerIndex + 0.5) * 360) / count
    const currentModulo = ((rotation.value % 360) + 360) % 360
    const targetModulo = ((360 - winnerCenter) % 360 + 360) % 360
    const alignment = (targetModulo - currentModulo + 360) % 360
    rotation.value += 5 * 360 + alignment

    const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
    await new Promise((resolve) => window.setTimeout(resolve, reducedMotion ? 350 : 4700))

    const enrichedResult: CheckinResult = {
      ...result,
      bonus_amount: result.bonus_amount || 0,
      total_amount: result.total_amount ?? result.amount + (result.bonus_amount || 0),
      streak_days: result.streak_days || streakDays.value + 1
    }
    const nextDaysUntilBonus =
      streakTarget.value - (enrichedResult.streak_days % streakTarget.value) || streakTarget.value
    successResult.value = enrichedResult
    status.value = {
      ...status.value,
      checked_today: true,
      can_checkin: false,
      streak_days: enrichedResult.streak_days,
      days_until_bonus: nextDaysUntilBonus,
      today_result: enrichedResult
    }
    await authStore.refreshUser()
    emit('rewardAdded')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('redeem.checkin.failed'))
    await loadStatus()
  } finally {
    spinning.value = false
  }
}

onMounted(loadStatus)
</script>

<style scoped>
.checkin-surface {
  background: linear-gradient(138deg, #f2fffd 0%, #ffffff 54%, #f9fbff 100%);
}

.checkin-icon {
  background: #d9faf5;
  color: #0b9e93;
}

.today-pill {
  background: #e4faf6;
  color: #0b998e;
}

.streak-card {
  border: 1px solid #dcefeb;
  background: rgba(255, 255, 255, 0.86);
}

.streak-hint {
  color: #0c9a8f;
}

.streak-day {
  color: #a0adba;
}

.day-dot {
  border: 1px solid #dce7ef;
  background: #f8fafc;
  color: #94a4b5;
}

.streak-day.is-done .day-dot {
  border-color: #b4e8df;
  background: #d9faf5;
  color: #0b9d91;
}

.streak-day.is-current .day-dot {
  border: 2px solid #0fafa2;
  background: #ffffff;
  color: #078d84;
  box-shadow: 0 0 0 4px #dff8f4;
}

.streak-day.is-bonus::after {
  content: '奖';
  position: absolute;
  right: 4px;
  top: -7px;
  display: grid;
  width: 14px;
  height: 14px;
  place-items: center;
  border-radius: 50%;
  background: #f5b544;
  color: white;
  font-size: 8px;
  font-weight: 900;
}

.wheel-shell {
  background: #ffffff;
  box-shadow: 0 14px 26px rgba(56, 79, 100, 0.2), 0 0 0 4px rgba(255, 255, 255, 0.6);
}

.wheel-label {
  font-size: 10px;
  font-weight: 900;
  paint-order: stroke;
  stroke: rgba(0, 0, 0, 0.18);
  stroke-width: 2px;
  stroke-linejoin: round;
}

.wheel-pointer {
  /* A shallow curved tail replaces a small section of the rim and the tip points into the segment. */
  position: absolute;
  top: 0;
  left: 50%;
  width: 32px;
  height: 46px;
  transform: translateX(-50%);
  filter: drop-shadow(0 3px 3px rgba(56, 48, 54, 0.17));
}

.wheel-pointer path {
  fill: #ec6d78;
}

.wheel-center {
  background: linear-gradient(135deg, #ffb526, #ff8e1a);
  box-shadow: 0 6px 14px rgba(155, 89, 0, 0.24);
}

.checkin-button,
.success-confirm {
  background: #0fafa2;
  box-shadow: 0 9px 18px rgba(15, 175, 162, 0.22);
}

.checkin-button:hover,
.success-confirm:hover {
  background: #078d84;
}

.success-modal {
  background: rgba(20, 41, 53, 0.46);
  backdrop-filter: blur(5px);
}

.success-dialog {
  border: 1px solid rgba(255, 255, 255, 0.72);
  background: linear-gradient(145deg, #ffffff, #f2fffc);
  box-shadow: 0 28px 70px rgba(17, 53, 69, 0.28);
  animation: modal-pop 0.34s cubic-bezier(0.18, 0.85, 0.3, 1.25) both;
}

.success-close {
  background: #eef7f6;
  color: #65908d;
}

.celebration-mark {
  position: relative;
  background: radial-gradient(circle at 35% 30%, #f7fffe, #d9faf5);
  box-shadow: 0 8px 18px rgba(15, 175, 162, 0.16);
}

.celebration-mark::before {
  content: '';
  position: absolute;
  width: 36px;
  height: 36px;
  background: linear-gradient(135deg, #ffd166, #f2a93b);
  clip-path: polygon(50% 0%, 61% 35%, 98% 35%, 68% 57%, 79% 96%, 50% 73%, 21% 96%, 32% 57%, 2% 35%, 39% 35%);
}

.celebration-mark::after {
  content: '';
  position: absolute;
  top: 8px;
  right: 11px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #ef7280;
  box-shadow: -43px 22px 0 #5ca9ec, 34px 38px 0 #35c99b;
}

.celebration-mark span {
  position: relative;
  z-index: 1;
  color: white;
  font-size: 19px;
  font-weight: 950;
  text-shadow: 0 1px 2px rgba(153, 101, 20, 0.25);
}

.success-amount {
  color: #0a9d91;
}

.success-balance {
  border: 1px solid #d5f0eb;
  background: #ecfbf8;
  color: #40827c;
}

.confetti-piece {
  position: absolute;
  top: -48px;
  left: var(--left);
  display: block;
  width: var(--width);
  height: var(--height);
  border-radius: 3px 5px 3px 5px;
  background: var(--color);
  box-shadow: 0 2px 3px rgba(31, 55, 70, 0.12);
  animation: ribbon-fall var(--duration) linear var(--delay) infinite;
}

@keyframes modal-pop {
  from { opacity: 0; transform: translateY(18px) scale(0.92); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

@keyframes ribbon-fall {
  0% { opacity: 0; transform: translate3d(0, -48px, 0) rotate(0deg) skewX(0deg); }
  12% { opacity: 1; }
  45% { transform: translate3d(calc(var(--drift) * 0.35), 45vh, 0) rotate(210deg) skewX(12deg); }
  100% { opacity: 0; transform: translate3d(var(--drift), calc(100vh + 80px), 0) rotate(620deg) skewX(-12deg); }
}

@media (max-width: 640px) {
  .checkin-content { gap: 12px; }
  .draw-copy { max-width: 155px; }
}

@media (max-width: 480px) {
  .checkin-content { flex-direction: column; }
  .draw-copy { max-width: 100%; text-align: center; }
}

@media (prefers-reduced-motion: reduce) {
  .success-dialog,
  .confetti-piece { animation: none; }
}
</style>
