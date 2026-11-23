import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import DashboardView from '../DashboardView.vue'
import { formatDateLocalInput } from '@/utils/format'

const api = vi.hoisted(() => ({
  getDashboardStats: vi.fn(async () => ({})),
  getDashboardModels: vi.fn(async () => ({ models: [] })),
  getDashboardTrend: vi.fn(async (): Promise<{ trend: string[] }> => ({ trend: [] })),
  getByDateRange: vi.fn(async () => ({ items: [] })),
}))
vi.mock('@/api/usage', () => ({ usageAPI: api }))
vi.mock('@/api/user', () => ({ getMyPlatformQuotas: async () => ({ platform_quotas: [] }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isSimpleMode: false, refreshUser: async () => {} }) }))

describe('dashboard date filters', () => {
  it('keeps the latest chart response and refreshes recent usage independently', async () => {
    const wrapper = mount(DashboardView, { shallow: true, global: {
      stubs: { AppLayout: { template: '<slot />' } },
    } })
    await flushPromises()
    const chart = () => wrapper.getComponent({ name: 'UserDashboardCharts' })
    let finishOld!: (value: { trend: string[] }) => void
    api.getDashboardTrend.mockImplementationOnce(() => new Promise(resolve => { finishOld = resolve }))
    chart().vm.$emit('update:startDate', '2020-02-01')
    chart().vm.$emit('dateRangeChange', { preset: 'lastMonth' })
    chart().vm.$emit('dateRangeChange', { preset: 'last24Hours' })
    await flushPromises()
    expect(api.getDashboardModels).toHaveBeenLastCalledWith({ period: '24h' })
    expect(api.getDashboardTrend).toHaveBeenLastCalledWith({ period: '24h', granularity: 'day' })
    finishOld({ trend: ['obsolete'] })
    await flushPromises()
    expect(chart().props('trend')).toEqual([])
    chart().vm.$emit('refresh')
    await flushPromises()
    const end = new Date()
    const start = new Date(end)
    start.setDate(start.getDate() - 6)
    expect(api.getByDateRange).toHaveBeenLastCalledWith(formatDateLocalInput(start), formatDateLocalInput(end))
    expect(api.getDashboardModels).toHaveBeenLastCalledWith({ period: '24h' })
    wrapper.unmount()
  })
})
