import type { WebSearchProviderEntity } from '@/api/web-search-provider';

export type AgentWebSearchConfig = {
  web_search_enabled?: boolean;
  web_search_provider_id?: string;
};

/** Resolve the search engine ID the agent will actually use (matches the backend's agent > tenant default logic) */
export function resolveAgentWebSearchProviderId(
  config: AgentWebSearchConfig | undefined,
  providers: WebSearchProviderEntity[],
): string | null {
  const explicitId = config?.web_search_provider_id?.trim();
  if (explicitId) {
    return providers.some((p) => p.id === explicitId) ? explicitId : null;
  }
  const defaultProvider = providers.find((p) => p.is_default);
  return defaultProvider?.id ?? null;
}

export function isAgentWebSearchEnabled(config: AgentWebSearchConfig | undefined): boolean {
  return config?.web_search_enabled === true;
}

/** The agent has web search enabled and a usable search engine resolves */
export function isAgentWebSearchReady(
  config: AgentWebSearchConfig | undefined,
  providers: WebSearchProviderEntity[],
  sourceWorkspaceReady?: boolean,
): boolean {
  if (!isAgentWebSearchEnabled(config)) return false;
  if (sourceWorkspaceReady !== undefined) return sourceWorkspaceReady;
  return resolveAgentWebSearchProviderId(config, providers) !== null;
}

/** Whether the space-level default search engine is usable (when no agent constrains it) */
export function isTenantWebSearchReady(providers: WebSearchProviderEntity[]): boolean {
  return providers.some((p) => p.is_default);
}
