import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getDeploymentCapabilities } from '@/api/system'
import { useAuthStore } from '@/stores/auth'
import {
  isDeploymentCapabilitySupported,
  type DeploymentCapabilityKey,
  type DeploymentCapabilityMap,
} from '@/config/deploymentCapabilities'

export const useDeploymentCapabilitiesStore = defineStore('deploymentCapabilities', () => {
  const edition = ref('')
  const capabilities = ref<DeploymentCapabilityMap>({})
  const imPlatforms = ref<string[] | undefined>(undefined)
  const loaded = ref(false)
  const loadError = ref('')
  let loadingPromise: Promise<void> | null = null

  const ensureLoaded = async (force = false): Promise<void> => {
    if (loaded.value && !force) return
    if (loadingPromise) return loadingPromise

    loadingPromise = (async () => {
      try {
        const response = await getDeploymentCapabilities()
        edition.value = response.data?.edition || ''
        capabilities.value = response.data?.capabilities || {}
        imPlatforms.value = response.data?.im_platforms
        loadError.value = ''
      } catch (error) {
        // Fail open when the capability probe fails; the backend routes still enforce permissions.
        capabilities.value = {}
        imPlatforms.value = undefined
        loadError.value = error instanceof Error ? error.message : String(error)
      } finally {
        loaded.value = true
        loadingPromise = null
      }
    })()

    return loadingPromise
  }

  const isSupported = (key?: DeploymentCapabilityKey) => {
    const authStore = useAuthStore()
    return isDeploymentCapabilitySupported(capabilities.value, key, {
      liteMode: authStore.isLiteMode,
      edition: edition.value,
    })
  }

  return {
    edition,
    capabilities,
    imPlatforms,
    loaded,
    loadError,
    ensureLoaded,
    isSupported,
  }
})
