import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ApiError, readCookie, request, setUnauthorizedHandler } from '../src/api/client.js'

function fakeFetch(status, body) {
  return vi.fn(async () => ({ ok: status >= 200 && status < 300, status, text: async () => (body === undefined ? '' : JSON.stringify(body)) }))
}

describe('api client', () => {
  beforeEach(() => {
    document.cookie = 'csrf=tok-123; path=/'
  })

  it('reads cookies', () => {
    expect(readCookie('csrf', 'a=1; csrf=x%20y; b=2')).toBe('x y')
    expect(readCookie('none', 'a=1')).toBe('')
  })

  it('sends CSRF header only for mutating requests (REQ-SEC-009)', async () => {
    const f = fakeFetch(200, { ok: true })
    await request('POST', '/wishlist', { ref: '570' }, { fetchImpl: f })
    const [url, opts] = f.mock.calls[0]
    expect(url).toBe('/api/v1/wishlist')
    expect(opts.headers['X-CSRF-Token']).toBe('tok-123')
    expect(opts.credentials).toBe('same-origin')
    expect(JSON.parse(opts.body)).toEqual({ ref: '570' })

    await request('GET', '/me', undefined, { fetchImpl: f })
    expect(f.mock.calls[1][1].headers['X-CSRF-Token']).toBeUndefined()
  })

  it('maps API errors with field and message', async () => {
    const f = fakeFetch(400, { error: { code: 'weak_password', message: 'Слабый пароль', field: 'password' } })
    const err = await request('POST', '/auth/register', {}, { fetchImpl: f }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(400)
    expect(err.code).toBe('weak_password')
    expect(err.field).toBe('password')
    expect(err.message).toBe('Слабый пароль')
  })

  it('calls unauthorized handler on 401', async () => {
    const handler = vi.fn()
    setUnauthorizedHandler(handler)
    await request('GET', '/me', undefined, { fetchImpl: fakeFetch(401, { error: { code: 'unauthorized', message: 'Войдите' } }) }).catch(() => {})
    expect(handler).toHaveBeenCalledOnce()
    // Неверный пароль при входе — тоже 401, но это не истёкшая сессия.
    await request('POST', '/auth/login', {}, { fetchImpl: fakeFetch(401, { error: { code: 'invalid_credentials' } }) }).catch(() => {})
    expect(handler).toHaveBeenCalledOnce()
  })

  it('reports network failures in Russian', async () => {
    const err = await request('GET', '/me', undefined, { fetchImpl: vi.fn(async () => { throw new TypeError('Failed') }) }).catch((e) => e)
    expect(err.code).toBe('network')
    expect(err.message).toMatch(/соединение/)
  })

  it('accepts empty 204 responses', async () => {
    await expect(request('DELETE', '/wishlist/1', undefined, { fetchImpl: fakeFetch(204) })).resolves.toBeNull()
  })
})
