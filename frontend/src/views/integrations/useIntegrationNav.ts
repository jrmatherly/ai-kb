import { useRouter } from 'vue-router'
import { useUIStore } from '@/stores/ui'

export function useIntegrationNav() {
  const router = useRouter()
  const uiStore = useUIStore()

  const openExternal = (url: string) => {
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  const openApiSettings = () => {
    router.push({ path: '/platform/settings', query: { section: 'integration-api' } })
    uiStore.openSettings('integration-api')
  }

  return { openExternal, openApiSettings }
}
