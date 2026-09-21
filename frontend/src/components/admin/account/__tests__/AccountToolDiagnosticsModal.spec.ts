import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountToolDiagnosticsModal from '../AccountToolDiagnosticsModal.vue'

const listSystemLogs = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin/ops', () => ({
  opsAPI: { listSystemLogs },
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string, fallback?: string) => fallback || key }),
}))

const account = { id: 42, name: 'OpenAI primary' } as any

describe('AccountToolDiagnosticsModal', () => {
  beforeEach(() => {
    listSystemLogs.mockReset()
  })

  it('queries and displays diagnostics for the selected account', async () => {
    listSystemLogs.mockResolvedValue({
      items: [{
        id: 7,
        created_at: '2026-09-21T12:00:00Z',
        host: 'host-a',
        level: 'info',
        component: 'service.openai_tool_diagnostics',
        message: 'openai_tool_diagnostics',
        request_id: 'req-7',
        account_id: 42,
        extra: {
          diagnosis: 'gateway_dropped_tools',
          inbound_tool_names: ['shell', 'search'],
          outbound_tool_names: ['search'],
          missing_tool_names: ['shell'],
        },
      }],
    })

    const wrapper = mount(AccountToolDiagnosticsModal, {
      props: { show: true, account },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(listSystemLogs).toHaveBeenCalledWith({
      account_id: 42,
      time_range: '24h',
      q: 'openai_tool_diagnostics',
      page: 1,
      page_size: 50,
    })
    expect(wrapper.text()).toContain('gateway_dropped_tools')
    expect(wrapper.text()).toContain('shell, search')
    expect(wrapper.text()).toContain('req-7')
  })
})
