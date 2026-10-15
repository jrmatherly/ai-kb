import assert from 'node:assert/strict'
import test from 'node:test'
import {
  agentHasConfiguredChatModel,
  agentRequiresRerankModel,
  canLocallyConfigureAgent,
  getAgentNotReadyReasonKeys,
  resolveAgentNotReadySection,
  resolveAgentNotReadyHighlight,
} from './agent-readiness.ts'

test('does not treat an unrelated built-in chat model as agent configuration', () => {
  assert.equal(agentHasConfiguredChatModel(
    {},
    [{ id: 'builtin-chat', type: 'KnowledgeQA' }],
  ), false)
})

test('accepts the chat model explicitly configured on the agent', () => {
  assert.equal(agentHasConfiguredChatModel(
    { model_id: 'builtin-chat' },
    [{ id: 'builtin-chat', type: 'KnowledgeQA' }],
  ), true)
})

test('rejects a configured chat model that no longer exists', () => {
  assert.equal(agentHasConfiguredChatModel(
    { model_id: 'deleted-chat' },
    [{ id: 'builtin-chat', type: 'KnowledgeQA' }],
  ), false)
})

test('requires rerank when search_knowledge has a knowledge-base scope', () => {
  assert.equal(agentRequiresRerankModel({
    kb_selection_mode: 'all',
    allowed_tools: ['search_knowledge'],
  }), true)
})

test('still requires rerank for configs saved with the retired knowledge_search name', () => {
  assert.equal(agentRequiresRerankModel({
    kb_selection_mode: 'all',
    allowed_tools: ['knowledge_search'],
  }), true)
  assert.equal(agentRequiresRerankModel({
    kb_selection_mode: 'selected',
    allowed_tools: ['thinking', 'grep_chunks'],
  }), true)
})

test('does not require rerank for read-only document tools', () => {
  assert.equal(agentRequiresRerankModel({
    kb_selection_mode: 'all',
    allowed_tools: ['read_document', 'list_documents'],
  }), false)
})

test('does not require rerank when knowledge bases are disabled', () => {
  assert.equal(agentRequiresRerankModel({
    kb_selection_mode: 'none',
    allowed_tools: ['search_knowledge'],
  }), false)
  assert.equal(agentRequiresRerankModel({
    kb_selection_mode: 'none',
    allowed_tools: ['knowledge_search'],
  }), false)
})

test('does not require rerank for wiki-only tools', () => {
  assert.equal(agentRequiresRerankModel({
    kb_selection_mode: 'all',
    allowed_tools: ['wiki_search', 'wiki_read_page'],
  }), false)
})

test('matches backend default-tool behavior', () => {
  assert.equal(agentRequiresRerankModel({
    kb_selection_mode: 'all',
    allowed_tools: [],
  }), true)
  assert.equal(agentRequiresRerankModel({
    kb_selection_mode: 'none',
    allowed_tools: [],
  }), false)
})

test('resolveAgentNotReadySection opens model config for model issues', () => {
  assert.equal(resolveAgentNotReadySection(['summary_model']), 'model')
  assert.equal(resolveAgentNotReadySection(['rerank_model']), 'model')
  assert.equal(resolveAgentNotReadySection(['summary_model', 'allowed_tools']), 'model')
})

test('resolveAgentNotReadySection opens tools config when only tools are missing', () => {
  assert.equal(resolveAgentNotReadySection(['allowed_tools']), 'tools')
})

test('resolveAgentNotReadyHighlight returns the first missing item', () => {
  assert.equal(resolveAgentNotReadyHighlight(['summary_model', 'rerank_model']), 'summary_model')
  assert.equal(resolveAgentNotReadyHighlight(['allowed_tools']), 'allowed_tools')
  assert.equal(resolveAgentNotReadyHighlight([]), undefined)
})

test('getAgentNotReadyReasonKeys flags missing chat model', () => {
  assert.deepEqual(getAgentNotReadyReasonKeys(
    {},
    [{ id: 'chat-1', type: 'KnowledgeQA' }],
    { isAgentMode: false, isSharedAgent: false },
  ), ['summary_model'])
})

test('getAgentNotReadyReasonKeys requires rerank only in agent mode with KB search', () => {
  assert.deepEqual(getAgentNotReadyReasonKeys(
    { kb_selection_mode: 'all', allowed_tools: ['search_knowledge'] },
    [{ id: 'chat-1', type: 'KnowledgeQA' }, { id: 'rerank-1', type: 'Rerank' }],
    { isAgentMode: true, isSharedAgent: false },
  ), ['summary_model', 'rerank_model'])
  assert.deepEqual(getAgentNotReadyReasonKeys(
    { kb_selection_mode: 'all', allowed_tools: ['knowledge_search'] },
    [{ id: 'chat-1', type: 'KnowledgeQA' }, { id: 'rerank-1', type: 'Rerank' }],
    { isAgentMode: true, isSharedAgent: false },
  ), ['summary_model', 'rerank_model'])
})

test('getAgentNotReadyReasonKeys does not treat current shared context as shared without sourceTenantId', () => {
  assert.deepEqual(getAgentNotReadyReasonKeys(
    { model_id: 'deleted-chat' },
    [{ id: 'chat-1', type: 'KnowledgeQA' }],
    { isAgentMode: false, isSharedAgent: false },
  ), ['summary_model'])
})

test('getAgentNotReadyReasonKeys treats empty allowed_tools as ready via backend defaults', () => {
  assert.deepEqual(getAgentNotReadyReasonKeys(
    {
      model_id: 'chat-1',
      rerank_model_id: 'rerank-1',
      kb_selection_mode: 'all',
      allowed_tools: [],
    },
    [{ id: 'chat-1', type: 'KnowledgeQA' }, { id: 'rerank-1', type: 'Rerank' }],
    { isAgentMode: true, isSharedAgent: false },
  ), [])
})

const tenantChat = { id: 'tenant-chat', type: 'KnowledgeQA', is_default: true }
const builtinChat = { id: 'builtin-chat', type: 'KnowledgeQA', is_default: true, is_builtin: true }
const tenantRerank = { id: 'tenant-rerank', type: 'Rerank', is_default: true }
const builtinRerank = { id: 'builtin-rerank', type: 'Rerank', is_default: true, is_builtin: true }
const smartConfig = { agent_mode: 'smart-reasoning', kb_selection_mode: 'all', allowed_tools: ['search_knowledge'] }
const builtinOwn = { isAgentMode: false, isSharedAgent: false, isBuiltinAgent: true }
const builtinOwnAgentMode = { isAgentMode: true, isSharedAgent: false, isBuiltinAgent: true }

test('a built-in agent with no model_id is ready on the workspace default chat model', () => {
  assert.deepEqual(getAgentNotReadyReasonKeys({}, [tenantChat], builtinOwn), [])
  assert.deepEqual(getAgentNotReadyReasonKeys({}, [builtinChat], builtinOwn), [])
})

test('a built-in agent is not ready on a chat model that is not the declared default', () => {
  assert.deepEqual(getAgentNotReadyReasonKeys(
    {},
    [{ id: 'builtin-chat', type: 'KnowledgeQA', is_builtin: true }],
    builtinOwn,
  ), ['summary_model'])
})

test('a custom agent still needs an explicit chat model when a default exists', () => {
  assert.deepEqual(getAgentNotReadyReasonKeys(
    {},
    [tenantChat, builtinChat],
    { isAgentMode: false, isSharedAgent: false, isBuiltinAgent: false },
  ), ['summary_model'])
})

test('a built-in agent with a stale model_id is not masked by the default', () => {
  assert.deepEqual(getAgentNotReadyReasonKeys({ model_id: 'deleted-chat' }, [tenantChat], builtinOwn), ['summary_model'])
})

test('a shared built-in agent does not use the receiving workspace default', () => {
  assert.deepEqual(getAgentNotReadyReasonKeys(
    {},
    [tenantChat],
    { isAgentMode: false, isSharedAgent: true, isBuiltinAgent: true },
  ), ['summary_model'])
})

test('a built-in smart-reasoning agent resolves its rerank slot to the declared default', () => {
  assert.deepEqual(getAgentNotReadyReasonKeys(smartConfig, [tenantChat, tenantRerank], builtinOwnAgentMode), [])
  assert.deepEqual(getAgentNotReadyReasonKeys(smartConfig, [builtinChat, builtinRerank], builtinOwnAgentMode), [])
  assert.deepEqual(getAgentNotReadyReasonKeys(
    smartConfig,
    [tenantChat, { id: 'plain-rerank', type: 'Rerank' }],
    builtinOwnAgentMode,
  ), ['rerank_model'])
  assert.deepEqual(getAgentNotReadyReasonKeys(
    { ...smartConfig, rerank_model_id: 'deleted-rerank' },
    [tenantChat, tenantRerank],
    builtinOwnAgentMode,
  ), ['rerank_model'])
})

test('canLocallyConfigureAgent is false for shared agents', () => {
  assert.equal(canLocallyConfigureAgent('42'), false)
  assert.equal(canLocallyConfigureAgent(undefined), true)
  assert.equal(canLocallyConfigureAgent(''), true)
})
