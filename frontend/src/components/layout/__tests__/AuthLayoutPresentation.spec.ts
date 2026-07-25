import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const authLayoutSource = readFileSync(resolve(dir, '../AuthLayout.vue'), 'utf8')
const authBrandPanelSource = readFileSync(resolve(dir, '../../auth/AuthBrandPanel.vue'), 'utf8')

describe('AuthLayout presentation contract', () => {
  it('composes the shared brand panel and light-theme behavior', () => {
    expect(authLayoutSource).toContain(
      "import AuthBrandPanel from '@/components/auth/AuthBrandPanel.vue'"
    )
    expect(authLayoutSource).toContain(
      "import { useAuthLightTheme } from '@/composables/useAuthLightTheme'"
    )
    expect(authLayoutSource).toMatch(/useAuthLightTheme\(\)/)
    expect(authLayoutSource).toContain('<AuthBrandPanel')
  })

  it('provides the shell, home route, and both content slots', () => {
    expect(authLayoutSource).toContain('class="auth-shell"')
    expect(authLayoutSource).toContain('to="/"')
    expect(authLayoutSource).toContain('<slot />')
    expect(authLayoutSource).toContain('<slot name="footer" />')
  })

  it('honors reduced-motion preferences', () => {
    expect(authLayoutSource).toContain('@media (prefers-reduced-motion: reduce)')
  })

  it('keeps the primary action shimmer behind all button content', () => {
    expect(authLayoutSource).toContain('isolation: isolate;')
    expect(authLayoutSource).toContain('z-index: -1;')
    expect(authLayoutSource).not.toContain(':deep(.auth-primary-action > *)')
  })

  it('uses the approved compact C-layout shell proportions', () => {
    expect(authLayoutSource).toContain('width: min(1280px, 100%);')
    expect(authLayoutSource).toContain('grid-template-columns: minmax(480px, 1fr) minmax(500px, 1fr);')
    expect(authLayoutSource).toContain('border-radius: 34px;')
  })

  it('uses the approved compact mobile brand treatment', () => {
    expect(authBrandPanelSource).toContain('@media (max-width: 900px)')
    expect(authBrandPanelSource).toContain('min-height: 230px;')
    expect(authBrandPanelSource).toContain('font-size: 29px;')
  })
})
