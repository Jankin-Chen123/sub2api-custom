<template>
  <div class="space-y-5">
    <div class="card p-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.redeem.checkin.title') }}
          </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('admin.redeem.checkin.description') }}
          </p>
        </div>
        <div class="flex flex-shrink-0 items-center gap-2">
          <button class="btn btn-secondary" type="button" :disabled="loading || saving" @click="loadPrizes">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button class="btn btn-primary whitespace-nowrap" type="button" :disabled="!canSave" @click="savePrizes">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </div>

      <div class="mt-5 flex flex-col gap-3 rounded-xl border border-amber-100 bg-amber-50/70 p-4 dark:border-amber-900/40 dark:bg-amber-900/10 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p class="text-sm font-semibold text-amber-900 dark:text-amber-200">
            {{ t('admin.redeem.checkin.streakBonusTitle') }}
          </p>
          <p class="mt-1 text-xs text-amber-700 dark:text-amber-300">
            {{ t('admin.redeem.checkin.streakBonusDescription', { days: streakTarget }) }}
          </p>
        </div>
        <label class="flex items-center gap-2 text-sm font-semibold text-amber-900 dark:text-amber-100">
          <span>{{ t('admin.redeem.checkin.streakBonusAmount') }}</span>
          <span class="relative">
            <span class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-amber-600">$</span>
            <input
              v-model.number="streakBonusAmount"
              class="input w-36 pl-7"
              type="number"
              min="0"
              max="1000000"
              step="0.01"
            />
          </span>
        </label>
      </div>

      <div class="mt-5 grid gap-3 sm:grid-cols-3">
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.redeem.checkin.prizeCount') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ prizes.length }}</p>
        </div>
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.redeem.checkin.totalProbability') }}</p>
          <p :class="['mt-1 text-2xl font-bold', probabilityValid ? 'text-emerald-600' : 'text-rose-600']">
            {{ probabilityTotal.toFixed(4).replace(/\.?(0+)$/, '') }}%
          </p>
        </div>
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.redeem.checkin.status') }}</p>
          <p :class="['mt-1 text-sm font-semibold', canSave ? 'text-emerald-600' : 'text-amber-600']">
            {{ canSave ? t('admin.redeem.checkin.ready') : t('admin.redeem.checkin.needsAttention') }}
          </p>
        </div>
      </div>

      <div v-if="!probabilityValid" class="mt-4 rounded-lg bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
        {{ t('admin.redeem.checkin.probabilityHint') }}
      </div>
    </div>

    <div class="card overflow-hidden">
      <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
        <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.redeem.checkin.prizes') }}</h3>
        <button class="btn btn-secondary" type="button" :disabled="prizes.length >= 50" @click="addPrize">
          <Icon name="plus" size="sm" class="mr-2" />
          {{ t('admin.redeem.checkin.addPrize') }}
        </button>
      </div>

      <div v-if="loading" class="flex justify-center py-16">
        <span class="h-8 w-8 animate-spin rounded-full border-4 border-primary-200 border-t-primary-500" />
      </div>

      <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
        <div
          v-for="(prize, index) in prizes"
          :key="prize.id"
          class="grid gap-3 p-5 lg:grid-cols-[40px_minmax(130px,1fr)_110px_120px_76px_110px] lg:items-end"
        >
          <div class="flex h-10 w-10 items-center justify-center rounded-full text-sm font-bold text-white shadow-sm" :style="{ backgroundColor: prize.color }">
            {{ index + 1 }}
          </div>
          <label class="block">
            <span class="input-label">{{ t('admin.redeem.checkin.name') }}</span>
            <input v-model.trim="prize.name" maxlength="80" class="input mt-1" type="text" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.redeem.checkin.amount') }}</span>
            <input v-model.number="prize.amount" min="0" max="1000000" step="0.01" class="input mt-1" type="number" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.redeem.checkin.probability') }}</span>
            <input v-model.number="prize.probability" min="0.000001" max="100" step="0.01" class="input mt-1" type="number" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.redeem.checkin.color') }}</span>
            <input v-model="prize.color" class="mt-1 h-10 w-full cursor-pointer rounded-lg border border-gray-300 bg-white p-1 dark:border-dark-600 dark:bg-dark-800" type="color" />
          </label>
          <div class="flex gap-1">
            <button class="btn btn-secondary px-3" type="button" :disabled="index === 0" @click="movePrize(index, -1)" aria-label="Move up">↑</button>
            <button class="btn btn-secondary px-3" type="button" :disabled="index === prizes.length - 1" @click="movePrize(index, 1)" aria-label="Move down">↓</button>
            <button class="btn px-3 text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-900/20" type="button" :disabled="prizes.length <= 2" @click="removePrize(index)" :aria-label="t('common.delete')">
              ×
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { CheckinPrize } from '@/api'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const prizes = ref<CheckinPrize[]>([])
const loading = ref(false)
const saving = ref(false)
const streakBonusAmount = ref(5)
const streakTarget = ref(7)
let draftID = -1

const palette = ['#60A5FA', '#34D399', '#FBBF24', '#FB7185', '#A78BFA', '#2DD4BF', '#F97316', '#EC4899']
const probabilityTotal = computed(() => prizes.value.reduce((sum, prize) => sum + Number(prize.probability || 0), 0))
const probabilityValid = computed(() => Math.abs(probabilityTotal.value - 100) < 0.000001)
const streakBonusValid = computed(() => Number.isFinite(streakBonusAmount.value) && streakBonusAmount.value >= 0 && streakBonusAmount.value <= 1000000)
const rowsValid = computed(() => prizes.value.length >= 2 && prizes.value.every((prize) =>
  prize.name.trim().length > 0 &&
  Number.isFinite(prize.amount) && prize.amount >= 0 && prize.amount <= 1000000 &&
  Number.isFinite(prize.probability) && prize.probability > 0 && prize.probability <= 100 &&
  /^#[0-9a-f]{6}$/i.test(prize.color)
))
const canSave = computed(() => !loading.value && !saving.value && probabilityValid.value && rowsValid.value && streakBonusValid.value)

const loadPrizes = async () => {
  loading.value = true
  try {
    const [loadedPrizes, config] = await Promise.all([
      adminAPI.checkin.listPrizes(),
      adminAPI.checkin.getConfig()
    ])
    prizes.value = loadedPrizes.map((prize) => ({ ...prize }))
    streakBonusAmount.value = config.streak_bonus_amount
    streakTarget.value = config.streak_target
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.redeem.checkin.loadFailed'))
  } finally {
    loading.value = false
  }
}

const addPrize = () => {
  prizes.value.push({
    id: draftID--,
    name: t('admin.redeem.checkin.newPrize'),
    amount: 0.01,
    probability: 1,
    color: palette[prizes.value.length % palette.length],
    sort_order: prizes.value.length
  })
}

const removePrize = (index: number) => prizes.value.splice(index, 1)

const movePrize = (index: number, offset: number) => {
  const target = index + offset
  if (target < 0 || target >= prizes.value.length) return
  const [item] = prizes.value.splice(index, 1)
  prizes.value.splice(target, 0, item)
}

const savePrizes = async () => {
  if (!canSave.value) return
  saving.value = true
  try {
    const [updatedPrizes, updatedConfig] = await Promise.all([
      adminAPI.checkin.replacePrizes(
        prizes.value.map((prize, index) => ({ ...prize, sort_order: index }))
      ),
      adminAPI.checkin.updateConfig(streakBonusAmount.value)
    ])
    prizes.value = updatedPrizes
    streakBonusAmount.value = updatedConfig.streak_bonus_amount
    streakTarget.value = updatedConfig.streak_target
    appStore.showSuccess(t('admin.redeem.checkin.saveSuccess'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.redeem.checkin.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(loadPrizes)
</script>
