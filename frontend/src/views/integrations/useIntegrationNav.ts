import { useRouter } from 'vue-router'
import { useUIStore } from '@/stores/ui'

export function useIntegrationNav() {
  const router = useRouter()
  const uiStore = useUIStore()

  const openExternal = (url: string) => {
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  const openIntegrationTab = (tab: 'api' | 'cli' | 'mcpserver') => {
    const section = `integration-${tab}`
    router.push({ path: '/platform/settings', query: { section } })
    uiStore.openSettings(section)
  }

  const openApiSettings = () => openIntegrationTab('api')

  return { openExternal, openApiSettings, openIntegrationTab }
}
