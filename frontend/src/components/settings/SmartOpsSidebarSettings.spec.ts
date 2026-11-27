import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { defineComponent, ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Panel from './SmartOpsSidebarSettings.vue'
import { DEFAULT_SMART_OPS_HIDDEN_MENU_ITEMS, SMART_OPS_MENU_ITEMS } from '@/constants/smartOpsMenu'
import zh from '@/i18n/locales/zh'
import en from '@/i18n/locales/en'

vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 41 } }) }))
function setup(locale = 'zh') {
  const pinia = createPinia()
  return { pinia, global: { plugins: [pinia, createI18n({ legacy: false, locale, messages: { zh, en } })] } }
}
beforeEach(() => localStorage.clear())
describe('smart operations sidebar settings', () => {
  it('edits the six-entry default draft and restores entries with bulk actions', async () => {
    const Harness = defineComponent({
      components: { Panel },
      setup: () => ({ hidden: ref<string[]>([...DEFAULT_SMART_OPS_HIDDEN_MENU_ITEMS, '/admin/future-entry']) }),
      template: '<form><Panel v-model="hidden"/></form>',
    })
    const wrapper = mount(Harness, { global: setup().global })
    expect(wrapper.findAll('[role="switch"]')).toHaveLength(11)
    expect(wrapper.findAll('[role="switch"][aria-checked="true"]')).toHaveLength(6)
    expect(wrapper.text()).toContain('已选择 6 / 11 个入口')
    expect(wrapper.findAll('button').every(button => button.attributes('type') === 'button')).toBe(true)
    expect(wrapper.get('#smart-ops-sidebar-overview').attributes('aria-checked')).toBe('false')
    await wrapper.get('#smart-ops-sidebar-overview').trigger('click')
    expect(wrapper.vm.hidden).not.toContain('/admin/qqbot/overview')
    await wrapper.get('#smart-ops-sidebar-overview').trigger('click')
    expect(wrapper.vm.hidden).toContain('/admin/qqbot/overview')
    for (const former of ['channel-operations', 'profit-operations', 'account-ops']) {
      expect(wrapper.find(`#smart-ops-sidebar-${former}`).exists()).toBe(false)
    }
    const channelConfig = wrapper.get('#smart-ops-sidebar-channel-config')
    expect(channelConfig.attributes('aria-label')).toBe('渠道配置')
    expect(channelConfig.attributes('aria-checked')).toBe('true')
    await channelConfig.trigger('click')
    expect(wrapper.vm.hidden).toContain('/admin/channel-config')
    expect(channelConfig.attributes('aria-checked')).toBe('false')
    expect(wrapper.get('#smart-ops-sidebar-token-guard-v2').attributes('aria-checked')).toBe('true')
    await wrapper.findAll('button').find(button => button.text() === '全部隐藏')!.trigger('click')
    expect(SMART_OPS_MENU_ITEMS.every(item => wrapper.vm.hidden.includes(item.path))).toBe(true)
    await wrapper.findAll('button').find(button => button.text() === '全部显示')!.trigger('click')
    expect(wrapper.vm.hidden).toEqual(['/admin/future-entry'])
    expect(wrapper.findAll('[role="switch"]').every(toggle => toggle.attributes('aria-checked') === 'true')).toBe(true)
  })
  it.each(['zh', 'en'])('renders translated labels and scope explanation in %s', locale => {
    const wrapper = mount(Panel, { props: { modelValue: [] }, global: setup(locale).global })
    expect(wrapper.text()).not.toMatch(/admin\.settings\.|\.title\b/)
    expect(wrapper.text()).toContain('11')
  })
})
