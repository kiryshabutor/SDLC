<script setup>
// Поиск игр в Steam (UC-04, REQ-FUNC-008): до 20 результатов в регионе
// пользователя, кнопка добавления в вишлист.
import { ref } from 'vue'
import { api } from '../api/client.js'
import { t } from '../i18n.js'
import { money, priceStatus } from '../utils/format.js'
import StateBox from '../components/StateBox.vue'

const q = ref('')
const items = ref(null)
const loading = ref(false)
const error = ref('')
const busyId = ref(0)
const rowError = ref({})

async function search() {
  if (q.value.trim().length < 2) {
    error.value = t('search.hint')
    return
  }
  loading.value = true
  error.value = ''
  try {
    items.value = (await api.search(q.value.trim())).items
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function add(it) {
  busyId.value = it.appid
  rowError.value = { ...rowError.value, [it.appid]: '' }
  try {
    await api.addToWishlist(String(it.appid))
    it.in_wishlist = true
  } catch (e) {
    rowError.value = { ...rowError.value, [it.appid]: e.message }
  } finally {
    busyId.value = 0
  }
}
</script>

<template>
  <section>
    <h1>{{ t('search.title') }}</h1>
    <form class="add-form add-form-wide" role="search" @submit.prevent="search">
      <input v-model="q" type="search" :placeholder="t('search.placeholder')" :aria-label="t('search.placeholder')" />
      <button class="btn btn-primary" :disabled="loading">{{ t('search.submit') }}</button>
    </form>
    <StateBox :loading="loading" :error="error" :empty="items && !items.length ? t('search.empty') : ''" @retry="search" />
    <ul v-if="!loading && !error && items && items.length" class="games">
      <li v-for="it in items" :key="it.appid" class="game">
        <img v-if="it.cover_url" :src="it.cover_url" alt="" class="cover" loading="lazy" />
        <div v-else class="cover cover-empty" aria-hidden="true"></div>
        <div class="game-main">
          <span class="game-name">{{ it.name }}</span>
          <div class="game-price">
            <template v-if="it.price && priceStatus(it.price).kind === 'available'">
              <strong>{{ money(it.price.final, it.price.currency) }}</strong>
              <span v-if="it.price.discount_pct > 0" class="badge badge-sale">−{{ it.price.discount_pct }} %</span>
            </template>
            <span v-else-if="it.price" class="muted">{{ priceStatus(it.price).text }}</span>
          </div>
          <small v-if="rowError[it.appid]" class="field-error">{{ rowError[it.appid] }}</small>
        </div>
        <span v-if="it.in_wishlist" class="badge">{{ t('search.in_wishlist') }}</span>
        <button v-else type="button" class="btn" :disabled="busyId === it.appid" @click="add(it)">+ {{ t('wishlist.add') }}</button>
      </li>
    </ul>
  </section>
</template>
