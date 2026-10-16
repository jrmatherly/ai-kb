import assert from 'node:assert/strict'
import { test } from 'node:test'

import { isEditable } from './guideDeferral'

test('isEditable recognises text fields, textareas and contenteditable hosts only', () => {
  const el = (tagName: string, extra: Partial<Record<string, unknown>> = {}) =>
    ({ tagName, isContentEditable: false, ...extra }) as unknown as Element
  assert.equal(isEditable(null), false)
  assert.equal(isEditable(el('DIV')), false)
  assert.equal(isEditable(el('BUTTON')), false)
  assert.equal(isEditable(el('TEXTAREA')), true)
  assert.equal(isEditable(el('INPUT', { type: 'text' })), true)
  assert.equal(isEditable(el('INPUT', { type: 'checkbox' })), false)
  assert.equal(isEditable(el('DIV', { isContentEditable: true })), true)
})
