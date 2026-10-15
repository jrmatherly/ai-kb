import assert from 'node:assert/strict'
import test from 'node:test'

import { resolveBuiltinDefaultModelId } from './modelDefaults.ts'

const tenantChat = { id: 'tenant-chat', type: 'KnowledgeQA', is_default: true }
const builtinChat = { id: 'builtin-chat', type: 'KnowledgeQA', is_default: true, is_builtin: true }
const plainChat = { id: 'plain-chat', type: 'KnowledgeQA' }
const defaultRerank = { id: 'default-rerank', type: 'Rerank', is_default: true }

test('resolveBuiltinDefaultModelId prefers the workspace default', () => {
  assert.equal(resolveBuiltinDefaultModelId([plainChat, builtinChat, tenantChat], 'KnowledgeQA'), 'tenant-chat')
})

test('resolveBuiltinDefaultModelId falls back to the built-in default', () => {
  assert.equal(resolveBuiltinDefaultModelId([plainChat, builtinChat], 'KnowledgeQA'), 'builtin-chat')
})

test('resolveBuiltinDefaultModelId never guesses without a declared default', () => {
  assert.equal(resolveBuiltinDefaultModelId([plainChat, defaultRerank], 'KnowledgeQA'), null)
  assert.equal(resolveBuiltinDefaultModelId([], 'Rerank'), null)
})

test('resolveBuiltinDefaultModelId matches the requested type', () => {
  assert.equal(resolveBuiltinDefaultModelId([tenantChat, defaultRerank], 'Rerank'), 'default-rerank')
})
