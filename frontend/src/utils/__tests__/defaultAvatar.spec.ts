import { describe, expect, it } from 'vitest'
import {
  DEFAULT_JOURNAL_AVATARS,
  getDefaultUserAvatar,
  isDefaultJournalAvatar,
  resolveUserAvatarUrl,
} from '../defaultAvatar'

describe('defaultAvatar', () => {
  it('defines 6 handcrafted journal default avatars', () => {
    expect(DEFAULT_JOURNAL_AVATARS).toHaveLength(6)
    expect(DEFAULT_JOURNAL_AVATARS.map((item) => item.url)).toEqual([
      '/sticker/avatars/avatar-1.webp',
      '/sticker/avatars/avatar-2.webp',
      '/sticker/avatars/avatar-3.webp',
      '/sticker/avatars/avatar-4.webp',
      '/sticker/avatars/avatar-5.webp',
      '/sticker/avatars/avatar-6.webp',
    ])
  })

  it('deterministically assigns a default avatar from user seeds', () => {
    const first = getDefaultUserAvatar(42)
    expect(first).toBe(getDefaultUserAvatar(42))
    expect(isDefaultJournalAvatar(first)).toBe(true)

    expect(getDefaultUserAvatar('')).toBe(DEFAULT_JOURNAL_AVATARS[0].url)
    expect(getDefaultUserAvatar(null)).toBe(DEFAULT_JOURNAL_AVATARS[0].url)
  })

  it('preserves explicit custom avatars and falls back to default avatars when absent', () => {
    expect(
      resolveUserAvatarUrl({ id: 7, username: 'alice', avatar_url: ' https://cdn.example.com/custom.png ' })
    ).toBe('https://cdn.example.com/custom.png')

    const fallback = resolveUserAvatarUrl({ id: 7, username: 'alice', avatar_url: null })
    expect(fallback).toBe(getDefaultUserAvatar(7))
    expect(isDefaultJournalAvatar(fallback)).toBe(true)

    expect(isDefaultJournalAvatar('https://cdn.example.com/custom.png')).toBe(false)
    expect(isDefaultJournalAvatar('')).toBe(false)
  })
})
