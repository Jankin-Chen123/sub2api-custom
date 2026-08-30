<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.newcomerCampaign.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.newcomerCampaign.description') }}
          </p>
        </div>
        <span
          v-if="configPhase"
          class="rounded-full bg-primary-100 px-3 py-1 text-xs font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300"
        >
          {{ t('admin.newcomerCampaign.phase', { phase: phaseLabel(configPhase) }) }}
        </span>
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="reconciling"
          @click="runReconcile"
        >
          <Icon name="refresh" size="sm" class="mr-2" :class="reconciling ? 'animate-spin' : ''" />
          {{ reconciling ? t('admin.newcomerCampaign.reconciling') : t('admin.newcomerCampaign.reconcile') }}
        </button>
      </div>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.newcomerCampaign.windowTitle') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.newcomerCampaign.windowHint') }}</p>
        </div>
        <div class="grid gap-4 p-5 sm:grid-cols-2">
          <label class="block">
            <span class="input-label">{{ t('admin.newcomerCampaign.startsAt') }}</span>
            <input v-model="startsDate" class="input" type="date" required />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.newcomerCampaign.endsAt') }}</span>
            <input v-model="endsDate" class="input" type="date" required />
          </label>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.newcomerCampaign.tiersTitle') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.newcomerCampaign.tiersHint') }}</p>
        </div>

        <div v-if="loadingConfig" class="p-5 text-sm text-gray-500 dark:text-gray-400">
          {{ t('common.loading') }}
        </div>
        <form v-else class="space-y-4 p-5" @submit.prevent="saveConfig">
          <div class="grid gap-4 md:grid-cols-3">
            <div
              v-for="tier in tierDrafts"
              :key="tier.key"
              class="rounded-2xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/60"
            >
              <div class="mb-4 flex items-center justify-between gap-3">
                <div>
                  <p class="font-semibold text-gray-900 dark:text-white">{{ tier.name }}</p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newcomerCampaign.threshold', { count: tier.threshold }) }}</p>
                </div>
                <span class="rounded-full bg-primary-100 px-2.5 py-1 text-xs font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                  {{ tier.key }}
                </span>
              </div>
              <label class="mb-3 block">
                <span class="input-label">{{ t('admin.newcomerCampaign.thresholdInput') }}</span>
                <input v-model.number="tier.threshold" class="input" type="number" min="1" step="1" required />
              </label>
              <label class="mb-3 block">
                <span class="input-label">{{ t('admin.newcomerCampaign.factor') }}</span>
                <input v-model.number="tier.factor" class="input" type="number" min="0.000001" max="1" step="0.000001" required />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.newcomerCampaign.durationDays') }}</span>
                <input v-model.number="tier.duration_days" class="input" type="number" min="1" max="3650" step="1" required />
              </label>
            </div>
          </div>
          <div class="flex justify-end">
            <button type="submit" class="btn btn-primary" :disabled="savingConfig">
              {{ savingConfig ? t('common.saving') : t('common.save') }}
            </button>
          </div>
        </form>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.newcomerCampaign.userTitle') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.newcomerCampaign.userHint') }}</p>
        </div>

        <div class="space-y-5 p-5">
          <form class="flex flex-wrap items-end gap-3" @submit.prevent="loadUserMembership">
            <label class="w-full sm:w-64">
              <span class="input-label">{{ t('admin.newcomerCampaign.userId') }}</span>
              <input v-model="userIdInput" class="input" type="number" min="1" required :placeholder="t('admin.newcomerCampaign.userIdPlaceholder')" />
            </label>
            <button type="submit" class="btn btn-secondary" :disabled="loadingUser">
              <Icon name="search" size="sm" class="mr-2" />
              {{ t('admin.newcomerCampaign.lookup') }}
            </button>
          </form>

          <div v-if="activeUser" class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(360px,.9fr)]">
            <div class="rounded-2xl border border-gray-200 p-5 dark:border-dark-700">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="font-semibold text-gray-900 dark:text-white">{{ activeUser.username || activeUser.email }}</p>
                  <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ activeUser.email }} · ID {{ activeUser.user_id }}</p>
                </div>
                <span class="rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                  {{ t('admin.newcomerCampaign.validInvites', { count: activeUser.valid_invite_count }) }}
                </span>
              </div>

              <div class="mt-5 grid gap-3 sm:grid-cols-2">
                <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800/70">
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newcomerCampaign.manualMembership') }}</p>
                  <p class="mt-1 font-semibold text-gray-900 dark:text-white">
                    {{ activeUser.manual_membership ? tierLabel(activeUser.manual_membership.tier_key) : t('admin.newcomerCampaign.none') }}
                  </p>
                  <p v-if="activeUser.manual_membership" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    ×{{ activeUser.manual_membership.factor }} · {{ formatDate(activeUser.manual_membership.starts_at) }} — {{ formatDate(activeUser.manual_membership.expires_at) }}
                  </p>
                </div>
                <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800/70">
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newcomerCampaign.effectiveMembership') }}</p>
                  <p class="mt-1 font-semibold text-gray-900 dark:text-white">
                    {{ activeUser.current_membership ? tierLabel(activeUser.current_membership.tier_key) : t('admin.newcomerCampaign.none') }}
                  </p>
                  <p v-if="activeUser.current_membership" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    ×{{ activeUser.current_membership.factor }} · {{ t('admin.newcomerCampaign.expiresAt', { date: formatDate(activeUser.current_membership.expires_at) }) }}
                  </p>
                </div>
              </div>
            </div>

            <form class="rounded-2xl border border-gray-200 p-5 dark:border-dark-700" @submit.prevent="saveUserMembership">
              <div class="mb-4">
                <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.newcomerCampaign.assignTitle') }}</h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newcomerCampaign.assignHint') }}</p>
              </div>
              <div class="space-y-4">
                <label class="block">
                  <span class="input-label">{{ t('admin.newcomerCampaign.tier') }}</span>
                  <select v-model="membershipForm.tier_key" class="input" required>
                    <option v-for="tier in tierDrafts" :key="tier.key" :value="tier.key">{{ tier.name }}</option>
                  </select>
                </label>
                <div class="grid gap-3 sm:grid-cols-2">
                  <label class="block">
                    <span class="input-label">{{ t('admin.newcomerCampaign.factorOptional') }}</span>
                    <input v-model="membershipForm.factor" class="input" type="number" min="0.000001" max="1" step="0.000001" :placeholder="selectedTier ? String(selectedTier.factor) : ''" />
                  </label>
                  <label class="block">
                    <span class="input-label">{{ t('admin.newcomerCampaign.durationOptional') }}</span>
                    <input v-model="membershipForm.duration_days" class="input" type="number" min="1" max="3650" step="1" :placeholder="selectedTier ? String(selectedTier.duration_days) : ''" />
                  </label>
                </div>
                <label class="block">
                  <span class="input-label">{{ t('admin.newcomerCampaign.reason') }}</span>
                  <input v-model="membershipForm.reason" class="input" maxlength="255" :placeholder="t('admin.newcomerCampaign.reasonPlaceholder')" />
                </label>
                <div class="flex flex-wrap justify-end gap-2">
                  <button type="button" class="btn btn-secondary" :disabled="clearingMembership || !activeUser.manual_membership" @click="clearUserMembership">
                    {{ clearingMembership ? t('admin.newcomerCampaign.clearing') : t('admin.newcomerCampaign.clear') }}
                  </button>
                  <button type="submit" class="btn btn-primary" :disabled="savingMembership">
                    {{ savingMembership ? t('admin.newcomerCampaign.assigning') : t('admin.newcomerCampaign.assign') }}
                  </button>
                </div>
              </div>
            </form>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import newcomerCampaignAdminAPI from '@/api/admin/campaign'
import type {
  NewcomerCampaignAdminUserMembership,
  NewcomerCampaignTier,
} from '@/types/campaign'

const { t } = useI18n()
const appStore = useAppStore()

const tierDrafts = ref<NewcomerCampaignTier[]>([])
const startsDate = ref('')
const endsDate = ref('')
const configPhase = ref('')
const loadingConfig = ref(true)
const savingConfig = ref(false)
const reconciling = ref(false)
const loadingUser = ref(false)
const savingMembership = ref(false)
const clearingMembership = ref(false)
const userIdInput = ref('')
const activeUser = ref<NewcomerCampaignAdminUserMembership | null>(null)
const membershipForm = ref({ tier_key: 'premium', factor: '', duration_days: '', reason: '' })

const selectedTier = computed(() => tierDrafts.value.find((tier) => tier.key === membershipForm.value.tier_key))

function errorMessage(error: unknown, fallback: string): string {
  if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string') {
    return error.message
  }
  return fallback
}

function dateOnlyInShanghai(value: string): string {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(new Date(value))
  const values = Object.fromEntries(parts.filter((part) => part.type !== 'literal').map((part) => [part.type, part.value]))
  return `${values.year}-${values.month}-${values.day}`
}

function addDays(value: string, days: number): string {
  const date = new Date(`${value}T00:00:00+08:00`)
  date.setUTCDate(date.getUTCDate() + days)
  return date.toISOString()
}

function boundaryRFC3339(value: string, endExclusive: boolean): string {
  return addDays(value, endExclusive ? 1 : 0)
}

function phaseLabel(phase: string): string {
  if (phase === 'active') return t('admin.newcomerCampaign.phaseActive')
  if (phase === 'ended') return t('admin.newcomerCampaign.phaseEnded')
  return t('admin.newcomerCampaign.phaseUpcoming')
}

async function loadConfig() {
  loadingConfig.value = true
  try {
    const config = await newcomerCampaignAdminAPI.getConfig()
    tierDrafts.value = config.tiers.map((tier) => ({ ...tier }))
    startsDate.value = dateOnlyInShanghai(config.starts_at)
    const exclusiveEnd = new Date(config.ends_at)
    exclusiveEnd.setUTCDate(exclusiveEnd.getUTCDate() - 1)
    endsDate.value = dateOnlyInShanghai(exclusiveEnd.toISOString())
    configPhase.value = config.phase
    if (tierDrafts.value.length && !tierDrafts.value.some((tier) => tier.key === membershipForm.value.tier_key)) {
      membershipForm.value.tier_key = tierDrafts.value[0].key
    }
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.newcomerCampaign.loadFailed')))
  } finally {
    loadingConfig.value = false
  }
}

async function saveConfig() {
  if (!startsDate.value || !endsDate.value || startsDate.value > endsDate.value) {
    appStore.showError(t('admin.newcomerCampaign.invalidDates'))
    return
  }
  savingConfig.value = true
  try {
    const config = await newcomerCampaignAdminAPI.updateConfig(
      tierDrafts.value,
      boundaryRFC3339(startsDate.value, false),
      boundaryRFC3339(endsDate.value, true)
    )
    tierDrafts.value = config.tiers.map((tier) => ({ ...tier }))
    startsDate.value = dateOnlyInShanghai(config.starts_at)
    const exclusiveEnd = new Date(config.ends_at)
    exclusiveEnd.setUTCDate(exclusiveEnd.getUTCDate() - 1)
    endsDate.value = dateOnlyInShanghai(exclusiveEnd.toISOString())
    configPhase.value = config.phase
    appStore.showSuccess(t('admin.newcomerCampaign.saved'))
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.newcomerCampaign.saveFailed')))
  } finally {
    savingConfig.value = false
  }
}

async function runReconcile() {
  reconciling.value = true
  try {
    const result = await newcomerCampaignAdminAPI.reconcile()
    appStore.showSuccess(t('admin.newcomerCampaign.reconciled', { count: result.repaired_users }))
    if (activeUser.value) await loadUserMembership()
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.newcomerCampaign.reconcileFailed')))
  } finally {
    reconciling.value = false
  }
}

async function loadUserMembership() {
  const userId = Number(userIdInput.value)
  if (!Number.isInteger(userId) || userId <= 0) {
    appStore.showError(t('admin.newcomerCampaign.invalidUserId'))
    return
  }
  loadingUser.value = true
  try {
    activeUser.value = await newcomerCampaignAdminAPI.getUserMembership(userId)
  } catch (error) {
    activeUser.value = null
    appStore.showError(errorMessage(error, t('admin.newcomerCampaign.userLoadFailed')))
  } finally {
    loadingUser.value = false
  }
}

async function saveUserMembership() {
  if (!activeUser.value) return
  const input: Parameters<typeof newcomerCampaignAdminAPI.setUserMembership>[1] = {
    tier_key: membershipForm.value.tier_key,
    reason: membershipForm.value.reason.trim() || undefined,
  }
  if (membershipForm.value.factor.trim()) input.factor = Number(membershipForm.value.factor)
  if (membershipForm.value.duration_days.trim()) input.duration_days = Number(membershipForm.value.duration_days)

  savingMembership.value = true
  try {
    activeUser.value = await newcomerCampaignAdminAPI.setUserMembership(activeUser.value.user_id, input)
    membershipForm.value.factor = ''
    membershipForm.value.duration_days = ''
    appStore.showSuccess(t('admin.newcomerCampaign.assigned'))
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.newcomerCampaign.assignFailed')))
  } finally {
    savingMembership.value = false
  }
}

async function clearUserMembership() {
  if (!activeUser.value?.manual_membership) return
  clearingMembership.value = true
  try {
    activeUser.value = await newcomerCampaignAdminAPI.clearUserMembership(activeUser.value.user_id)
    appStore.showSuccess(t('admin.newcomerCampaign.cleared'))
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.newcomerCampaign.clearFailed')))
  } finally {
    clearingMembership.value = false
  }
}

function tierLabel(key: string): string {
  const tier = tierDrafts.value.find((item) => item.key === key)
  return tier?.name || key
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

onMounted(loadConfig)
</script>
