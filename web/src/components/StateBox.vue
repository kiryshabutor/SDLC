<script setup>
// Состояния загрузки, пустоты и ошибки (REQ-INT-003).
import { t } from '../i18n.js'

defineProps({
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  empty: { type: String, default: '' },
})
defineEmits(['retry'])
</script>

<template>
  <div v-if="loading" class="state" role="status">
    <span class="spinner" aria-hidden="true"></span>{{ t('common.loading') }}
  </div>
  <div v-else-if="error" class="state state-error" role="alert">
    <p>{{ error }}</p>
    <button type="button" class="btn" @click="$emit('retry')">{{ t('common.retry') }}</button>
  </div>
  <div v-else-if="empty" class="state state-empty">
    <p>{{ empty }}</p>
    <slot name="empty-action"></slot>
  </div>
</template>
