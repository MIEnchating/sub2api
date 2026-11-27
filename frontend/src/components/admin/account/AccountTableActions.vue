<template>
  <div class="flex flex-wrap items-center gap-3">
    <slot name="before"></slot>
    <BaseButton @click="$emit('refresh')" :disabled="loading" class="btn-secondary">
      <Icon name="refresh" size="md" :class="[loading ? 'animate-spin' : '']" />
    </BaseButton>
    <slot name="after"></slot>
    <slot name="beforeCreate"></slot>
    <BaseButton @click="$emit('create')" class="btn-primary">{{ t('admin.accounts.createAccount') }}</BaseButton>
    <slot name="afterCreate"></slot>
  </div>
</template>

<script setup lang="ts">
import BaseButton from '@/components/common/BaseButton.vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

defineProps(['loading'])
defineEmits(['refresh', 'create'])

const { t } = useI18n()
</script>
