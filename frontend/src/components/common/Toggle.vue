<template>
  <button
    type="button"
    class="form-switch"
    :class="{ 'form-switch-compact': compact, 'bg-primary-600': modelValue }"
    role="switch"
    :aria-checked="!!modelValue"
    :disabled="disabled"
    @click="emit('update:modelValue', !modelValue)"
  />
</template>

<script setup lang="ts">
defineProps<{
  modelValue?: boolean | null
  compact?: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()
</script>
