// Клиент REST API /api/v1 (DV-005). Сессия — cookie sid (HttpOnly), для
// изменяющих запросов заголовок X-CSRF-Token со значением cookie csrf.
import { t } from '../i18n.js'

export class ApiError extends Error {
  constructor(status, body) {
    const e = body?.error ?? {}
    super(e.message || t('common.error'))
    this.status = status
    this.code = e.code || (status === 0 ? 'network' : 'unknown')
    this.field = e.field || ''
    this.details = e
  }
}

export function readCookie(name, source = document.cookie) {
  for (const part of source.split(';')) {
    const [k, ...v] = part.trim().split('=')
    if (k === name) return decodeURIComponent(v.join('='))
  }
  return ''
}

let onUnauthorized = () => {}
export function setUnauthorizedHandler(fn) {
  onUnauthorized = fn
}

export async function request(method, path, body, { fetchImpl = fetch } = {}) {
  const headers = { Accept: 'application/json' }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (!['GET', 'HEAD'].includes(method)) headers['X-CSRF-Token'] = readCookie('csrf')
  let resp
  try {
    resp = await fetchImpl('/api/v1' + path, {
      method,
      headers,
      credentials: 'same-origin',
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, null)
  }
  const text = await resp.text()
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = null
    }
  }
  if (!resp.ok) {
    const err = new ApiError(resp.status, data)
    if (resp.status === 401 && err.code === 'unauthorized') onUnauthorized()
    throw err
  }
  return data
}

export const api = {
  // Учётная запись
  me: () => request('GET', '/me'),
  register: (form) => request('POST', '/auth/register', form),
  login: (email, password) => request('POST', '/auth/login', { email, password }),
  logout: () => request('POST', '/auth/logout'),
  verifyEmail: (token) => request('POST', '/auth/verify-email', { token }),
  resendVerification: () => request('POST', '/auth/resend-verification'),
  passwordReset: (email) => request('POST', '/auth/password-reset', { email }),
  passwordResetConfirm: (token, password, password_confirm) =>
    request('POST', '/auth/password-reset/confirm', { token, password, password_confirm }),
  updateMe: (patch) => request('PATCH', '/me', patch),
  deleteMe: (password) => request('DELETE', '/me', { password }),
  telegramLink: () => request('POST', '/me/telegram/link'),
  telegramUnlink: () => request('DELETE', '/me/telegram'),
  regions: () => request('GET', '/regions'),

  // Вишлист и поиск
  wishlist: ({ sort = 'added', filters = [] } = {}) => {
    const q = new URLSearchParams({ sort })
    if (filters.length) q.set('filter', filters.join(','))
    return request('GET', '/wishlist?' + q)
  },
  addToWishlist: (ref) => request('POST', '/wishlist', { ref }),
  removeFromWishlist: (appid) => request('DELETE', `/wishlist/${appid}`),
  setCondition: (appid, condition) => request('PATCH', `/wishlist/${appid}`, { condition }),
  search: (q) => request('GET', '/games/search?q=' + encodeURIComponent(q)),
}
