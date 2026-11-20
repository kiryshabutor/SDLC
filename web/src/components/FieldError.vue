<script setup>
// Сообщение об ошибке у поля формы (REQ-FUNC-001).
defineProps({ error: { type: Object, default: null }, field: { type: String, required: true } })
</script>

<template>
  <small v-if="error && error.field === field" class="field-error" role="alert">{{ error.message }}</small>
</template>
