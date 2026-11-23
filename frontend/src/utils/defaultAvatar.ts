export interface DefaultJournalAvatar {
  id: string
  url: string
  labelKey: string
}

export interface AvatarUserLike {
  id?: number
  username?: string | null
  email?: string | null
  avatar_url?: string | null
}

export const DEFAULT_JOURNAL_AVATARS: readonly DefaultJournalAvatar[] = [
  { id: 'pen', url: '/sticker/avatars/avatar-1.webp', labelKey: 'profile.avatar.presets.pen' },
  { id: 'coffee', url: '/sticker/avatars/avatar-2.webp', labelKey: 'profile.avatar.presets.coffee' },
  { id: 'camera', url: '/sticker/avatars/avatar-3.webp', labelKey: 'profile.avatar.presets.camera' },
  { id: 'plant', url: '/sticker/avatars/avatar-4.webp', labelKey: 'profile.avatar.presets.plant' },
  { id: 'compass', url: '/sticker/avatars/avatar-5.webp', labelKey: 'profile.avatar.presets.compass' },
  { id: 'vinyl', url: '/sticker/avatars/avatar-6.webp', labelKey: 'profile.avatar.presets.vinyl' },
] as const

function hashAvatarSeed(seed: string): number {
  let hash = 2166136261
  for (let i = 0; i < seed.length; i += 1) {
    hash ^= seed.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return hash >>> 0
}

export function getDefaultUserAvatar(seed?: string | number | null): string {
  const normalized = String(seed ?? '').trim()
  if (!normalized) {
    return DEFAULT_JOURNAL_AVATARS[0].url
  }
  const index = hashAvatarSeed(normalized) % DEFAULT_JOURNAL_AVATARS.length
  return DEFAULT_JOURNAL_AVATARS[index].url
}

export function resolveUserAvatarUrl(user?: AvatarUserLike | null): string {
  const explicit = user?.avatar_url?.trim()
  if (explicit) {
    return explicit
  }
  return getDefaultUserAvatar(user?.id ?? user?.username ?? user?.email)
}

export function isDefaultJournalAvatar(url?: string | null): boolean {
  const trimmed = url?.trim()
  if (!trimmed) {
    return false
  }
  return DEFAULT_JOURNAL_AVATARS.some((item) => item.url === trimmed)
}
