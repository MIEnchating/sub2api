export const SETTINGS_TABS = ['gptDirect', 'general', 'agreement', 'features', 'security', 'users', 'gateway', 'payment', 'email', 'backup'] as const
export type SettingsTab = typeof SETTINGS_TABS[number]

interface SettingsSection {
  tab: SettingsTab
  id: string
}

// Match the headings and stable anchors in SettingsView. These are settings
// entry points, including switches for features whose business pages are hidden.
export const SETTINGS_SECTIONS: SettingsSection[] = [
  { tab: 'security', id: 'admin-api-key' },
  { tab: 'gateway', id: 'overload-cooldown' },
  { tab: 'gateway', id: 'rate-limit429cooldown' },
  { tab: 'gateway', id: 'openai-response-header-timeout' },
  { tab: 'gateway', id: 'stream-timeout' },
  { tab: 'gateway', id: 'account-performance' },
  { tab: 'gateway', id: 'rectifier' },
  { tab: 'gateway', id: 'beta-policy' },
  { tab: 'gateway', id: 'openai-fast-policy' },
  { tab: 'security', id: 'registration' },
  { tab: 'security', id: 'api-key-acl' },
  { tab: 'security', id: 'panel-rate-limit' },
  { tab: 'security', id: 'captcha' },
  { tab: 'security', id: 'linuxdo' },
  { tab: 'security', id: 'email-oauth' },
  { tab: 'security', id: 'wechat-connect' },
  { tab: 'security', id: 'dingtalk' },
  { tab: 'security', id: 'oidc' },
  { tab: 'users', id: 'defaults' },
  { tab: 'users', id: 'auth-source-defaults' },
  { tab: 'gateway', id: 'claude-code' },
  { tab: 'gateway', id: 'gateway-forwarding-codex-hardening-title' },
  { tab: 'gateway', id: 'upstream-billing-probe' },
  { tab: 'gateway', id: 'ollama-cloud-usage' },
  { tab: 'gateway', id: 'opencode-go-usage' },
  { tab: 'gateway', id: 'scheduling' },
  { tab: 'gateway', id: 'gateway-forwarding' },
  { tab: 'gateway', id: 'web-search-emulation' },
  { tab: 'gateway', id: 'usage-records' },
  { tab: 'general', id: 'site' },
  { tab: 'general', id: 'custom-menu' },
  { tab: 'agreement', id: 'login-agreement' },
  { tab: 'features', id: 'request-capture' },
  { tab: 'features', id: 'smart-ops-sidebar' },
  { tab: 'features', id: 'api-key-multi-group' },
  { tab: 'features', id: 'features-excel-bps-images' },
  { tab: 'features', id: 'features-channel-monitor' },
  { tab: 'features', id: 'features-available-channels' },
  { tab: 'features', id: 'features-pelican-showcase' },
  { tab: 'features', id: 'features-model-plaza' },
  { tab: 'features', id: 'features-site-billing-mode' },
  { tab: 'features', id: 'features-plugin-management' },
  { tab: 'features', id: 'features-support-tickets' },
  { tab: 'features', id: 'features-risk-control' },
  { tab: 'features', id: 'features-affiliate' },
  { tab: 'payment', id: 'payment' },
  { tab: 'email', id: 'smtp' },
  { tab: 'email', id: 'test-email' }
]

export function settingsLocation(tab: unknown, hash: string) {
  const section = SETTINGS_SECTIONS.find(item => hash === `#settings-section-${item.id}`)
  const validTab = SETTINGS_TABS.find(item => item === tab)
  return { tab: section?.tab ?? validTab ?? 'general', anchor: section ? `settings-section-${section.id}` : undefined, linked: Boolean(section || validTab) }
}
