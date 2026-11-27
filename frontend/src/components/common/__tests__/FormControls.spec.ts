import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { h, ref } from 'vue'
import BaseButton from '@/components/common/BaseButton.vue'
import FormInput from '@/components/common/FormInput.vue'
import FormTextarea from '@/components/common/FormTextarea.vue'
import Toggle from '@/components/common/Toggle.vue'

describe('shared form controls', () => {
  it('keeps number coercion, form constraints, and submit behavior on native elements', async () => {
    const count = ref<number | string>('')
    const submit = vi.fn((event: SubmitEvent) => event.preventDefault())
    const wrapper = mount({
      setup: () => () => h('form', { onSubmit: submit }, [
        h(FormInput, {
          type: 'number', name: 'count', min: 2, required: true,
          modelValue: count.value,
          'onUpdate:modelValue': (value: number | string) => { count.value = value }
        }),
        h(BaseButton, { type: 'submit', class: 'btn-primary' }, () => 'Save')
      ])
    }, { attachTo: document.body })
    try {
      const input = wrapper.get('input')
      expect(input.element.checkValidity()).toBe(false)
      await input.setValue('3')
      expect(count.value).toBe(3)
      expect(input.element.checkValidity()).toBe(true)
      wrapper.get('button').element.click()
      expect(submit).toHaveBeenCalledOnce()
    } finally {
      wrapper.unmount()
    }
  })

  it('retains checkbox array values and lazy trimmed textarea updates', async () => {
    const selected = ref<number[]>([])
    const note = ref('')
    const wrapper = mount({
      setup: () => () => h('div', [
        h(FormInput, {
          type: 'checkbox', value: 7, modelValue: selected.value,
          'onUpdate:modelValue': (value: number[]) => { selected.value = value }
        }),
        h(FormTextarea, {
          modelValue: note.value, modelModifiers: { lazy: true, trim: true },
          'onUpdate:modelValue': (value: string) => { note.value = value }
        }),
        h(Toggle, { modelValue: true, disabled: true })
      ])
    })
    await wrapper.get('input').setValue(true)
    expect(selected.value).toEqual([7])
    const textarea = wrapper.get('textarea')
    textarea.element.value = '  note  '
    await textarea.trigger('input')
    expect(note.value).toBe('')
    await textarea.trigger('change')
    expect(note.value).toBe('note')
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.get('button').element.disabled).toBe(true)
  })
})
