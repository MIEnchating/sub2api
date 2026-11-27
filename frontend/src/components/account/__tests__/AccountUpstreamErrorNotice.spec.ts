import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import NewFeatureBadge from '@/components/common/NewFeatureBadge.vue'

it('omits the retired feature announcement', () => {
  const wrapper = mount(NewFeatureBadge, { props: { noticeId: 'upstream-error-rate', userId: 41, compact: true } })
  expect(wrapper.find('button').exists()).toBe(false)
  expect(wrapper.text()).toBe('')
  wrapper.unmount()
})
