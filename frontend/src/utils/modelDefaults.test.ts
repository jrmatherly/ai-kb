import assert from 'node:assert/strict'
import test from 'node:test'

import { initialChatModelId, resolveBuiltinDefaultModelId } from './modelDefaults.ts'

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

test('resolveBuiltinDefaultModelId skips defaults that are not active', () => {
  const downloading = { id: 'downloading-chat', type: 'KnowledgeQA', is_default: true, status: 'downloading' }
  const activeBuiltin = { ...builtinChat, status: 'active' }
  assert.equal(resolveBuiltinDefaultModelId([downloading, activeBuiltin], 'KnowledgeQA'), 'builtin-chat')
  assert.equal(resolveBuiltinDefaultModelId([downloading], 'KnowledgeQA'), null)
})

test('initialChatModelId is empty when the agent has no effective model', () => {
  assert.equal(initialChatModelId(null, ['last-pick'], ['last-pick']), '')
})

test('initialChatModelId prefers an available earlier pick over the agent model', () => {
  assert.equal(initialChatModelId('agent-chat', ['last-pick', 'store-pick'], ['agent-chat', 'last-pick']), 'last-pick')
  assert.equal(initialChatModelId('agent-chat', ['', 'store-pick'], ['agent-chat', 'store-pick']), 'store-pick')
})

test('initialChatModelId ignores a pick that is no longer available', () => {
  assert.equal(initialChatModelId('agent-chat', ['deleted-pick'], ['agent-chat']), 'agent-chat')
})
