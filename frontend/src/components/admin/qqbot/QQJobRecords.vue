<template>
  <section class="card space-y-4 p-5">
    <form class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4" @submit.prevent="search">
      <label class="text-sm">{{ t('qqbot.group') }}<FormInput v-model.trim="filters.group_id" maxlength="128" data-testid="qq-job-group" /></label>
      <QQMemberPicker v-model="filters.member_id" :group-id="filters.group_id" :app-id="appId" :disabled="loading" @select="selectMember" />
      <label class="text-sm">{{ t('qqbot.workspace.jobKind') }}<FormSelect v-model="filters.kind" data-testid="qq-job-kind"><option value="">{{ t('qqbot.workspace.all') }}</option><option v-for="kind in kinds" :key="kind" :value="kind">{{ kindLabel(kind) }}</option></FormSelect></label>
      <label class="text-sm">{{ t('common.status') }}<FormSelect v-model="filters.status"><option value="">{{ t('qqbot.workspace.all') }}</option><option v-for="status in statuses" :key="status" :value="status">{{ status }}</option></FormSelect></label>
      <label class="text-sm">{{ t('qqbot.siteQuery.kindFilter') }}<FormSelect v-model="filters.query_kind" data-testid="qq-job-query-kind"><option value="">{{ t('qqbot.workspace.all') }}</option><option v-for="kind in queryKinds" :key="kind" :value="kind">{{ t(`qqbot.siteQuery.kinds.${kind}`) }}</option></FormSelect></label>
      <label class="text-sm">{{ t('qqbot.siteQuery.statusFilter') }}<FormSelect v-model="filters.query_status" data-testid="qq-job-query-status"><option value="">{{ t('qqbot.workspace.all') }}</option><option v-for="status in queryStatuses" :key="status" :value="status">{{ t(`qqbot.siteQuery.statuses.${status}`) }}</option></FormSelect></label>
      <label class="text-sm">{{ t('qqbot.workspace.from') }}<FormInput v-model="filters.from" type="datetime-local" /></label>
      <label class="text-sm">{{ t('qqbot.workspace.to') }}<FormInput v-model="filters.to" type="datetime-local" /></label>
      <BaseButton class="btn-primary self-end" :disabled="loading">{{ t('common.search') }}</BaseButton>
    </form>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <p class="text-xs text-gray-500">{{ t('qqbot.workspace.recordsHelp') }}</p>
    <article v-for="job in data.items" :key="job.id" class="space-y-2 rounded-lg border p-4 text-sm dark:border-dark-700">
      <p class="flex flex-wrap justify-between gap-2"><strong>{{ kindLabel(job.kind || 'event') }} · {{ job.status }}</strong><time>{{ new Date(job.created_at).toLocaleString() }}</time></p>
      <p class="break-all text-xs text-gray-500">{{ job.group_id }} · {{ job.member_id }} · {{ job.id }}</p>
      <p v-if="job.trigger_reason || job.decision_reason">{{ job.trigger_reason }} → {{ job.decision_reason || job.status }}</p>
      <div v-if="job.site_query?.kind" class="space-y-1 rounded bg-gray-50 p-3 text-xs dark:bg-dark-800" data-testid="qq-job-site-query">
        <p>{{ t('qqbot.siteQuery.title') }}: {{ queryLabel('kinds', job.site_query.kind) }} · {{ queryLabel('statuses', job.site_query.status) }}</p>
        <p><span v-if="job.site_query.model">{{ job.site_query.model }} · </span>{{ t('qqbot.siteQuery.page', { page: job.site_query.page }) }} · {{ t('qqbot.siteQuery.duration', { ms: job.site_query.duration_ms }) }}</p>
        <p v-if="job.site_query.error_code">{{ t('qqbot.siteQuery.code') }}: {{ job.site_query.error_code }}</p>
        <p class="text-gray-500">{{ t('qqbot.siteQuery.help') }}</p>
      </div>
      <div v-if="job.chat_execution?.attempts" class="space-y-1 text-xs" data-testid="qq-job-chat-execution">
        <p>{{ t('qqbot.chatExecution.attempts', { attempts: job.chat_execution.attempts, retries: job.chat_execution.retries }) }}</p>
        <p v-if="job.chat_execution.last_error_code">{{ t('qqbot.chatExecution.lastError') }}: {{ job.chat_execution.last_error_code }}</p>
      </div>
      <p>{{ t(job.chat_execution?.usage_complete === false ? 'qqbot.chatExecution.knownTokens' : 'qqbot.chatExecution.tokens') }}: {{ job.input_tokens }} / {{ job.output_tokens }}</p>
      <p v-if="job.chat_execution?.usage_complete === false" class="text-xs text-amber-700 dark:text-amber-300" data-testid="qq-job-usage-incomplete">{{ t('qqbot.chatExecution.incompleteUsage') }}</p>
      <p v-if="job.kind === 'spam_report'" class="text-xs text-gray-500">{{ t('qqbot.workspace.reportReviewHelp') }}</p>
      <p v-if="job.error" class="break-all text-red-600">{{ job.error }}</p>
    </article>
    <p v-if="!loading && !data.items.length">{{ t('common.noData') }}</p>
    <div class="flex flex-wrap items-center justify-between gap-3 text-sm"><span>{{ t('qqbot.recordsPage', { total: data.total, page: data.page, pages }) }}</span><div class="flex gap-2"><BaseButton class="btn-secondary" :disabled="loading || data.page <= 1" @click="load(data.page - 1)">{{ t('qqbot.previousPage') }}</BaseButton><BaseButton class="btn-secondary" :disabled="loading || data.page >= pages" @click="load(data.page + 1)">{{ t('qqbot.nextPage') }}</BaseButton></div></div>
  </section>
</template>
<script setup lang="ts">
import BaseButton from '@/components/common/BaseButton.vue'
import FormInput from '@/components/common/FormInput.vue'
import FormSelect from '@/components/common/FormSelect.vue'
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { qqbotAPI, type QQJobPage, type QQMemberOption, type QQSiteQueryKind } from '@/api/qqbot'
import QQMemberPicker from './QQMemberPicker.vue'
const props = defineProps<{ notifications?: boolean; appId?: string }>()
const { t, te } = useI18n()
const filters = reactive({ group_id: '', member_id: '', kind: props.notifications ? 'notifications' : '', status: '', query_kind: '', query_status: '', from: '', to: '' })
watch(() => filters.group_id, () => { filters.member_id = '' }, { flush: 'sync' })
function selectMember(member: QQMemberOption) { filters.group_id = member.group_id; filters.member_id = member.member_id }
let applied = { ...filters }
function search() { applied = { ...filters }; void load(1) }
const kinds = ['chat', 'ambient_chat', 'spam_report', 'notifications', 'price_notification', 'health_notification', 'ranking_notification', 'announcement']
function kindLabel(kind: string) { const key = `qqbot.workspace.jobKinds.${kind}`; return te(key) ? t(key) : kind }
const statuses = ['queued', 'running', 'completed', 'skipped', 'failed', 'delivery_failed', 'expired', 'interrupted', 'cancelled']
const queryKinds: QQSiteQueryKind[] = ['model_price', 'channel_status', 'activity_rules', 'account_balance', 'today_usage', 'lottery_chances']
const queryStatuses = ['succeeded', 'not_found', 'unavailable', 'failed']
function queryLabel(group: 'kinds' | 'statuses', value: string) { const key = `qqbot.siteQuery.${group}.${value}`; return te(key) ? t(key) : value }
const loading = ref(false), error = ref('')
const data = ref<QQJobPage>({ items: [], total: 0, page: 1, page_size: 20 })
const pages = computed(() => Math.max(1, Math.ceil(data.value.total / data.value.page_size)))
let request = 0
async function load(page: number) {
 const seq = ++request
 error.value = ''
 if (props.appId === '') { loading.value = false; return }
 loading.value = true
 try {
  const result = await qqbotAPI.jobs({ ...applied, from: applied.from ? new Date(applied.from).toISOString() : '', to: applied.to ? new Date(applied.to).toISOString() : '', page, page_size: 20 })
  if (seq === request) data.value = result
 } catch (e) { if (seq === request) error.value = (e as Error).message } finally { if (seq === request) loading.value = false }
}
watch([() => props.appId, () => props.notifications], ([appId, notifications], [previousAppId]) => {
 if (appId !== previousAppId) {
  ++request
  Object.assign(filters, { group_id: '', member_id: '', status: '', query_kind: '', query_status: '', from: '', to: '' })
  data.value = { items: [], total: 0, page: 1, page_size: 20 }
 }
 filters.kind = notifications ? 'notifications' : ''
 search()
}, { immediate: true })
onBeforeUnmount(() => { ++request })
</script>
