<script setup>
// Вишлист (UC-08, REQ-FUNC-012…016): цены, исторический минимум, условие,
// сортировка и фильтры, удаление, добавление по ссылке или appid.
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { api } from '../api/client.js'
import { useAuth } from '../stores/auth.js'
import { plural, t } from '../i18n.js'
import { conditionLabel, dateTime, money, priceStatus } from '../utils/format.js'
import StateBox from '../components/StateBox.vue'
import VerifyBanner from '../components/VerifyBanner.vue'
import ConditionEditor from '../components/ConditionEditor.vue'

const SORT_KEY = 'wishlist.view'
const auth = useAuth()
const items = ref([])
const loading = ref(true)
const error = ref('')
const flash = ref('')
const busyId = ref(0)
const view = reactive(loadView())
const addRef = ref('')
const adding = ref(false)
const addError = ref('')

function loadView() {
  try {
    const v = JSON.parse(localStorage.getItem(SORT_KEY) || '{}')
    return { sort: v.sort || 'added', onlyDiscount: !!v.onlyDiscount, onlyHistLow: !!v.onlyHistLow }
  } catch {
    return { sort: 'added', onlyDiscount: false, onlyHistLow: false }
  }
}

const filtered = computed(() => view.onlyDiscount || view.onlyHistLow)
const lastChecked = computed(() => {
  const times = items.value.map((i) => i.checked_at).filter(Boolean).sort()
  return times.length ? times[times.length - 1] : null
})

async function load() {
  loading.value = true
  error.value = ''
  const filters = []
  if (view.onlyDiscount) filters.push('discount')
  if (view.onlyHistLow) filters.push('hist_low')
  try {
    items.value = (await api.wishlist({ sort: view.sort, filters })).items
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

watch(view, () => {
  try {
    localStorage.setItem(SORT_KEY, JSON.stringify(view))
  } catch {
    // недоступно в приватном режиме — сортировка просто не запомнится
  }
  load()
})
onMounted(load)

function say(text) {
  flash.value = text
  setTimeout(() => { if (flash.value === text) flash.value = '' }, 4000)
}

async function add() {
  adding.value = true
  addError.value = ''
  try {
    const it = await api.addToWishlist(addRef.value)
    addRef.value = ''
    say(t('wishlist.added', { name: it.name }))
    await load()
  } catch (e) {
    addError.value = e.message
  } finally {
    adding.value = false
  }
}

async function remove(it) {
  busyId.value = it.appid
  try {
    await api.removeFromWishlist(it.appid)
    items.value = items.value.filter((x) => x.appid !== it.appid)
    say(t('wishlist.removed', { name: it.name }))
  } catch (e) {
    say(e.message)
  } finally {
    busyId.value = 0
  }
}

async function saveCondition(it, condition) {
  busyId.value = it.appid
  try {
    const r = await api.setCondition(it.appid, condition)
    it.condition = r.condition
    say(t('cond.saved'))
  } catch (e) {
    say(e.message)
  } finally {
    busyId.value = 0
  }
}

function currencyOf(it) {
  return it.price?.currency || ''
}
</script>

<template>
  <section>
    <VerifyBanner v-if="auth.user && !auth.user.email_verified" />

    <div class="row-between">
      <div>
        <h1>{{ t('wishlist.title') }}</h1>
        <p class="muted">
          {{ t('wishlist.summary', { count: items.length, games: plural(items.length, 'game'), region: auth.user?.region }) }}
          <template v-if="lastChecked"> · {{ t('wishlist.checked', { time: dateTime(lastChecked) }) }}</template>
        </p>
      </div>
      <form class="add-form" @submit.prevent="add">
        <input v-model="addRef" :placeholder="t('wishlist.add_placeholder')" :aria-label="t('wishlist.add_placeholder')" required />
        <button class="btn btn-primary" :disabled="adding || !addRef">+ {{ t('wishlist.add') }}</button>
        <small v-if="addError" class="field-error" role="alert">{{ addError }}</small>
      </form>
    </div>

    <div class="toolbar">
      <label>{{ t('wishlist.sort') }}
        <select v-model="view.sort">
          <option value="added">{{ t('wishlist.sort.added') }}</option>
          <option value="name">{{ t('wishlist.sort.name') }}</option>
          <option value="price">{{ t('wishlist.sort.price') }}</option>
          <option value="discount">{{ t('wishlist.sort.discount') }}</option>
        </select>
      </label>
      <label class="check"><input v-model="view.onlyDiscount" type="checkbox" /> {{ t('wishlist.only_discount') }}</label>
      <label class="check"><input v-model="view.onlyHistLow" type="checkbox" /> {{ t('wishlist.only_hist_low') }}</label>
    </div>

    <p v-if="flash" class="flash" role="status">{{ flash }}</p>

    <StateBox :loading="loading" :error="error" :empty="items.length ? '' : (filtered ? t('wishlist.empty_filtered') : t('wishlist.empty'))" @retry="load">
      <template #empty-action>
        <RouterLink v-if="!filtered" class="btn" to="/search">{{ t('nav.search') }}</RouterLink>
      </template>
    </StateBox>

    <ul v-if="!loading && !error && items.length" class="games">
      <li v-for="it in items" :key="it.appid" class="game" :class="{ busy: busyId === it.appid }">
        <img v-if="it.cover_url" :src="it.cover_url" alt="" class="cover" loading="lazy" />
        <div v-else class="cover cover-empty" aria-hidden="true"></div>
        <div class="game-main">
          <a class="game-name" :href="`https://store.steampowered.com/app/${it.appid}/`" target="_blank" rel="noopener">{{ it.name }}</a>
          <div class="game-price">
            <template v-if="priceStatus(it.price).kind === 'available'">
              <strong>{{ money(it.price.final, it.price.currency) }}</strong>
              <template v-if="it.price.discount_pct > 0">
                <span class="badge badge-sale">−{{ it.price.discount_pct }} %</span>
                <span class="muted">{{ t('wishlist.regular', { price: money(it.price.initial, it.price.currency) }) }}</span>
              </template>
            </template>
            <span v-else class="muted">{{ priceStatus(it.price).text }}</span>
          </div>
          <div class="game-meta">
            <span v-if="it.at_hist_low" class="badge badge-low">{{ t('wishlist.at_hist_low') }}</span>
            <span v-else-if="it.hist_low != null && it.price" class="muted">{{ t('wishlist.hist_low', { price: money(it.hist_low, it.price.currency) }) }}</span>
            <span v-if="it.checked_at" class="muted small">{{ t('wishlist.checked', { time: dateTime(it.checked_at) }) }}</span>
          </div>
        </div>
        <div class="game-cond">
          <ConditionEditor
            :condition="it.condition" :currency="currencyOf(it)" :busy="busyId === it.appid"
            :title="conditionLabel(it.condition, currencyOf(it))" @save="(c) => saveCondition(it, c)"
          />
        </div>
        <button
          type="button" class="btn-icon" :title="t('wishlist.remove')" :aria-label="t('wishlist.remove') + ': ' + it.name"
          :disabled="busyId === it.appid" @click="remove(it)"
        >
          ✕
        </button>
      </li>
    </ul>
  </section>
</template>
