<script setup>
// Переход по ссылке из письма (REQ-FUNC-002): истёкшая или использованная
// ссылка — сообщение и предложение отправить письмо заново.
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api/client.js'
import { useAuth } from '../stores/auth.js'
import { t } from '../i18n.js'
import VerifyBanner from '../components/VerifyBanner.vue'

const route = useRoute()
const auth = useAuth()
const state = ref('progress')
const error = ref('')

onMounted(async () => {
  const token = typeof route.query.token === 'string' ? route.query.token : ''
  try {
    await api.verifyEmail(token)
    if (auth.user) auth.user.email_verified = true
    state.value = 'done'
  } catch (e) {
    error.value = e.message
    state.value = 'error'
  }
})
</script>

<template>
  <section class="card card-narrow">
    <h1>{{ t('verify.title') }}</h1>
    <p v-if="state === 'progress'" role="status">{{ t('verify.progress') }}</p>
    <template v-else-if="state === 'done'">
      <p class="text-ok" role="status">{{ t('verify.done') }}</p>
      <RouterLink class="btn btn-primary" to="/wishlist">{{ t('verify.to_wishlist') }}</RouterLink>
    </template>
    <template v-else>
      <p class="form-error" role="alert">⚠ {{ error }}</p>
      <VerifyBanner v-if="auth.loggedIn && !auth.user.email_verified" />
      <RouterLink v-else-if="!auth.loggedIn" to="/login">{{ t('login.submit') }}</RouterLink>
    </template>
  </section>
</template>
