import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import CampaignTierIcon from '../CampaignTierIcon.vue'

describe('CampaignTierIcon', () => {
  it.each(['premium', 'gold', 'diamond', 'unknown-tier'])(
    'renders handcrafted SVG sticker artwork without img tags for "%s"',
    (tierKey) => {
      const wrapper = mount(CampaignTierIcon, {
        props: { tierKey },
      })

      expect(wrapper.find('svg').exists()).toBe(true)
      expect(wrapper.find('img').exists()).toBe(false)
      expect(wrapper.find('filter').exists()).toBe(true)
    }
  )

  it('assigns unique SVG filter IDs across multiple instances', () => {
    const first = mount(CampaignTierIcon, { props: { tierKey: 'premium' } })
    const second = mount(CampaignTierIcon, { props: { tierKey: 'gold' } })

    const firstId = first.get('filter').attributes('id')
    const secondId = second.get('filter').attributes('id')

    expect(firstId).toBeTruthy()
    expect(secondId).toBeTruthy()
    expect(firstId).not.toBe(secondId)
  })

  it('applies xs, sm, and md size classes', () => {
    const xs = mount(CampaignTierIcon, { props: { tierKey: 'gold', size: 'xs' } })
    const sm = mount(CampaignTierIcon, { props: { tierKey: 'gold', size: 'sm' } })
    const md = mount(CampaignTierIcon, { props: { tierKey: 'gold', size: 'md' } })

    expect(xs.get('svg').classes()).toEqual(expect.arrayContaining(['h-5', 'w-5']))
    expect(sm.get('svg').classes()).toEqual(expect.arrayContaining(['h-9', 'w-9']))
    expect(md.get('svg').classes()).toEqual(expect.arrayContaining(['h-10', 'w-10']))
  })
})
