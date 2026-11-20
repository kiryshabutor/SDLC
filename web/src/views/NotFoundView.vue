<script setup>
import { t } from '../i18n.js'
</script>

<template>
  <section class="card card-narrow">
    <h1>{{ t('notfound.title') }}</h1>
    <RouterLink class="btn" to="/">{{ t('notfound.home') }}</RouterLink>
  </section>
</template>
