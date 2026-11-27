import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import KeysView from '@/views/user/KeysView.vue'
import UserApiKeysModal from '../UserApiKeysModal.vue'
import { NEW_FEATURE_NOTICES } from '@/constants/newFeatureNotices'
import { useAuthStore } from '@/stores/auth'
import type { AdminUser, ApiKey, User } from '@/types'

const { login, logout, currentUser, listKeys } = vi.hoisted(() => ({
  login: vi.fn(), logout: vi.fn(), currentUser: vi.fn(), listKeys: vi.fn(),
}))
vi.mock('@/api', () => ({
  authAPI: { login, logout, getCurrentUser: currentUser, getPublicSettings: vi.fn().mockResolvedValue({}) },
  isTotp2FARequired: () => false,
  passkeyAPI: {},
  keysAPI: { list: listKeys, getConcurrency: vi.fn().mockResolvedValue({ items: [] }) },
  usageAPI: { getDashboardApiKeysUsage: vi.fn().mockResolvedValue({ stats: {} }) },
  userGroupsAPI: { getAvailable: vi.fn().mockResolvedValue([]), getUserGroupRates: vi.fn().mockResolvedValue({}) },
}))
vi.mock('@/api/admin', () => ({ adminAPI: {
  users: { getUserApiKeys: listKeys }, groups: { getAll: vi.fn().mockResolvedValue([]) },
} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ isCurrentStep: () => false }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

const admin = { id: 41, role: 'admin', email: 'admin@example.test' } as User
const customer = { id: 99, email: 'customer@example.test', username: 'customer' } as AdminUser
const key = {
  id: 17, user_id: 41, name: 'Fixture key', key: 'sk-notice-fixture-only', status: 'active',
  group_id: null, group_ids: [], created_at: '2026-11-04', quota: 0, quota_used: 0,
  rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0,
} as ApiKey
const storageKey = 'sub2api:new-feature-notices:v1:41'
const noticeA = NEW_FEATURE_NOTICES.protocolFeatureSwitches
// A future announcement fixture; it does not register a production feature.
const noticeC = 'notice-regression-future-feature-c'
const badge = '[data-testid="new-feature-badge"]'

const stubs = {
  AppLayout: { template: '<div><slot /></div>' },
  TablePageLayout: { template: '<div><slot name="table" /></div>' },
  DataTable: { props: ['data'], template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>' },
  BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
  ApiKeyModelPolicyDialog: true, ConfirmDialog: true, EmptyState: true, Select: true,
  Pagination: true, Icon: true, UseKeyModal: true, BulkEditKeysModal: true,
  EndpointPopover: true, GroupBadge: true, GroupOptionItem: true,
}

async function mountEntries(pinia: Pinia, realPolicyDialog = false) {
  const entryStubs = { ...stubs, ApiKeyModelPolicyDialog: !realPolicyDialog }
  const view = mount(KeysView, { global: { plugins: [pinia], stubs: entryStubs } })
  const modal = mount(UserApiKeysModal, {
    props: { show: false, user: customer }, global: { plugins: [pinia], stubs: entryStubs },
  })
  await modal.setProps({ show: true })
  await flushPromises()
  const edit = view.findAll('button').find(button => button.text() === 'common.edit')
  expect(edit, 'the real KeysView edit action must be present').toBeDefined()
  await edit!.trigger('click')
  return { view, modal }
}



enableAutoUnmount(afterEach)
beforeEach(() => {
  localStorage.clear()
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] })
  listKeys.mockResolvedValue({ items: [key], total: 1, page: 1, page_size: 20, pages: 1 })
  login.mockResolvedValue({ access_token: 'fixture-token', token_type: 'Bearer', user: admin })
  logout.mockResolvedValue(undefined)
  currentUser.mockResolvedValue({ data: admin })
})
afterEach(() => {
  vi.clearAllTimers()
  vi.useRealTimers()
  vi.clearAllMocks()
})

describe('API key model policy announcement at both real administrator entries', () => {
  it('registers a stable unique ID separate from the old feature', () => {
    expect(NEW_FEATURE_NOTICES.apiKeyModelPolicy).toBe('api-key-model-policy-initial-announcement')
    const ids = Object.values(NEW_FEATURE_NOTICES)
    expect(new Set(ids).size).toBe(ids.length)
  })

})

describe('API key long-context announcement', () => {
  const longContextNotice = NEW_FEATURE_NOTICES.apiKeyLongContext

  it('registers a stable unique ID independent of the model-policy announcement', () => {
    expect(longContextNotice).toBe('api-key-long-context-control-initial-announcement')
    expect(longContextNotice).not.toBe(NEW_FEATURE_NOTICES.apiKeyModelPolicy)
    expect(new Set(Object.values(NEW_FEATURE_NOTICES)).size).toBe(Object.values(NEW_FEATURE_NOTICES).length)
  })

  it('does not show the new administrator notice to ordinary users', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    login.mockResolvedValueOnce({ access_token: 'fixture-token', token_type: 'Bearer', user: { ...admin, role: 'user' } })
    await useAuthStore(pinia).login({ email: admin.email, password: 'fixture-password' })
    const entries = await mountEntries(pinia, true)
    expect(entries.view.find('[data-test="edit-key-model-policy"]').exists()).toBe(false)
    for (const wrapper of Object.values(entries)) expect(wrapper.findAll(badge)).toHaveLength(0)
  })
})

describe('API key context token ceiling announcement', () => {
  const longContextNotice = NEW_FEATURE_NOTICES.apiKeyContextTokenLimit

  it('registers a stable unique ID independent of the model-policy announcement', () => {
    expect(longContextNotice).toBe('api-key-context-token-limit-initial-announcement')
    expect(longContextNotice).not.toBe(NEW_FEATURE_NOTICES.apiKeyModelPolicy)
    expect(new Set(Object.values(NEW_FEATURE_NOTICES)).size).toBe(Object.values(NEW_FEATURE_NOTICES).length)
  })

  it('does not show the new administrator notice to ordinary users', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    login.mockResolvedValueOnce({ access_token: 'fixture-token', token_type: 'Bearer', user: { ...admin, role: 'user' } })
    await useAuthStore(pinia).login({ email: admin.email, password: 'fixture-password' })
    const entries = await mountEntries(pinia, true)
    expect(entries.view.find('[data-test="edit-key-model-policy"]').exists()).toBe(false)
    for (const wrapper of Object.values(entries)) expect(wrapper.findAll(badge)).toHaveLength(0)
  })
})
