<template>
  <div class="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg bg-primary-50 p-3 dark:bg-primary-900/20">
    <div class="flex flex-wrap items-center gap-2">
      <span v-if="allResultsSelected" class="text-sm font-medium text-primary-900 dark:text-primary-100">
        {{ t('admin.accounts.bulkActions.selectedAll', { count: selectedIds.length }) }}
      </span>
      <span v-else-if="selectedIds.length > 0" class="text-sm font-medium text-primary-900 dark:text-primary-100">
        {{ t('admin.accounts.bulkActions.selected', { count: selectedIds.length }) }}
      </span>
      <span v-else class="text-sm font-medium text-primary-900 dark:text-primary-100">
        {{ t('admin.accounts.bulkEdit.title') }}
      </span>
      <template v-if="selectedIds.length > 0">
        <button
          @click="$emit('select-page')"
          class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
        >
          {{ t('admin.accounts.bulkActions.selectCurrentPage') }}
        </button>
      </template>
      <template v-if="!allResultsSelected && totalResults > selectedIds.length">
        <span v-if="selectedIds.length > 0" class="text-gray-300 dark:text-primary-800">•</span>
        <button
          :disabled="selectingAll"
          @click="$emit('select-all-results')"
          class="text-xs font-medium text-primary-700 hover:text-primary-800 disabled:cursor-not-allowed disabled:opacity-60 dark:text-primary-300 dark:hover:text-primary-200"
        >
          {{
            selectingAll
              ? t('admin.accounts.bulkActions.selectingAll')
              : t('admin.accounts.bulkActions.selectAllResults', { count: totalResults })
          }}
        </button>
      </template>
      <template v-if="selectedIds.length > 0">
        <span class="text-gray-300 dark:text-primary-800">•</span>
        <button
          @click="$emit('clear')"
          class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
        >
          {{ t('admin.accounts.bulkActions.clear') }}
        </button>
      </template>
    </div>
    <div class="flex flex-wrap justify-end gap-2">
      <template v-if="selectedIds.length > 0">
        <BaseButton @click="$emit('delete')" class="btn-danger btn-sm">{{ t('admin.accounts.bulkActions.delete') }}</BaseButton>
        <BaseButton @click="$emit('reset-status')" class="btn-secondary btn-sm">{{ t('admin.accounts.bulkActions.resetStatus') }}</BaseButton>
        <BaseButton @click="$emit('refresh-token')" class="btn-secondary btn-sm">{{ t('admin.accounts.bulkActions.refreshToken') }}</BaseButton>
        <BaseButton
          @click="$emit('sync-upstream-models')"
          :title="t('admin.accounts.syncUpstreamModelsReplaceHint')"
          :disabled="syncingUpstreamModels || selectingAll"
          class="btn-secondary btn-sm disabled:cursor-not-allowed disabled:opacity-60"
        >
          {{ syncingUpstreamModels ? t('admin.accounts.bulkActions.syncUpstreamModelsLoading') : t('admin.accounts.bulkActions.syncUpstreamModels') }}
        </BaseButton>
        <BaseButton @click="$emit('probe-upstream-billing')" class="btn-secondary btn-sm">{{ t('admin.accounts.bulkActions.probeUpstreamBilling') }}</BaseButton>
        <BaseButton @click="$emit('toggle-schedulable', true)" class="btn-success btn-sm">{{ t('admin.accounts.bulkActions.enableScheduling') }}</BaseButton>
        <BaseButton @click="$emit('toggle-schedulable', false)" class="btn-warning btn-sm">{{ t('admin.accounts.bulkActions.disableScheduling') }}</BaseButton>
        <BaseButton @click="$emit('edit-selected')" class="btn-primary btn-sm">{{ t('admin.accounts.bulkActions.edit') }}</BaseButton>
        <BaseButton v-if="canExport2fa" @click="$emit('export-2fa')" class="btn-secondary btn-sm">{{ t('admin.accounts.bulkExport2fa.title') }}</BaseButton>
      </template>
      <BaseButton v-if="selectedIds.length === 0" @click="$emit('edit-filtered')" class="btn-primary btn-sm">
        {{ t('admin.accounts.bulkActions.editFiltered', { count: totalResults }) }}
      </BaseButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import BaseButton from '@/components/common/BaseButton.vue'
import { useI18n } from 'vue-i18n'

defineProps<{
  selectedIds: number[]
  totalResults: number
  selectingAll: boolean
  allResultsSelected: boolean
  syncingUpstreamModels?: boolean
  canExport2fa?: boolean
}>()

defineEmits([
  'delete',
  'edit-selected',
  'edit-filtered',
  'clear',
  'select-page',
  'select-all-results',
  'toggle-schedulable',
  'reset-status',
  'refresh-token',
  'sync-upstream-models',
  'probe-upstream-billing',
  'export-2fa'
])

const { t } = useI18n()
</script>
