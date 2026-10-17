import assert from 'node:assert/strict'
import { test } from 'node:test'

import { countsAsTyping, isEditable } from './guideDeferral'

test('a keydown defers the guide only when typed into an editable field', () => {
  const ev = (type: string, target: unknown) => ({ type, target }) as unknown as Event
  const textarea = { tagName: 'TEXTAREA', isContentEditable: false }
  const button = { tagName: 'BUTTON', isContentEditable: false }
  assert.equal(countsAsTyping(ev('keydown', textarea)), true)
  assert.equal(countsAsTyping(ev('keydown', button)), false)
  assert.equal(countsAsTyping(ev('keydown', null)), false)
  assert.equal(countsAsTyping(ev('input', button)), true)
})

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
