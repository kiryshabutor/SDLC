// Форматирование цен и дат. Цены приходят в минимальных единицах валюты
// (3999 = 39,99), как их возвращает Steam (DV-004).
import { t } from '../i18n.js'

export function money(minor, currency) {
  if (minor == null || !currency) return ''
  try {
    return new Intl.NumberFormat('ru-RU', { style: 'currency', currency }).format(minor / 100)
  } catch {
    return (minor / 100).toFixed(2) + ' ' + currency
  }
}

export function dateTime(iso) {
  if (!iso) return ''
  return new Intl.DateTimeFormat('ru-RU', { dateStyle: 'short', timeStyle: 'short' }).format(new Date(iso))
}

// Статус цены для элемента вишлиста (REQ-FUNC-016).
export function priceStatus(price) {
  if (!price) return { kind: 'pending', text: t('price.pending') }
  switch (price.availability) {
    case 'available':
      return { kind: 'available' }
    case 'free':
      return { kind: 'special', text: t('price.free') }
    case 'unreleased':
      return { kind: 'special', text: t('price.unreleased') }
    default:
      return { kind: 'special', text: t('price.unavailable') }
  }
}

// Перевод значения условия «цена ниже X» между вводом пользователя в
// валюте (12,5) и API в минимальных единицах (1250).
export function toMinor(input) {
  const n = Number(String(input).trim().replace(',', '.'))
  if (!Number.isFinite(n) || n <= 0) return null
  return Math.round(n * 100)
}

export function fromMinor(minor) {
  if (!minor) return ''
  return (minor / 100).toFixed(2).replace(/\.00$/, '').replace('.', ',')
}

// Подпись условия для списка (REQ-FUNC-015).
export function conditionLabel(cond, currency) {
  switch (cond?.type) {
    case 'min_pct':
      return t('cond.short_min_pct', { n: cond.value })
    case 'below':
      return t('cond.short_below', { price: money(cond.value, currency) })
    case 'hist_low':
      return t('cond.hist_low')
    default:
      return t('cond.any')
  }
}
