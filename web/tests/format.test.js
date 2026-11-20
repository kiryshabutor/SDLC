import { describe, expect, it } from 'vitest'
import { conditionLabel, fromMinor, money, priceStatus, toMinor } from '../src/utils/format.js'
import { hasKey, keys, plural, t } from '../src/i18n.js'

describe('format', () => {
  it('formats minor units', () => {
    expect(money(3999, 'USD').replace(/\s/g, ' ')).toMatch(/39,99/)
    expect(money(99900, 'RUB')).toMatch(/999/)
    expect(money(null, 'USD')).toBe('')
  })

  it('converts condition values', () => {
    expect(toMinor('12,5')).toBe(1250)
    expect(toMinor('10')).toBe(1000)
    expect(toMinor('0.01')).toBe(1)
    expect(toMinor('0')).toBeNull()
    expect(toMinor('-3')).toBeNull()
    expect(toMinor('abc')).toBeNull()
    expect(fromMinor(1250)).toBe('12,50')
    expect(fromMinor(1000)).toBe('10')
    expect(fromMinor(0)).toBe('')
  })

  it('shows special states (REQ-FUNC-016)', () => {
    expect(priceStatus(null).text).toBe('Цена появится после проверки')
    expect(priceStatus({ availability: 'unavailable' }).text).toBe('Недоступна в вашем регионе')
    expect(priceStatus({ availability: 'free' }).text).toBe('Бесплатная')
    expect(priceStatus({ availability: 'unreleased' }).text).toBe('Цена не объявлена')
    expect(priceStatus({ availability: 'available' }).kind).toBe('available')
  })

  it('labels conditions', () => {
    expect(conditionLabel({ type: 'any' })).toBe('любая скидка')
    expect(conditionLabel({ type: 'min_pct', value: 30 })).toBe('скидка ≥ 30 %')
    expect(conditionLabel({ type: 'below', value: 1000 }, 'USD')).toMatch(/^цена ≤ 10,00/)
  })
})

describe('i18n (REQ-INT-001, F-21)', () => {
  it('substitutes params and keeps unknown keys visible', () => {
    expect(t('wishlist.summary', { count: 3, games: plural(3, 'game'), region: 'BY' })).toBe('3 игры · регион BY')
    expect(t('no.such.key')).toBe('no.such.key')
  })

  it('declines nouns by number', () => {
    expect([1, 2, 5, 11, 21, 22, 25, 0].map((n) => plural(n, 'game'))).toEqual(['игра', 'игры', 'игр', 'игр', 'игра', 'игры', 'игр', 'игр'])
  })

  it('has only Russian texts', () => {
    // «Email» — подпись поля из согласованного макета (SRS, ui-01).
    const allowed = new Set(['common.email'])
    for (const k of keys().filter((k) => !allowed.has(k))) expect(t(k), k).toMatch(/\p{Script=Cyrillic}/u)
  })

  it('has every key used in source files', async () => {
    const { readFileSync, readdirSync } = await import('node:fs')
    const { join } = await import('node:path')
    const walk = (dir) => readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
      e.isDirectory() ? walk(join(dir, e.name)) : [join(dir, e.name)])
    const used = new Set()
    for (const f of walk('src').filter((f) => /\.(vue|js)$/.test(f))) {
      for (const m of readFileSync(f, 'utf8').matchAll(/\bt\('([a-z_.]+)'/g)) used.add(m[1])
    }
    const missing = [...used].filter((k) => !hasKey(k))
    expect(missing).toEqual([])
  })
})
