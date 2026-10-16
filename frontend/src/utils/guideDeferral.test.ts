import assert from 'node:assert/strict'
import { test } from 'node:test'

import { isEditable, shouldOpenGuide } from './guideDeferral'

test('a guide does not open over input the user already started', () => {
  assert.equal(shouldOpenGuide({ inputSeen: true }), false)
})

test('a page without typing opens the guide (a click into the composer alone does not defer it)', () => {
  assert.equal(shouldOpenGuide({ inputSeen: false }), true)
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
