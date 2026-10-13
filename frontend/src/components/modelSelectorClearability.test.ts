import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'

const selector = readFileSync(new URL('./ModelSelector.vue', import.meta.url), 'utf8')
const agentEditor = readFileSync(new URL('../views/agent/AgentEditorModal.vue', import.meta.url), 'utf8')
const kbModelConfig = readFileSync(new URL('../views/knowledge/settings/KBModelConfig.vue', import.meta.url), 'utf8')
const kbEditor = readFileSync(new URL('../views/knowledge/KnowledgeBaseEditorModal.vue', import.meta.url), 'utf8')
const uploadConfirm = readFileSync(new URL('../views/knowledge/components/UploadConfirmDialog.vue', import.meta.url), 'utf8')

function modelSelectorTag(source: string, selectedModelBinding: string): string {
  const tag = source
    .match(/<ModelSelector\b[\s\S]*?\/>/g)
    ?.find((candidate) => candidate.includes(selectedModelBinding))
  assert.ok(tag, `Model selector not found: ${selectedModelBinding}`)
  return tag
}

function assertClearable(tag: string): void {
  assert.match(tag, /(?:\s|:)clearable(?:\s|=|\/>)/)
}

function assertNotClearable(tag: string): void {
  assert.doesNotMatch(tag, /(?:\s|:)clearable(?:\s|=|\/>)/)
}

test('ModelSelector sends an empty string to the parent when cleared, and stays non-clearable by default', () => {
  assert.match(selector, /:clearable="clearable"/)
  assert.match(selector, /clearable:\s*false/)
  assert.match(selector, /emit\('update:selectedModelId', value \|\| ''\)/)
})

test('optional agent models that can inherit or be turned off can be reset to empty', () => {
  const rerank = modelSelectorTag(agentEditor, 'formData.config.rerank_model_id')
  assert.match(rerank, /:clearable="!needsRerankModel"/)
  assertClearable(modelSelectorTag(agentEditor, 'formData.config.query_understand_model_id'))
  assertClearable(modelSelectorTag(agentEditor, 'formData.config.asr_model_id'))
  assertClearable(modelSelectorTag(agentEditor, 'formData.config.question_suggestions.follow_ups.model_id'))
})

test('the knowledge base allows resetting to empty only when the model is truly optional', () => {
  const embedding = modelSelectorTag(kbModelConfig, 'config.embeddingModelId')
  assert.match(embedding, /:clearable="ragEnabled === false && wikiEnabled"/)
  assertClearable(modelSelectorTag(kbModelConfig, 'config.wikiSynthesisModelId'))
})

test('required models stay non-clearable', () => {
  assertNotClearable(modelSelectorTag(agentEditor, 'formData.config.model_id'))
  assertNotClearable(modelSelectorTag(agentEditor, 'formData.config.vlm_model_id'))
  assertNotClearable(modelSelectorTag(kbModelConfig, 'config.llmModelId'))
  assertNotClearable(modelSelectorTag(kbEditor, 'formData.multimodalConfig.vllmModelId'))
  assertNotClearable(modelSelectorTag(kbEditor, 'formData.asrConfig.modelId'))
  assertNotClearable(modelSelectorTag(uploadConfirm, 'uiState.multimodalConfig.vllmModelId'))
  assertNotClearable(modelSelectorTag(uploadConfirm, 'uiState.asrConfig.modelId'))
})
