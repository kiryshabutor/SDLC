<script setup>
// Профиль и настройки (UC-11, UC-15): регион с подтверждением сброса
// условий (REQ-FUNC-005), каналы уведомлений (REQ-FUNC-006-2), Telegram
// (REQ-FUNC-031), выход, удаление учётной записи (REQ-FUNC-007).
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client.js'
import { useAuth } from '../stores/auth.js'
import { t } from '../i18n.js'
import VerifyBanner from '../components/VerifyBanner.vue'
import FieldError from '../components/FieldError.vue'

const auth = useAuth()
const router = useRouter()
const regions = ref([])
const form = reactive({ region: '', email_enabled: true, telegram_enabled: true })
const saving = ref(false)
const saveMsg = ref('')
const saveErr = ref('')

const tg = ref(null)
const tgBusy = ref(false)
const tgErr = ref('')

const delPassword = ref('')
const delErr = ref(null)
const deleting = ref(false)

function fill(u) {
  form.region = u.region
  form.email_enabled = u.email_enabled
  form.telegram_enabled = u.telegram_enabled
}

onMounted(async () => {
  fill(auth.user)
  try {
    regions.value = (await api.regions()).regions
  } catch (e) {
    saveErr.value = e.message
  }
})

async function save(confirmReset = false) {
  saving.value = true
  saveMsg.value = ''
  saveErr.value = ''
  try {
    auth.user = await api.updateMe({ ...form, confirm_region_reset: confirmReset })
    fill(auth.user)
    saveMsg.value = t('common.saved')
  } catch (e) {
    if (e.code === 'region_reset_required') {
      saving.value = false
      if (window.confirm(t('profile.region_confirm', { count: e.details.affected_conditions }))) return save(true)
      form.region = auth.user.region
      return
    }
    saveErr.value = e.message
  } finally {
    saving.value = false
  }
}

async function connectTelegram() {
  tgBusy.value = true
  tgErr.value = ''
  try {
    tg.value = await api.telegramLink()
  } catch (e) {
    tgErr.value = e.message
  } finally {
    tgBusy.value = false
  }
}

async function refreshTelegram() {
  await auth.load()
  fill(auth.user)
  if (auth.user.telegram_linked) tg.value = null
}

async function disconnectTelegram() {
  tgBusy.value = true
  try {
    await api.telegramUnlink()
    await refreshTelegram()
  } catch (e) {
    tgErr.value = e.message
  } finally {
    tgBusy.value = false
  }
}

async function logout() {
  await auth.logout()
  router.push('/login')
}

async function remove() {
  if (!window.confirm(t('profile.delete_confirm'))) return
  deleting.value = true
  delErr.value = null
  try {
    await api.deleteMe(delPassword.value)
    auth.forget()
    router.push('/register')
  } catch (e) {
    delErr.value = { field: e.field || 'password', message: e.message }
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <section>
    <h1>{{ t('profile.title') }}</h1>
    <VerifyBanner v-if="!auth.user.email_verified" />
    <div class="grid">
      <form class="card form" @submit.prevent="save(false)">
        <h2>{{ t('profile.account') }}</h2>
        <p>
          {{ t('common.email') }}: <strong>{{ auth.user.email }}</strong>
          <span :class="auth.user.email_verified ? 'text-ok' : 'text-warn'">
            {{ auth.user.email_verified ? '✓ ' + t('profile.verified') : t('profile.unverified') }}
          </span>
        </p>
        <label>{{ t('profile.region') }}
          <select v-model="form.region">
            <option v-for="r in regions" :key="r.code" :value="r.code">{{ r.name }} ({{ r.code }})</option>
          </select>
        </label>
        <p v-if="form.region !== auth.user.region" class="banner banner-inline">⚠ {{ t('profile.region_warning') }}</p>
        <label class="check"><input v-model="form.email_enabled" type="checkbox" /> {{ t('profile.email_enabled') }}</label>
        <label v-if="auth.user.telegram_linked" class="check">
          <input v-model="form.telegram_enabled" type="checkbox" /> {{ t('profile.telegram_enabled') }}
        </label>
        <p v-if="saveErr" class="form-error" role="alert">⚠ {{ saveErr }}</p>
        <p v-if="saveMsg" class="text-ok" role="status">{{ saveMsg }}</p>
        <div class="row">
          <button class="btn btn-primary" :disabled="saving">{{ t('common.save') }}</button>
          <button type="button" class="btn" @click="logout">{{ t('profile.logout') }}</button>
        </div>
      </form>

      <div class="card form">
        <h2>{{ t('profile.telegram') }}</h2>
        <template v-if="auth.user.telegram_linked">
          <p class="text-ok">✓ {{ t('profile.telegram_linked') }}</p>
          <button type="button" class="btn" :disabled="tgBusy" @click="disconnectTelegram">{{ t('profile.telegram_disconnect') }}</button>
        </template>
        <template v-else-if="tg">
          <p>{{ t('profile.telegram_instructions') }}</p>
          <a class="btn btn-primary" :href="tg.link" target="_blank" rel="noopener">{{ t('profile.telegram_open') }}</a>
          <code class="code">{{ tg.code }}</code>
          <button type="button" class="btn" @click="refreshTelegram">{{ t('profile.telegram_check') }}</button>
        </template>
        <button v-else type="button" class="btn btn-primary" :disabled="tgBusy" @click="connectTelegram">{{ t('profile.telegram_connect') }}</button>
        <p v-if="tgErr" class="form-error" role="alert">⚠ {{ tgErr }}</p>
      </div>

      <form class="card form" @submit.prevent="remove">
        <h2>{{ t('profile.delete') }}</h2>
        <p class="muted">{{ t('profile.delete_warning') }}</p>
        <label>{{ t('profile.current_password') }}
          <input v-model="delPassword" type="password" autocomplete="current-password" required />
          <FieldError :error="delErr" field="password" />
        </label>
        <button class="btn btn-danger" :disabled="deleting || !delPassword">{{ t('profile.delete_submit') }}</button>
      </form>
    </div>
  </section>
</template>
