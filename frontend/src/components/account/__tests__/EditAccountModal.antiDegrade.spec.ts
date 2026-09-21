import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { Account } from '@/types'

const mocks = vi.hoisted(() => ({
  update: vi.fn(),
  apply: vi.fn(),
  revert: vi.fn(),
  preview: vi.fn(),
  getById: vi.fn(),
  strategies: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: mocks.showError, showSuccess: mocks.showSuccess, showInfo: vi.fn() })
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isSimpleMode: true }) }))
vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      update: mocks.update,
      applyAntiDegrade: mocks.apply,
      revertAntiDegrade: mocks.revert,
      previewAntiDegrade: mocks.preview,
      getById: mocks.getById,
      listAntiDegradeStrategies: mocks.strategies,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false })
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({})
    },
    tlsFingerprintProfiles: { list: vi.fn().mockResolvedValue([]) }
  }
}))
vi.mock('@/api/admin/accounts', () => ({ getAntigravityDefaultModelMapping: vi.fn() }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import EditAccountModal from '../EditAccountModal.vue'

const BaseDialogStub = defineComponent({
  props: { show: Boolean },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})
const ConfirmDialogStub = defineComponent({
  props: { show: Boolean, title: String },
  emits: ['confirm', 'cancel'],
  template: `<div v-if="show" data-testid="confirmation">
    <span>{{ title }}</span>
    <button data-testid="confirm" @click="$emit('confirm')">confirm</button>
    <button data-testid="cancel" @click="$emit('cancel')">cancel</button>
  </div>`
})

function buildAccount(extra: Record<string, unknown> = {}): Account {
  return {
    id: 71,
    name: 'Existing OAuth account',
    notes: '',
    platform: 'openai',
    type: 'oauth',
    credentials: { access_token: 'test-token' },
    extra,
    proxy_id: null,
    concurrency: 3,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as Account
}

function protectedAccount(mode = 'legacy'): Account {
  return buildAccount({
    anti_degradation: true,
    anti_degrade: { enabled: true, mode, max_concurrency: 3, prev: { concurrency: 3 } },
    protection_scope: mode === 'mode1' ? 'codex_v3' : 'legacy',
    codex_fingerprint_mode: mode === 'mode1' ? 'device' : 'session',
    enable_tls_fingerprint: mode !== 'mode1',
    ...(mode !== 'mode1' ? { tls_fingerprint_builtin: 'nodejs24' } : {})
  })
}

function mountModal(account = buildAccount()) {
  return mount(EditAccountModal, {
    props: { show: true, account, proxies: [], groups: [] },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        ConfirmDialog: ConfirmDialogStub,
        Icon: true,
        ProxySelector: true,
        GroupSelector: true,
        ModelWhitelistSelector: true
      }
    }
  })
}

async function chooseFingerprint(wrapper: ReturnType<typeof mountModal>, label: string) {
  await wrapper.get('[data-testid="edit-codex-fingerprint-mode-select"] button').trigger('click')
  const option = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]'))
    .find(option => option.textContent?.trim() === `admin.accounts.openai.${label}`)
  expect(option).toBeDefined()
  option!.click()
  await flushPromises()
}

async function restoreLegacySettings(wrapper: ReturnType<typeof mountModal>) {
  await wrapper.get('[data-testid="legacy-protection-restore"]').trigger('click')
  await wrapper.get('[data-testid="confirm"]').trigger('click')
  await flushPromises()
}

enableAutoUnmount(afterEach)

describe('EditAccountModal independent settings and legacy restoration', () => {
  beforeEach(() => {
    Object.values(mocks).forEach(mock => mock.mockReset())
    mocks.update.mockResolvedValue(buildAccount())
    mocks.getById.mockResolvedValue(buildAccount())
  })

  it('shows independent fingerprint and adaptive concurrency settings without preset controls or requests', async () => {
    const wrapper = mountModal()
    await flushPromises()

    expect(wrapper.find('[data-testid="account-anti-degrade"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="anti-degrade-mode"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="legacy-account-protection"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="edit-codex-fingerprint-mode-select"] button').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="adaptive-concurrency"]').text()).toContain('adaptiveConcurrencyTitle')
    expect(mocks.strategies).not.toHaveBeenCalled()
    expect(mocks.preview).not.toHaveBeenCalled()
    expect(mocks.apply).not.toHaveBeenCalled()
  })

  it('persists an explicit off fingerprint instead of re-enabling the default', async () => {
    const wrapper = mountModal(buildAccount({ codex_fingerprint_mode: 'single_machine_multi_window' }))
    await flushPromises()
    await chooseFingerprint(wrapper, 'codexFingerprintOff')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(mocks.update.mock.calls[0][1].extra.codex_fingerprint_mode).toBe('off')
    expect(mocks.update.mock.calls[0][1].extra).not.toHaveProperty('anti_degrade')
  })

  it('saves adaptive concurrency independently and only when Update is pressed', async () => {
    const wrapper = mountModal(buildAccount({ codex_fingerprint_mode: 'device', unrelated: 'keep' }))
    await flushPromises()
    const adaptive = wrapper.get('[data-testid="adaptive-concurrency"]')
    await adaptive.get('[role="switch"]').trigger('click')
    await adaptive.get('input[type="checkbox"]').setValue(true)
    await adaptive.findAll('input[type="checkbox"]')[1].setValue(true)
    await adaptive.get('input[type="number"]').setValue(2)
    expect(mocks.update).not.toHaveBeenCalled()
    expect(mocks.apply).not.toHaveBeenCalled()

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(mocks.update.mock.calls[0][1].extra).toMatchObject({
      codex_fingerprint_mode: 'device', unrelated: 'keep',
      account_protection_policy: {
        enabled: true, adaptive_concurrency: true, adaptive_mode: 'automatic', adaptive_min_concurrency: 2
      }
    })
    expect(mocks.update.mock.calls[0][1].extra).not.toHaveProperty('anti_degrade')
  })

  it('loads and disables existing adaptive concurrency without changing a legacy preset', async () => {
    const initial = protectedAccount()
    initial.extra = { ...initial.extra, account_protection_policy: {
      enabled: true, adaptive_concurrency: true, adaptive_mode: 'automatic', adaptive_min_concurrency: 2
    } }
    const wrapper = mountModal(initial)
    await flushPromises()
    const adaptive = wrapper.get('[data-testid="adaptive-concurrency"]')
    expect(adaptive.get('[role="switch"]').attributes('aria-checked')).toBe('true')
    expect(adaptive.get<HTMLInputElement>('input[type="number"]').element.value).toBe('2')
    await adaptive.get('[role="switch"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    const extra = mocks.update.mock.calls[0][1].extra
    expect(extra).not.toHaveProperty('account_protection_policy')
    expect(extra.anti_degrade).toEqual(initial.extra?.anti_degrade)
    expect(mocks.revert).not.toHaveBeenCalled()
  })

  it('requires confirmation and waits for restoration before unlocking identity settings', async () => {
    let finishRestore!: (account: Account) => void
    mocks.revert.mockReturnValue(new Promise<Account>(resolve => { finishRestore = resolve }))
    const wrapper = mountModal(protectedAccount())
    await flushPromises()
    const fingerprint = () => wrapper.get('[data-testid="edit-codex-fingerprint-mode-select"] button')
    expect(fingerprint().attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="legacy-protection-restore"]').trigger('click')
    expect(mocks.revert).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="cancel"]').trigger('click')
    expect(mocks.revert).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="legacy-protection-restore"]').trigger('click')
    await wrapper.get('[data-testid="confirm"]').trigger('click')
    expect(mocks.revert).toHaveBeenCalledWith(71, true)
    expect(wrapper.get('[data-tour="account-form-submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(mocks.update).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="legacy-account-protection"]').exists()).toBe(true)

    finishRestore(buildAccount({ anti_degradation: false, protection_scope: 'disabled', codex_fingerprint_mode: 'off' }))
    await flushPromises()
    expect(wrapper.find('[data-testid="legacy-account-protection"]').exists()).toBe(false)
    expect(fingerprint().attributes('disabled')).toBeUndefined()
    expect(fingerprint().text()).toContain('codexFingerprintOff')
  })

  it('retains a failed restoration and reports the error', async () => {
    const initial = protectedAccount()
    mocks.revert.mockRejectedValue(new Error('restore failed'))
    mocks.getById.mockResolvedValue(initial)
    const wrapper = mountModal(initial)
    await flushPromises()
    await restoreLegacySettings(wrapper)
    expect(mocks.getById).toHaveBeenCalledWith(71)
    expect(mocks.showError).toHaveBeenCalled()
    expect(mocks.showSuccess).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="legacy-account-protection"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="legacy-protection-restore"]').attributes('disabled')).toBeUndefined()
  })

  it('keeps unrelated draft edits after restoration and saves subsequent fingerprint edits', async () => {
    const restored = buildAccount({ anti_degradation: false, protection_scope: 'disabled', codex_fingerprint_mode: 'off', unrelated: 'keep' })
    restored.concurrency = 8
    mocks.revert.mockResolvedValue(restored)
    const wrapper = mountModal(protectedAccount())
    await flushPromises()
    await wrapper.get('[data-tour="edit-account-form-name"]').setValue('Unsaved name')
    const adaptive = wrapper.get('[data-testid="adaptive-concurrency"]')
    await adaptive.get('[role="switch"]').trigger('click')
    await adaptive.get('input[type="checkbox"]').setValue(true)
    await restoreLegacySettings(wrapper)
    await wrapper.setProps({ account: restored })
    await chooseFingerprint(wrapper, 'codexFingerprintDevice')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    const payload = mocks.update.mock.calls[0][1]
    expect(payload.name).toBe('Unsaved name')
    expect(payload.concurrency).toBe(8)
    expect(payload.extra).toMatchObject({
      codex_fingerprint_mode: 'device', anti_degradation: false, unrelated: 'keep',
      account_protection_policy: { enabled: true, adaptive_concurrency: true, adaptive_mode: 'observe' }
    })
    expect(payload.extra).not.toHaveProperty('anti_degrade')
    expect(payload.extra).not.toHaveProperty('tls_fingerprint_builtin')
  })

  it('uses fresh account data after the restoration response has been echoed', async () => {
    const restored = buildAccount({ anti_degradation: false, protection_scope: 'disabled', codex_fingerprint_mode: 'off', unrelated: 'old' })
    mocks.revert.mockResolvedValue(restored)
    const wrapper = mountModal(protectedAccount())
    await flushPromises()
    await restoreLegacySettings(wrapper)
    await wrapper.setProps({ account: restored })

    const refreshed = buildAccount({ ...restored.extra, codex_fingerprint_mode: 'full', unrelated: 'fresh' })
    await wrapper.setProps({ account: refreshed })
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(mocks.update.mock.calls[0][1].extra).toMatchObject({
      codex_fingerprint_mode: 'full', unrelated: 'fresh', anti_degradation: false
    })
  })

  it.each([
    ['OpenAI API key', { type: 'apikey', credentials: { api_key: 'test-key', base_url: 'https://example.com' } }],
    ['Gemini OAuth', { platform: 'gemini' }],
    ['shadow account', { parent_account_id: 9 }],
    ['multiple proxies', { proxy_ids: [4, 5] }],
    ['random proxy', { extra: { proxy_mode: 'random' } }]
  ] as const)('does not expose presets for %s', async (_name, overrides) => {
    const wrapper = mountModal({ ...buildAccount(), ...overrides } as Account)
    await flushPromises()
    expect(wrapper.find('[data-testid="anti-degrade-mode"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="legacy-account-protection"]').exists()).toBe(false)
    expect(mocks.strategies).not.toHaveBeenCalled()
  })

  it('preserves existing generic protection on ordinary saves without locking fingerprints', async () => {
    const initial = protectedAccount('generic')
    const wrapper = mountModal(initial)
    await flushPromises()
    expect(wrapper.find('[data-testid="legacy-account-protection"]').exists()).toBe(true)
    await chooseFingerprint(wrapper, 'codexFingerprintOff')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    const extra = mocks.update.mock.calls[0][1].extra
    expect(extra.anti_degrade).toEqual(initial.extra?.anti_degrade)
    expect(extra.codex_fingerprint_mode).toBe('off')
    expect(mocks.revert).not.toHaveBeenCalled()
  })

  it('preserves active identity presets on ordinary saves', async () => {
    const initial = protectedAccount('mode1')
    const wrapper = mountModal(initial)
    await flushPromises()
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(mocks.update.mock.calls[0][1].extra).toMatchObject(initial.extra!)
    expect(mocks.revert).not.toHaveBeenCalled()
  })

  it('does not apply a completed restore response to another account', async () => {
    let finishRestore!: (account: Account) => void
    mocks.revert.mockReturnValue(new Promise<Account>(resolve => { finishRestore = resolve }))
    const wrapper = mountModal(protectedAccount())
    await flushPromises()
    await wrapper.get('[data-testid="legacy-protection-restore"]').trigger('click')
    await wrapper.get('[data-testid="confirm"]').trigger('click')
    await wrapper.setProps({ account: { ...buildAccount({ codex_fingerprint_mode: 'full' }), id: 72 } })
    finishRestore(buildAccount({ anti_degradation: false, codex_fingerprint_mode: 'off' }))
    await flushPromises()
    expect(wrapper.emitted('updated')).toBeUndefined()
    expect(wrapper.get('[data-testid="edit-codex-fingerprint-mode-select"] button').text()).toContain('codexFingerprintFull')
  })
})
