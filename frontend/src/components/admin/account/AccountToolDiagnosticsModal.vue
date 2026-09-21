<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.toolDiagnostics.title')"
    width="extra-wide"
    @close="emit('close')"
  >
    <div v-if="account" class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 pb-3 dark:border-dark-700">
        <div>
          <div class="font-semibold text-gray-900 dark:text-white">{{ account.name }}</div>
          <div class="text-xs text-gray-500 dark:text-gray-400">#{{ account.id }} · {{ t('admin.accounts.toolDiagnostics.window') }}</div>
        </div>
        <button type="button" class="btn btn-secondary px-2 py-1 text-xs" :disabled="loading" @click="load">
          <Icon name="refresh" size="xs" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <div v-if="loading" class="py-10 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="error" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800/50 dark:bg-red-900/20 dark:text-red-300">{{ error }}</div>
      <div v-else-if="logs.length === 0" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accounts.toolDiagnostics.empty') }}</div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full text-left text-sm">
          <thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
            <tr>
              <th class="px-2 py-2 font-medium">{{ t('admin.accounts.toolDiagnostics.time') }}</th>
              <th class="px-2 py-2 font-medium">{{ t('admin.accounts.toolDiagnostics.status') }}</th>
              <th class="px-2 py-2 font-medium">{{ t('admin.accounts.toolDiagnostics.inbound') }}</th>
              <th class="px-2 py-2 font-medium">{{ t('admin.accounts.toolDiagnostics.outbound') }}</th>
              <th class="px-2 py-2 font-medium">{{ t('admin.accounts.toolDiagnostics.missing') }}</th>
              <th class="px-2 py-2 font-medium">{{ t('admin.accounts.toolDiagnostics.requestId') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="log in logs" :key="log.id" class="align-top">
              <td class="whitespace-nowrap px-2 py-2 text-xs text-gray-500 dark:text-gray-400">{{ formatTime(log.created_at) }}</td>
              <td class="px-2 py-2"><span :class="['inline-flex rounded px-2 py-0.5 text-xs font-medium', diagnosisClass(diagnosis(log))]">{{ diagnosisLabel(diagnosis(log)) }}</span></td>
              <td class="max-w-[15rem] px-2 py-2 text-xs text-gray-700 dark:text-gray-300">{{ names(log, 'inbound_tool_names') }}</td>
              <td class="max-w-[15rem] px-2 py-2 text-xs text-gray-700 dark:text-gray-300">{{ names(log, 'outbound_tool_names', 'outbound_discovered_tool_names') }}</td>
              <td class="max-w-[15rem] px-2 py-2 text-xs text-red-600 dark:text-red-400">{{ names(log, 'missing_tool_names') }}</td>
              <td class="px-2 py-2 font-mono text-[11px] text-gray-500 dark:text-gray-400">{{ log.request_id || log.client_request_id || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { opsAPI, type OpsSystemLog } from '@/api/admin/ops'
import type { Account } from '@/types'

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const logs = ref<OpsSystemLog[]>([])
const loading = ref(false)
const error = ref('')

const extra = (log: OpsSystemLog) => log.extra || {}
const values = (log: OpsSystemLog, key: string): string[] => {
  const value = extra(log)[key]
  return Array.isArray(value) ? value.map(item => String(item)).filter(Boolean) : []
}
const names = (log: OpsSystemLog, ...keys: string[]) => {
  const result = keys.flatMap(key => values(log, key))
  return result.length ? result.join(', ') : '-'
}
const diagnosis = (log: OpsSystemLog) => String(extra(log).diagnosis || 'unknown')
const diagnosisLabel = (value: string) => t(`admin.accounts.toolDiagnostics.diagnoses.${value}`, value)
const diagnosisClass = (value: string) => value === 'tools_present' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : value === 'no_tools_declared' ? 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300' : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
const formatTime = (value: string) => value ? new Date(value).toLocaleString() : '-'

const load = async () => {
  if (!props.account) return
  loading.value = true
  error.value = ''
  try {
    const result = await opsAPI.listSystemLogs({ account_id: props.account.id, time_range: '24h', q: 'openai_tool_diagnostics', page: 1, page_size: 50 })
    logs.value = result.items || []
  } catch (cause) {
    logs.value = []
    error.value = cause instanceof Error ? cause.message : t('admin.accounts.toolDiagnostics.loadFailed')
  } finally {
    loading.value = false
  }
}

watch(() => [props.show, props.account?.id], ([show]) => { if (show) void load() }, { immediate: true })
</script>
