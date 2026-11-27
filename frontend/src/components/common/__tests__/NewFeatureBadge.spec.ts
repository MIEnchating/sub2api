import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import NewFeatureBadge from '../NewFeatureBadge.vue'

it.each([false, true])('renders no announcement markup (compact=%s)', (compact) => {
  const wrapper = mount(NewFeatureBadge, { props: { noticeId: 'new-id', userId: 41, compact } })
  expect(wrapper.find('button').exists()).toBe(false)
  expect(wrapper.text()).toBe('')
  wrapper.unmount()
})
