import type { PublicSettings } from '@/types'

export interface NavigationPageDefinition {
  readonly path: string
  readonly labelKey: string
}

export const USER_NAVIGATION_PAGES: readonly NavigationPageDefinition[] = [
  { path: '/keys', labelKey: 'nav.apiKeys' },
  { path: '/batch-image', labelKey: 'nav.batchImage' },
  { path: '/usage', labelKey: 'nav.usage' },
  { path: '/available-channels', labelKey: 'nav.availableChannels' },
  { path: '/monitor', labelKey: 'nav.channelStatus' },
  { path: '/subscriptions', labelKey: 'nav.mySubscriptions' },
  { path: '/purchase', labelKey: 'nav.buySubscription' },
  { path: '/orders', labelKey: 'nav.myOrders' },
  { path: '/redeem', labelKey: 'nav.redeem' },
  { path: '/affiliate', labelKey: 'nav.affiliate' },
  { path: '/profile', labelKey: 'nav.profile' },
] as const

export const ADMIN_NAVIGATION_PAGES: readonly NavigationPageDefinition[] = [
  { path: '/admin/ops', labelKey: 'nav.ops' },
  { path: '/admin/users', labelKey: 'nav.users' },
  { path: '/admin/groups', labelKey: 'nav.groups' },
  { path: '/admin/channels/pricing', labelKey: 'nav.channelPricing' },
  { path: '/admin/channels/monitor', labelKey: 'nav.channelMonitor' },
  { path: '/admin/subscriptions', labelKey: 'nav.subscriptions' },
  { path: '/admin/accounts', labelKey: 'nav.accounts' },
  { path: '/admin/announcements', labelKey: 'nav.announcements' },
  { path: '/admin/proxies', labelKey: 'nav.proxies' },
  { path: '/admin/risk-control', labelKey: 'nav.contentModeration' },
  { path: '/admin/prompt-audit', labelKey: 'nav.promptAudit' },
  { path: '/admin/redeem', labelKey: 'nav.redeemCodes' },
  { path: '/admin/promo-codes', labelKey: 'nav.promoCodes' },
  { path: '/admin/affiliates/invites', labelKey: 'nav.affiliateInviteRecords' },
  { path: '/admin/affiliates/rebates', labelKey: 'nav.affiliateRebateRecords' },
  { path: '/admin/affiliates/transfers', labelKey: 'nav.affiliateTransferRecords' },
  { path: '/admin/orders/dashboard', labelKey: 'nav.paymentDashboard' },
  { path: '/admin/orders', labelKey: 'nav.orderManagement' },
  { path: '/admin/orders/plans', labelKey: 'nav.paymentPlans' },
  { path: '/admin/usage', labelKey: 'nav.usage' },
  { path: '/admin/audit-logs', labelKey: 'nav.auditLogs' },
] as const

export const ALL_CONFIGURABLE_NAVIGATION_PAGES = [
  ...USER_NAVIGATION_PAGES,
  ...ADMIN_NAVIGATION_PAGES,
] as const

const routeAliases: Readonly<Record<string, string>> = {
  '/docs/batch-image': '/batch-image',
}

const protectedNavigationPaths = new Set([
  '/dashboard',
  '/admin/dashboard',
  '/admin/settings',
])

function canonicalNavigationPath(path: string): string {
  const withoutTrailingSlash = path.length > 1 ? path.replace(/\/+$/, '') : path
  return routeAliases[withoutTrailingSlash] ?? withoutTrailingSlash
}

type NavigationSettings = Pick<
  PublicSettings,
  'navigation_item_visibility' | 'user_subscriptions_page_enabled' | 'admin_subscriptions_page_enabled'
>

export function isNavigationItemVisible(
  settings: Partial<NavigationSettings> | null | undefined,
  path: string,
): boolean {
  const canonicalPath = canonicalNavigationPath(path)
  if (protectedNavigationPaths.has(canonicalPath)) return true
  const configured = settings?.navigation_item_visibility?.[canonicalPath]
  if (typeof configured === 'boolean') return configured

  // Compatibility with public-settings payloads cached before the generic map
  // was introduced.
  if (canonicalPath === '/subscriptions') {
    return settings?.user_subscriptions_page_enabled !== false
  }
  if (canonicalPath === '/admin/subscriptions') {
    return settings?.admin_subscriptions_page_enabled !== false
  }
  return true
}

export function normalizeNavigationItemVisibility(
  input: Record<string, boolean> | null | undefined,
  legacy?: Pick<Partial<NavigationSettings>, 'user_subscriptions_page_enabled' | 'admin_subscriptions_page_enabled'>,
): Record<string, boolean> {
  const result: Record<string, boolean> = {}
  for (const [path, visible] of Object.entries(input ?? {})) {
    if (typeof visible === 'boolean') result[path] = visible
  }
  for (const page of ALL_CONFIGURABLE_NAVIGATION_PAGES) {
    if (typeof result[page.path] !== 'boolean') result[page.path] = true
  }
  if (input?.['/subscriptions'] === undefined && legacy?.user_subscriptions_page_enabled === false) {
    result['/subscriptions'] = false
  }
  if (input?.['/admin/subscriptions'] === undefined && legacy?.admin_subscriptions_page_enabled === false) {
    result['/admin/subscriptions'] = false
  }
  return result
}
