import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const viewSources = {
  login: readFileSync(resolve(dir, '../LoginView.vue'), 'utf8'),
  register: readFileSync(resolve(dir, '../RegisterView.vue'), 'utf8'),
  forgot: readFileSync(resolve(dir, '../ForgotPasswordView.vue'), 'utf8'),
  reset: readFileSync(resolve(dir, '../ResetPasswordView.vue'), 'utf8')
}

describe('auth page presentation contract', () => {
  it('applies the shared login and registration presentation hooks', () => {
    expect(viewSources.login).toContain('<AuthModeTabs active="login" />')
    expect(viewSources.register).toContain('<AuthModeTabs active="register" />')

    for (const source of [viewSources.login, viewSources.register]) {
      expect(source).toContain('auth-view-heading')
      expect(source).toContain('auth-primary-action')
    }
  })

  it('keeps Turnstile on credential entry pages only', () => {
    for (const source of [viewSources.login, viewSources.register, viewSources.forgot]) {
      expect(source).toContain('v-if="turnstileEnabled && turnstileSiteKey"')
      expect(source).toContain('<TurnstileWidget')
    }

    expect(viewSources.reset).not.toContain('TurnstileWidget')
  })

  it('does not expose infrastructure vendor names in auth page source', () => {
    for (const source of Object.values(viewSources)) {
      expect(source).not.toContain('CLOUDFLARE')
      expect(source).not.toContain('Cloudflare')
    }
  })
})
