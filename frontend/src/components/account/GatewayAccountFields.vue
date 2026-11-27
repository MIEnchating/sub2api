<template>
  <fieldset class="space-y-4" data-testid="gateway-account-fields">
    <legend class="flex items-center gap-2 font-medium">
      Sub2API / New API
      <NewFeatureBadge :notice-id="NEW_FEATURE_NOTICES.crossPlatformGatewayAccounts" :user-id="authStore.user?.id" />
    </legend>
    <label class="block">
      <span class="input-label">{{ t('admin.accounts.gatewayProvider') }}</span>
      <FormSelect v-model="kind" required>
        <option value="sub2api">Sub2API</option>
        <option value="new_api">New API</option>
      </FormSelect>
    </label>
    <label class="block">
      <span class="input-label">{{ t('admin.accounts.upstream.baseUrl') }}</span>
      <FormInput v-model.trim="baseUrl" required type="url" placeholder="https://relay.example.com" />
    </label>
    <label class="block">
      <span class="input-label">API Key</span>
      <FormInput v-model.trim="apiKey" :required="!editing" type="password" autocomplete="new-password"
        :placeholder="editing ? t('admin.accounts.leaveEmptyToKeep') : ''" />
    </label>
    <label v-if="kind === 'sub2api'" class="flex items-center gap-2">
      <FormInput v-model="probe" type="checkbox" />
      {{ t('admin.accounts.upstreamBilling.autoProbe') }}
    </label>
  </fieldset>
</template>

<script setup lang="ts">
import FormInput from '@/components/common/FormInput.vue'
import FormSelect from '@/components/common/FormSelect.vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores'
import NewFeatureBadge from '@/components/common/NewFeatureBadge.vue'
import { NEW_FEATURE_NOTICES } from '@/constants/newFeatureNotices'

defineProps<{ editing?: boolean }>()
const kind = defineModel<'sub2api' | 'new_api'>('kind', { required: true })
const baseUrl = defineModel<string>('baseUrl', { required: true })
const apiKey = defineModel<string>('apiKey', { required: true })
const probe = defineModel<boolean>('probe', { required: true })
const authStore = useAuthStore()
const { t } = useI18n()
</script>
