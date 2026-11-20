<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../stores/auth.js'
import { t } from '../i18n.js'

const auth = useAuth()
const route = useRoute()
const router = useRouter()
const email = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await auth.login(email.value, password.value)
    const next = typeof route.query.next === 'string' && route.query.next.startsWith('/') ? route.query.next : '/wishlist'
    router.push(next)
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="card card-narrow">
    <h1>{{ t('login.title') }}</h1>
    <form class="form" @submit.prevent="submit">
      <label>{{ t('common.email') }}
        <input v-model="email" type="email" autocomplete="email" required placeholder="ivan@mail.by" />
      </label>
      <label>{{ t('common.password') }}
        <input v-model="password" type="password" autocomplete="current-password" required />
      </label>
      <p v-if="error" class="form-error" role="alert">⚠ {{ error }}</p>
      <button class="btn btn-primary" :disabled="busy">{{ t('login.submit') }}</button>
    </form>
    <p class="links">
      <RouterLink to="/register">{{ t('login.register') }}</RouterLink>
      <RouterLink to="/forgot-password">{{ t('login.forgot') }}</RouterLink>
    </p>
  </section>
</template>
