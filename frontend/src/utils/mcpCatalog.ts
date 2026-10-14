import type { MCPMetadata, MCPService } from '@/api/mcp-service'

// The list endpoint derives catalog.tool_count from the stored tools array, so
// a refresh response carries everything the service card shows.
export function applyMetadataRefresh(services: MCPService[], saved: MCPMetadata) {
  const service = services.find((s) => s.id === saved.service_id)
  if (!service) return
  service.catalog = { tool_count: saved.tools.length, stale: saved.stale, synced_at: saved.synced_at }
}
