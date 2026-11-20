<script setup>
// Баннер неподтверждённого email с повторной отправкой письма (REQ-FUNC-002).
import { ref } from 'vue'
import { api } from '../api/client.js'
import { t } from '../i18n.js'

const sending = ref(false)
const message = ref('')
const isError = ref(false)

async function resend() {
  sending.value = true
  message.value = ''
  try {
    await api.resendVerification()
    message.value = t('banner.sent')
    isError.value = false
  } catch (e) {
    message.value = e.message
    isError.value = true
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div class="banner" role="status">
    <span>⚠ {{ t('banner.unverified') }}</span>
    <button type="button" class="btn btn-small" :disabled="sending" @click="resend">{{ t('banner.resend') }}</button>
    <span v-if="message" :class="isError ? 'text-error' : 'text-ok'">{{ message }}</span>
  </div>
</template>
