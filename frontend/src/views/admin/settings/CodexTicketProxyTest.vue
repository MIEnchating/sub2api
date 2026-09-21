<template>
  <div class="mt-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600" data-testid="codex-ticket-proxy-test">
    <div class="flex flex-wrap items-center gap-2">
      <button type="button" class="btn btn-secondary inline-flex items-center gap-2 text-sm" :disabled="!canTest || loading" data-testid="codex-ticket-test-exit" @click="testProxy">
        <Icon v-if="loading" name="refresh" size="sm" class="animate-spin" />
        {{ t(loading ? `${key}.testing` : `${key}.test`) }}
      </button>
    </div>
    <p v-if="dirty" class="mt-2 text-xs text-amber-600 dark:text-amber-400">{{ t(`${key}.saveFirst`) }}</p>
    <p v-else-if="!proxyConfigured" class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t(`${key}.noProxy`) }}</p>
    <p v-else class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t(`${key}.hint`) }}</p>
    <div v-if="result || requestError" class="mt-2 text-sm" aria-live="polite" data-testid="codex-ticket-proxy-test-result">
      <p v-if="result?.success" class="break-all text-emerald-600 dark:text-emerald-400">{{ t(`${key}.success`, { index: result.proxy_index, ip: result.exit_ip || '-', latency: result.latency_ms }) }}</p>
      <p v-else class="text-amber-600 dark:text-amber-400">{{ t(`${key}.failed`, { reason: errorLabel }) }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import type { CodexTicketProxyTestResult } from '@/api/admin/settings'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ proxyUrl: string; savedProxyUrl: string; disabled?: boolean }>()
const { t } = useI18n()
const key = 'admin.settings.gatewayForwarding.codexTicketProxyTest'
const loading = ref(false)
const result = ref<CodexTicketProxyTestResult | null>(null)
const requestError = ref('')
let revision = 0
let controller: AbortController | undefined
const knownErrors = new Set([
  'invalid_proxy_index', 'proxy_not_configured', 'settings_unavailable', 'invalid_proxy',
  'busy', 'timeout', 'canceled', 'proxy_auth', 'proxy_error', 'redirect_blocked',
  'upstream_error', 'invalid_response',
])
const dirty = computed(() => props.proxyUrl !== props.savedProxyUrl)
const proxyConfigured = computed(() => props.savedProxyUrl.trim() !== '')
const canTest = computed(() => !props.disabled && !dirty.value && proxyConfigured.value)
const errorLabel = computed(() => {
  const code = requestError.value || result.value?.error_code || ''
  return t(`${key}.errors.${knownErrors.has(code) ? code : 'unknown'}`)
})

async function testProxy() {
  if (!canTest.value || loading.value) return
  const requestRevision = ++revision
  controller = new AbortController()
  loading.value = true
  result.value = null
  requestError.value = ''
  try {
    // The server resolves the saved proxy; no URL or masked password leaves this component.
    const response = await adminAPI.settings.testCodexTicketProxy(1, controller.signal)
    if (requestRevision === revision) result.value = response
  } catch (error: unknown) {
    if (requestRevision !== revision) return
    const structured = error as { code?: unknown; error?: unknown }
    const value = typeof structured?.error === 'string' ? structured.error : structured?.code
    requestError.value = typeof value === 'string' && knownErrors.has(value) ? value : 'unknown'
  } finally {
    if (requestRevision === revision) loading.value = false
  }
}

watch(() => [props.proxyUrl, props.savedProxyUrl, props.disabled], () => {
  revision += 1
  controller?.abort()
  loading.value = false
  result.value = null
  requestError.value = ''
})

onUnmounted(() => {
  revision += 1
  controller?.abort()
})
</script>
