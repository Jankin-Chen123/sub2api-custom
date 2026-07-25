import { afterEach, describe, expect, it } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'

import { useAuthLightTheme } from '@/composables/useAuthLightTheme'

const Harness = defineComponent({
  name: 'AuthLightThemeHarness',
  setup() {
    useAuthLightTheme()
    return () => h('div')
  },
})

const wrappers: VueWrapper[] = []

function mountHarness(): VueWrapper {
  const wrapper = mount(Harness)
  wrappers.push(wrapper)
  return wrapper
}

async function flushThemeRestore(): Promise<void> {
  await Promise.resolve()
  await nextTick()
}

afterEach(async () => {
  wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  await flushThemeRestore()
  document.documentElement.classList.remove('dark')
})

describe('useAuthLightTheme', () => {
  it('restores an initially dark document after the auth scope unmounts', async () => {
    document.documentElement.classList.add('dark')

    const wrapper = mountHarness()
    expect(document.documentElement.classList.contains('dark')).toBe(false)

    wrapper.unmount()
    await flushThemeRestore()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('keeps the document light when one auth scope is immediately replaced by another', async () => {
    document.documentElement.classList.add('dark')

    const first = mountHarness()
    first.unmount()
    const second = mountHarness()
    await flushThemeRestore()

    expect(document.documentElement.classList.contains('dark')).toBe(false)

    second.unmount()
    await flushThemeRestore()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('restores dark mode only after the last overlapping auth scope unmounts', async () => {
    document.documentElement.classList.add('dark')

    const first = mountHarness()
    const second = mountHarness()
    expect(document.documentElement.classList.contains('dark')).toBe(false)

    first.unmount()
    await flushThemeRestore()

    expect(document.documentElement.classList.contains('dark')).toBe(false)

    second.unmount()
    await flushThemeRestore()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('leaves an initially light document light after the auth scope unmounts', async () => {
    const wrapper = mountHarness()
    expect(document.documentElement.classList.contains('dark')).toBe(false)

    wrapper.unmount()
    await flushThemeRestore()

    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })
})
