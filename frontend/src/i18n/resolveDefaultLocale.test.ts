import assert from 'node:assert/strict'
import { test } from 'node:test'

import { resolveDefaultLocale } from './resolveDefaultLocale.ts'

test('resolveDefaultLocale accepts en-US from runtime or build time', () => {
  assert.equal(resolveDefaultLocale('en-US', undefined), 'en-US')
  assert.equal(resolveDefaultLocale('', 'en-US'), 'en-US')
  assert.equal(resolveDefaultLocale(' en-US '), 'en-US')
})

test('resolveDefaultLocale falls back to en-US for removed or unknown values', () => {
  assert.equal(resolveDefaultLocale(undefined, undefined), 'en-US')
  assert.equal(resolveDefaultLocale('zh-CN', 'ja-JP'), 'en-US')
  assert.equal(resolveDefaultLocale('fr-FR'), 'en-US')
  assert.equal(resolveDefaultLocale('en-US"};alert(1);//'), 'en-US')
  assert.equal(resolveDefaultLocale('   '), 'en-US')
})
