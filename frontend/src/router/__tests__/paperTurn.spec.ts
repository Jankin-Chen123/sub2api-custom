import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Router } from 'vue-router'
import {
  clipRectHalfPlane,
  computePageCurlState,
  resolvePaperTransition,
  sampleHumanTear,
  sampleTearVelocity,
} from '../paperTurn'
import {
  isPaperSoundEnabled,
  playMascotHighFiveSound,
  playPaperFlipSound,
  playPaperSoundToggleCue,
  playPaperTearSound,
  setPaperSoundEnabled,
  stopPaperSound,
  unlockPaperAudio,
} from '../paperSound'

describe('resolvePaperTransition', () => {
  it('triggers home cover tear when entering auth or workspace routes from /home', () => {
    expect(resolvePaperTransition('/home', '/login')).toBe('tear-home')
    expect(resolvePaperTransition('/home', '/register')).toBe('tear-home')
    expect(resolvePaperTransition('/home', '/dashboard')).toBe('tear-home')
    expect(resolvePaperTransition('/home', '/admin/dashboard')).toBe('tear-home')
  })

  it('triggers auth cover tear when returning to /home from /login or /register', () => {
    expect(resolvePaperTransition('/login', '/home')).toBe('tear-auth')
    expect(resolvePaperTransition('/register', '/home')).toBe('tear-auth')
  })

  it('triggers directional page flip between /login and /register', () => {
    expect(resolvePaperTransition('/login', '/register')).toBe('flip-forward')
    expect(resolvePaperTransition('/register', '/login')).toBe('flip-backward')
  })

  it('returns none for same-route or non-paper transitions', () => {
    expect(resolvePaperTransition('/home', '/home')).toBe('none')
    expect(resolvePaperTransition('/login', '/login')).toBe('none')
    expect(resolvePaperTransition('/dashboard', '/keys')).toBe('none')
    expect(resolvePaperTransition('/forgot-password', '/home')).toBe('none')
    expect(resolvePaperTransition('', '/home')).toBe('none')
  })
})

describe('sampleHumanTear & sampleTearVelocity', () => {
  it('maps endpoints cleanly and increases strictly monotonically across [0, 1]', () => {
    expect(sampleHumanTear(-0.5)).toBe(0)
    expect(sampleHumanTear(0)).toBe(0)
    expect(sampleHumanTear(1)).toBe(1)
    expect(sampleHumanTear(1.5)).toBe(1)

    let prev = sampleHumanTear(0)
    for (let i = 1; i <= 50; i++) {
      const curr = sampleHumanTear(i / 50)
      expect(curr).toBeGreaterThan(prev)
      prev = curr
    }
  })

  it('returns positive tear velocity across the entire [0, 1] interval', () => {
    for (let i = 0; i <= 20; i++) {
      const v = sampleTearVelocity(i / 20)
      expect(v).toBeGreaterThan(0)
    }
  })
})

describe('computePageCurlState & clipRectHalfPlane', () => {
  const applyMatrix = (
    m: [number, number, number, number, number, number],
    x: number,
    y: number
  ): [number, number] => [m[0] * x + m[2] * y + m[4], m[1] * x + m[3] * y + m[5]]

  it('starts completely flat at u = 0', () => {
    const state = computePageCurlState(0, 600, 750)
    expect(state.active).toBe(false)
    expect(state.frontClip).toBe('none')
    expect(state.curlRadius).toBe(0)
  })

  it('accurately reflects bottom-right corner to (px, py) while keeping fold line fixed', () => {
    const w = 600
    const h = 750
    for (const u of [0.08, 0.35, 0.65, 1.0]) {
      const state = computePageCurlState(u, w, h)
      expect(state.active).toBe(true)

      const [rx, ry] = applyMatrix(state.matrix, w, h)
      expect(rx).toBeCloseTo(state.px, 4)
      expect(ry).toBeCloseTo(state.py, 4)

      const [fx2, fy2] = applyMatrix(state.matrix, state.fx, state.fy)
      expect(fx2).toBeCloseTo(state.fx, 4)
      expect(fy2).toBeCloseTo(state.fy, 4)
    }
  })

  it('peels diagonally at early turn u=0.08 and aligns vertically at u=1.0', () => {
    const w = 600
    const h = 750
    const early = computePageCurlState(0.08, w, h)
    expect(early.angleDeg).toBeGreaterThan(20)
    expect(early.angleDeg).toBeLessThan(50)
    expect(early.frontClip).not.toBe('polygon(0px 0px, 0px 0px, 0px 0px)')
    expect(early.backClip).not.toBe('polygon(0px 0px, 0px 0px, 0px 0px)')

    const done = computePageCurlState(1.0, w, h)
    expect(Math.abs(done.angleDeg)).toBeCloseTo(0, 3)
    expect(done.d).toBeCloseTo(0, 3)
    expect(done.frontClip).toBe('polygon(0px 0px, 0px 0px, 0px 0px)')
    expect(clipRectHalfPlane(w, h, done.nx, done.ny, done.d, 'right')).toContain('600.00px 750.00px')
  })
})

it('skips muted gestures and reuses the prewarmed buffers across both flip directions', async () => {
  vi.resetModules()
  const { installPaperTurns } = await import('../paperTurn')
  const { playPaperFlipSound, setPaperSoundEnabled } = await import('../paperSound')
  const createBuffer = vi.fn((_channels: number, length: number) => ({
    getChannelData: () => new Float32Array(length),
  }))
  const start = vi.fn()
  class AudioContextStub {
    sampleRate = 8000
    state = 'running'
    createBuffer = createBuffer
    createBufferSource = () => ({ connect: vi.fn(), start, stop: vi.fn() })
    createBiquadFilter = () => ({ connect: vi.fn(), frequency: {}, Q: {}, gain: {} })
    createGain = () => ({ connect: vi.fn(), gain: {} })
  }
  vi.stubGlobal('AudioContext', AudioContextStub)
  const dispose = installPaperTurns({ afterEach: () => () => {} } as unknown as Router)
  try {
    setPaperSoundEnabled(false)
    window.dispatchEvent(new Event('pointerdown'))
    expect(createBuffer).not.toHaveBeenCalled()

    setPaperSoundEnabled(true)
    window.dispatchEvent(new Event('keydown'))
    const warmedBuffers = createBuffer.mock.calls.length
    expect(warmedBuffers).toBe(4)
    for (const direction of ['forward', 'backward'] as const) {
      playPaperFlipSound(0.66, direction)
      window.dispatchEvent(new Event('pointerdown'))
    }
    expect(start).toHaveBeenCalledTimes(2)
    expect(createBuffer).toHaveBeenCalledTimes(warmedBuffers)
  } finally {
    dispose()
    vi.unstubAllGlobals()
  }
})

describe('paperSound fail-safe behavior without Web Audio', () => {
  beforeEach(() => {
    localStorage.clear()
    setPaperSoundEnabled(true)
  })

  it('fails silently when AudioContext is unavailable in JSDOM', () => {
    expect(() => unlockPaperAudio()).not.toThrow()
    expect(() => playPaperTearSound(0.76, sampleTearVelocity)).not.toThrow()
    expect(() => playPaperFlipSound(0.56, 'forward')).not.toThrow()
    expect(() => playPaperFlipSound(0.56, 'backward')).not.toThrow()
    expect(() => playMascotHighFiveSound()).not.toThrow()
    expect(() => playMascotHighFiveSound(3)).not.toThrow()
    expect(() => playPaperSoundToggleCue()).not.toThrow()
    expect(() => stopPaperSound()).not.toThrow()

    setPaperSoundEnabled(false)
    expect(() => playMascotHighFiveSound(1)).not.toThrow()
  })

  it('persists page sound preference safely in localStorage', () => {
    expect(isPaperSoundEnabled()).toBe(true)

    setPaperSoundEnabled(false)
    expect(isPaperSoundEnabled()).toBe(false)
    expect(localStorage.getItem('sticker_paper_sound_enabled')).toBe('false')

    setPaperSoundEnabled(true)
    expect(isPaperSoundEnabled()).toBe(true)
    expect(localStorage.getItem('sticker_paper_sound_enabled')).toBe('true')
    expect(() => playPaperSoundToggleCue()).not.toThrow()
  })
})
