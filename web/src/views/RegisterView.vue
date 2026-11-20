<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client.js'
import { useAuth } from '../stores/auth.js'
import { t } from '../i18n.js'
import FieldError from '../components/FieldError.vue'

const auth = useAuth()
const router = useRouter()
const regions = ref([])
const form = reactive({ email: '', password: '', password_confirm: '', region: 'BY', consent: false })
const error = ref(null)
const busy = ref(false)

onMounted(async () => {
  try {
    regions.value = (await api.regions()).regions
  } catch (e) {
    error.value = { field: '', message: e.message }
  }
})

async function submit() {
  busy.value = true
  error.value = null
  try {
    await auth.register({ ...form })
    router.push('/wishlist')
  } catch (e) {
    error.value = { field: e.field, message: e.message }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="card card-narrow">
    <h1>{{ t('register.title') }}</h1>
    <form class="form" novalidate @submit.prevent="submit">
      <label>{{ t('common.email') }}
        <input v-model="form.email" type="email" autocomplete="email" required placeholder="ivan@mail.by" />
        <FieldError :error="error" field="email" />
      </label>
      <label>{{ t('common.password') }}
        <input v-model="form.password" type="password" autocomplete="new-password" required />
        <small class="hint">{{ t('common.password_hint') }}</small>
        <FieldError :error="error" field="password" />
      </label>
      <label>{{ t('common.password_confirm') }}
        <input v-model="form.password_confirm" type="password" autocomplete="new-password" required />
        <FieldError :error="error" field="password_confirm" />
      </label>
      <label>{{ t('register.region') }}
        <select v-model="form.region">
          <option v-for="r in regions" :key="r.code" :value="r.code">{{ r.name }} ({{ r.code }})</option>
        </select>
        <FieldError :error="error" field="region" />
      </label>
      <label class="check">
        <input v-model="form.consent" type="checkbox" />
        <span>{{ t('register.consent') }} (<RouterLink to="/privacy" target="_blank">{{ t('register.consent_link') }}</RouterLink>)</span>
      </label>
      <FieldError :error="error" field="consent" />
      <p v-if="error && !error.field" class="form-error" role="alert">⚠ {{ error.message }}</p>
      <button class="btn btn-primary" :disabled="busy">{{ t('register.submit') }}</button>
      <small class="hint">{{ t('register.mail_hint') }}</small>
    </form>
    <p class="links"><RouterLink to="/login">{{ t('register.have_account') }}</RouterLink></p>
  </section>
</template>
