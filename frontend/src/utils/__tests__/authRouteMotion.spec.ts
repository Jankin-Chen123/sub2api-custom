import { describe, expect, it } from 'vitest'
import { resolveAuthRouteMotion } from '@/utils/authRouteMotion'

describe('resolveAuthRouteMotion', () => {
  it.each([
    ['/home', '/login', 'initial'],
    ['/login', '/register', 'forward'],
    ['/login', '/forgot-password', 'forward'],
    ['/forgot-password', '/login', 'backward'],
    ['/forgot-password', '/reset-password', 'forward'],
    ['/reset-password', '/forgot-password', 'backward'],
    ['/register', '/forgot-password', 'neutral'],
    ['/login', '/dashboard', 'neutral'],
  ])('maps %s -> %s to %s', (from, to, expected) => {
    expect(resolveAuthRouteMotion(from, to)).toBe(expected)
  })
})
