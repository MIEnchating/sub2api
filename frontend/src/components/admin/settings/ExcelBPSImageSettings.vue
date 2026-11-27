<template>
  <div class="card" data-testid="excel-bps-image-settings">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 id="settings-section-features-excel-bps-images" tabindex="-1" class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('admin.settings.features.excelBpsImages.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.settings.features.excelBpsImages.description') }}
      </p>
    </div>
    <form class="space-y-5 p-6" novalidate @submit.prevent="save">
      <div class="flex items-center justify-between gap-4">
        <div>
          <label for="excel-bps-image-enabled" class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.features.excelBpsImages.enabled') }}
          </label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.features.excelBpsImages.enabledHint') }}
          </p>
        </div>
        <Toggle id="excel-bps-image-enabled" v-model="form.excel_bps_image_relay_enabled" />
      </div>
      <div v-if="form.excel_bps_image_relay_enabled">
        <label for="excel-bps-image-mode" class="input-label">{{ t('admin.settings.features.excelBpsImages.mode') }}</label>
        <FormSelect id="excel-bps-image-mode" v-model="form.excel_bps_image_mode">
          <option value="relay">{{ t('admin.settings.features.excelBpsImages.modeRelay') }}</option>
          <option value="native">{{ t('admin.settings.features.excelBpsImages.modeNative') }}</option>
        </FormSelect>
        <p v-if="form.excel_bps_image_mode === 'native'" class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.features.excelBpsImages.nativeHint') }}
        </p>
        <div v-if="form.excel_bps_image_mode === 'relay'" class="mt-5">
          <label for="excel-bps-image-base-url" class="input-label">
            {{ t('admin.settings.features.excelBpsImages.baseUrl') }}
          </label>
          <FormInput
            id="excel-bps-image-base-url"
            v-model.trim="form.excel_bps_image_base_url"
            type="url"
            placeholder="https://your-api.example.com"
            required
          />
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.features.excelBpsImages.baseUrlHint') }}
          </p>
        </div>
        <div class="mt-5 space-y-3" data-testid="bps-image-policy">
          <label class="input-label" for="bps-image-limit-policy">{{ t('admin.settings.features.excelBpsImages.policyTitle') }}</label>
          <FormSelect id="bps-image-limit-policy" v-model="form.excel_bps_image_limit_policy">
            <option value="off">{{ t('admin.settings.features.excelBpsImages.policyOff') }}</option>
            <option value="auto_compact">{{ t('admin.settings.features.excelBpsImages.policyAuto') }}</option>
            <option value="warn">{{ t('admin.settings.features.excelBpsImages.policyWarn') }}</option>
          </FormSelect>
          <p class="text-xs text-gray-500">{{ t('admin.settings.features.excelBpsImages.policyHint') }}</p>
          <div v-if="form.excel_bps_image_limit_policy === 'warn'" class="grid gap-4 sm:grid-cols-2">
            <label class="space-y-1"><span class="input-label">{{ t('admin.settings.features.excelBpsImages.policyWarning') }}</span><FormInput id="bps-image-warning-remaining" v-model.number="form.excel_bps_image_warning_remaining" type="number" min="1" :max="excelBPSImageLimits.images" required /></label>
            <label class="space-y-1"><span class="input-label">{{ t('admin.settings.features.excelBpsImages.policyReserve') }}</span><FormInput id="bps-image-compact-reserve" v-model.number="form.excel_bps_image_compact_reserve" type="number" min="1" :max="excelBPSImageLimits.images" required /></label>
            <p class="text-xs text-gray-500 sm:col-span-2">{{ t('admin.settings.features.excelBpsImages.policyMarginsHint') }}</p>
          </div>
        </div>
        <h4 class="mt-6 input-label">{{ t('admin.settings.features.excelBpsImages.requestLimitsTitle') }}</h4>
        <div class="mt-5 grid gap-4 sm:grid-cols-4">
          <div class="space-y-1">
            <label for="excel-bps-image-body-limit" class="input-label">{{ t('admin.settings.features.excelBpsImages.bodyLimit') }}</label>
            <FormInput id="excel-bps-image-body-limit" v-model.number="form.excel_bps_image_body_limit_mib" type="number" min="1" :max="excelBPSImageLimits.bodyMiB" step="1" required />
          </div>
          <div class="space-y-1">
            <label for="excel-bps-image-budget" class="input-label">{{ t('admin.settings.features.excelBpsImages.budget') }}</label>
            <FormInput id="excel-bps-image-budget" v-model.number="form.excel_bps_image_budget_mib" type="number" :min="excelBPSImageLimits.minBudgetMiB" :max="excelBPSImageLimits.budgetMiB" step="1" required />
          </div>
          <div class="space-y-1">
            <label for="excel-bps-image-max-requests" class="input-label">{{ t('admin.settings.features.excelBpsImages.maxRequests') }}</label>
            <FormInput id="excel-bps-image-max-requests" v-model.number="form.excel_bps_image_max_requests" type="number" min="1" :max="excelBPSImageLimits.requests" step="1" required />
          </div>
          <div class="space-y-1">
            <label for="excel-bps-image-max-images" class="input-label">{{ t('admin.settings.features.excelBpsImages.maxImages') }}</label>
            <FormInput id="excel-bps-image-max-images" v-model.number="form.excel_bps_image_max_images" type="number" min="1" :max="excelBPSImageLimits.images" step="1" required />
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.features.excelBpsImages.limitRange', { max: excelBPSImageLimits.images }) }}</p>
          </div>
        </div>
        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.features.excelBpsImages.budgetHint', excelBPSImageLimits) }}
        </p>
        <p v-if="form.excel_bps_image_mode === 'native'" class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.features.excelBpsImages.nativeRetentionHint', { maxImages: form.excel_bps_image_max_images }) }}
        </p>
        <template v-if="form.excel_bps_image_mode === 'relay'">
          <h4 class="mt-6 input-label">{{ t('admin.settings.features.excelBpsImages.imageLimitsTitle') }}</h4>
          <div class="mt-3 grid gap-4 sm:grid-cols-3">
            <div class="space-y-1">
              <label for="excel-bps-image-max-image-mib" class="input-label">{{ t('admin.settings.features.excelBpsImages.maxImageMiB') }}</label>
              <FormInput id="excel-bps-image-max-image-mib" v-model.number="form.excel_bps_image_max_image_mib" type="number" min="1" :max="excelBPSImageLimits.imageMiB" step="1" required />
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.features.excelBpsImages.limitRange', { max: excelBPSImageLimits.imageMiB }) }}</p>
            </div>
            <div class="space-y-1">
              <label for="excel-bps-image-max-total-mib" class="input-label">{{ t('admin.settings.features.excelBpsImages.maxTotalMiB') }}</label>
              <FormInput id="excel-bps-image-max-total-mib" v-model.number="form.excel_bps_image_max_total_mib" type="number" min="1" :max="excelBPSImageLimits.totalMiB" step="1" required />
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.features.excelBpsImages.limitRange', { max: excelBPSImageLimits.totalMiB }) }}</p>
            </div>
            <div class="space-y-1">
              <label for="excel-bps-image-storage-mib" class="input-label">{{ t('admin.settings.features.excelBpsImages.storageMiB') }}</label>
              <FormInput id="excel-bps-image-storage-mib" v-model.number="form.excel_bps_image_storage_mib" type="number" min="1" :max="excelBPSImageLimits.storageMiB" step="1" required />
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.features.excelBpsImages.limitRange', { max: excelBPSImageLimits.storageMiB }) }}</p>
            </div>
            <div class="space-y-1">
              <label for="excel-bps-image-storage-entries" class="input-label">{{ t('admin.settings.features.excelBpsImages.storageEntries') }}</label>
              <FormInput id="excel-bps-image-storage-entries" v-model.number="form.excel_bps_image_storage_entries" type="number" min="1" :max="excelBPSImageLimits.storageEntries" step="1" required />
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.features.excelBpsImages.limitRange', { max: excelBPSImageLimits.storageEntries }) }}</p>
            </div>
            <div class="space-y-1">
              <label for="excel-bps-image-ttl-minutes" class="input-label">{{ t('admin.settings.features.excelBpsImages.ttlMinutes') }}</label>
              <FormInput id="excel-bps-image-ttl-minutes" v-model.number="form.excel_bps_image_ttl_minutes" type="number" min="1" :max="excelBPSImageLimits.ttlMinutes" step="1" required />
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.features.excelBpsImages.limitRange', { max: excelBPSImageLimits.ttlMinutes }) }}</p>
            </div>
          </div>
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.features.excelBpsImages.retentionHint') }}
          </p>
        </template>
      </div>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
      <BaseButton type="submit" class="btn-primary" :disabled="!loaded || saving">{{ t('common.save') }}</BaseButton>
    </form>
  </div>
</template>

<script setup lang="ts">
import BaseButton from '@/components/common/BaseButton.vue'
import FormInput from '@/components/common/FormInput.vue'
import FormSelect from '@/components/common/FormSelect.vue'
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { getSettings, updateSettings } from '@/api/admin/settings'
import { excelBPSImageLimits } from '@/utils/excelBPSImageLimits'
import { useAppStore } from '@/stores/app'
const { t } = useI18n()
const app = useAppStore()
const loaded = ref(false)
const saving = ref(false)
const error = ref('')
const form = reactive({
  excel_bps_image_mode: 'native' as 'relay' | 'native',
  excel_bps_image_relay_enabled: true,
  excel_bps_image_base_url: '',
  excel_bps_image_body_limit_mib: 64,
  excel_bps_image_budget_mib: 1024,
  excel_bps_image_max_requests: 128,
  excel_bps_image_max_image_mib: 20,
  excel_bps_image_max_images: 20,
  excel_bps_image_limit_policy: "off" as "off" | "auto_compact" | "warn",
  excel_bps_image_warning_remaining: 8,
  excel_bps_image_compact_reserve: 3,
  excel_bps_image_max_total_mib: 32,
  excel_bps_image_storage_mib: 1024,
  excel_bps_image_storage_entries: 512,
  excel_bps_image_ttl_minutes: 30,
})
onMounted(async () => {
  try {
    const settings = await getSettings()
    for (const key of Object.keys(form) as (keyof typeof form)[]) {
      const value = settings[key]
      if (value != null) Object.assign(form, { [key]: value })
    }
    loaded.value = true
  } catch (cause) { error.value = (cause as Error).message }
})
async function save() {
  if (!loaded.value || saving.value) return
  error.value = ''
  try {
    const imageBaseUrl = form.excel_bps_image_base_url.trim();
    if ((form.excel_bps_image_relay_enabled && form.excel_bps_image_mode === 'relay') || imageBaseUrl) {
      try {
        const parsed = new URL(imageBaseUrl);
        if (parsed.protocol !== 'https:' || !parsed.hostname || parsed.username || parsed.password ||
            (parsed.pathname !== '/' && parsed.pathname !== '') || imageBaseUrl.includes('?') || imageBaseUrl.includes('#')) {
          throw new Error('invalid image origin');
        }
        form.excel_bps_image_base_url = parsed.origin;
      } catch {
        error.value = t('admin.settings.features.excelBpsImages.invalidBaseUrl')
        return;
      }
    }
    if (
      !Number.isInteger(form.excel_bps_image_body_limit_mib) || form.excel_bps_image_body_limit_mib < 1 || form.excel_bps_image_body_limit_mib > excelBPSImageLimits.bodyMiB ||
      !Number.isInteger(form.excel_bps_image_budget_mib) || form.excel_bps_image_budget_mib < excelBPSImageLimits.minBudgetMiB || form.excel_bps_image_budget_mib > excelBPSImageLimits.budgetMiB || form.excel_bps_image_budget_mib < form.excel_bps_image_body_limit_mib * 8 ||
      !Number.isInteger(form.excel_bps_image_max_requests) || form.excel_bps_image_max_requests < 1 || form.excel_bps_image_max_requests > excelBPSImageLimits.requests
    ) {
      error.value = t('admin.settings.features.excelBpsImages.invalidCapacity', excelBPSImageLimits)
      return;
    }
    if (
      !Number.isInteger(form.excel_bps_image_max_image_mib) || form.excel_bps_image_max_image_mib < 1 || form.excel_bps_image_max_image_mib > excelBPSImageLimits.imageMiB ||
      !Number.isInteger(form.excel_bps_image_warning_remaining) || form.excel_bps_image_warning_remaining < 1 || form.excel_bps_image_warning_remaining > excelBPSImageLimits.images ||
      !Number.isInteger(form.excel_bps_image_compact_reserve) || form.excel_bps_image_compact_reserve < 1 || form.excel_bps_image_compact_reserve > excelBPSImageLimits.images ||
      (form.excel_bps_image_limit_policy === 'warn' && (!Number.isInteger(form.excel_bps_image_warning_remaining) || !Number.isInteger(form.excel_bps_image_compact_reserve) || form.excel_bps_image_compact_reserve < 1 || form.excel_bps_image_compact_reserve >= form.excel_bps_image_warning_remaining || form.excel_bps_image_warning_remaining >= form.excel_bps_image_max_images)) ||
      !Number.isInteger(form.excel_bps_image_max_images) || form.excel_bps_image_max_images < 1 || form.excel_bps_image_max_images > excelBPSImageLimits.images ||
      !Number.isInteger(form.excel_bps_image_max_total_mib) || form.excel_bps_image_max_total_mib < 1 || form.excel_bps_image_max_total_mib > excelBPSImageLimits.totalMiB ||
      !Number.isInteger(form.excel_bps_image_storage_mib) || form.excel_bps_image_storage_mib < 1 || form.excel_bps_image_storage_mib > excelBPSImageLimits.storageMiB ||
      !Number.isInteger(form.excel_bps_image_storage_entries) || form.excel_bps_image_storage_entries < 1 || form.excel_bps_image_storage_entries > excelBPSImageLimits.storageEntries ||
      !Number.isInteger(form.excel_bps_image_ttl_minutes) || form.excel_bps_image_ttl_minutes < 1 || form.excel_bps_image_ttl_minutes > excelBPSImageLimits.ttlMinutes ||
      form.excel_bps_image_max_total_mib < form.excel_bps_image_max_image_mib ||
      form.excel_bps_image_storage_mib < form.excel_bps_image_max_total_mib ||
      form.excel_bps_image_storage_entries < form.excel_bps_image_max_images
    ) {
      error.value = t('admin.settings.features.excelBpsImages.invalidLimits')
      return;
    }
    saving.value = true
    const updated = await updateSettings({ ...form })
    for (const key of Object.keys(form) as (keyof typeof form)[]) {
      const value = updated?.[key]
      if (value != null) Object.assign(form, { [key]: value })
    }
    app.showSuccess(t('common.saved'))
  } catch (cause) { error.value = (cause as Error).message }
  finally { saving.value = false }
}
</script>
