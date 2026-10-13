import assert from 'node:assert/strict'
import test from 'node:test'

import { normalizeAPIKeyKnowledgeBaseIDs } from './apiKeyScope.ts'

/**
 * A fully authorized key's null/undefined scope becomes an empty array, so the list does not blank out reading length.
 * Passes the empty values the server may return and expects the empty array that means "all knowledge bases".
 */
test('normalizes missing API key knowledge base scope to an empty array', () => {
  assert.deepEqual(normalizeAPIKeyKnowledgeBaseIDs(null), [])
  assert.deepEqual(normalizeAPIKeyKnowledgeBaseIDs(undefined), [])
})

/**
 * A scoped key's knowledge base IDs are copied in full, and the result is not the original array, so the edit form cannot corrupt the list data.
 * Passes two knowledge base IDs and expects a new array in the original order.
 */
test('copies configured API key knowledge base scope', () => {
  const ids = ['kb-1', 'kb-2']
  const normalized = normalizeAPIKeyKnowledgeBaseIDs(ids)

  assert.deepEqual(normalized, ids)
  assert.notEqual(normalized, ids)
  assert.deepEqual(normalizeAPIKeyKnowledgeBaseIDs([]), [])
})
