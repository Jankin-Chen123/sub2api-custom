import { nextTick } from 'vue'
import type { Router } from 'vue-router'
import '@/styles/paper-turn.css'

function tearFrames(width: number, height: number, back: boolean) {
  // Anchor both ends to the 45° diagonal; the middle follows a few broad bends.
  const between = (lo: number, hi: number) => lo + Math.random() * (hi - lo)
  const amplitude = Math.min(52, Math.min(width, height) * .065)
  const side = Math.random() < .5 ? -1 : 1
  const bends = [
    {at: 0, x: 0},
    {at: between(.18, .28), x: side * amplitude * between(.68, .98)},
    {at: between(.44, .55), x: -side * amplitude},
    {at: between(.72, .84), x: side * amplitude * between(.48, .84)},
    {at: 1, x: 0},
  ]
  const spine = [{y: 0, dent: 0, fray: 1}]
  let segment = 0
  // Loose patches tear across fibres; tighter grain leaves a shorter, quieter edge.
  let patchEnd = 0, roughness = 1, snagAt = between(13, 30)
  for (let y = between(1, 3); y < height; y += between(1.6, 3.4 + roughness / 2)) {
    if (y >= patchEnd) {
      roughness = between(1.5, 5)
      patchEnd = y + between(14, 43)
    }
    const at = y / height
    while (at > bends[segment + 1].at) segment++
    const start = bends[segment], end = bends[segment + 1]
    const t = (at - start.at) / (end.at - start.at)
    const smooth = t * t * (3 - 2 * t)
    const contour = start.x + (end.x - start.x) * smooth
    const snag = y >= snagAt
    if (snag) snagAt = y + between(20, 55)
    const notch = snag ? -between(4, 9) : 0
    const dent = contour + notch + between(-roughness, roughness)
    const fray = snag ? between(1.18, 1.55) : between(.50, 1.13)
    spine.push({y, dent, fray})
  }
  spine.push({y: height, dent: 0, fray: 1})
  function polygon(progress: number, lip: number) {
    const clearance = amplitude + 16
    const intercept = width + height + clearance - progress * (width + height + 2 * clearance)
    const point = (x: number, y: number) => (back ? width - x : x).toFixed(1) + 'px ' + (back ? height - y : y).toFixed(1) + 'px'
    const edge = spine.map(({y, dent, fray}) => point(intercept - y + dent + lip * fray, y))
    return 'polygon(' + [point(0, 0), ...edge, point(0, height)].join(',') + ')'
  }
  // The exposed underside widens as the sheet lifts, then settles at the far corner.
  // Integrate a continuous speed curve: the pull catches twice and releases
  // without ever stopping or reversing. All three layers share this timing.
  const catch1 = between(.29, .34), catch2 = between(.60, .66)
  const speeds = [[0,.12], [.16,1.15], [catch1-.045,1.40], [catch1,.86],
    [catch1+.065,1.54], [catch2-.05,1.28], [catch2,.82],
    [catch2+.07,1.31], [.86,.86], [1,.09]]
  let distance = 0
  const stops = speeds.map(([offset, speed], index) => {
    const stop = {offset, distance, easing:'linear'}
    const next = speeds[index + 1]
    if (next) {
      const [nextTime, nextSpeed] = next
      const mean = (speed + nextSpeed) / 2
      distance += (nextTime - offset) * mean
      stop.easing = 'cubic-bezier(.333333,' + speed/(3*mean) + ',.666667,' + (1-nextSpeed/(3*mean)) + ')'
    }
    return stop
  })
  const layer = (scale: number) => stops.map(stop => {
    const progress = stop.distance / distance
    const lip = 17 * Math.sin(Math.PI * progress)
    return {offset:stop.offset, easing:stop.easing, clipPath:polygon(progress, lip * scale)}
  })
  return {sheet: layer(0), fibre: layer(1), shadow: layer(1.45)}
}

function capturePaper(source: HTMLElement) {
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
  Object.assign(stage.style, { left: left + 'px', top: top + 'px', width: width + 'px', height: height + 'px' })
  const sheet = document.createElement('div')
  sheet.className = 'paper-turn-sheet'
  if (source.closest('.sticker-dashboard')) sheet.classList.add('sticker-dashboard')

  const snapshot = source.cloneNode(true) as HTMLElement
  snapshot.querySelectorAll('[id]').forEach(element => element.removeAttribute('id'))
  snapshot.querySelectorAll('iframe, video, audio').forEach(element => element.remove())
  const canvases = source.querySelectorAll('canvas')
  snapshot.querySelectorAll('canvas').forEach((canvas, index) => {
    const original = canvases[index]
    if (original.width && original.height) canvas.getContext('2d')?.drawImage(original, 0, 0)
  })
  Object.assign(snapshot.style, {
    position: 'absolute', left: rect.left - left + 'px', top: rect.top - top + 'px',
    width: rect.width + 'px', maxWidth: 'none', margin: '0',
  })
  sheet.append(snapshot)
  stage.append(sheet)
  document.body.append(stage)
  return { stage, sheet, width, height }
}

/** Animate only the brand cover and the notebook's overview/usage sheet. */
export function installPaperTurns(router: Router) {
  let active: HTMLElement | undefined
  let cleanupTimer: ReturnType<typeof setTimeout> | undefined
  let previousPosition = router.options.history.state.position

  function clear() {
    clearTimeout(cleanupTimer)
    active?.getAnimations({ subtree: true }).forEach(animation => animation.cancel())
    active?.remove()
    active = undefined
  }

  const unregister = router.afterEach(async (to, from, failure) => {
    if (failure) return
    const position = router.options.history.state.position
    const backwards = typeof position === 'number' && typeof previousPosition === 'number' && position < previousPosition
    previousPosition = position
    if (from.path === to.path) return

    clear()
    if (!document.documentElement.animate || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
    const tear = from.path === '/home' && ['/login', '/dashboard', '/admin/dashboard'].includes(to.path)
    const flip = (from.path === '/dashboard' && to.path === '/usage') ||
      (from.path === '/usage' && to.path === '/dashboard')
    if (!tear && !flip) return
    const source = document.querySelector<HTMLElement>(tear ? '[data-sticker-home]' : '[data-app-content]')
    if (!source) return
    const paper = capturePaper(source)
    if (!paper) return
    const { stage, sheet, width, height } = paper
    active = stage
    await nextTick()
    if (active !== stage) return

    const duration = tear ? 1080 : 580
    cleanupTimer = setTimeout(clear, duration + 160)
    let motion: Animation
    if (tear) {
      const frames = tearFrames(width, height, backwards)
      const shadow = document.createElement('div')
      shadow.className = 'paper-turn-shadow'
      const fibre = document.createElement('div')
      fibre.className = 'paper-turn-fibre'
      stage.prepend(shadow, fibre)
      const timing: KeyframeAnimationOptions = { duration, fill: 'forwards' }
      shadow.animate(frames.shadow, timing)
      fibre.animate(frames.fibre, timing)
      motion = sheet.animate(frames.sheet, timing)
    } else {
      const reverse = backwards || from.path === '/usage'
      sheet.style.transformOrigin = reverse ? 'right center' : 'left center'
      const direction = reverse ? 1 : -1
      motion = sheet.animate([
        { transform: 'perspective(1700px) rotateY(0deg)', filter: 'brightness(1)' },
        { offset: .55, transform: `perspective(1700px) rotateY(${direction * 52}deg)`, filter: 'brightness(.85)' },
        { transform: `perspective(1700px) rotateY(${direction * 100}deg)`, filter: 'brightness(.68)', opacity: .25 },
      ], { duration, easing: 'cubic-bezier(.32,.02,.18,1)', fill: 'forwards' })
    }
    motion.finished.then(() => { if (active === stage) clear() }, () => {})
  })

  return () => {
    unregister()
    clear()
  }
}
