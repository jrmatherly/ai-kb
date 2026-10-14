import assert from 'node:assert/strict'
import test from 'node:test'
import { applyMetadataRefresh } from './mcpCatalog.ts'

const tool = (name: string) => ({ name, description: '', inputSchema: {} })

test('a refresh clears the stale catalog on the matching service only', () => {
  const services = [
    { id: 'a', name: 'A', catalog: { tool_count: 3, stale: true, synced_at: '2026-10-14T10:00:00Z' } },
    { id: 'b', name: 'B', catalog: { tool_count: 2, stale: true, synced_at: '2026-10-14T09:00:00Z' } },
  ]
  applyMetadataRefresh(services as any, {
    service_id: 'a',
    tools: [tool('x'), tool('y')],
    instructions: '',
    server_name: '',
    server_version: '',
    server_description: '',
    synced_at: '2026-10-14T11:00:00Z',
    stale: false,
  })
  assert.deepEqual(services[0].catalog, { tool_count: 2, stale: false, synced_at: '2026-10-14T11:00:00Z' })
  assert.deepEqual(services[1].catalog, { tool_count: 2, stale: true, synced_at: '2026-10-14T09:00:00Z' })
})

test('a first sync gives a never-synced service its catalog', () => {
  const services = [{ id: 'a', name: 'A' }]
  applyMetadataRefresh(services as any, {
    service_id: 'a',
    tools: [tool('x')],
    instructions: '',
    server_name: '',
    server_version: '',
    server_description: '',
    synced_at: '2026-10-14T11:00:00Z',
    stale: false,
  })
  assert.deepEqual((services[0] as any).catalog, { tool_count: 1, stale: false, synced_at: '2026-10-14T11:00:00Z' })
})
