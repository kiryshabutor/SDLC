<script setup>
// Выбор условия уведомления для игры (REQ-FUNC-015). Для «скидка не менее
// N %» и «цена ниже X» показывается поле значения; X вводится в валюте
// региона и передаётся в API в минимальных единицах.
import { computed, ref, watch } from 'vue'
import { t } from '../i18n.js'
import { fromMinor, toMinor } from '../utils/format.js'

const props = defineProps({
  condition: { type: Object, required: true },
  currency: { type: String, default: '' },
  busy: { type: Boolean, default: false },
})
const emit = defineEmits(['save'])

const type = ref(props.condition.type)
const value = ref('')
const localError = ref('')

function reset() {
  type.value = props.condition.type
  value.value = props.condition.type === 'below' ? fromMinor(props.condition.value)
    : props.condition.type === 'min_pct' ? String(props.condition.value) : ''
}
reset()
watch(() => props.condition, reset)

const needsValue = computed(() => type.value === 'min_pct' || type.value === 'below')

function save() {
  localError.value = ''
  if (!needsValue.value) {
    emit('save', { type: type.value, value: 0 })
    return
  }
  const v = type.value === 'below' ? toMinor(value.value) : Number(value.value)
  if (v === null || !Number.isInteger(v)) {
    localError.value = t('error.invalid_condition_value')
    return
  }
  emit('save', { type: type.value, value: v })
}

function onTypeChange() {
  value.value = ''
  if (!needsValue.value) save()
}
</script>

<template>
  <div class="cond">
    <label class="sr-only" :for="'cond-' + $.uid">{{ t('cond.label') }}</label>
    <select :id="'cond-' + $.uid" v-model="type" :disabled="busy" @change="onTypeChange">
      <option value="any">{{ t('cond.any') }}</option>
      <option value="min_pct">{{ t('cond.min_pct') }}</option>
      <option value="below">{{ t('cond.below') }}</option>
      <option value="hist_low">{{ t('cond.hist_low') }}</option>
    </select>
    <template v-if="needsValue">
      <input
        v-model="value" class="cond-value" inputmode="decimal" :disabled="busy"
        :placeholder="type === 'below' ? t('cond.price_placeholder', { currency }) : t('cond.pct_placeholder')"
        @keyup.enter="save"
      />
      <button type="button" class="btn btn-small" :disabled="busy || !value" @click="save">{{ t('cond.apply') }}</button>
    </template>
    <small v-if="localError" class="field-error">{{ localError }}</small>
  </div>
</template>
