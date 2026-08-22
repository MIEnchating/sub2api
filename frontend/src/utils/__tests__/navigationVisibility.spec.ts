import { describe, expect, it } from 'vitest'

import {
  isNavigationItemVisible,
  normalizeNavigationItemVisibility,
} from '@/utils/navigationVisibility'

describe('navigation visibility', () => {
  it('keeps missing routes visible and honors any configured path', () => {
    const settings = {
      navigation_item_visibility: {
        '/usage': false,
        '/custom/example': false,
      },
    }

    expect(isNavigationItemVisible(settings, '/keys')).toBe(true)
    expect(isNavigationItemVisible(settings, '/usage')).toBe(false)
    expect(isNavigationItemVisible(settings, '/custom/example')).toBe(false)
  })

  it('keeps protected fallback pages visible', () => {
    const settings = {
      navigation_item_visibility: {
        '/dashboard': false,
        '/admin/dashboard': false,
        '/admin/settings': false,
      },
    }

    expect(isNavigationItemVisible(settings, '/dashboard')).toBe(true)
    expect(isNavigationItemVisible(settings, '/admin/dashboard')).toBe(true)
    expect(isNavigationItemVisible(settings, '/admin/settings')).toBe(true)
  })

  it('normalizes all current pages and preserves unknown future routes', () => {
    const normalized = normalizeNavigationItemVisibility({
      '/usage': false,
      '/future-page': false,
    })

    expect(normalized['/usage']).toBe(false)
    expect(normalized['/keys']).toBe(true)
    expect(normalized['/future-page']).toBe(false)
  })

  it('reads the old subscription switches when the generic map is absent', () => {
    expect(isNavigationItemVisible({ user_subscriptions_page_enabled: false }, '/subscriptions')).toBe(false)
    expect(isNavigationItemVisible({ admin_subscriptions_page_enabled: false }, '/admin/subscriptions')).toBe(false)
  })
})
