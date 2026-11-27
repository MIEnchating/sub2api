import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProviderCard from '@/components/payment/ProviderCard.vue'
import type { ProviderInstance } from '@/types/payment'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string, values?: Record<string, unknown>) => `${key}${values && typeof values === 'object' ? ` ${Object.values(values).join(' ')}` : ''}` }),
}))

const provider: ProviderInstance = {
  id: 1, name: 'Example provider', provider_key: 'airwallex', config: {},
  supported_types: ['airwallex'], enabled: true, payment_mode: '',
  refund_enabled: false, allow_user_refund: false, limits: '', sort_order: 0,
  fee_rate: null, method_fee_rates: { airwallex: 0 },
}

describe('provider fee summaries and notices', () => {
  beforeEach(() => localStorage.clear())

  it('distinguishes inherited rates, explicit zero, and method overrides on cards', async () => {
    const wrapper = mount(ProviderCard, { props: { provider, enabled: true, availableTypes: [], globalFeeRate: 2.5 } })
    expect(wrapper.get('[data-testid="provider-card-fee"]').text()).toContain('feeGlobalSummary 2.5')
    expect(wrapper.text()).toContain('feeMethodOverrides 1')
    await wrapper.setProps({ provider: { ...provider, fee_rate: 0 } })
    expect(wrapper.get('[data-testid="provider-card-fee"]').text()).toContain('feeCustomSummary 0')
  })

})
