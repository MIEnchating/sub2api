<template>
  <div ref="containerRef" class="relative">
    <button
      type="button"
      class="select-trigger"
      :class="[isOpen && 'select-trigger-open', disabled && 'select-trigger-disabled']"
      :disabled="disabled"
      @click="toggle"
    >
      <span class="select-value truncate">{{ selectedLabel }}</span>
      <span class="select-icon">
        <Icon
          name="chevronDown"
          size="md"
          :class="['transition-transform duration-200', isOpen && 'rotate-180']"
        />
      </span>
    </button>

    <Transition name="select-dropdown">
      <div v-if="isOpen" class="select-dropdown">
        <div class="select-header">
          <div class="select-search">
            <Icon name="search" size="sm" class="text-gray-400" />
            <input
              ref="searchInputRef"
              v-model="searchQuery"
              type="text"
              class="select-search-input"
              :placeholder="t('admin.proxies.searchProxies')"
              @click.stop
            />
          </div>
          <button
            v-if="modelValue.length > 0"
            type="button"
            class="clear-btn"
            :title="t('admin.accounts.proxyPoolClear')"
            @click.stop="emit('update:modelValue', [])"
          >
            <Icon name="x" size="sm" />
          </button>
        </div>

        <div class="select-options max-h-64">
          <button
            v-for="proxy in filteredProxies"
            :key="proxy.id"
            type="button"
            class="select-option w-full text-left"
            :class="isSelected(proxy.id) && 'select-option-selected'"
            :disabled="!isSelected(proxy.id) && modelValue.length >= maxSelection"
            @click.stop="toggleProxy(proxy.id)"
          >
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="truncate font-medium">{{ proxy.name }}</span>
                <span
                  v-if="proxy.account_count !== undefined"
                  class="inline-flex shrink-0 rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-600 dark:bg-dark-600 dark:text-gray-400"
                >
                  {{ proxy.account_count }}
                </span>
              </div>
              <div class="truncate text-xs text-gray-500 dark:text-gray-400">
                {{ proxy.protocol }}://{{ proxy.host }}:{{ proxy.port }}
              </div>
            </div>
            <Icon
              v-if="isSelected(proxy.id)"
              name="check"
              size="sm"
              class="shrink-0 text-primary-500"
            />
          </button>

          <div v-if="filteredProxies.length === 0" class="select-empty">
            {{ searchQuery ? t('common.noOptionsFound') : t('admin.accounts.proxyPoolNoOptions') }}
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { Proxy } from '@/types'

const props = withDefaults(
  defineProps<{
    modelValue: number[]
    proxies: Proxy[]
    primaryProxyId: number | null
    disabled?: boolean
    maxSelection?: number
  }>(),
  { disabled: false, maxSelection: 20 }
)

const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

const { t } = useI18n()
const containerRef = ref<HTMLElement | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)
const searchQuery = ref('')
const isOpen = ref(false)

const availableProxies = computed(() =>
  props.proxies.filter((proxy) => proxy.id !== props.primaryProxyId)
)

const filteredProxies = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return availableProxies.value
  return availableProxies.value.filter(
    (proxy) => proxy.name.toLowerCase().includes(query) || proxy.host.toLowerCase().includes(query)
  )
})

const selectedLabel = computed(() => {
  if (props.modelValue.length === 0) return t('admin.accounts.proxyPoolEmpty')
  return t('admin.accounts.proxyPoolSelected', { count: props.modelValue.length })
})

const isSelected = (id: number) => props.modelValue.includes(id)

const toggle = () => {
  if (props.disabled) return
  isOpen.value = !isOpen.value
  if (isOpen.value) void nextTick(() => searchInputRef.value?.focus())
}

const toggleProxy = (id: number) => {
  if (isSelected(id)) {
    emit('update:modelValue', props.modelValue.filter((value) => value !== id))
    return
  }
  if (props.modelValue.length >= props.maxSelection) return
  emit('update:modelValue', [...props.modelValue, id])
}

const handleClickOutside = (event: MouseEvent) => {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    isOpen.value = false
    searchQuery.value = ''
  }
}

watch(
  () => props.disabled,
  (disabled) => {
    if (disabled) {
      isOpen.value = false
      searchQuery.value = ''
    }
  }
)

onMounted(() => document.addEventListener('mousedown', handleClickOutside))
onUnmounted(() => document.removeEventListener('mousedown', handleClickOutside))
</script>

<style scoped>
.select-trigger {
  @apply flex w-full cursor-pointer items-center justify-between gap-2 rounded-lg border border-gray-200 bg-white px-4 py-2.5 text-sm text-gray-900 transition-all duration-200 hover:border-gray-300 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100 dark:hover:border-dark-500;
}

.select-trigger-open {
  @apply border-primary-500 ring-2 ring-primary-500/30;
}

.select-trigger-disabled {
  @apply cursor-not-allowed bg-gray-100 opacity-60 dark:bg-dark-900;
}

.select-value {
  @apply flex-1 text-left;
}

.select-icon {
  @apply shrink-0 text-gray-400 dark:text-dark-400;
}

.select-dropdown {
  @apply absolute z-[100] mt-2 w-full overflow-hidden rounded-lg border border-gray-200 bg-white shadow-lg shadow-black/10 dark:border-dark-700 dark:bg-dark-800 dark:shadow-black/30;
}

.select-header {
  @apply flex items-center gap-2 border-b border-gray-100 px-3 py-2 dark:border-dark-700;
}

.select-search {
  @apply flex flex-1 items-center gap-2;
}

.select-search-input {
  @apply flex-1 bg-transparent text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none dark:text-gray-100 dark:placeholder:text-dark-400;
}

.clear-btn {
  @apply shrink-0 rounded p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200;
}

.select-options {
  @apply overflow-y-auto py-1;
}

.select-option {
  @apply flex cursor-pointer items-center justify-between gap-2 px-4 py-2.5 text-sm text-gray-700 transition-colors duration-150 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-300 dark:hover:bg-dark-700;
}

.select-option-selected {
  @apply bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300;
}

.select-empty {
  @apply px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400;
}

.select-dropdown-enter-active,
.select-dropdown-leave-active {
  transition: all 0.2s ease;
}

.select-dropdown-enter-from,
.select-dropdown-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
