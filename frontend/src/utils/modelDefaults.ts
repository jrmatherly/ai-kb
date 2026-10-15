export interface ModelDefaultCandidate {
  id?: string
  type: string
  status?: string
  is_default?: boolean
  is_builtin?: boolean
}

/**
 * Pick a creation-time model: prefer the declared default, fall back to the
 * first active model.
 */
export function selectInitialModelId(
  models: readonly ModelDefaultCandidate[],
  modelType: string,
): string | null {
  const active = models.filter(
    model => Boolean(model.id?.trim())
      && model.type === modelType
      && (!model.status || model.status === 'active'),
  )
  return active.find(model => model.is_default)?.id?.trim() ?? active[0]?.id?.trim() ?? null
}

/**
 * The model a built-in agent's empty slot runs with, mirroring the backend
 * types.AgentModelSlot: the workspace's own active default of the type, else the
 * active built-in default. Never a guess: no declared default means null.
 */
export function resolveBuiltinDefaultModelId(
  models: readonly ModelDefaultCandidate[],
  modelType: string,
): string | null {
  const defaults = models.filter(
    model => model.is_default
      && model.type === modelType
      && Boolean(model.id?.trim())
      && (!model.status || model.status === 'active'),
  )
  const chosen = defaults.find(model => !model.is_builtin) ?? defaults[0]
  return chosen?.id?.trim() ?? null
}

/**
 * The composer's starting chat model. An agent without an effective model
 * starts empty ("Not configured"); otherwise the first earlier pick that is
 * still available wins, else the agent's model.
 */
export function initialChatModelId(
  agentModelId: string | null | undefined,
  earlierPicks: readonly string[],
  availableIds: readonly string[],
): string {
  if (!agentModelId) return ''
  return earlierPicks.find(id => id && availableIds.includes(id)) ?? agentModelId
}
