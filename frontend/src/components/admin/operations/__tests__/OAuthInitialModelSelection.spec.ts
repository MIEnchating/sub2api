import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { reactive, ref } from 'vue'
import OAuthInitialModelSelection from '../OAuthInitialModelSelection.vue'

vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }) }))

enableAutoUnmount(afterEach)
beforeEach(() => {
 localStorage.clear()
 vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval', 'setTimeout', 'clearTimeout'] })
})
afterEach(() => { vi.clearAllTimers(); vi.useRealTimers(); vi.clearAllMocks() })

describe('OAuth initial model selection', () => {
 it('shows current platform candidates and prevents malformed custom names', async () => {
  const pinia = createPinia(); setActivePinia(pinia)
  const props = reactive({ modelValue: ['custom-saved'], platform: 'openai', userId: 41, mappings: [] })
  const wrapper = mount(OAuthInitialModelSelection, { props, global: { plugins: [pinia] } })
  expect(wrapper.find('[data-model="gpt-6-luna"]').exists()).toBe(true)
  expect(wrapper.find('[data-model="custom-saved"]').exists()).toBe(true)
  await wrapper.get('[data-testid="model-selection-custom"]').setValue('invalid*')
  expect(wrapper.get('[data-testid="model-selection-add"]').attributes('disabled')).toBeDefined()
  await wrapper.setProps({ platform: 'anthropic', modelValue: [] })
  expect(wrapper.find('[data-model="gpt-6-luna"]').exists()).toBe(false)
  expect(wrapper.find('[data-model="claude-sonnet-4-6"]').exists()).toBe(true)
 })
})
