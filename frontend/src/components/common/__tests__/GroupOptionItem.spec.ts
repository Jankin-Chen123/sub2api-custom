import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import GroupOptionItem from '../GroupOptionItem.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null }),
}))

describe('GroupOptionItem description layout', () => {
  it('applies multiline and overflow-safe text styles', () => {
    const description = 'First section\nvery-long-unbroken-description-value-that-must-not-overflow'
    const wrapper = mount(GroupOptionItem, {
      props: {
        name: 'Example group',
        platform: 'openai',
        description,
      },
      global: {
        stubs: {
          GroupBadge: true,
        },
      },
    })

    const descriptionElement = wrapper
      .findAll('span')
      .find((element) => element.text() === description)

    expect(descriptionElement).toBeDefined()
    expect(descriptionElement?.classes()).toContain('whitespace-pre-line')
    expect(descriptionElement?.classes()).toContain('[overflow-wrap:anywhere]')
    expect(descriptionElement?.classes()).toContain('line-clamp-3')
    expect(wrapper.find('[title]').attributes('title')).toBe(description)
  })

  it('shows the final campaign rate with the existing rate presentation', () => {
    const wrapper = mount(GroupOptionItem, {
      props: {
        name: 'Example group',
        platform: 'openai',
        rateMultiplier: 0.15,
        userRateMultiplier: 0.14,
        campaignFactor: 0.96,
      },
      global: {
        stubs: {
          GroupBadge: true,
        },
      },
    })

    const ratePill = wrapper.get('[data-test="group-rate-pill"]')
    expect(ratePill.text()).toContain('0.14x')
    expect(ratePill.text()).toContain('0.1344x')
    expect(ratePill.text()).not.toContain('0.15x')
    expect(ratePill.text()).not.toContain('→')
    expect(ratePill.attributes('title')).toBe('campaign.membershipFactor')
  })

  it('keeps the original rate presentation without an active campaign factor', () => {
    const wrapper = mount(GroupOptionItem, {
      props: {
        name: 'Example group',
        platform: 'openai',
        rateMultiplier: 0.15,
        campaignFactor: null,
      },
      global: {
        stubs: {
          GroupBadge: true,
        },
      },
    })

    const ratePill = wrapper.get('[data-test="group-rate-pill"]')
    expect(ratePill.text()).toContain('0.15x')
    expect(ratePill.text()).not.toContain('→')
    expect(ratePill.attributes('title')).toBeUndefined()
  })
})
