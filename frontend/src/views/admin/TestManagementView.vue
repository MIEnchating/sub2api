<template>
  <AppLayout>
    <div class="min-w-0 space-y-6">
      <header class="card p-4 sm:p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h1 class="page-title flex items-center gap-2">
            <Icon name="beaker" size="md" class="text-primary-600 dark:text-primary-400" />
            {{ t('admin.tests.title') }}
          </h1>
          <BaseButton type="button" class="btn-secondary" :disabled="loading" @click="refresh">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />{{ t('common.refresh') }}
          </BaseButton>
        </div>
        <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
          <nav class="tabs inline-flex" role="tablist" :aria-label="t('admin.tests.title')">
            <TabButton
              v-for="tab in tabs"
              :key="tab.key"
              type="button"
              role="tab"
              class="tab"
              :class="{ 'tab-active': activeTab === tab.key }"
              :data-quality-section="tab.key"
              :aria-selected="activeTab === tab.key"
              @click="navigate(tab.key)"
            >{{ t(tab.label) }}</TabButton>
          </nav>
          <IntelligenceVisibilitySwitch v-if="activeTab === 'intelligence'" ref="visibilitySwitch" />
        </div>
      </header>
      <p v-if="catalogError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ catalogError }}</p>
      <ActiveQualityPanel v-if="activeTab === 'plans' || activeTab === 'types'" ref="active" :tab="activeTab" />
      <QualityOperationsPanel v-else-if="activeTab === 'operations'" ref="operations" :visible="true" :accounts="accounts" />
      <IntelligencePolicyPanel
        v-else-if="activeTab === 'intelligence' && catalogReady"
        ref="intelligencePanel"
        :groups="groups"
        :accounts="accounts"
        :plans="plans"
        :catalog-ready="catalogReady"
        :initial-plan-id="Number(route.query.plan_id) || undefined"
        @changed="reloadCatalog"
      />
      <p v-else-if="loading" role="status" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import BaseButton from '@/components/common/BaseButton.vue'
import TabButton from '@/components/common/TabButton.vue'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ActiveQualityPanel from '@/components/tests/ActiveQualityPanel.vue'
import IntelligenceVisibilitySwitch from '@/components/tests/IntelligenceVisibilitySwitch.vue'
import IntelligencePolicyPanel from '@/components/tests/IntelligencePolicyPanel.vue'
import QualityOperationsPanel from '@/components/tests/QualityOperationsPanel.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { loadTestAccounts } from '@/utils/testAccounts'
import type { AccountListItem, AdminGroup, TestPlan } from '@/types'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const tabs = [
  { key: 'intelligence', label: 'channelMonitorV2.intelligence.monitor' },
  { key: 'plans', label: 'admin.tests.experience.scheduled' },
  { key: 'types', label: 'admin.tests.types' },
  { key: 'operations', label: 'admin.tests.operations.title' },
] as const
const activeTab = computed(() => {
  if (route.query.plan_id && route.query.tab !== 'plans') return 'intelligence'
  return tabs.find(tab => tab.key === route.query.tab)?.key ?? 'intelligence'
})
const active = ref<InstanceType<typeof ActiveQualityPanel> | null>(null)
const plans = ref<TestPlan[]>([])
const groups = ref<AdminGroup[]>([])
const accounts = ref<AccountListItem[]>([])
const catalogReady = ref(false)
const loading = ref(false)
const catalogError = ref('')
const visibilitySwitch = ref<InstanceType<typeof IntelligenceVisibilitySwitch> | null>(null)
const intelligencePanel = ref<InstanceType<typeof IntelligencePolicyPanel> | null>(null)
const operations = ref<InstanceType<typeof QualityOperationsPanel> | null>(null)
let catalogRequest = 0

async function reloadCatalog() {
  const request = ++catalogRequest
  loading.value = true
  catalogError.value = ''
  try {
    const [loadedPlans, loadedGroups, loadedAccounts] = await Promise.all([
      adminAPI.tests.listPlans(),
      adminAPI.groups.getAll(),
      loadTestAccounts(),
    ])
    if (request !== catalogRequest) return
    plans.value = loadedPlans
    groups.value = loadedGroups
    accounts.value = loadedAccounts
    catalogReady.value = true
  } catch (error) {
    if (request === catalogRequest) catalogError.value = extractApiErrorMessage(error, t('tests.loadFailed'))
  } finally {
    if (request === catalogRequest) loading.value = false
  }
}

function navigate(tab: typeof tabs[number]['key']) {
  void router.replace({ query: { tab } })
}

function refresh() {
  void reloadCatalog()
  if (activeTab.value === 'operations') void operations.value?.refresh()
  else if (activeTab.value === 'intelligence') void intelligencePanel.value?.refresh()
  else void active.value?.refresh()
}

onMounted(reloadCatalog)
</script>
