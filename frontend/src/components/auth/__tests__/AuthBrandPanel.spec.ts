import { mount } from '@vue/test-utils'
import { baseCompile } from '@intlify/message-compiler'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'
import AuthBrandPanel from '@/components/auth/AuthBrandPanel.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messageCompiler: (message: string) => new Function(`return ${baseCompile(message).code}`)(),
  messages: {
    en: {
      auth: {
        brand: {
          kicker: 'WELCOME TO AI GATEWAY',
          headline: 'Every route to AI, carefully maintained',
          description: 'Clear, stable and transparent.',
          billingTitle: 'Traceable billing',
          billingDescription: 'Every unit of usage can be reviewed',
          routingTitle: 'Flexible routing',
          routingDescription: 'Automatically selects an available route',
        },
      },
    },
  },
})

describe('AuthBrandPanel', () => {
  it('renders one site logo and the localized brand story', () => {
    const wrapper = mount(AuthBrandPanel, {
      props: {
        siteName: 'MySub2API',
        siteLogo: 'https://example.com/logo.png',
      },
      global: {
        plugins: [i18n],
      },
    })

    const logos = wrapper.findAll('img')
    expect(logos).toHaveLength(1)
    expect(logos[0].attributes('src')).toBe('https://example.com/logo.png')
    expect(logos[0].attributes('alt')).toBe('MySub2API')
    expect(wrapper.text()).toContain('MySub2API')
    expect(wrapper.text()).toContain('Every route to AI, carefully maintained')
    expect(wrapper.text()).toContain('Traceable billing')
    expect(wrapper.text()).toContain('Flexible routing')
  })
})
