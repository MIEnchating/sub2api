<template>
  <div class="space-y-2" data-testid="quality-model-shortcuts">
    <p class="text-sm text-gray-500">{{ t('qualityOps.modelShortcuts') }}</p>
    <div class="flex flex-wrap gap-2">
      <span v-for="model in shortcuts" :key="model" class="inline-flex max-w-full items-center gap-1">
        <BaseButton type="button" class="btn-secondary btn-sm max-w-full break-all" data-testid="quality-model-shortcut"
          :disabled="disabled || selectedModels.includes(model)" :aria-pressed="selectedModels.includes(model)" @click="emit('select', model)">{{ model }}</BaseButton>
        <button v-if="!builtins.includes(model)" type="button" class="shrink-0 rounded p-1 text-gray-500 hover:text-red-600"
          data-testid="quality-remove-model-shortcut" :aria-label="t('qualityOps.removeModelShortcut', { model })" @click="removeShortcut(model)"><Icon name="x" size="xs" /></button>
      </span>
    </div>
    <details>
      <summary class="cursor-pointer text-sm text-primary-600">{{ t('qualityOps.addModelShortcuts') }}</summary>
      <div class="mt-2 space-y-2">
        <label class="block space-y-1 text-sm"><span>{{ t('qualityOps.modelShortcutsInput') }}</span>
          <FormTextarea v-model="draft" rows="2" class="text-sm" data-testid="quality-model-shortcuts-input" :placeholder="t('qualityOps.modelShortcutsPlaceholder')" />
        </label>
        <BaseButton type="button" class="btn-secondary btn-sm" data-testid="quality-save-model-shortcuts" :disabled="!draft.trim()" @click="saveShortcuts">{{ t('qualityOps.saveModelShortcuts') }}</BaseButton>
      </div>
    </details>
    <p class="text-xs text-gray-500">{{ t('qualityOps.modelShortcutsHint') }}</p>
    <p v-if="error" role="alert" class="text-sm text-red-600" data-testid="quality-model-shortcuts-error">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import BaseButton from '@/components/common/BaseButton.vue'
import FormTextarea from '@/components/common/FormTextarea.vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ selectedModels: string[]; userId?: number; disabled?: boolean }>()
const emit = defineEmits<{ select: [model: string] }>()
const { t } = useI18n()
const builtins = ['gpt-6-astra', 'gpt-6.1-sol', 'gpt-5.6-sol']
const custom = ref<string[]>([]), draft = ref(''), error = ref('')
const shortcuts = computed(() => [...builtins, ...custom.value])
const storageKey = computed(() => props.userId ? `sub2api-quality-model-shortcuts:${props.userId}` : null)
const validModel = (value: unknown): value is string => typeof value === 'string' && value.length > 0 && value.length <= 100 && !/[\s,，;；]/.test(value)

watch(storageKey, key => {
  custom.value = []; draft.value = ''; error.value = ''
  if (!key) return
  try {
    const saved: unknown = JSON.parse(localStorage.getItem(key) || '[]')
    if (Array.isArray(saved)) custom.value = [...new Set(saved.filter(validModel))].filter(model => !builtins.includes(model))
  } catch {
    error.value = t('qualityOps.modelShortcutsLoadFailed')
  }
}, { immediate: true })

function persist(models: string[]) {
  custom.value = models
  error.value = ''
  try {
    if (!storageKey.value) throw new Error('Missing user scope')
    localStorage.setItem(storageKey.value, JSON.stringify(models))
  } catch {
    error.value = t('qualityOps.modelShortcutsSaveFailed')
  }
}
function saveShortcuts() {
  const models = draft.value.split(/[\s,，;；]+/).filter(Boolean)
  if (!models.length) return
  if (!models.every(validModel)) { error.value = t('qualityOps.modelShortcutTooLong'); return }
  persist([...new Set([...custom.value, ...models])].filter(model => !builtins.includes(model)))
  draft.value = ''
}
function removeShortcut(model: string) { persist(custom.value.filter(value => value !== model)) }
</script>
