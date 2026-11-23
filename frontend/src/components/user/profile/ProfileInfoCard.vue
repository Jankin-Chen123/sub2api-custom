<template>
  <section data-testid="profile-overview-hero" class="journal-banner-sky rounded-[22px] p-2.5 sm:p-3">
    <div class="journal-banner-stitch space-y-5 rounded-[16px] p-4 sm:p-6">
      <ProfileAvatarCard :user="user" embedded>
        <template #summary="{ previewUrl, editing, toggleEditor }">
          <div class="flex items-start gap-3 sm:gap-5">
            <button
              type="button"
              class="relative shrink-0 rounded-2xl focus-visible:outline-2 focus-visible:outline-offset-4"
              :disabled="!user || editingName"
              :aria-label="t('profile.changeAvatar')"
              :title="t('profile.changeAvatar')"
              :aria-expanded="editing"
              @click="toggleEditor"
            >
              <img :src="previewUrl" alt="" class="h-16 w-16 rounded-2xl bg-white object-cover shadow-md ring-2 ring-white/80 sm:h-20 sm:w-20" />
              <span class="absolute -bottom-1 -right-1 grid h-6 w-6 place-items-center rounded-full border border-[var(--craft-border)] bg-[var(--craft-paper)] text-[var(--craft-ink)]">
                <Icon name="edit" size="xs" />
              </span>
            </button>
            <div class="min-w-0 flex-1 space-y-2">
              <ProfileEditForm
                v-if="editingName"
                :initial-username="user?.username || ''"
                embedded
                @done="finishNameEdit"
              />
              <div v-else class="flex flex-wrap items-center gap-2">
                <h2 class="min-w-0 break-all text-2xl font-black tracking-tight text-[var(--craft-ink)]">{{ displayName }}</h2>
                <button
                  ref="nameButton"
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="!user || editing"
                  :aria-label="t('profile.editUsername')"
                  :title="t('profile.editUsername')"
                  @click="editingName = true"
                >
                  <Icon name="edit" size="sm" />
                </button>
                <CraftSparks variant="right" size="sm" />
                <span :class="['badge', user?.role === 'admin' ? 'badge-primary' : 'badge-gray']">
                  {{ user?.role === 'admin' ? t('profile.administrator') : t('profile.user') }}
                </span>
                <span
                  :class="['badge', user?.status === 'active' ? 'badge-success' : 'badge-danger']"
                >
                  {{
                    user?.status === 'active'
                      ? t('common.active')
                      : t('common.disabled')
                  }}
                </span>
              </div>
              <p class="break-all text-sm font-medium text-[#4a4236] dark:text-gray-300">
                {{ primaryEmailDisplay }}
              </p>
              <div
                v-if="sourceHints.length"
                class="flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400"
              >
                <span
                  v-for="hint in sourceHints"
                  :key="hint.key"
                  class="inline-flex items-center gap-1 rounded-full bg-white/85 px-3 py-1 ring-1 ring-[#d6c2a2] dark:bg-dark-900/70 dark:ring-primary-900/40"
                >
                  <Icon name="link" size="sm" />
                  {{ hint.text }}
                </span>
              </div>
            </div>
          </div>
        </template>
      </ProfileAvatarCard>

      <div class="grid gap-3 sm:grid-cols-3">
        <div
          data-testid="profile-overview-metric-balance"
          class="journal-kpi-warm rounded-2xl px-4 py-3"
        >
          <p class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-[0.12em] text-[var(--craft-muted)]">
            <CraftIcon name="wallet" size="xs" />
            <span>{{ t('profile.accountBalance') }}</span>
          </p>
          <p class="mt-1.5 text-lg font-black text-[var(--craft-ink)]">
            {{ formatCurrency(user?.balance || 0) }}
          </p>
        </div>
        <div
          data-testid="profile-overview-metric-concurrency"
          class="journal-kpi-warm rounded-2xl px-4 py-3"
        >
          <p class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-[0.12em] text-[var(--craft-muted)]">
            <CraftIcon name="bolt" size="xs" />
            <span>{{ t('profile.concurrencyLimit') }}</span>
          </p>
          <p class="mt-1.5 text-lg font-black text-[var(--craft-ink)]">
            {{ user?.concurrency || 0 }}
          </p>
        </div>
        <div
          data-testid="profile-overview-metric-member-since"
          class="journal-kpi-warm rounded-2xl px-4 py-3"
        >
          <p class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-[0.12em] text-[var(--craft-muted)]">
            <CraftIcon name="calendar" size="xs" />
            <span>{{ t('profile.memberSince') }}</span>
          </p>
          <p class="mt-1.5 text-lg font-black text-[var(--craft-ink)]">
            {{ memberSinceLabel }}
          </p>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import CraftIcon from '@/components/craft/CraftIcon.vue'
import CraftSparks from '@/components/craft/CraftSparks.vue'
import Icon from '@/components/icons/Icon.vue'
import ProfileAvatarCard from '@/components/user/profile/ProfileAvatarCard.vue'
import ProfileEditForm from '@/components/user/profile/ProfileEditForm.vue'
import type { User, UserAuthBindingStatus, UserAuthProvider, UserProfileSourceContext } from '@/types'

const props = withDefaults(defineProps<{
  user: User | null
  oidcProviderName?: string
}>(), {
  oidcProviderName: 'OIDC',
})

const { t } = useI18n()
const editingName = ref(false)
const nameButton = ref<HTMLButtonElement>()

async function finishNameEdit() {
  editingName.value = false
  await nextTick()
  nameButton.value?.focus()
}

function normalizeBindingStatus(binding: boolean | UserAuthBindingStatus | undefined): boolean | null {
  if (typeof binding === 'boolean') {
    return binding
  }
  if (!binding) {
    return null
  }
  if (typeof binding.bound === 'boolean') {
    return binding.bound
  }
  return Boolean(binding.provider_subject || binding.issuer || binding.provider_key)
}

function isEmailBound(user: User | null | undefined): boolean {
  if (typeof user?.email_bound === 'boolean') {
    return user.email_bound
  }

  const nested = user?.auth_bindings?.email ?? user?.identity_bindings?.email
  const normalized = normalizeBindingStatus(nested)
  return normalized ?? false
}

const displayName = computed(() => props.user?.username?.trim() || props.user?.email?.trim() || t('profile.user'))
const primaryEmailDisplay = computed(() => {
  const email = props.user?.email?.trim() || ''
  if (!email) {
    return ''
  }
  if (email.endsWith('.invalid') && !isEmailBound(props.user)) {
    return ''
  }
  return email
})
const memberSinceLabel = computed(() => {
  const raw = props.user?.created_at?.trim()
  if (!raw) {
    return '-'
  }

  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }

  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
  }).format(date)
})

const providerLabels = computed<Record<UserAuthProvider, string>>(() => ({
  email: t('profile.authBindings.providers.email'),
  linuxdo: t('profile.authBindings.providers.linuxdo'),
  dingtalk: t('profile.authBindings.providers.dingtalk'),
  oidc: t('profile.authBindings.providers.oidc', { providerName: props.oidcProviderName }),
  wechat: t('profile.authBindings.providers.wechat'),
  github: 'GitHub',
  google: 'Google'
}))

function formatCurrency(value: number): string {
  return `$${value.toFixed(2)}`
}

function normalizeProvider(value: string): UserAuthProvider | null {
  const normalized = value.trim().toLowerCase()
  if (
    normalized === 'email' ||
    normalized === 'linuxdo' ||
    normalized === 'dingtalk' ||
    normalized === 'wechat' ||
    normalized === 'github' ||
    normalized === 'google'
  ) {
    return normalized
  }
  if (normalized === 'oidc' || normalized.startsWith('oidc:') || normalized.startsWith('oidc/')) {
    return 'oidc'
  }
  return null
}

function readObjectString(source: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = source[key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}

function resolveThirdPartySource(
  rawSource: string | UserProfileSourceContext | null | undefined
): { provider: UserAuthProvider; label: string } | null {
  if (!rawSource) {
    return null
  }

  if (typeof rawSource === 'string') {
    const provider = normalizeProvider(rawSource)
    if (!provider || provider === 'email') {
      return null
    }
    return {
      provider,
      label: providerLabels.value[provider]
    }
  }

  const sourceRecord = rawSource as Record<string, unknown>
  const provider = normalizeProvider(
    readObjectString(sourceRecord, 'provider', 'source', 'provider_type', 'auth_provider')
  )
  if (!provider || provider === 'email') {
    return null
  }

  const explicitLabel = readObjectString(
    sourceRecord,
    'provider_label',
    'label',
    'provider_name',
    'providerName'
  )

  return {
    provider,
    label: explicitLabel || providerLabels.value[provider]
  }
}

const sourceHints = computed(() => {
  const currentUser = props.user
  if (!currentUser) {
    return []
  }

  const hints: Array<{ key: string; text: string }> = []
  const avatarSource = resolveThirdPartySource(
    currentUser.profile_sources?.avatar ?? currentUser.avatar_source
  )
  const usernameSource = resolveThirdPartySource(
    currentUser.profile_sources?.username ??
      currentUser.profile_sources?.display_name ??
      currentUser.profile_sources?.nickname ??
      currentUser.display_name_source ??
      currentUser.username_source ??
      currentUser.nickname_source
  )

  if (avatarSource) {
    hints.push({
      key: 'avatar',
      text: t('profile.authBindings.source.avatar', { providerName: avatarSource.label })
    })
  }

  if (usernameSource) {
    hints.push({
      key: 'username',
      text: t('profile.authBindings.source.username', { providerName: usernameSource.label })
    })
  }

  return hints
})
</script>
