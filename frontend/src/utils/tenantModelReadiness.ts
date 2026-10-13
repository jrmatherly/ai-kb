import type { ModelConfig } from '@/api/model'

export interface TenantModelReadiness {
  chatCount: number
  embeddingCount: number
  hasChat: boolean
  hasEmbedding: boolean
  /** Whether the models a document knowledge base needs for its default vector and keyword retrieval are all configured */
  isReadyForDocumentKb: boolean
  /** Creating an agent needs at least a chat model */
  isReadyForAgent: boolean
}

export function evaluateTenantModelReadiness(models: ModelConfig[]): TenantModelReadiness {
  const chatCount = models.filter((m) => m.type === 'KnowledgeQA').length
  const embeddingCount = models.filter((m) => m.type === 'Embedding').length
  const hasChat = chatCount > 0
  const hasEmbedding = embeddingCount > 0
  return {
    chatCount,
    embeddingCount,
    hasChat,
    hasEmbedding,
    isReadyForDocumentKb: hasChat && hasEmbedding,
    isReadyForAgent: hasChat,
  }
}
