<script setup>
import { ref } from 'vue'
import { api } from '../api/client.js'
import { t } from '../i18n.js'

const email = ref('')
const done = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  try {
    done.value = (await api.passwordReset(email.value)).message
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="card card-narrow">
    <h1>{{ t('forgot.title') }}</h1>
    <p v-if="done" class="text-ok" role="status">{{ done }}</p>
    <form v-else class="form" @submit.prevent="submit">
      <label>{{ t('common.email') }}
        <input v-model="email" type="email" autocomplete="email" required placeholder="ivan@mail.by" />
      </label>
      <p v-if="error" class="form-error" role="alert">⚠ {{ error }}</p>
      <button class="btn btn-primary" :disabled="busy">{{ t('forgot.submit') }}</button>
    </form>
    <p class="links"><RouterLink to="/login">{{ t('login.title') }}</RouterLink></p>
  </section>
</template>
