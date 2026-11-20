<script setup>
import { reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api/client.js'
import { useAuth } from '../stores/auth.js'
import { t } from '../i18n.js'
import FieldError from '../components/FieldError.vue'

const route = useRoute()
const auth = useAuth()
const token = typeof route.query.token === 'string' ? route.query.token : ''
const form = reactive({ password: '', password_confirm: '' })
const error = ref(null)
const done = ref(false)
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = null
  try {
    await api.passwordResetConfirm(token, form.password, form.password_confirm)
    auth.forget() // все сессии завершены на сервере (REQ-FUNC-004)
    done.value = true
  } catch (e) {
    error.value = { field: e.field === 'token' ? '' : e.field, message: e.message }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="card card-narrow">
    <h1>{{ t('reset.title') }}</h1>
    <p v-if="!token" class="form-error">{{ t('reset.no_token') }}</p>
    <template v-else-if="done">
      <p class="text-ok" role="status">{{ t('reset.done') }}</p>
      <RouterLink class="btn btn-primary" to="/login">{{ t('login.submit') }}</RouterLink>
    </template>
    <form v-else class="form" novalidate @submit.prevent="submit">
      <label>{{ t('reset.new') }}
        <input v-model="form.password" type="password" autocomplete="new-password" required />
        <small class="hint">{{ t('common.password_hint') }}</small>
        <FieldError :error="error" field="password" />
      </label>
      <label>{{ t('common.password_confirm') }}
        <input v-model="form.password_confirm" type="password" autocomplete="new-password" required />
        <FieldError :error="error" field="password_confirm" />
      </label>
      <p v-if="error && !error.field" class="form-error" role="alert">⚠ {{ error.message }}</p>
      <button class="btn btn-primary" :disabled="busy">{{ t('reset.submit') }}</button>
    </form>
    <p v-if="error && !error.field" class="links"><RouterLink to="/forgot-password">{{ t('forgot.title') }}</RouterLink></p>
  </section>
</template>
