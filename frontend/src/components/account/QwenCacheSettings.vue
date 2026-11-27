<template>
  <div class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-600" data-testid="qwen-cache-settings">
    <div class="flex items-center gap-2 font-medium">
      {{ t('common.qwenCache.title') }}
      <NewFeatureBadge v-if="userId != null" :notice-id="NEW_FEATURE_NOTICES.qwenCache" :user-id="userId" />
    </div>
    <label class="flex items-center gap-2">
      <FormInput type="checkbox" data-testid="qwen-session-cache" :checked="sessionCache" @change="emit('update:sessionCache', ($event.target as HTMLInputElement).checked)" />
      {{ t('common.qwenCache.enable') }}
    </label>
    <p class="input-hint">{{ t('common.qwenCache.cacheHint') }}</p>
    <label class="flex items-center gap-2">
      <FormInput type="checkbox" data-testid="qwen-strict-affinity" :checked="strictAffinity" @change="emit('update:strictAffinity', ($event.target as HTMLInputElement).checked)" />
      {{ t('common.qwenCache.strict') }}
    </label>
    <p class="input-hint">{{ t('common.qwenCache.strictHint') }}</p>
    <div class="flex flex-wrap gap-2">
      <BaseButton type="button" class="btn-secondary btn-sm" data-testid="qwen-token-plan-url" @click="emit('baseUrl', 'https://token-plan.maas.qianwenaiapi.com/compatible-mode/v1')">{{ t('common.qwenCache.tokenPlan') }}</BaseButton>
      <BaseButton type="button" class="btn-secondary btn-sm" data-testid="qwen-payg-url" @click="emit('baseUrl', 'https://maas.qianwenaiapi.com/compatible-mode/v1')">{{ t('common.qwenCache.payg') }}</BaseButton>
    </div>
    <p class="input-hint">{{ t('common.qwenCache.endpointHint') }}</p>
  </div>
</template>

<script setup lang="ts">
import BaseButton from '@/components/common/BaseButton.vue'
import FormInput from '@/components/common/FormInput.vue'
import { useI18n } from 'vue-i18n'
import NewFeatureBadge from '@/components/common/NewFeatureBadge.vue'
import { NEW_FEATURE_NOTICES } from '@/constants/newFeatureNotices'

defineProps<{ sessionCache: boolean; strictAffinity: boolean; userId?: number }>()
const emit = defineEmits<{
  'update:sessionCache': [value: boolean]
  'update:strictAffinity': [value: boolean]
  baseUrl: [value: string]
}>()
const { t } = useI18n()
</script>
