import { nextTick } from 'vue'
import type { Router } from 'vue-router'
import '@/styles/paper-turn.css'
import {
  PAPER_FLIP_DURATION_MS,
  PAPER_TEAR_DURATION_MS,
  playPaperFlipSound,
  playPaperTearSound,
  stopPaperSound,
  unlockPaperAudio,
} from './paperSound'

export type PaperTransitionKind = 'tear-home' | 'tear-auth' | 'flip-forward' | 'flip-backward' | 'none'

const TEAR_DESTINATIONS = new Set(['/login', '/register', '/dashboard', '/admin/dashboard'])
const AUTH_FLIP_PATHS = new Set(['/login', '/register'])

const CURVE_KEYS: [number, number][] = [
  [0.0, 0.16],
  [0.08, 0.56],
  [0.14, 0.94],
  [0.24, 1.46],
  [0.34, 0.96],
  [0.44, 1.56],
  [0.58, 0.92],
  [0.68, 1.48],
  [0.80, 1.58],
  [0.91, 1.12],
  [1.0, 0.28],
]

// Pre-integrate smooth monotonic 3-phase human tear curve once at module load
const INTEGRATED_CURVE = (() => {
  const samples: { t: number; p: number }[] = []
  let dist = 0
  const nodes: { t: number; dist: number; v0: number; v1: number }[] = []

  for (let i = 0; i < CURVE_KEYS.length; i++) {
    const [t, v] = CURVE_KEYS[i]
    nodes.push({ t, dist, v0: v, v1: v })
    if (i < CURVE_KEYS.length - 1) {
      const [nt, nv] = CURVE_KEYS[i + 1]
      dist += (nt - t) * ((v + nv) / 2)
    }
  }

  const totalDist = dist || 1
  for (let i = 0; i < CURVE_KEYS.length; i++) {
    samples.push({ t: nodes[i].t, p: nodes[i].dist / totalDist })
  }
  return samples
})()

export function resolvePaperTransition(fromPath: string, toPath: string): PaperTransitionKind {
  if (!fromPath || !toPath || fromPath === toPath) return 'none'
  if (fromPath === '/home' && TEAR_DESTINATIONS.has(toPath)) return 'tear-home'
  if (AUTH_FLIP_PATHS.has(fromPath) && toPath === '/home') return 'tear-auth'
  if (fromPath === '/login' && toPath === '/register') return 'flip-forward'
  if (fromPath === '/register' && toPath === '/login') return 'flip-backward'
  return 'none'
}

export function sampleHumanTear(rawT: number): number {
  const t = Math.min(1, Math.max(0, rawT))
  for (let i = 0; i < INTEGRATED_CURVE.length - 1; i++) {
    const a = INTEGRATED_CURVE[i]
    const b = INTEGRATED_CURVE[i + 1]
    if (t <= b.t) {
      const u = (t - a.t) / (b.t - a.t || 1)
      const ease = u * u * (3 - 2 * u)
      return a.p + (b.p - a.p) * ease
    }
  }
  return 1
}

export function sampleTearVelocity(rawT: number): number {
  const t = Math.min(1, Math.max(0, rawT))
  for (let i = 0; i < CURVE_KEYS.length - 1; i++) {
    const [t0, v0] = CURVE_KEYS[i]
    const [t1, v1] = CURVE_KEYS[i + 1]
    if (t <= t1) {
      const u = (t - t0) / (t1 - t0 || 1)
      return v0 + (v1 - v0) * u
    }
  }
  return CURVE_KEYS[CURVE_KEYS.length - 1][1]
}

function easePageTurn(rawT: number): number {
  const t = Math.min(1, Math.max(0, rawT))
  return t * t * t * (t * (t * 6 - 15) + 10)
}

export interface PageCurlState {
  active: boolean
  px: number
  py: number
  nx: number
  ny: number
  d: number
  fx: number
  fy: number
  angleDeg: number
  matrix: [number, number, number, number, number, number]
  frontClip: string
  backClip: string
  curlRadius: number
}

export function clipRectHalfPlane(
  width: number,
  height: number,
  nx: number,
  ny: number,
  d: number,
  side: 'left' | 'right'
): string {
  const rect: [number, number][] = [
    [0, 0],
    [width, 0],
    [width, height],
    [0, height],
  ]

  const eps = 1e-4
  const isInside = (pt: [number, number]) => {
    const val = pt[0] * nx + pt[1] * ny - d
    return side === 'left' ? val <= eps : val >= -eps
  }

  const out: [number, number][] = []
  for (let i = 0; i < rect.length; i++) {
    const a = rect[i]
    const b = rect[(i + 1) % rect.length]
    const aVal = a[0] * nx + a[1] * ny - d
    const bVal = b[0] * nx + b[1] * ny - d
    const aIn = isInside(a)
    const bIn = isInside(b)

    if (aIn && bIn) {
      out.push(b)
    } else if (aIn && !bIn) {
      const denom = aVal - bVal
      const alpha = Math.abs(denom) > 1e-7 ? aVal / denom : 0
      out.push([a[0] + alpha * (b[0] - a[0]), a[1] + alpha * (b[1] - a[1])])
    } else if (!aIn && bIn) {
      const denom = aVal - bVal
      const alpha = Math.abs(denom) > 1e-7 ? aVal / denom : 0
      out.push([a[0] + alpha * (b[0] - a[0]), a[1] + alpha * (b[1] - a[1])])
      out.push(b)
    }
  }

  if (out.length < 3) {
    return 'polygon(0px 0px, 0px 0px, 0px 0px)'
  }

  return `polygon(${out.map(([x, y]) => `${x.toFixed(2)}px ${y.toFixed(2)}px`).join(', ')})`
}

export function computePageCurlState(u: number, width: number, height: number): PageCurlState {
  const t = Math.min(1, Math.max(0, u))
  if (t <= 0.001) {
    return {
      active: false,
      px: width,
      py: height,
      nx: 1,
      ny: 0,
      d: width,
      fx: width,
      fy: height / 2,
      angleDeg: 0,
      matrix: [1, 0, 0, 1, 0, 0],
      frontClip: 'none',
      backClip: 'polygon(0px 0px, 0px 0px, 0px 0px)',
      curlRadius: 0,
    }
  }

  const px = width * (1 - 2 * t)
  const lift = 0.28 * height * Math.pow(Math.sin(Math.PI * t), 0.72) * (1 - t)
  const py = height - lift

  const vx = width - px
  const vy = height - py
  const dist = Math.hypot(vx, vy) || 1
  const nx = vx / dist
  const ny = vy / dist

  const mx = (width + px) / 2
  const my = (height + py) / 2
  const d = mx * nx + my * ny

  const matrix: [number, number, number, number, number, number] = [
    1 - 2 * nx * nx,
    -2 * nx * ny,
    -2 * nx * ny,
    1 - 2 * ny * ny,
    2 * d * nx,
    2 * d * ny,
  ]

  const centerOffset = (width / 2) * nx + (height / 2) * ny - d
  const fx = width / 2 - centerOffset * nx
  const fy = height / 2 - centerOffset * ny
  const angleDeg = (Math.atan2(ny, nx) * 180) / Math.PI
  const curlRadius = Math.min(126, Math.max(28, dist * 0.22))

  const frontClip =
    t >= 0.999
      ? 'polygon(0px 0px, 0px 0px, 0px 0px)'
      : clipRectHalfPlane(width, height, nx, ny, d, 'left')
  const backClip = clipRectHalfPlane(width, height, nx, ny, d, 'right')

  return {
    active: true,
    px,
    py,
    nx,
    ny,
    d,
    fx,
    fy,
    angleDeg,
    matrix,
    frontClip,
    backClip,
    curlRadius,
  }
}

function pinClonedPanel(clonedPanel: HTMLElement, widthPx: number, heightPx: number) {
  clonedPanel.style.width = `${widthPx}px`
  clonedPanel.style.height = `${heightPx}px`
  clonedPanel.style.minHeight = `${heightPx}px`
  clonedPanel.style.margin = '0'
}

function cloneNodeCleanly(
  source: HTMLElement,
  options: { stripBinderRings?: boolean; preserveDiecut?: boolean } = {}
): HTMLElement {
  const clone = source.cloneNode(true) as HTMLElement

  const srcFields = source.querySelectorAll<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>(
    'input, textarea, select'
  )
  const cloneFields = clone.querySelectorAll<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>(
    'input, textarea, select'
  )
  srcFields.forEach((field, index) => {
    const target = cloneFields[index]
    if (!target) return
    target.value = field.value
    if (field instanceof HTMLInputElement && target instanceof HTMLInputElement) {
      target.checked = field.checked
    }
  })

  clone.querySelectorAll('iframe, video, audio').forEach((element) => element.remove())
  clone.querySelectorAll('[autofocus]').forEach((element) => element.removeAttribute('autofocus'))
  if (options.stripBinderRings) {
    clone.querySelectorAll('.auth-binder-rings').forEach((element) => element.remove())
  }

  if (options.preserveDiecut) {
    const filterEl = clone.querySelector<SVGFilterElement>('#sticker-diecut')
    if (filterEl) {
      filterEl.id = 'pt-sticker-diecut'
    }
    clone.querySelectorAll<HTMLElement>('[id]').forEach((element) => {
      if (element.id !== 'pt-sticker-diecut') {
        element.removeAttribute('id')
      }
    })
    if (filterEl) {
      clone.querySelectorAll<HTMLElement>('.sticker-title, .sticker-setup-title').forEach((el) => {
        el.style.filter = 'url(#pt-sticker-diecut)'
      })
    }
  } else {
    clone.querySelectorAll<HTMLElement>('[id]').forEach((element) => {
      element.removeAttribute('id')
    })
  }

  return clone
}

function capturePaper(
  source: HTMLElement,
  options: { preserveDiecut?: boolean; sheetBackground?: string } = {}
) {
  const rect = source.getBoundingClientRect()
  const left = Math.max(rect.left, 0)
  const top = Math.max(rect.top, 0)
  const width = Math.min(rect.right, document.documentElement.clientWidth) - left
  const height = Math.min(rect.bottom, window.innerHeight) - top
  if (width <= 0 || height <= 0) return undefined

  const stage = document.createElement('div')
  stage.className = 'paper-turn-stage'
  stage.setAttribute('aria-hidden', 'true')
  stage.inert = true
  Object.assign(stage.style, {
    left: `${left}px`,
    top: `${top}px`,
    width: `${width}px`,
    height: `${height}px`,
  })

  const sheet = document.createElement('div')
  sheet.className = 'paper-turn-sheet'
  if (options.sheetBackground) {
    sheet.style.background = options.sheetBackground
  }

  const snapshot = cloneNodeCleanly(source, { preserveDiecut: options.preserveDiecut })
  Object.assign(snapshot.style, {
    position: 'absolute',
    left: `${rect.left - left}px`,
    top: `${rect.top - top}px`,
    width: `${rect.width}px`,
    maxWidth: 'none',
    margin: '0',
  })

  sheet.append(snapshot)
  stage.append(sheet)
  document.body.append(stage)
  return { stage, sheet, width, height }
}

/** Tear covers with a lock-step cotton seam & audio, and flip open-notebook pages between /login and /register. */
export function installPaperTurns(router: Router) {
  if (typeof Image !== 'undefined') {
    const seamPreload = new Image()
    seamPreload.src = '/sticker/torn-seam-strip.webp'
  }

  let active: HTMLElement | undefined
  let rafId = 0

  function clear() {
    if (rafId) {
      cancelAnimationFrame(rafId)
      rafId = 0
    }
    stopPaperSound()
    active?.remove()
    active = undefined
    document.documentElement.classList.remove('paper-flip-active')
  }

  function runTear(kind: 'tear-home' | 'tear-auth') {
    const isHome = kind === 'tear-home'
    const source = document.querySelector<HTMLElement>(isHome ? '[data-sticker-home]' : '.auth-page')
    if (!source) return
    const paper = capturePaper(source, {
      preserveDiecut: isHome,
      sheetBackground: isHome ? '#fff7e8' : '#f6ede0',
    })
    if (!paper) return

    const { stage, sheet, width, height } = paper
    active = stage

    const startAnimation = async () => {
      await nextTick()
      if (active !== stage) return
      await new Promise<void>((resolve) => {
        rafId = requestAnimationFrame(() => {
          if (active !== stage) return resolve()
          rafId = requestAnimationFrame(() => resolve())
        })
      })
      if (active !== stage) return

      const diagLength = Math.ceil(Math.SQRT2 * Math.max(width, height) + 320)
      const seamWidth = 160
      const seam = document.createElement('div')
      seam.className = 'paper-turn-seam'
      seam.style.width = `${seamWidth}px`
      seam.style.height = `${diagLength}px`
      stage.append(seam)

      const margin = 84
      const duration = PAPER_TEAR_DURATION_MS
      playPaperTearSound(duration / 1000, sampleTearVelocity)
      const startTime = performance.now()

      const renderFrame = (now: number) => {
        if (active !== stage) return
        const rawT = Math.min(1, (now - startTime) / duration)
        const p = sampleHumanTear(rawT)
        const intercept = width + height + margin - p * (width + height + 2 * margin)

        const curlScale = 0.72 + 0.28 * Math.sin(Math.PI * Math.min(1, Math.max(0, p)))
        const clipIntercept = intercept - 24 * Math.SQRT2 * curlScale

        if (clipIntercept <= 0) {
          sheet.style.clipPath = 'polygon(0px 0px, 0px 0px, 0px 0px)'
        } else if (clipIntercept >= width + height) {
          sheet.style.clipPath = 'none'
        } else {
          const pts: string[] = ['0px 0px', `${Math.min(width, clipIntercept).toFixed(1)}px 0px`]
          if (clipIntercept > width) {
            pts.push(`${width.toFixed(1)}px ${(clipIntercept - width).toFixed(1)}px`)
          }
          if (clipIntercept > height) {
            pts.push(`${(clipIntercept - height).toFixed(1)}px ${height.toFixed(1)}px`)
          }
          pts.push(`0px ${Math.min(height, clipIntercept).toFixed(1)}px`)
          sheet.style.clipPath = `polygon(${pts.join(', ')})`
        }

        // Lock-step seam strip along the exact diagonal x + y = intercept
        const cx = intercept / 2
        const cy = intercept / 2
        seam.style.transform = `translate3d(${(cx - seamWidth / 2).toFixed(1)}px, ${(
          cy -
          diagLength / 2
        ).toFixed(1)}px, 0) rotate(45deg) scaleX(${curlScale.toFixed(3)})`

        const opacity = rawT <= 0.88 ? 1 : Math.max(0, 1 - (rawT - 0.88) / 0.12)
        sheet.style.opacity = opacity.toFixed(3)
        seam.style.opacity = opacity.toFixed(3)

        if (rawT < 1) {
          rafId = requestAnimationFrame(renderFrame)
        } else {
          clear()
        }
      }

      rafId = requestAnimationFrame(renderFrame)
    }

    void startAnimation()
  }

  function runFlip(direction: 'forward' | 'backward') {
    const oldShell = document.querySelector<HTMLElement>('.auth-shell')
    const oldPanel = document.querySelector<HTMLElement>('.auth-form-panel')
    if (!oldShell || !oldPanel) return

    const oldShellRect = oldShell.getBoundingClientRect()
    const oldPanelRect = oldPanel.getBoundingClientRect()
    if (oldShellRect.width <= 0 || oldPanelRect.width <= 0 || oldPanelRect.height <= 0) return

    const isSpread = oldShellRect.width - oldPanelRect.width > 120
    const shellRadius = getComputedStyle(oldShell).borderRadius || '34px'
    const panelLeft = oldPanelRect.left - oldShellRect.left
    const panelTop = oldPanelRect.top - oldShellRect.top
    const panelWidth = oldPanelRect.width
    const panelHeight = oldPanelRect.height
    const frontRadius = isSpread ? `0 ${shellRadius} ${shellRadius} 0` : `0 0 ${shellRadius} ${shellRadius}`

    document.documentElement.classList.add('paper-flip-active')

    const stage = document.createElement('div')
    stage.className = 'paper-flip-stage'
    stage.setAttribute('aria-hidden', 'true')
    stage.inert = true
    Object.assign(stage.style, {
      left: `${oldShellRect.left}px`,
      top: `${oldShellRect.top}px`,
      width: `${oldShellRect.width}px`,
      height: `${oldShellRect.height}px`,
    })
    active = stage

    const buildRingsOverlay = (sourceShell: Element | null, shellRect: DOMRect) => {
      if (!isSpread || !sourceShell) return undefined
      const srcRings = sourceShell.querySelector<HTMLElement>('.auth-binder-rings')
      if (!srcRings) return undefined
      const ringRect = srcRings.getBoundingClientRect()
      const ringsWrap = document.createElement('div')
      ringsWrap.className = 'paper-flip-rings'
      Object.assign(ringsWrap.style, {
        left: `${ringRect.left - shellRect.left}px`,
        top: `${ringRect.top - shellRect.top}px`,
      })
      const clonedRings = cloneNodeCleanly(srcRings)
      clonedRings.style.position = 'static'
      clonedRings.style.top = 'auto'
      clonedRings.style.left = 'auto'
      clonedRings.style.margin = '0'
      ringsWrap.append(clonedRings)
      return ringsWrap
    }

    const syncRingsOverlay = (
      ringsWrap: HTMLElement | undefined,
      sourceShell: Element | null,
      shellRect: DOMRect
    ) => {
      if (!ringsWrap || !sourceShell) return
      const liveRings = sourceShell.querySelector<HTMLElement>('.auth-binder-rings')
      if (!liveRings) return
      const ringRect = liveRings.getBoundingClientRect()
      ringsWrap.style.left = `${ringRect.left - shellRect.left}px`
      ringsWrap.style.top = `${ringRect.top - shellRect.top}px`
    }

    const clipBox = document.createElement('div')
    clipBox.className = 'paper-curl-clip-box'
    Object.assign(clipBox.style, {
      left: `${panelLeft}px`,
      top: `${panelTop}px`,
      width: `${panelWidth}px`,
      height: `${panelHeight}px`,
      borderRadius: frontRadius,
    })

    const bandLength = Math.ceil(Math.hypot(panelWidth, panelHeight) * 2.4 + 240)

    const underShadowBand = document.createElement('div')
    underShadowBand.className = 'paper-curl-under-shadow-band'
    underShadowBand.style.height = `${bandLength}px`
    underShadowBand.style.opacity = '0'

    const frontSheet = document.createElement('div')
    frontSheet.className = 'paper-curl-front'

    const frontCrease = document.createElement('div')
    frontCrease.className = 'paper-curl-front-crease'
    frontCrease.style.height = `${bandLength}px`
    frontCrease.style.opacity = '0'

    const backWrap = document.createElement('div')
    backWrap.className = 'paper-curl-back-wrap'
    Object.assign(backWrap.style, {
      left: `${panelLeft}px`,
      top: `${panelTop}px`,
      width: `${panelWidth}px`,
      height: `${panelHeight}px`,
    })

    const backSheet = document.createElement('div')
    backSheet.className = 'paper-curl-back'
    backSheet.style.borderRadius = frontRadius

    const rollBand = document.createElement('div')
    rollBand.className = 'paper-curl-roll-band'
    rollBand.style.height = `${bandLength}px`
    rollBand.style.opacity = '0'

    backSheet.append(rollBand)
    backWrap.append(backSheet)

    let underPage: HTMLElement | undefined
    let ringsOverlay = buildRingsOverlay(oldShell, oldShellRect)

    const populateCurlContent = (sourcePanel: HTMLElement, w: number, h: number) => {
      const clonedFront = cloneNodeCleanly(sourcePanel, { stripBinderRings: true })
      pinClonedPanel(clonedFront, w, h)
      frontSheet.replaceChildren(clonedFront, frontCrease)
    }

    if (direction === 'forward') {
      populateCurlContent(oldPanel, panelWidth, panelHeight)
      clipBox.append(underShadowBand, frontSheet)
    } else {
      const clonedUnder = cloneNodeCleanly(oldPanel, { stripBinderRings: true })
      pinClonedPanel(clonedUnder, panelWidth, panelHeight)
      underPage = document.createElement('div')
      underPage.className = 'paper-curl-under-page'
      underPage.append(clonedUnder)
      clipBox.append(underPage, underShadowBand, frontSheet)
      backWrap.style.opacity = '0'
      frontSheet.style.clipPath = 'polygon(0px 0px, 0px 0px, 0px 0px)'
      backSheet.style.clipPath = 'polygon(0px 0px, 0px 0px, 0px 0px)'
    }

    stage.append(clipBox, backWrap)
    if (ringsOverlay) stage.append(ringsOverlay)

    document.body.append(stage)

    const startFlipAnimation = async () => {
      await nextTick()
      if (active !== stage) return
      await new Promise<void>((resolve) => {
        rafId = requestAnimationFrame(() => resolve())
      })
      if (active !== stage) return

      const newShell = document.querySelector<HTMLElement>('.auth-shell')
      const newPanel = document.querySelector<HTMLElement>('.auth-form-panel')
      if (!newShell || !newPanel) {
        clear()
        return
      }

      const newAuthPage = document.querySelector<HTMLElement>('.auth-page')
      newAuthPage?.classList.add('paper-flip-settled')

      const newShellRect = newShell.getBoundingClientRect()
      const newPanelRect = newPanel.getBoundingClientRect()
      const syncedLeft = newPanelRect.left - newShellRect.left
      const syncedTop = newPanelRect.top - newShellRect.top
      const curlWidth = direction === 'forward' ? panelWidth : newPanelRect.width || panelWidth
      const curlHeight = direction === 'forward' ? panelHeight : newPanelRect.height || panelHeight

      Object.assign(stage.style, {
        left: `${newShellRect.left}px`,
        top: `${newShellRect.top}px`,
        width: `${newShellRect.width}px`,
        height: `${newShellRect.height}px`,
      })

      Object.assign(clipBox.style, {
        left: `${syncedLeft}px`,
        top: `${syncedTop}px`,
        width: `${curlWidth}px`,
        height: `${curlHeight}px`,
      })

      Object.assign(backWrap.style, {
        left: `${syncedLeft}px`,
        top: `${syncedTop}px`,
        width: `${curlWidth}px`,
        height: `${curlHeight}px`,
      })

      const activeBandLen = Math.ceil(Math.hypot(curlWidth, curlHeight) * 2.4 + 240)
      underShadowBand.style.height = `${activeBandLen}px`
      frontCrease.style.height = `${activeBandLen}px`
      rollBand.style.height = `${activeBandLen}px`

      if (direction === 'backward') {
        populateCurlContent(newPanel, curlWidth, curlHeight)
      }

      if (!ringsOverlay) {
        ringsOverlay = buildRingsOverlay(newShell, newShellRect)
        if (ringsOverlay) stage.append(ringsOverlay)
      } else {
        syncRingsOverlay(ringsOverlay, newShell, newShellRect)
      }

      const duration = PAPER_FLIP_DURATION_MS
      playPaperFlipSound(duration / 1000, direction)
      const startTime = performance.now()

      const renderFrame = (now: number) => {
        if (active !== stage) return
        const rawT = Math.min(1, (now - startTime) / duration)
        const e = easePageTurn(rawT)
        const u = direction === 'forward' ? e : 1 - e
        const state = computePageCurlState(u, curlWidth, curlHeight)

        let backOpacity = 1
        if (direction === 'forward') {
          const fadeStart = isSpread ? 0.78 : 0.64
          backOpacity = rawT <= fadeStart ? 1 : Math.max(0, 1 - (rawT - fadeStart) / (1 - fadeStart))
        } else {
          const fadeInEnd = isSpread ? 0.16 : 0.20
          backOpacity = rawT <= fadeInEnd ? rawT / fadeInEnd : 1
          if (underPage) {
            const underFade = rawT <= 0.76 ? 1 : Math.max(0, 1 - (rawT - 0.76) / 0.20)
            underPage.style.opacity = underFade.toFixed(3)
          }
        }

        if (!state.active) {
          frontSheet.style.clipPath = 'none'
          backSheet.style.clipPath = 'polygon(0px 0px, 0px 0px, 0px 0px)'
          backWrap.style.opacity = '0'
          backWrap.style.filter = 'none'
          underShadowBand.style.opacity = '0'
          frontCrease.style.opacity = '0'
          rollBand.style.opacity = '0'
        } else {
          frontSheet.style.clipPath = state.frontClip
          backSheet.style.clipPath = state.backClip
          backSheet.style.transform = `matrix(${state.matrix.map((v) => v.toFixed(5)).join(', ')})`

          const halfBand = activeBandLen / 2
          const foldY = state.fy - halfBand
          const rot = `rotate(${state.angleDeg.toFixed(2)}deg)`
          const liftPresence = Math.sin(Math.PI * u)

          const underW = Math.min(82, Math.max(20, state.curlRadius * 0.68))
          underShadowBand.style.width = `${underW.toFixed(1)}px`
          underShadowBand.style.transform = `translate3d(${state.fx.toFixed(2)}px, ${foldY.toFixed(
            2
          )}px, 0) ${rot}`
          underShadowBand.style.opacity = (Math.min(1, liftPresence * 1.15) * (direction === 'backward' && u > 0.88 ? backOpacity : 1)).toFixed(3)

          const creaseW = Math.min(44, Math.max(16, state.curlRadius * 0.42))
          frontCrease.style.width = `${creaseW.toFixed(1)}px`
          frontCrease.style.transform = `translate3d(${(state.fx - creaseW).toFixed(2)}px, ${foldY.toFixed(
            2
          )}px, 0) ${rot}`
          frontCrease.style.opacity = Math.min(1, liftPresence * 1.2).toFixed(3)

          const rollW = state.curlRadius * 1.15
          rollBand.style.width = `${rollW.toFixed(1)}px`
          rollBand.style.transform = `translate3d(${state.fx.toFixed(2)}px, ${foldY.toFixed(2)}px, 0) ${rot}`
          rollBand.style.opacity = Math.min(0.85, liftPresence * 0.9 + 0.1).toFixed(3)

          backWrap.style.opacity = backOpacity.toFixed(3)
          if (liftPresence > 0.02) {
            backWrap.style.filter = `drop-shadow(${(-10 * liftPresence).toFixed(1)}px ${(
              10 * liftPresence
            ).toFixed(1)}px ${(18 * liftPresence).toFixed(1)}px rgba(56, 38, 16, ${(
              0.28 * liftPresence
            ).toFixed(3)}))`
          } else {
            backWrap.style.filter = 'none'
          }
        }

        if (rawT < 1) {
          rafId = requestAnimationFrame(renderFrame)
        } else {
          clear()
        }
      }

      rafId = requestAnimationFrame(renderFrame)
    }

    void startFlipAnimation()
  }

  function removeUnlockListeners() {
    window.removeEventListener('pointerdown', handleGestureUnlock)
    window.removeEventListener('keydown', handleGestureUnlock)
  }
  function handleGestureUnlock() {
    if (unlockPaperAudio(sampleTearVelocity)) removeUnlockListeners()
  }
  if (typeof window !== 'undefined') {
    window.addEventListener('pointerdown', handleGestureUnlock, { passive: true })
    window.addEventListener('keydown', handleGestureUnlock, { passive: true })
  }

  const unregister = router.afterEach((to, from, failure) => {
    if (failure) return
    const kind = resolvePaperTransition(from.path, to.path)
    if (kind === 'none') return

    clear()
    if (typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      return
    }

    if (kind === 'tear-home' || kind === 'tear-auth') {
      runTear(kind)
    } else if (kind === 'flip-forward' || kind === 'flip-backward') {
      runFlip(kind === 'flip-forward' ? 'forward' : 'backward')
    }
  })

  return () => {
    if (typeof window !== 'undefined') {
      removeUnlockListeners()
    }
    unregister()
    clear()
  }
}
