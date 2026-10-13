import { computed, onMounted, ref } from 'vue'
import { useChatResourcesStore } from '@/stores/chatResources'
import { evaluateTenantModelReadiness } from '@/utils/tenantModelReadiness'

/**
 * Whether the current space has the models needed to create a knowledge base or an agent.
 *
 * Derived straight from the chatResources model snapshot: adding or removing a model in settings calls `replaceModels`,
 * so this computed updates with it and needs no indirect signal such as "settings dialog closed" to refetch.
 */
export function useTenantModelReadiness() {
  const chatResources = useChatResourcesStore()
  const loaded = ref(false)
  const loading = ref(false)

  const readiness = computed(() => evaluateTenantModelReadiness(chatResources.allModels))

  const refresh = async (force = false) => {
    loading.value = true
    try {
      await chatResources.ensureModels(force)
    } finally {
      loading.value = false
      loaded.value = true
    }
  }

  onMounted(() => {
    refresh()
  })

  const isReadyForDocumentKb = computed(() => readiness.value.isReadyForDocumentKb)

  const isReadyForAgent = computed(() => readiness.value.isReadyForAgent)

  const hasChat = computed(() => readiness.value.hasChat)

  return {
    readiness,
    loaded,
    loading,
    refresh,
    isReadyForDocumentKb,
    isReadyForAgent,
    hasChat,
  }
}
