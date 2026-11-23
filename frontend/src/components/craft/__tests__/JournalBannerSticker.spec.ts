import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import JournalBannerSticker from '../JournalBannerSticker.vue'

const ALL_STICKER_NAMES = [
  'wallet',
  'key',
  'palette',
  'batch',
  'bars',
  'channels',
  'radio',
  'crown',
  'recharge',
  'receipt',
  'gift',
  'share',
  'plug',
  'admin',
  'contact',
  'sparkles',
] as const

describe('JournalBannerSticker', () => {
  it.each(ALL_STICKER_NAMES)('renders die-cut SVG sticker without img tags for "%s"', (name) => {
    const wrapper = mount(JournalBannerSticker, {
      props: { name },
    })

    const hit = wrapper.get('.jbs-hit')
    expect(hit.classes()).toContain(`jbs-icon--${name}`)
    expect(hit.attributes('aria-hidden')).toBe('true')
    expect(wrapper.find('svg.jbs-svg').exists()).toBe(true)
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('filter').exists()).toBe(true)
  })

  it('falls back to sparkles for unknown icon names', () => {
    const wrapper = mount(JournalBannerSticker, {
      props: { name: 'non-existent-icon' },
    })

    expect(wrapper.get('.jbs-hit').classes()).toContain('jbs-icon--sparkles')
  })

  it('assigns unique SVG filter IDs across multiple instances', () => {
    const first = mount(JournalBannerSticker, { props: { name: 'wallet' } })
    const second = mount(JournalBannerSticker, { props: { name: 'gift' } })

    const firstId = first.get('filter').attributes('id')
    const secondId = second.get('filter').attributes('id')

    expect(firstId).toBeTruthy()
    expect(secondId).toBeTruthy()
    expect(firstId).not.toBe(secondId)
    expect(first.get('.jbs-art').attributes('filter')).toBe(`url(#${firstId})`)
    expect(second.get('.jbs-art').attributes('filter')).toBe(`url(#${secondId})`)
  })

  it('applies sm, md, and lg sizing classes', () => {
    const sm = mount(JournalBannerSticker, { props: { name: 'key', size: 'sm' } })
    const md = mount(JournalBannerSticker, { props: { name: 'key', size: 'md' } })
    const lg = mount(JournalBannerSticker, { props: { name: 'key', size: 'lg' } })

    expect(sm.get('.jbs-hit').classes()).toContain('w-[52px]')
    expect(sm.get('.jbs-hit').classes()).toContain('jbs-size--sm')
    expect(md.get('.jbs-hit').classes()).toContain('w-[60px]')
    expect(md.get('.jbs-hit').classes()).toContain('jbs-size--md')
    expect(lg.get('.jbs-hit').classes()).toContain('w-[68px]')
    expect(lg.get('.jbs-hit').classes()).toContain('jbs-size--lg')
  })
})
