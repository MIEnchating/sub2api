import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { h } from 'vue'
import FormSelect from '@/components/common/FormSelect.vue'

describe('FormSelect', () => {
  it('preserves typed option values and the native change event', async () => {
    const update = vi.fn()
    const change = vi.fn()
    const wrapper = mount(FormSelect, {
      props: { modelValue: false },
      attrs: { 'onUpdate:modelValue': update, onChange: change },
      slots: {
        default: () => [
          h('option', { value: false }, 'Disabled'),
          h('option', { value: true }, 'Enabled')
        ]
      }
    })

    const select = wrapper.get('select')
    expect(select.element.value).toBe('false')
    await select.setValue('true')
    expect(update).toHaveBeenCalledWith(true)
    expect(change.mock.calls[0][0].target).toBe(select.element)
    await wrapper.setProps({ modelValue: true })
    await wrapper.setProps({ modelValue: false })
    expect(select.element.value).toBe('false')
  })

  it('supports grouped numeric multiple selections and native form attributes', async () => {
    const update = vi.fn()
    const wrapper = mount(FormSelect, {
      props: { modelValue: [], modelModifiers: { number: true } },
      attrs: { 'onUpdate:modelValue': update, name: 'groups', multiple: true, required: true },
      slots: {
        default: () => h('optgroup', { label: 'Groups' }, [
          h('option', { value: '1' }, 'First'),
          h('option', { value: '2' }, 'Second')
        ])
      }
    })

    const select = wrapper.get('select')
    expect(select.element.name).toBe('groups')
    expect(select.element.checkValidity()).toBe(false)
    await select.setValue(['1', '2'])
    expect(update).toHaveBeenCalledWith([1, 2])
    expect(select.element.checkValidity()).toBe(true)
    await wrapper.setProps({ modelValue: [2] })
    expect([...select.element.selectedOptions].map(option => option.value)).toEqual(['2'])
  })
})
