/**
 * Procedural Web Audio synthesizer for handcrafted journal tear & page-flip acoustics.
 * Zero external audio assets required; fails silently when AudioContext is blocked or absent.
 */

type VelocitySampler = (t: number) => number

interface WindowWithWebkitAudio extends Window {
  webkitAudioContext?: typeof AudioContext
}

const PAPER_SOUND_STORAGE_KEY = 'sticker_paper_sound_enabled'
export const PAPER_FLIP_DURATION_MS = 660
export const PAPER_TEAR_DURATION_MS = 760

function readInitialSoundEnabled(): boolean {
  try {
    if (typeof window === 'undefined' || typeof window.localStorage === 'undefined') return true
    return window.localStorage.getItem(PAPER_SOUND_STORAGE_KEY) !== 'false'
  } catch {
    return true
  }
}

let soundEnabled = readInitialSoundEnabled()
let audioCtx: AudioContext | undefined
let activeGain: GainNode | undefined
let activeSource: AudioBufferSourceNode | undefined

let cachedSampleRate = 0
let cachedTearDuration = 0
let cachedTearPool: AudioBuffer[] = []
let tearVariantCursor = 0

let cachedFlipDuration = 0
let cachedFlipForward: AudioBuffer | undefined
let cachedFlipBackward: AudioBuffer | undefined

const DEFAULT_VELOCITY: VelocitySampler = (t) => 0.4 + 1.1 * Math.sin(Math.PI * Math.min(1, Math.max(0, t)))

function getAudioContextClass(): typeof AudioContext | undefined {
  if (typeof window === 'undefined') return undefined
  return window.AudioContext || (window as WindowWithWebkitAudio).webkitAudioContext
}

function ensureAudioContext(): AudioContext | undefined {
  try {
    if (!audioCtx) {
      const Ctor = getAudioContextClass()
      if (!Ctor) return undefined
      audioCtx = new Ctor()
    }
    if (audioCtx.state === 'suspended') {
      audioCtx.resume().catch(() => {})
    }
    return audioCtx
  } catch {
    return undefined
  }
}

function createRng(seed: number) {
  let state = seed >>> 0 || 1
  return () => {
    state ^= state << 13
    state ^= state >>> 17
    state ^= state << 5
    return ((state >>> 0) & 0x7fffffff) / 0x7fffffff
  }
}

function normalizeStereo(left: Float32Array, right: Float32Array, peakTarget = 0.88) {
  let peak = 0
  for (let i = 0; i < left.length; i++) {
    const a = Math.abs(left[i])
    const b = Math.abs(right[i])
    if (a > peak) peak = a
    if (b > peak) peak = b
  }
  if (peak <= 1e-5) return
  const scale = peakTarget / peak
  for (let i = 0; i < left.length; i++) {
    left[i] *= scale
    right[i] *= scale
  }
}

function buildTearBuffer(
  ctx: AudioContext,
  durationSec: number,
  sampleVelocity: VelocitySampler,
  seed: number
): AudioBuffer {
  const sr = ctx.sampleRate || 44100
  const length = Math.max(Math.floor(sr * 0.25), Math.floor(sr * durationSec))
  const buffer = ctx.createBuffer(2, length, sr)
  const left = buffer.getChannelData(0)
  const right = buffer.getChannelData(1)
  const rand = createRng(seed)

  // Precompute velocity & slow tactile friction modulation curve across buffer frames
  const modCurve = new Float32Array(length)
  const velCurve = new Float32Array(length)
  let walk = 0.72
  const dt = 1 / sr

  for (let i = 0; i < length; i++) {
    const t = i / (length - 1 || 1)
    const vel = Math.max(0.12, sampleVelocity(t))
    velCurve[i] = vel
    if (i % 64 === 0) {
      walk = Math.max(0.32, Math.min(1.25, walk * 0.84 + (rand() * 0.34 + 0.02)))
    }
    modCurve[i] = walk
  }

  // Layer A: Initial pinch crack (0.012s..0.055s) as thumb & finger initiate the edge tear
  const pinchStart = Math.floor(0.012 * sr)
  const pinchLen = Math.min(length - pinchStart, Math.floor(0.045 * sr))
  let pinchLp = 0
  for (let k = 0; k < pinchLen; k++) {
    const idx = pinchStart + k
    const u = k / (pinchLen || 1)
    const env = Math.sin(Math.min(1, u * 5) * (Math.PI / 2)) * Math.exp(-5.4 * u)
    const white = rand() * 2 - 1
    pinchLp += 0.36 * (white - pinchLp)
    const body = (white - pinchLp * 0.55) * env * 0.52
    left[idx] += body * 0.44
    right[idx] += body * 0.64
  }

  // Layer B: Granular cellulose fiber micro-fractures spaced inversely with tear velocity
  let cursor = Math.floor(0.016 * sr)
  const stopAt = Math.max(cursor, length - Math.floor(0.075 * sr))
  while (cursor < stopAt) {
    const t = cursor / (length - 1 || 1)
    const vel = velCurve[cursor]
    const burstSamples = Math.max(Math.floor(0.0025 * sr), Math.floor((0.0026 + rand() * 0.009) * sr))
    const amp = (0.34 + rand() * 0.62) * (0.48 + 0.55 * vel)
    const pan = 0.28 - 0.56 * t + (rand() - 0.5) * 0.14
    const gL = Math.sqrt(Math.max(0.05, 0.5 * (1 - pan)))
    const gR = Math.sqrt(Math.max(0.05, 0.5 * (1 + pan)))

    let prev = 0
    for (let j = 0; j < burstSamples && cursor + j < length; j++) {
      const u = j / burstSamples
      const grainEnv = Math.sin(Math.PI * Math.min(1, u * 4)) * Math.exp(-4.6 * u)
      const n = rand() * 2 - 1
      const hp = n - 0.68 * prev
      prev = n
      const sample = hp * amp * grainEnv
      left[cursor + j] += sample * gL
      right[cursor + j] += sample * gR
    }

    const meanGapMs = Math.max(2.4, 8.6 / Math.max(0.4, vel))
    const gapSamples = Math.max(Math.floor(0.0018 * sr), Math.floor(((0.35 + rand() * 1.35) * meanGapMs) * 0.001 * sr))
    cursor += gapSamples
  }

  // Layer C & D: Continuous modulated cellulose shred + warm cardstock resonance + clean edge fades
  let lpA = 0
  let lpB = 0
  let bpL = 0
  let bpR = 0
  const fadeInSamples = Math.max(1, Math.floor(0.008 * sr))
  const fadeOutSamples = Math.max(1, Math.floor(0.092 * sr))
  const cutoffBase = 2 * Math.PI * 820 * dt

  for (let i = 0; i < length; i++) {
    const t = i / (length - 1 || 1)
    const vel = velCurve[i]
    const velGain = Math.min(1.45, 0.28 + 0.68 * vel) * modCurve[i]

    const nL = rand() * 2 - 1
    const nR = rand() * 2 - 1
    lpA += 0.28 * (nL - lpA)
    lpB += 0.11 * (nL - lpB)
    const midBand = (lpA - lpB) * 0.65
    const crisp = (nL - lpA) * 0.38
    const deco = (nR - lpA) * 0.22

    // Subtle shifting resonant paper mode tracks tear speed
    const resAlpha = Math.min(0.42, cutoffBase * (0.85 + 0.35 * vel))
    bpL += resAlpha * ((nL * 0.5 + midBand) - bpL)
    bpR += resAlpha * ((nR * 0.5 + midBand) - bpR)

    const pan = 0.28 - 0.56 * t
    const gL = Math.sqrt(Math.max(0.05, 0.5 * (1 - pan)))
    const gR = Math.sqrt(Math.max(0.05, 0.5 * (1 + pan)))

    let env = 1
    if (i < fadeInSamples) {
      env = i / fadeInSamples
    } else if (i > length - fadeOutSamples) {
      env = Math.max(0, (length - i) / fadeOutSamples)
      env = env * env
    }

    const base = (midBand * 0.45 + crisp * 0.35) * velGain
    left[i] = (left[i] * 0.62 + (base + bpL * 0.26) * gL) * env
    right[i] = (right[i] * 0.62 + (base + deco + bpR * 0.26) * gR) * env
  }

  normalizeStereo(left, right, 0.88)
  return buffer
}

function buildFlipBuffer(
  ctx: AudioContext,
  durationSec: number,
  direction: 'forward' | 'backward',
  seed: number
): AudioBuffer {
  const sr = ctx.sampleRate || 44100
  const length = Math.max(Math.floor(sr * 0.25), Math.floor(sr * durationSec))
  const buffer = ctx.createBuffer(2, length, sr)
  const left = buffer.getChannelData(0)
  const right = buffer.getChannelData(1)
  const rand = createRng(seed)

  let lpLiftL = 0
  let lpLiftR = 0
  let lpAirL = 0
  let lpAirR = 0
  let lpLowL = 0
  let lpLowR = 0

  const fadeIn = Math.max(1, Math.floor(0.012 * sr))
  const fadeOut = Math.max(1, Math.floor(0.065 * sr))

  for (let i = 0; i < length; i++) {
    const t = i / (length - 1 || 1)
    const wL = rand() * 2 - 1
    const wR = rand() * 2 - 1

    // Smooth dual-rate noise shaping (soft paper sheen + warm page thump)
    lpLiftL += 0.22 * (wL - lpLiftL)
    lpLiftR += 0.22 * (wR - lpLiftR)
    lpAirL += 0.075 * (wL - lpAirL)
    lpAirR += 0.075 * (wR - lpAirR)
    lpLowL += 0.032 * (wL - lpLowL)
    lpLowR += 0.032 * (wR - lpLowR)

    const textureL = (lpLiftL - lpAirL) * 0.72 + (lpAirL - lpLowL) * 0.55
    const textureR = (lpLiftR - lpAirR) * 0.72 + (lpAirR - lpLowR) * 0.55

    // Stage 1: light finger tip lift brush (0..0.18)
    const liftEnv = t < 0.22 ? Math.sin(Math.min(1, t / 0.18) * Math.PI) * 0.42 : 0
    // Stage 2: airy page arc sweep peaking near mid-turn (0.12..0.72)
    const sweepPos = Math.max(0, Math.min(1, (t - 0.08) / 0.68))
    const sweepEnv = sweepPos > 0 && sweepPos < 1 ? Math.pow(Math.sin(Math.PI * sweepPos), 1.45) * 0.95 : 0
    // Stage 3: cushioned warm settle pat on cream notebook paper (0.68..0.95)
    const settlePos = Math.max(0, Math.min(1, (t - 0.66) / 0.30))
    const settleEnv = settlePos > 0 && settlePos < 1 ? Math.sin(Math.PI * settlePos) * 0.58 : 0

    let env = 1
    if (i < fadeIn) env = i / fadeIn
    else if (i > length - fadeOut) env = Math.pow(Math.max(0, (length - i) / fadeOut), 1.6)

    const panDir = direction === 'forward' ? 0.32 - 0.64 * t : -0.32 + 0.64 * t
    const gL = Math.sqrt(Math.max(0.05, 0.5 * (1 - panDir)))
    const gR = Math.sqrt(Math.max(0.05, 0.5 * (1 + panDir)))

    const ampL = (textureL * (liftEnv + sweepEnv) + lpLowL * settleEnv * 1.15) * env
    const ampR = (textureR * (liftEnv + sweepEnv) + lpLowR * settleEnv * 1.15) * env

    left[i] = ampL * gL
    right[i] = ampR * gR
  }

  normalizeStereo(left, right, 0.85)
  return buffer
}

function getTearBuffer(ctx: AudioContext, durationSec: number, sampleVelocity?: VelocitySampler): AudioBuffer {
  const sampler = sampleVelocity || DEFAULT_VELOCITY
  const sr = ctx.sampleRate || 44100
  if (
    cachedTearPool.length === 0 ||
    cachedSampleRate !== sr ||
    Math.abs(cachedTearDuration - durationSec) > 0.02
  ) {
    cachedSampleRate = sr
    cachedTearDuration = durationSec
    cachedTearPool = [
      buildTearBuffer(ctx, durationSec, sampler, 0x51a9d31f),
      buildTearBuffer(ctx, durationSec, sampler, 0x7f3b8c25),
    ]
  }
  const idx = tearVariantCursor % cachedTearPool.length
  tearVariantCursor++
  return cachedTearPool[idx]
}

function getFlipBuffer(ctx: AudioContext, durationSec: number, direction: 'forward' | 'backward'): AudioBuffer {
  const sr = ctx.sampleRate || 44100
  if (
    cachedSampleRate !== sr ||
    Math.abs(cachedFlipDuration - durationSec) > 0.02 ||
    !cachedFlipForward ||
    !cachedFlipBackward
  ) {
    cachedSampleRate = sr
    cachedFlipDuration = durationSec
    cachedFlipForward = buildFlipBuffer(ctx, durationSec, 'forward', 0x3b7192cd)
    cachedFlipBackward = buildFlipBuffer(ctx, durationSec, 'backward', 0x6284e19a)
  }
  return direction === 'forward' ? cachedFlipForward : cachedFlipBackward
}

/** Return whether journal paper tear & page-flip sound effects are enabled. */
export function isPaperSoundEnabled(): boolean {
  try {
    if (typeof window !== 'undefined' && typeof window.localStorage !== 'undefined') {
      const stored = window.localStorage.getItem(PAPER_SOUND_STORAGE_KEY)
      if (stored === 'false') soundEnabled = false
      else if (stored === 'true') soundEnabled = true
    }
  } catch {
    // Ignore storage read restrictions
  }
  return soundEnabled
}

/** Enable or mute journal paper sound effects and persist preference cleanly. */
export function setPaperSoundEnabled(enabled: boolean): void {
  soundEnabled = enabled
  try {
    if (typeof window !== 'undefined' && typeof window.localStorage !== 'undefined') {
      window.localStorage.setItem(PAPER_SOUND_STORAGE_KEY, enabled ? 'true' : 'false')
    }
  } catch {
    // Ignore storage write restrictions
  }
  if (!enabled) {
    stopPaperSound()
  }
}

/** Play a brief soft paper-tap cue when re-enabling page sound effects. */
export function playPaperSoundToggleCue(): void {
  if (!isPaperSoundEnabled()) return
  try {
    const ctx = ensureAudioContext()
    if (!ctx) return
    stopPaperSound()

    const sr = ctx.sampleRate || 44100
    const length = Math.max(1, Math.floor(sr * 0.11))
    const buffer = ctx.createBuffer(1, length, sr)
    const ch = buffer.getChannelData(0)
    const rand = createRng(0x2a94f17b)
    let lp = 0
    for (let i = 0; i < length; i++) {
      const t = i / (length - 1 || 1)
      const env = Math.sin(Math.PI * Math.min(1, t * 4)) * Math.exp(-6.5 * t)
      const n = rand() * 2 - 1
      lp += 0.25 * (n - lp)
      ch[i] = (n - lp * 0.5) * env * 0.72
    }

    const source = ctx.createBufferSource()
    source.buffer = buffer

    const hp = ctx.createBiquadFilter()
    hp.type = 'highpass'
    hp.frequency.value = 260

    const lpFilter = ctx.createBiquadFilter()
    lpFilter.type = 'lowpass'
    lpFilter.frequency.value = 3800

    const gain = ctx.createGain()
    gain.gain.value = 0.22

    source.connect(hp)
    hp.connect(lpFilter)
    lpFilter.connect(gain)
    gain.connect(ctx.destination)

    activeSource = source
    activeGain = gain

    source.onended = () => {
      if (activeSource === source) {
        activeSource = undefined
        activeGain = undefined
      }
      source.disconnect()
      hp.disconnect()
      lpFilter.disconnect()
      gain.disconnect()
    }

    source.start()
  } catch {
    // Silent fallback
  }
}

/** Lazily unlock Web Audio and warm paper buffers on first user pointer/key gesture. */
export function unlockPaperAudio(sampleVelocity?: VelocitySampler): boolean {
  if (!isPaperSoundEnabled()) return false
  try {
    const ctx = ensureAudioContext()
    if (ctx?.state !== 'running') return false
    getTearBuffer(ctx, PAPER_TEAR_DURATION_MS / 1000, sampleVelocity)
    getFlipBuffer(ctx, PAPER_FLIP_DURATION_MS / 1000, 'forward')
    return true
  } catch {
    return false
  }
}

/** Softly fade out and release any currently playing paper sound. */
export function stopPaperSound(): void {
  try {
    const prevSource = activeSource
    const prevGain = activeGain
    activeSource = undefined
    activeGain = undefined
    if (!prevSource || !prevGain || !audioCtx) return

    const now = audioCtx.currentTime
    prevGain.gain.cancelScheduledValues(now)
    prevGain.gain.setValueAtTime(Math.max(0.0001, prevGain.gain.value), now)
    prevGain.gain.linearRampToValueAtTime(0.0001, now + 0.025)
    prevSource.stop(now + 0.03)
  } catch {
    // Ignore stop errors if node already stopped
  }
}

/** Play stereo cotton-paper tear audio synchronized to visual duration & velocity curve. */
export function playPaperTearSound(durationSec = PAPER_TEAR_DURATION_MS / 1000, sampleVelocity?: VelocitySampler): void {
  if (!isPaperSoundEnabled()) return
  try {
    const ctx = ensureAudioContext()
    if (!ctx) return
    stopPaperSound()

    const buffer = getTearBuffer(ctx, durationSec, sampleVelocity)
    const source = ctx.createBufferSource()
    source.buffer = buffer

    const hp = ctx.createBiquadFilter()
    hp.type = 'highpass'
    hp.frequency.value = 160

    const bodyPeak = ctx.createBiquadFilter()
    bodyPeak.type = 'peaking'
    bodyPeak.frequency.value = 950
    bodyPeak.Q.value = 0.9
    bodyPeak.gain.value = 2.5

    const shelf = ctx.createBiquadFilter()
    shelf.type = 'highshelf'
    shelf.frequency.value = 5800
    shelf.gain.value = -4.5

    const lp = ctx.createBiquadFilter()
    lp.type = 'lowpass'
    lp.frequency.value = 9200

    const gain = ctx.createGain()
    gain.gain.value = 0.40

    source.connect(hp)
    hp.connect(bodyPeak)
    bodyPeak.connect(shelf)
    shelf.connect(lp)
    lp.connect(gain)
    gain.connect(ctx.destination)

    activeSource = source
    activeGain = gain

    source.onended = () => {
      if (activeSource === source) {
        activeSource = undefined
        activeGain = undefined
      }
      source.disconnect()
      hp.disconnect()
      bodyPeak.disconnect()
      shelf.disconnect()
      lp.disconnect()
      gain.disconnect()
    }

    source.start()
  } catch {
    // Silent fallback
  }
}

/** Play gentle stereo page-turn paper rustle when flipping between auth journal sheets. */
export function playPaperFlipSound(durationSec = PAPER_FLIP_DURATION_MS / 1000, direction: 'forward' | 'backward' = 'forward'): void {
  if (!isPaperSoundEnabled()) return
  try {
    const ctx = ensureAudioContext()
    if (!ctx) return
    stopPaperSound()

    const buffer = getFlipBuffer(ctx, durationSec, direction)
    const source = ctx.createBufferSource()
    source.buffer = buffer

    const hp = ctx.createBiquadFilter()
    hp.type = 'highpass'
    hp.frequency.value = 180

    const lp = ctx.createBiquadFilter()
    lp.type = 'lowpass'
    lp.frequency.value = 4200

    const gain = ctx.createGain()
    gain.gain.value = 0.28

    source.connect(hp)
    hp.connect(lp)
    lp.connect(gain)
    gain.connect(ctx.destination)

    activeSource = source
    activeGain = gain

    source.onended = () => {
      if (activeSource === source) {
        activeSource = undefined
        activeGain = undefined
      }
      source.disconnect()
      hp.disconnect()
      lp.disconnect()
      gain.disconnect()
    }

    source.start()
  } catch {
    // Silent fallback
  }
}

/** Play tactile High-Five palm-slap + stamp thump sound synced with mascot hover-click combo. */
export function playMascotHighFiveSound(comboStep = 0): void {
  if (!isPaperSoundEnabled()) return
  try {
    const ctx = ensureAudioContext()
    if (!ctx) return

    const now = ctx.currentTime
    const sr = ctx.sampleRate || 44100
    const master = ctx.createGain()
    master.gain.value = 0.85
    master.connect(ctx.destination)

    // Layer 1: crisp paper palm-slap / stamp noise transient
    const slapLen = Math.max(1, Math.floor(sr * 0.075))
    const slapBuf = ctx.createBuffer(1, slapLen, sr)
    const slapData = slapBuf.getChannelData(0)
    const rand = createRng(0x48494635 + Math.abs(comboStep) * 97)
    for (let i = 0; i < slapLen; i++) {
      const u = i / (slapLen - 1 || 1)
      const env = Math.sin(Math.min(1, u * 8) * (Math.PI / 2)) * Math.exp(-5.5 * u)
      slapData[i] = (rand() * 2 - 1) * env
    }

    const slapSrc = ctx.createBufferSource()
    slapSrc.buffer = slapBuf
    const slapFilter = ctx.createBiquadFilter()
    slapFilter.type = 'bandpass'
    slapFilter.frequency.value = 1150
    slapFilter.Q.value = 1.1
    const slapGain = ctx.createGain()
    slapGain.gain.value = 0.26

    slapSrc.connect(slapFilter)
    slapFilter.connect(slapGain)
    slapGain.connect(master)
    slapSrc.onended = () => {
      slapSrc.disconnect()
      slapFilter.disconnect()
      slapGain.disconnect()
    }
    slapSrc.start(now + 0.025)

    // Layer 2: soft notebook desk thump
    const thumpOsc = ctx.createOscillator()
    thumpOsc.type = 'sine'
    thumpOsc.frequency.setValueAtTime(175, now + 0.025)
    thumpOsc.frequency.exponentialRampToValueAtTime(72, now + 0.1)
    const thumpGain = ctx.createGain()
    thumpGain.gain.setValueAtTime(0.0001, now + 0.025)
    thumpGain.gain.linearRampToValueAtTime(0.15, now + 0.033)
    thumpGain.gain.exponentialRampToValueAtTime(0.0001, now + 0.105)

    thumpOsc.connect(thumpGain)
    thumpGain.connect(master)
    thumpOsc.onended = () => {
      thumpOsc.disconnect()
      thumpGain.disconnect()
    }
    thumpOsc.start(now + 0.025)
    thumpOsc.stop(now + 0.115)

    // Layer 3: upbeat playful rubbery chime / pop cycling up a 5-step pentatonic curve
    const semitones = [0, 2, 4, 7, 9][Math.abs(comboStep) % 5]
    const ratio = Math.pow(2, semitones / 12)
    const popOsc = ctx.createOscillator()
    popOsc.type = 'sine'
    popOsc.frequency.setValueAtTime(460 * ratio, now + 0.042)
    popOsc.frequency.exponentialRampToValueAtTime(620 * ratio, now + 0.137)
    const popGain = ctx.createGain()
    popGain.gain.setValueAtTime(0.0001, now + 0.042)
    popGain.gain.linearRampToValueAtTime(0.10, now + 0.056)
    popGain.gain.exponentialRampToValueAtTime(0.0001, now + 0.187)

    popOsc.connect(popGain)
    popGain.connect(master)
    popOsc.onended = () => {
      popOsc.disconnect()
      popGain.disconnect()
      master.disconnect()
    }
    popOsc.start(now + 0.042)
    popOsc.stop(now + 0.195)
  } catch {
    // Silent fallback
  }
}
