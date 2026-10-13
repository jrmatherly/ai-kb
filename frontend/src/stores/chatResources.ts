import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { listKnowledgeBases, getKnowledgeBaseById } from '@/api/knowledge-base'
import { listAgents, type CustomAgent } from '@/api/agent'
import { listModels, type ModelConfig } from '@/api/model'
import { listWebSearchProviders, type WebSearchProviderEntity } from '@/api/web-search-provider'
import { isNamedSandboxBackend, listSandboxConfigs, type SandboxConfigRecord } from '@/api/system'
import { useOrganizationStore } from '@/stores/organization'
import { getCurrentLanguage } from '@/utils/request'
import { isLocalizedSnapshotUsable, shouldForceLocalizedRefetch } from './localizedResourceCache'
import { createCachedResource, createKeyedSnapshotCache } from './resourceCache'

type ResourceKey = 'knowledgeBases' | 'agents' | 'models' | 'webSearchProviders' | 'sandboxConfigs'

export type ListCreatorFilter = 'all' | 'mine' | 'others'

function isKbModelReady(kb: any): boolean {
  if (!kb.summary_model_id || kb.summary_model_id === '') return false
  const strategy = kb.indexing_strategy
  const needsEmbedding = !strategy || strategy.vector_enabled || strategy.keyword_enabled
  if (needsEmbedding && (!kb.embedding_model_id || kb.embedding_model_id === '')) return false
  return true
}

/**
 * Shared snapshot of space-level resources (knowledge bases / agents / models / web search / sandbox backends).
 *
 * No TTL: every `ensureX()` sends a request (at most one in flight at a time, shared by mount bursts),
 * and after a write the caller refreshes explicitly with `ensureX(true)`, `replaceModels()` or `invalidate()`.
 * Pages render straight from the refs here, and the old snapshot holds until new data arrives.
 */
export const useChatResourcesStore = defineStore('chatResources', () => {
  const rawKnowledgeBases = ref<any[]>([])
  const agents = ref<CustomAgent[]>([])
  const disabledOwnAgentIds = ref<string[]>([])
  const allModels = ref<ModelConfig[]>([])
  const webSearchProviders = ref<WebSearchProviderEntity[]>([])
  const sandboxConfigs = ref<SandboxConfigRecord[]>([])

  const validKnowledgeBases = computed(() => rawKnowledgeBases.value.filter(isKbModelReady))
  const chatModels = computed(() => allModels.value.filter((m) => m.type === 'KnowledgeQA'))

  const knowledgeBasesResource = createCachedResource(
    async () => {
      const res: any = await listKnowledgeBases()
      return res?.data && Array.isArray(res.data) ? (res.data as any[]) : []
    },
    (data) => {
      rawKnowledgeBases.value = data
    },
  )

  // Built-in agent names and descriptions come back localized by the backend per Accept-Language; the snapshot is valid only for the
  // UI language at fetch time. agentsLoadedLocale must be the language "when the request was sent", not read after await.
  let agentsLoadedLocale = ''
  let agentsInflightLocale = ''
  const agentsResource = createCachedResource(
    async () => {
      const locale = getCurrentLanguage()
      agentsInflightLocale = locale
      const res = (await listAgents()) as { data?: CustomAgent[]; disabled_own_agent_ids?: string[] }
      return { data: res.data || [], disabled: res.disabled_own_agent_ids || [], locale }
    },
    ({ data, disabled, locale }) => {
      agents.value = data
      disabledOwnAgentIds.value = disabled
      agentsLoadedLocale = locale
    },
  )

  watch(
    () => getCurrentLanguage(),
    (locale) => {
      if (agentsLoadedLocale && agentsLoadedLocale !== locale) {
        agentsResource.invalidate()
        agentsLoadedLocale = ''
      }
    },
  )

  const modelsResource = createCachedResource(
    async () => {
      const models = await listModels()
      return Array.isArray(models) ? models : []
    },
    (models) => {
      allModels.value = models
    },
  )

  const webSearchProvidersResource = createCachedResource(
    async () => {
      const response = await listWebSearchProviders()
      const providers = (response as any)?.data
      return Array.isArray(providers) ? (providers as WebSearchProviderEntity[]) : []
    },
    (providers) => {
      webSearchProviders.value = providers
    },
  )

  // Swallow failures, do not throw: this is an optional resource. Without it the only option left is "no sandbox",
  // and the agent can still be edited and saved. Callers usually put it in the same Promise.all as a set of required
  // resources, so throwing here would fail the whole editor's dependency load with it
  // (no skill availability means the skill config group disappears).
  const sandboxConfigsResource = createCachedResource(
    async () => {
      try {
        const res = await listSandboxConfigs()
        const rows = Array.isArray(res?.data) ? res.data : []
        return rows.filter((cfg) => isNamedSandboxBackend(cfg.sandbox_type))
      } catch {
        return [] as SandboxConfigRecord[]
      }
    },
    (rows) => {
      sandboxConfigs.value = rows
    },
  )

  const resources: Record<ResourceKey, ReturnType<typeof createCachedResource>> = {
    knowledgeBases: knowledgeBasesResource,
    agents: agentsResource,
    models: modelsResource,
    webSearchProviders: webSearchProvidersResource,
    sandboxConfigs: sandboxConfigsResource,
  }

  // Knowledge bases visible to an agent and single knowledge base details: once fetched they are reused until explicitly invalidated.
  // The @ mention list fills counts one by one, so these per-key reads cannot hit the API every time.
  const agentKbCache = createKeyedSnapshotCache(async (cacheKey: string) => {
    const [agentId, sourceTenantId] = cacheKey.split(':')
    const res: any = await listKnowledgeBases({
      agent_id: agentId,
      agent_source_tenant_id: sourceTenantId === 'current' ? undefined : sourceTenantId,
    })
    return res?.data && Array.isArray(res.data) ? (res.data as any[]) : []
  })
  const kbDetailCache = createKeyedSnapshotCache(async (kbId: string) => {
    try {
      const res: any = await getKnowledgeBaseById(kbId)
      return res?.data ?? null
    } catch {
      return null
    }
  })

  /** Whether a snapshot exists (as opposed to "is it fresh": this store no longer has a freshness window). */
  function isLoaded(key: ResourceKey): boolean {
    if (key === 'agents') {
      return isLocalizedSnapshotUsable(agentsResource.isLoaded(), agentsLoadedLocale, getCurrentLanguage())
    }
    return resources[key].isLoaded()
  }

  /**
   * Knowledge base list (supports a creator filter). creator=all uses the shared snapshot so the chat page can reuse it.
   */
  async function fetchKnowledgeBasesForList(
    params?: { creator?: ListCreatorFilter },
    force = false,
  ): Promise<any[]> {
    const creator = params?.creator ?? 'all'
    // A creator-filtered list is for the list page only and is not snapshotted; the request passes straight through.
    // The forced refresh after a write also refreshes the full snapshot, otherwise the chat input bar would show deleted knowledge bases.
    if (creator !== 'all') {
      if (force) void ensureKnowledgeBases(true).catch(() => {})
      const res: any = await listKnowledgeBases({ creator })
      return res?.data && Array.isArray(res.data) ? res.data : []
    }
    await ensureKnowledgeBases(force)
    return rawKnowledgeBases.value
  }

  // Shared resources are a side refresh: their endpoint failing must not take down the user's own list, otherwise the caller's catch
  // would clear the user's own knowledge bases and agents it already has (manual document editor, API integration page).
  // When the shared list fails, keep the last snapshot and let the page decide whether to force a separate refresh.
  const swallow = (p: Promise<unknown>) => p.catch(() => undefined)

  async function ensureKnowledgeBases(force = false): Promise<void> {
    const orgStore = useOrganizationStore()
    await Promise.all([
      knowledgeBasesResource.ensure(force),
      swallow(orgStore.fetchSharedKnowledgeBases({ force })),
    ])
  }

  /**
   * Agent list (supports a creator filter). creator=all uses the shared snapshot.
   */
  async function fetchAgentsForList(
    params?: { creator?: ListCreatorFilter },
    force = false,
  ): Promise<{ data: CustomAgent[]; disabled_own_agent_ids: string[] }> {
    const creator = params?.creator ?? 'all'
    const orgStore = useOrganizationStore()

    // A creator-filtered list is not snapshotted, but shared agents still need a refresh (consistent with the full path).
    if (creator !== 'all') {
      if (force) void ensureAgents(true).catch(() => {})
      const [agentsRes] = await Promise.all([
        listAgents({ creator }),
        orgStore.fetchSharedAgents({ force }),
      ])
      const res = agentsRes as { data?: CustomAgent[]; disabled_own_agent_ids?: string[] }
      return { data: res.data || [], disabled_own_agent_ids: res.disabled_own_agent_ids || [] }
    }

    await ensureAgents(force)
    return { data: agents.value, disabled_own_agent_ids: disabledOwnAgentIds.value }
  }

  async function ensureAgents(force = false): Promise<void> {
    const orgStore = useOrganizationStore()
    const locale = getCurrentLanguage()
    // The in-flight request was sent in another language: it cannot be reused, so queue another one after it.
    const mustForce =
      force ||
      shouldForceLocalizedRefetch(agentsResource.hasInFlightRequest(), agentsInflightLocale, locale)
    await Promise.all([
      agentsResource.ensure(mustForce),
      swallow(orgStore.fetchSharedAgents({ force })),
    ])
  }

  async function ensureModels(force = false): Promise<void> {
    return modelsResource.ensure(force)
  }

  /** Overwrite the snapshot with a freshly fetched list (called after models are added, removed or changed in settings), and void the in-flight old request. */
  function replaceModels(models: ModelConfig[]) {
    allModels.value = Array.isArray(models) ? models : []
    modelsResource.markLoaded()
  }

  /** @deprecated Use ensureModels; the alias stays for the chat input bar */
  async function ensureChatModels(force = false): Promise<void> {
    return ensureModels(force)
  }

  async function ensureWebSearchProviders(force = false): Promise<void> {
    return webSearchProvidersResource.ensure(force)
  }

  /**
   * Sandbox backend configs, for the backend picker in the agent editor.
   * Not in prefetchChatInput: only agent editing needs it; the chat input bar does not.
   */
  async function ensureSandboxConfigs(force = false): Promise<void> {
    return sandboxConfigsResource.ensure(force)
  }

  /** Prefetch in parallel the space-level resources the chat input bar and list pages commonly use */
  async function prefetchChatInput(force = false): Promise<void> {
    const orgStore = useOrganizationStore()
    await Promise.all([
      ensureKnowledgeBases(force),
      ensureAgents(force),
      ensureModels(force),
      ensureWebSearchProviders(force),
      orgStore.fetchOrganizations({ force }),
    ])
  }

  async function ensureAgentKnowledgeBases(agentId: string, sourceTenantId?: string, force = false): Promise<any[]> {
    return agentKbCache.ensure(`${agentId}:${sourceTenantId || 'current'}`, force)
  }

  /** Single knowledge base details (shared by the sidebar and the details page, concurrent requests deduplicated) */
  async function fetchKnowledgeBaseById(kbId: string, force = false): Promise<any | null> {
    if (!kbId) return null
    return kbDetailCache.ensure(kbId, force)
  }

  function invalidateKnowledgeBaseDetail(kbId?: string) {
    kbDetailCache.invalidate(kbId)
  }

  function invalidate(...keys: ResourceKey[]) {
    if (keys.length === 0) {
      ;(Object.keys(resources) as ResourceKey[]).forEach((k) => resources[k].invalidate())
      rawKnowledgeBases.value = []
      agents.value = []
      disabledOwnAgentIds.value = []
      allModels.value = []
      webSearchProviders.value = []
      sandboxConfigs.value = []
      agentsLoadedLocale = ''
      agentsInflightLocale = ''
      agentKbCache.invalidate()
      invalidateKnowledgeBaseDetail()
      return
    }
    keys.forEach((k) => resources[k].invalidate())
    if (keys.includes('knowledgeBases')) {
      agentKbCache.invalidate()
      invalidateKnowledgeBaseDetail()
    }
    if (keys.includes('agents')) {
      agentsLoadedLocale = ''
    }
  }

  return {
    rawKnowledgeBases,
    validKnowledgeBases,
    agents,
    disabledOwnAgentIds,
    allModels,
    chatModels,
    webSearchProviders,
    sandboxConfigs,
    isLoaded,
    fetchKnowledgeBasesForList,
    fetchAgentsForList,
    ensureKnowledgeBases,
    ensureAgents,
    ensureModels,
    replaceModels,
    ensureChatModels,
    ensureWebSearchProviders,
    ensureSandboxConfigs,
    ensureAgentKnowledgeBases,
    prefetchChatInput,
    fetchKnowledgeBaseById,
    invalidateKnowledgeBaseDetail,
    invalidate,
  }
})
