import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it } from 'vitest'
import SmartOpsNav from '../SmartOpsNav.vue'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useAuthStore } from '@/stores/auth'
import type { User } from '@/types'

const smartOpsPaths = ['/admin/lottery', '/admin/auto-config', '/admin/priority-scheduling',
  '/admin/qqbot/overview', '/admin/channel-config', '/admin/account-quality', '/admin/controlled-experiments', '/admin/token-guard-v2', '/admin/pelican-tests', '/admin/group-cache-tests']

async function mountAt(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [...smartOpsPaths, '/admin/request-captures', '/admin/ai-ops'].map(route => ({ path: route, component: { template: '<div />' } }))
  })
  await router.push(path)
  await router.isReady()
  const i18n = createI18n({ legacy: false, locale: 'zh', missingWarn: false, fallbackWarn: false, messages: { zh: {} } })
  return mount(SmartOpsNav, { global: { plugins: [router, i18n] } })
}

const hrefs = (wrapper: Awaited<ReturnType<typeof mountAt>>) => wrapper.findAll('a').map(link => link.attributes('href'))

describe('SmartOpsNav', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
    useAuthStore().user = { id: 41, role: 'admin' } as User
  })

  it('keeps lottery administration reachable and marks the selected pool management page', async () => {
    const wrapper = await mountAt('/admin/lottery')
    expect(hrefs(wrapper)).toContain('/admin/lottery')
    expect(wrapper.find('[aria-current="page"]').text()).toBe('lottery.adminTitle')
  })

  it('keeps credential operations and hides request capture while the feature is off', async () => {
    const wrapper = await mountAt('/admin/token-guard-v2')
    expect(hrefs(wrapper)).toEqual([...smartOpsPaths, '/admin/ai-ops'])
    expect(wrapper.find('[aria-current="page"]').text()).toBe('tokenGuardV2.title')
  })

  it('lists request capture before the ticket harvest flow when the feature is on', async () => {
    useAdminSettingsStore().setRequestCaptureEnabledLocal(true)
    const wrapper = await mountAt('/admin/request-captures')
    expect(hrefs(wrapper)).toEqual([...smartOpsPaths, '/admin/request-captures', '/admin/ai-ops'])
    expect(wrapper.find('[aria-current="page"]').text()).toBe('admin.requestCapture.title')
  })

  it('follows the feature switch without remounting', async () => {
    const wrapper = await mountAt('/admin/pelican-tests')
    expect(hrefs(wrapper)).not.toContain('/admin/request-captures')
    useAdminSettingsStore().setRequestCaptureEnabledLocal(true)
    await nextTick()
    expect(hrefs(wrapper)).toContain('/admin/request-captures')
  })

  it('hides the AI operations entry from observers', async () => {
    useAuthStore().user = { id: 42, role: 'observer' } as User
    const wrapper = await mountAt('/admin/pelican-tests')
    expect(hrefs(wrapper)).not.toContain('/admin/ai-ops')
    expect(hrefs(wrapper)).not.toContain('/admin/qqbot/overview')
  })

  it("renders the AI operations link without a feature badge or extra padding", async () => {
    const wrapper = await mountAt('/admin/ai-ops')
    const entry = wrapper.get('a[href="/admin/ai-ops"]')
    expect(entry.classes()).not.toContain('pr-16')
    expect(wrapper.find('[data-testid="new-feature-badge"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it("keeps channel configuration reachable without an announcement", async () => {
    const wrapper = await mountAt('/admin/channel-config')
    expect(wrapper.find('a[href="/admin/channel-config"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="new-feature-badge"]').exists()).toBe(false)
    wrapper.unmount()
  })

})
