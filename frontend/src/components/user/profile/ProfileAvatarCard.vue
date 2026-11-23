<template>
  <div :class="props.embedded ? 'space-y-4' : 'card'">
    <slot name="summary" :preview-url="avatarPreviewUrl" :editing="editing" :toggle-editor="toggleEditor" />
    <div
      v-if="!props.embedded"
      class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
    >
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('profile.avatar.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('profile.avatar.description') }}
      </p>
    </div>

    <div
      v-if="!$slots.summary || editing"
      :class="[
        $slots.summary && 'rounded-2xl border border-white/80 bg-[var(--craft-paper)] p-4',
        props.embedded ? 'space-y-3' : 'flex flex-col gap-5 px-6 py-6 sm:flex-row sm:items-start'
      ]"
    >
      <div
        v-if="!$slots.summary"
        :class="props.embedded
          ? 'flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-2xl border border-[#e3d2b6] bg-[#fffdf8] text-xl font-bold text-[#27221d] shadow-md shadow-[#8f6c41]/15 dark:border-dark-600 dark:bg-dark-800 dark:text-white'
          : 'flex h-24 w-24 shrink-0 items-center justify-center overflow-hidden rounded-2xl border border-[#e3d2b6] bg-[#fffdf8] text-3xl font-bold text-[#27221d] shadow-md shadow-[#8f6c41]/15 dark:border-dark-600 dark:bg-dark-800 dark:text-white'"
      >
        <img
          v-if="avatarPreviewUrl"
          data-testid="profile-avatar-preview"
          :src="avatarPreviewUrl"
          :alt="displayName"
          class="h-full w-full object-cover"
        >
        <span v-else>{{ avatarInitial }}</span>
      </div>

      <div :class="props.embedded ? 'space-y-3' : 'min-w-0 flex-1 space-y-4'">
        <div class="space-y-1">
          <p v-if="props.embedded" class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('profile.avatar.title') }}
          </p>
          <p v-else class="text-sm font-medium text-gray-900 dark:text-white">
            {{ displayName }}
          </p>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('profile.avatar.uploadHint') }}
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-3">
          <label class="btn btn-secondary btn-sm cursor-pointer focus-within:ring-2">
            <input
              data-testid="profile-avatar-file-input"
              type="file"
              accept="image/*"
              class="sr-only"
              :disabled="avatarSaving"
              @change="handleAvatarFileChange"
            >
            {{ t('profile.avatar.uploadAction') }}
          </label>

          <button
            data-testid="profile-avatar-save"
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="avatarSaving || preparing || !avatarDraft"
            @click="handleAvatarSave"
          >
            {{ t('common.save') }}
          </button>

          <button
            data-testid="profile-avatar-delete"
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="avatarSaving || preparing || !props.user?.avatar_url"
            @click="handleAvatarDelete"
          >
            {{ t('common.delete') }}
          </button>
          <button v-if="$slots.summary" type="button" class="btn btn-secondary btn-sm" :disabled="avatarSaving" @click="toggleEditor">
            {{ t('common.cancel') }}
          </button>
        </div>

        <div
          data-testid="profile-default-avatar-picker"
          class="space-y-2 border-t border-dashed border-[#e5d5ba] pt-3 dark:border-dark-700"
        >
          <div class="flex flex-wrap items-baseline justify-between gap-2">
            <span class="text-xs font-bold text-[#5a4e3e] dark:text-dark-200">
              {{ t('profile.avatar.presetTitle') }}
            </span>
            <span class="text-[11px] text-[#857563] dark:text-dark-400">
              {{ t('profile.avatar.presetHint') }}
            </span>
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="preset in DEFAULT_JOURNAL_AVATARS"
              :key="preset.id"
              :data-testid="`profile-default-avatar-${preset.id}`"
              type="button"
              :disabled="avatarSaving"
              :title="t(preset.labelKey)"
              :aria-label="t(preset.labelKey)"
              :class="[
                'group relative overflow-hidden rounded-xl border p-0.5 transition-all hover:-translate-y-0.5 focus:outline-none focus:ring-2 focus:ring-primary-500/40',
                selectedPresetId === preset.id
                  ? 'border-primary-500 bg-primary-50/70 ring-2 ring-primary-400/50 dark:bg-primary-900/30'
                  : 'border-[#e3d2b6] bg-[#fffdf8] hover:border-[#cfa97a] dark:border-dark-600 dark:bg-dark-800'
              ]"
              @click="selectPresetAvatar(preset.url, preset.id)"
            >
              <img
                :src="preset.url"
                :alt="t(preset.labelKey)"
                class="h-11 w-11 rounded-[10px] object-cover"
              >
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { userAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { User } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { DEFAULT_JOURNAL_AVATARS, resolveUserAvatarUrl } from '@/utils/defaultAvatar'

const props = withDefaults(defineProps<{
  user: User | null
  embedded?: boolean
}>(), {
  embedded: false,
})

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const targetAvatarUploadBytes = 20 * 1024
const avatarScaleSteps = [1, 0.92, 0.84, 0.76, 0.68, 0.6, 0.52, 0.44, 0.36]
const avatarQualitySteps = [0.92, 0.84, 0.76, 0.68, 0.6, 0.52, 0.44, 0.36]
const avatarDraft = ref('')
const selectedPresetIdDraft = ref('')
const avatarSaving = ref(false)
const editing = ref(false)
const preparing = ref(false)
let selectionVersion = 0

function toggleEditor() {
  if (avatarSaving.value) return
  selectionVersion++
  preparing.value = false
  avatarDraft.value = ''
  selectedPresetIdDraft.value = ''
  editing.value = !editing.value
}

const displayName = computed(() => props.user?.username?.trim() || props.user?.email?.trim() || t('profile.user'))
const avatarInitial = computed(() => displayName.value.charAt(0).toUpperCase() || 'U')
const avatarPreviewUrl = computed(() => avatarDraft.value.trim() || resolveUserAvatarUrl(props.user))
const selectedPresetId = computed(() => {
  if (selectedPresetIdDraft.value) {
    return selectedPresetIdDraft.value
  }
  if (avatarDraft.value.trim()) {
    return ''
  }
  const activeUrl = resolveUserAvatarUrl(props.user)
  return DEFAULT_JOURNAL_AVATARS.find((item) => item.url === activeUrl)?.id ?? ''
})

watch(
  () => props.user?.avatar_url,
  () => {
    avatarDraft.value = ''
    selectedPresetIdDraft.value = ''
  }
)

function normalizeUploadedAvatar(value: string): string | null {
  const normalized = value.trim()
  if (!normalized) {
    return null
  }

  if (!/^data:image\/[a-zA-Z0-9.+-]+;base64,/i.test(normalized)) {
    appStore.showError(t('profile.avatar.uploadRequired'))
    return null
  }

  return normalized
}

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '')
    reader.onerror = () => reject(reader.error ?? new Error('avatar_read_failed'))
    reader.readAsDataURL(file)
  })
}

function loadImage(dataURL: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error(t('profile.avatar.readFailed')))
    image.src = dataURL
  })
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (!blob) {
        reject(new Error(t('profile.avatar.compressFailed')))
        return
      }
      resolve(blob)
    }, type, quality)
  })
}

async function compressAvatarFile(file: File): Promise<File> {
  const sourceDataURL = await readFileAsDataURL(file)
  const image = await loadImage(sourceDataURL)
  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    throw new Error(t('profile.avatar.compressFailed'))
  }

  for (const scale of avatarScaleSteps) {
    const width = Math.max(1, Math.round(image.naturalWidth * scale))
    const height = Math.max(1, Math.round(image.naturalHeight * scale))
    canvas.width = width
    canvas.height = height
    ctx.clearRect(0, 0, width, height)
    ctx.drawImage(image, 0, 0, width, height)

    for (const quality of avatarQualitySteps) {
      const blob = await canvasToBlob(canvas, 'image/webp', quality)
      if (blob.size <= targetAvatarUploadBytes) {
        const fileName = file.name.replace(/\.[^.]+$/, '') || 'avatar'
        return new File([blob], `${fileName}.webp`, { type: 'image/webp' })
      }
    }
  }

  throw new Error(t('profile.avatar.compressTooLarge'))
}

async function prepareAvatarUpload(file: File): Promise<File> {
  if (!file.type.startsWith('image/')) {
    throw new Error(t('profile.avatar.invalidType'))
  }
  if (file.type === 'image/gif') {
    if (file.size > targetAvatarUploadBytes) {
      throw new Error(t('profile.avatar.gifTooLarge'))
    }
    return file
  }
  if (file.size <= targetAvatarUploadBytes) {
    return file
  }
  return compressAvatarFile(file)
}

async function handleAvatarFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0]
  if (input) {
    input.value = ''
  }
  if (!file) {
    return
  }

  const version = ++selectionVersion
  preparing.value = true
  try {
    const preparedFile = await prepareAvatarUpload(file)
    const dataURL = await readFileAsDataURL(preparedFile)
    if (version !== selectionVersion) return
    const normalized = normalizeUploadedAvatar(dataURL)
    if (!normalized) {
      return
    }
    avatarDraft.value = normalized
    selectedPresetIdDraft.value = ''
  } catch (error: unknown) {
    if (version === selectionVersion) appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    if (version === selectionVersion) preparing.value = false
  }
}

async function selectPresetAvatar(presetUrl: string, presetId: string) {
  if (avatarSaving.value) {
    return
  }
  const version = ++selectionVersion
  preparing.value = true
  try {
    const response = await fetch(presetUrl)
    if (!response.ok) {
      throw new Error(t('profile.avatar.presetLoadFailed'))
    }
    const blob = await response.blob()
    const file = new File([blob], `${presetId}.webp`, { type: 'image/webp' })
    const preparedFile = await prepareAvatarUpload(file)
    const dataURL = await readFileAsDataURL(preparedFile)
    if (version !== selectionVersion) return
    const normalized = normalizeUploadedAvatar(dataURL)
    if (!normalized) {
      return
    }
    avatarDraft.value = normalized
    selectedPresetIdDraft.value = presetId
  } catch (error: unknown) {
    if (version === selectionVersion) appStore.showError(extractApiErrorMessage(error, t('profile.avatar.presetLoadFailed')))
  } finally {
    if (version === selectionVersion) preparing.value = false
  }
}

async function handleAvatarSave() {
  if (avatarSaving.value || preparing.value) return
  const normalized = normalizeUploadedAvatar(avatarDraft.value)
  if (!normalized) {
    return
  }

  selectionVersion++
  avatarSaving.value = true
  try {
    const updated = await userAPI.updateProfile({ avatar_url: normalized })
    authStore.user = updated
    editing.value = false
    avatarDraft.value = ''
    appStore.showSuccess(t('profile.avatar.saveSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    avatarSaving.value = false
  }
}

async function handleAvatarDelete() {
  if (avatarSaving.value) {
    return
  }
  if (!avatarDraft.value.trim() && !props.user?.avatar_url?.trim()) {
    appStore.showError(t('profile.avatar.emptyDeleteHint'))
    return
  }

  selectionVersion++
  avatarSaving.value = true
  try {
    const updated = await userAPI.updateProfile({ avatar_url: '' })
    authStore.user = updated
    avatarDraft.value = ''
    editing.value = false
    selectedPresetIdDraft.value = ''
    appStore.showSuccess(t('profile.avatar.deleteSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    avatarSaving.value = false
  }
}
</script>
