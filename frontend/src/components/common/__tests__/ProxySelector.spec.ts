import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { Proxy } from '@/types'
import ProxySelector from '../ProxySelector.vue'

vi.mock('@/api/admin', () => ({ default: { proxies: {} } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const proxies = [
  { id: 11, name: 'Proxy A', protocol: 'http', host: 'a.example', port: 8080 },
  { id: 22, name: 'Proxy B', protocol: 'http', host: 'b.example', port: 8080 }
] as Proxy[]

function mountSelector(modelValue: number | null, availableProxies = proxies) {
  return mount(ProxySelector, { props: { modelValue, proxies: availableProxies }, global: { stubs: { Icon: true } } })
}

describe('ProxySelector', () => {
  it('keeps scalar selection and closes for single-select consumers', async () => {
    const wrapper = mountSelector(null)
    await wrapper.get('.select-trigger').trigger('click')
    await wrapper.findAll('.select-option')[1]!.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[11]])
    expect(wrapper.get('.select-trigger').classes()).not.toContain('select-trigger-open')
    wrapper.unmount()
  })

  it('replaces or clears the fixed proxy and closes after each selection', async () => {
    const wrapper = mountSelector(11)
    expect(wrapper.get('.select-value').text()).toContain('Proxy A')
    await wrapper.get('.select-trigger').trigger('click')
    await wrapper.findAll('.select-option')[2]!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([22])
    expect(wrapper.get('.select-trigger').classes()).not.toContain('select-trigger-open')
    await wrapper.setProps({ modelValue: 22 })
    await wrapper.get('.select-trigger').trigger('click')
    await wrapper.findAll('.select-option')[0]!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([null])
    expect(wrapper.get('.select-trigger').classes()).not.toContain('select-trigger-open')
    wrapper.unmount()
  })
})

describe('managed IPv6 selection', () => {
  const managed = [
    ...proxies,
    { id: 33, name: 'reserved', protocol: 'http', host: 'private', port: 8080, managed_ipv6: '2001:db8::33', managed_ipv6_state: 'reserved' },
    { id: 44, name: 'other-owner', protocol: 'http', host: 'private', port: 8080, managed_ipv6: '2001:db8::44', managed_ipv6_state: 'bound' }
  ] as Proxy[]

  it('switches between ordinary and reserved IPv6 proxies while hiding other accounts’ addresses', async () => {
    const wrapper = mountSelector(11, managed)
    await wrapper.get('.select-trigger').trigger('click')
    expect(wrapper.text()).not.toContain('other-owner')
    await wrapper.findAll('.select-option').find(option => option.text().includes('reserved'))!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([33])
    await wrapper.setProps({ modelValue: 33 })
    expect(wrapper.get('.select-value').text()).toContain('2001:db8::33')
    await wrapper.get('.select-trigger').trigger('click')
    await wrapper.findAll('.select-option').find(option => option.text().includes('Proxy A'))!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([11])
    wrapper.unmount()
  })
})
