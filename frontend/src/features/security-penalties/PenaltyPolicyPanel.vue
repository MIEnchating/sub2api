<template>
  <section class="my-6 space-y-4 rounded-xl border p-5 dark:border-dark-600" data-testid="penalty-policy">
    <h2 class="text-lg font-semibold">{{ t('securityRecords.policyTitle') }}</h2>
    <p class="text-sm text-gray-500">{{ t('securityRecords.policyHint') }}</p>
    <p v-if="error" role="alert" class="text-red-600">{{ error }}</p>
    <p v-if="notice" role="status" class="text-emerald-600">{{ notice }}</p>
    <BaseButton v-if="!config" class="btn-secondary" @click="load">{{ t('common.refresh') }}</BaseButton>
    <form v-if="config" class="space-y-4" @submit.prevent="save">
      <div class="flex flex-wrap gap-5">
        <label><FormInput v-model="config.enabled" type="checkbox" /> {{ t('securityRecords.enabled') }}</label>
        <label><FormInput v-model="config.observe_only" type="checkbox" /> {{ t('securityRecords.observeOnly') }}</label>
      </div>
      <p v-if="!config.risk_control_enabled" class="text-amber-600">{{ t('securityRecords.masterOff') }}</p>
      <div class="grid gap-4 sm:grid-cols-2">
        <label>{{ t('securityRecords.policy') }}<FormSelect v-model="config.policy"><option value="session_block">{{ t('securityRecords.sessionPolicy') }}</option><option value="user_block">{{ t('securityRecords.userPolicy') }}</option><option value="session_route_then_block">{{ t('securityRecords.tieredPolicy') }}</option></FormSelect></label>
        <label>{{ t('securityRecords.threshold') }}<FormInput v-model.number="config.threshold" type="number" min="1" max="1000" required /></label>
        <label>{{ t('securityRecords.window') }}<FormInput v-model.number="config.window_hours" type="number" min="1" max="87600" required /></label>
        <label>{{ t('securityRecords.duration') }}<FormInput v-model.number="config.duration_hours" type="number" min="1" max="87600" required /></label>
        <label v-if="config.policy === 'session_route_then_block'">{{ t('securityRecords.targetGroup') }}<FormSelect v-model="config.target_group_id" :required="config.enabled"><option :value="null">—</option><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option></FormSelect></label>
      </div>
      <p v-if="config.policy === 'session_route_then_block'" class="text-sm text-amber-700">{{ t('securityRecords.routeHint') }}</p>
      <label class="block"><FormInput v-model="config.all_groups" type="checkbox" /> {{ t('securityRecords.allGroups') }}</label>
      <div v-if="!config.all_groups" class="flex flex-wrap gap-3"><label v-for="group in groups" :key="group.id"><FormInput v-model="config.group_ids" :value="group.id" type="checkbox" /> {{ group.name }}</label></div>
      <label class="block">{{ t('securityRecords.exempt') }}<FormInput v-model="exemptText" placeholder="12, 34" /></label>
      <BaseButton class="btn-primary" :disabled="saving">{{ t('common.save') }}</BaseButton>
    </form>
    <form class="flex flex-wrap items-end gap-3 border-t pt-4 dark:border-dark-600" @submit.prevent="unrestrict">
      <label>{{ t('securityRecords.releaseUser') }}<FormInput v-model.number="releaseUser" type="number" min="1" required /></label>
      <label>{{ t('securityRecords.releaseReason') }}<FormInput v-model.trim="releaseReason" required maxlength="1000" /></label>
      <BaseButton class="btn-secondary" :disabled="saving">{{ t('securityRecords.release') }}</BaseButton>
    </form>
    <SecurityRecords ref="records" admin />
  </section>
</template>
<script setup lang="ts">
import BaseButton from '@/components/common/BaseButton.vue'
import FormInput from '@/components/common/FormInput.vue'
import FormSelect from '@/components/common/FormSelect.vue'
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getConfig, release, saveConfig, type PenaltyConfig } from './api'
import { apiClient } from '@/api/client'
import SecurityRecords from './SecurityRecords.vue'
const { t } = useI18n()
const config = ref<PenaltyConfig | null>(null)
const groups = ref<Array<{ id: number; name: string }>>([])
const exemptText = ref('')
const releaseUser = ref<number | undefined>()
const releaseReason = ref('')
const records = ref<InstanceType<typeof SecurityRecords> | null>(null)
const saving = ref(false)
const error = ref('')
const notice = ref('')
async function load() {
  error.value = ''
  try {
    const [cfg, response] = await Promise.all([getConfig(), apiClient.get<Array<{ id: number; name: string }>>('/admin/groups/all')])
    config.value = cfg
    exemptText.value = cfg.exempt_user_ids.join(', ')
    groups.value = response.data
  } catch { error.value = t('securityRecords.loadFailed') }
}
async function save() {
  if (!config.value) return
  saving.value = true; error.value = ''; notice.value = ''
  try {
    const ids = exemptText.value.split(/[,，\s]+/).filter(Boolean).map(Number)
    if (ids.some(id => !Number.isSafeInteger(id) || id <= 0)) throw new Error('invalid IDs')
    config.value = await saveConfig({ ...config.value, exempt_user_ids: [...new Set(ids)] })
    notice.value = t('securityRecords.saved')
  } catch { error.value = t('securityRecords.saveFailed') }
  finally { saving.value = false }
}
async function unrestrict() {
  if (!releaseUser.value || !releaseReason.value || !window.confirm(t('securityRecords.releaseConfirm'))) return
  saving.value = true; error.value = ''; notice.value = ''
  try {
    await release(releaseUser.value, releaseReason.value)
    notice.value = t('securityRecords.released')
    releaseReason.value = ''
    await records.value?.refresh()
  } catch { error.value = t('securityRecords.saveFailed') }
  finally { saving.value = false }
}
onMounted(load)
</script>
