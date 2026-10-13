import assert from 'node:assert/strict'
import test from 'node:test'

import {
  MODEL_DISPLAY_NAME_MAX_LEN,
  generateCopyDisplayName,
  modelCopyLabel,
} from './modelCopyName.ts'

test('modelCopyLabel prefers a trimmed display name and otherwise uses name', () => {
  assert.equal(modelCopyLabel({ display_name: ' Production GPT ', name: 'gpt-4o' }), 'Production GPT')
  assert.equal(modelCopyLabel({ display_name: '   ', name: 'gpt-4o' }), 'gpt-4o')
  assert.equal(modelCopyLabel({ name: 'qwen3:8b' }), 'qwen3:8b')
})

test('generateCopyDisplayName suffixes the display label and keeps later copies numbered', () => {
  assert.equal(generateCopyDisplayName('Production GPT', ['Production GPT'], ' copy'), 'Production GPT copy')
  assert.equal(
    generateCopyDisplayName('Production GPT', ['Production GPT', 'Production GPT copy'], ' copy'),
    'Production GPT copy 2',
  )
})

test('generateCopyDisplayName treats a name-only row as occupying that label', () => {
  assert.equal(
    generateCopyDisplayName('gpt-4o', ['gpt-4o', 'gpt-4o copy'], ' copy'),
    'gpt-4o copy 2',
  )
})

test('generateCopyDisplayName fits the label into the display_name column', () => {
  const suffix = ' copy'
  // A multi-byte character, so the limit is counted in characters, not bytes.
  const base = '\u540d'.repeat(300)
  const first = generateCopyDisplayName(base, [], suffix)
  assert.equal(Array.from(first).length, MODEL_DISPLAY_NAME_MAX_LEN)
  assert.equal(first.endsWith(suffix), true)

  const second = generateCopyDisplayName(base, [first], suffix)
  assert.equal(Array.from(second).length, MODEL_DISPLAY_NAME_MAX_LEN)
  assert.equal(second.endsWith(`${suffix} 2`), true)
  assert.notEqual(first, second)
})
