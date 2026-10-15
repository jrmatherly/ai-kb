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
 * agentModelSlot: the workspace's own default of the type, else the built-in
 * default. Never a guess: no declared default means null.
 */
export function resolveBuiltinDefaultModelId(
  models: readonly ModelDefaultCandidate[],
  modelType: string,
): string | null {
  const defaults = models.filter(model => model.is_default && model.type === modelType && model.id?.trim())
  const chosen = defaults.find(model => !model.is_builtin) ?? defaults[0]
  return chosen?.id?.trim() ?? null
}
