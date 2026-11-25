import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MonitorSettingsPanel from '../MonitorSettingsPanel.vue'
import { GROUP_PLATFORM_OPTIONS } from '@/constants/platforms'
import type { MonitorConfig } from '@/api/channelMonitorV2'

const mocks = vi.hoisted(() => ({
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  groups: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))
vi.mock('@/api/channelMonitorV2', () => ({
  getConfig: mocks.getConfig,
  updateConfig: mocks.updateConfig,
  MONITOR_ERROR_CATEGORIES: ['authentication', 'network'],
}))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { getAllIncludingInactive: mocks.groups } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({
  cachedPublicSettings: { channel_monitor_enabled: true },
  showError: mocks.showError,
  showSuccess: mocks.showSuccess,
}) }))
vi.mock('@/utils/featureFlags', () => ({
  getChannelMonitorMode: () => 'v2',
  isChannelMonitorV2Mode: () => true,
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, te: () => false }),
}))

function config(): MonitorConfig {
  return {
    version: 2,
    enabled: true,
    refresh_interval_seconds: 60,
    platforms: [
      { platform: 'openai', enabled: false, models: ['gpt-4'] },
      { platform: 'custom-provider', enabled: true, models: ['custom-model'] },
    ],
    group_ids: [42],
    ignored_error_categories: [],
    health_thresholds: {
      minimum_sample: 70, warning_error_rate: 0.1, critical_error_rate: 0.3,
      target_ttft_ms: 1500, warning_ttft_ms: 2000, critical_ttft_ms: 5000,
      warning_cache_rate: 0.7, critical_cache_rate: 0.4,
      error_weight: 0.5, ttft_weight: 0.3, cache_weight: 0.2,
    },
  }
}

async function panel(value = config()) {
  mocks.getConfig.mockResolvedValue(value)
  const wrapper = mount(MonitorSettingsPanel, { global: { stubs: { RouterLink: true, teleport: true } } })
  await flushPromises()
  return wrapper
}

function button(wrapper: Awaited<ReturnType<typeof panel>>, key: string) {
  return wrapper.findAll('button').find((item) => item.text() === 'channelMonitorV2.settings.' + key)!
}

async function add(wrapper: Awaited<ReturnType<typeof panel>>, platform: string) {
  await wrapper.get('[data-testid="monitor-v2-platform-select"]').setValue(platform)
  await button(wrapper, 'addPlatform').trigger('click')
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.groups.mockResolvedValue([])
  mocks.updateConfig.mockImplementation(async (value) => JSON.parse(JSON.stringify(value)))
})

describe('MonitorSettingsPanel platform configuration', () => {
  it('adds a missing provider for automatic called-model discovery without changing existing settings', async () => {
    const original = config()
    const wrapper = await panel(original)
    expect(button(wrapper, 'save').attributes('disabled')).toBeDefined()
    expect(wrapper.find('option[value="openai"]').exists()).toBe(false)

    await add(wrapper, 'deepseek')
    expect(button(wrapper, 'addPlatform').attributes('disabled')).toBeDefined()
    expect(wrapper.find('option[value="deepseek"]').exists()).toBe(false)
    await button(wrapper, 'save').trigger('click')
    await flushPromises()

    expect(mocks.updateConfig).toHaveBeenCalledWith({
      ...original,
      candy_probes: [],
      candy_history_limit: 100,
      card_timeline_mode: 'windows',
      group_order: [],
      platforms: [...original.platforms, {
        platform: 'deepseek', enabled: true, models: [],
      }],
    })
    expect(button(wrapper, 'save').attributes('disabled')).toBeDefined()
    expect(original.platforms).toHaveLength(2)

    const persisted = mocks.updateConfig.mock.results[0].value
    wrapper.unmount()
    const reloaded = await panel(await persisted)
    expect(reloaded.findAll('input[type="text"]').map(input => input.attributes('data-testid')))
      .toEqual(['monitor-v2-platform-custom'])
    expect(reloaded.find('option[value="deepseek"]').exists()).toBe(false)
    reloaded.unmount()
  })

  it('can start with no configured platforms and rejects duplicate additions', async () => {
    const wrapper = await panel({ ...config(), platforms: [] })
    expect(button(wrapper, 'addPlatform').attributes('disabled')).toBeDefined()
    await add(wrapper, 'kimi')
    await wrapper.get('[data-testid="monitor-v2-platform-custom"]').setValue('KIMI')
    await button(wrapper, 'addPlatform').trigger('click')
    expect(mocks.showError).toHaveBeenCalledWith('channelMonitorV2.settings.platformDuplicate')
    await button(wrapper, 'save').trigger('click')
    await flushPromises()
    expect(mocks.updateConfig.mock.calls[0][0].platforms).toEqual([
      { platform: 'kimi', enabled: true, models: [] },
    ])
    wrapper.unmount()
  })

  it('restores the server config and available options when saving fails', async () => {
    const wrapper = await panel()
    mocks.updateConfig.mockRejectedValueOnce(new Error('save failed'))
    await add(wrapper, 'minimax')
    await button(wrapper, 'save').trigger('click')
    await flushPromises()
    expect(mocks.showError).toHaveBeenCalledOnce()
    expect(wrapper.find('option[value="minimax"]').exists()).toBe(true)
    expect(wrapper.get<HTMLInputElement>('[data-testid="monitor-v2-platform-custom"]').element.value).toBe('')
    expect(button(wrapper, 'save').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('keeps custom platform entry available after all known platforms are configured', async () => {
    const platforms = new Set([
      'anthropic', 'openai', 'grok', 'kiro', 'gemini', 'antigravity',
      ...GROUP_PLATFORM_OPTIONS.map(({ value }) => value),
    ])
    const wrapper = await panel({
      ...config(),
      platforms: [...platforms].map(platform => ({ platform, enabled: true, models: [] })),
    })
    expect(wrapper.find('[data-testid="monitor-v2-platform-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="monitor-v2-platform-custom"]').exists()).toBe(true)
    expect(button(wrapper, 'addPlatform').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
})
