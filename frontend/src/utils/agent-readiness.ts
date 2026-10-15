import type { CustomAgentConfig } from '@/api/agent'
import type { ModelConfig } from '@/api/model'
import { allowedToolsInclude } from './legacy-tool-names'
import { resolveBuiltinDefaultModelId } from './modelDefaults'

export type AgentNotReadyReasonKey = 'summary_model' | 'rerank_model' | 'allowed_tools'

/**
 * An agent is chat-ready only when it references a usable chat model. For a
 * custom agent that reference must be explicit: some model in the tenant must
 * not hide incomplete configuration. getAgentNotReadyReasonKeys lets the
 * caller's own built-in agents fill an empty slot with the declared default.
 *
 * For shared agents the model belongs to the source tenant, so the caller can
 * require a non-empty ID but must leave existence validation to the backend.
 */
export function agentHasConfiguredChatModel(
  config: Pick<CustomAgentConfig, 'model_id'> | undefined,
  models: Pick<ModelConfig, 'id' | 'type'>[],
  sourceModelIsRemote = false,
): boolean {
  const modelID = config?.model_id?.trim()
  if (!modelID) return false
  if (sourceModelIsRemote) return true
  return models.some(model => model.type === 'KnowledgeQA' && model.id === modelID)
}

/**
 * Keep this aligned with backend agentRequiresRerankModel.
 *
 * Rerank is needed only when search_knowledge can actually run. Explicitly
 * disabling the knowledge-base scope makes KB tools ineffective, even if an
 * older agent configuration still contains the tool in allowed_tools.
 *
 * Stored configs may still carry the retired `knowledge_search` name (and
 * `grep_chunks`, which was folded into search_knowledge); those count too.
 */
export function agentRequiresRerankModel(
  config: Pick<CustomAgentConfig, 'kb_selection_mode' | 'allowed_tools'> | undefined,
): boolean {
  if (!config || config.kb_selection_mode === 'none') return false

  const allowedTools = config.allowed_tools || []
  // The backend falls back to DefaultAllowedTools when the list is empty,
  // and that default includes search_knowledge.
  if (allowedTools.length === 0) return true

  return allowedToolsInclude(allowedTools, 'search_knowledge')
}

export function getAgentNotReadyReasonKeys(
  config: Pick<
    CustomAgentConfig,
    'model_id' | 'rerank_model_id' | 'kb_selection_mode' | 'allowed_tools' | 'agent_mode'
  > | undefined,
  models: Pick<ModelConfig, 'id' | 'type' | 'is_default' | 'is_builtin'>[],
  options: { isAgentMode: boolean; isSharedAgent: boolean; isBuiltinAgent: boolean },
): AgentNotReadyReasonKey[] {
  const reasons: AgentNotReadyReasonKey[] = []
  // Mirrors backend types.AgentModelSlot: only the caller's own built-in agents
  // resolve an empty slot to the declared default.
  const slot = (configured: string | undefined, type: ModelConfig['type']) =>
    configured?.trim() || (options.isBuiltinAgent && !options.isSharedAgent
      ? resolveBuiltinDefaultModelId(models, type) ?? ''
      : '')

  if (!agentHasConfiguredChatModel(
    { model_id: slot(config?.model_id, 'KnowledgeQA') },
    models,
    options.isSharedAgent,
  )) {
    reasons.push('summary_model')
  }

  if (options.isAgentMode && agentRequiresRerankModel(config)) {
    const rerankModelID = slot(config?.rerank_model_id, 'Rerank')
    const rerankExists = !!rerankModelID && (
      options.isSharedAgent
      || models.some(model => model.type === 'Rerank' && model.id === rerankModelID)
    )
    if (!rerankExists) {
      reasons.push('rerank_model')
    }
  }

  return reasons
}

/** Shared agents are owned by another tenant; receivers cannot edit their config. */
export function canLocallyConfigureAgent(sourceTenantId?: string): boolean {
  return !sourceTenantId
}

/** Map missing-config reasons to the agent editor section that fixes them. */
export function resolveAgentNotReadySection(reasons: AgentNotReadyReasonKey[]): string {
  if (reasons.includes('summary_model') || reasons.includes('rerank_model')) {
    return 'model'
  }
  if (reasons.includes('allowed_tools')) {
    return 'tools'
  }
  return 'model'
}

/** First missing item - used to highlight the corresponding editor field after navigation. */
export function resolveAgentNotReadyHighlight(
  reasons: AgentNotReadyReasonKey[],
): AgentNotReadyReasonKey | undefined {
  return reasons[0]
}
