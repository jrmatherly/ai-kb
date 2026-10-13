/**
 * Normalizes the knowledge base scope the API returns into an array the frontend form can use safely.
 *
 * @param ids The API key's knowledge base IDs; for a fully authorized key the server may return null.
 * @returns A new array of knowledge base IDs; null or undefined returns an empty array, meaning all knowledge bases.
 */
export function normalizeAPIKeyKnowledgeBaseIDs(
  ids: readonly string[] | null | undefined,
): string[] {
  return ids ? [...ids] : []
}
