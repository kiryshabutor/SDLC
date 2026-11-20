<script setup>
// Центр уведомлений (REQ-FUNC-028) наполняется с F-09 (итерация 5).
import { t } from '../i18n.js'
import StateBox from '../components/StateBox.vue'
</script>

<template>
  <section>
    <h1>{{ t('notifications.title') }}</h1>
    <StateBox :empty="t('notifications.empty')" />
  </section>
</template>
