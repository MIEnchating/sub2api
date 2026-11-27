import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import NewFeatureBadge from '@/components/common/NewFeatureBadge.vue'

it('omits the retired feature announcement', () => {
  const wrapper = mount(NewFeatureBadge, { props: { noticeId: 'oauth-429', userId: 41 } })
  expect(wrapper.find('button').exists()).toBe(false)
  expect(wrapper.text()).toBe('')
  wrapper.unmount()
})
