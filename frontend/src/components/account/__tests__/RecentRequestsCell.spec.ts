import { nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import RecentRequestsCell from '../RecentRequestsCell.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('RecentRequestsCell', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('shows success and error markers with request details on hover', async () => {
    const wrapper = mount(RecentRequestsCell, {
      attachTo: document.body,
      props: {
        requests: [
          {
            kind: 'success',
            created_at: '2026-09-10T01:02:03Z',
            request_id: 'success-1',
            status_code: 200,
            user_id: 11,
            group_id: 22,
            duration_ms: 345
          },
          {
            kind: 'error',
            created_at: '2026-09-10T01:01:03Z',
            request_id: 'error-1',
            status_code: 429,
            user_id: 12,
            group_id: 23,
            duration_ms: 678,
            message: 'rate limited'
          }
        ]
      }
    })

    expect(wrapper.findAll('.bg-emerald-500')).toHaveLength(1)
    expect(wrapper.findAll('.bg-red-500')).toHaveLength(1)

    await wrapper.findAll('.group')[1].trigger('mouseenter')
    await nextTick()
    const visibleTooltip = Array.from(document.body.querySelectorAll('[role="tooltip"]'))
      .find(element => (element as HTMLElement).style.display !== 'none')
    expect(visibleTooltip?.textContent).toContain('429')
    expect(visibleTooltip?.textContent).toContain('rate limited')
    expect(visibleTooltip?.textContent).toContain('678 ms')

    wrapper.unmount()
  })
})
