// Тексты интерфейса берутся из файла локализации по ключу (REQ-INT-001, F-21).
import ru from './locales/ru.json'

const catalogs = { ru }
let current = catalogs.ru

export function t(key, params = {}) {
  let text = current[key] ?? key
  for (const [k, v] of Object.entries(params)) {
    text = text.replaceAll(`{${k}}`, String(v))
  }
  return text
}

// Форма слова для числа: plural(2, 'game') → «игры» (ключи plural.<слово>.one|few|many).
const rules = new Intl.PluralRules('ru-RU')
export function plural(n, word) {
  const form = rules.select(n)
  return t(`plural.${word}.${form === 'one' || form === 'few' ? form : 'many'}`)
}

export function hasKey(key) {
  return key in current
}

export function keys() {
  return Object.keys(current)
}
