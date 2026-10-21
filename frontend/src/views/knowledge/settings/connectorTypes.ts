import { getConnectorTypes, type ConnectorMeta } from '@/api/datasource'

// Module scope, so every editor open after the first reuses the reply; a failure is dropped so the next open retries.
let connectorTypes: Promise<ConnectorMeta[]> | undefined

export function fetchConnectorTypes(): Promise<ConnectorMeta[]> {
  return (connectorTypes ??= getConnectorTypes()
    .then((res: any) => (Array.isArray(res) ? res : res?.data || []))
    .catch((err) => {
      connectorTypes = undefined
      throw err
    }))
}
