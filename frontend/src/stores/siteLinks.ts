import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getSystemLinks } from '@/api/system'

/** Public deployment of website-docs (VitePress, base `/docs/`); AIKB_DOCS_URL overrides it. */
export const DEFAULT_DOCS_URL = 'https://ai-kb.matherly.net/docs/'

export const useSiteLinksStore = defineStore('siteLinks', () => {
  const docsUrl = ref(DEFAULT_DOCS_URL)
  const githubUrl = ref('')
  const loaded = ref(false)
  let loadingPromise: Promise<void> | null = null

  const ensureLoaded = async (): Promise<void> => {
    if (loaded.value) return
    if (loadingPromise) return loadingPromise
    loadingPromise = (async () => {
      try {
        const links = await getSystemLinks()
        // An older backend has no such route; keep the defaults.
        if (typeof links?.docs_url === 'string' && links.docs_url) docsUrl.value = links.docs_url
        githubUrl.value = typeof links?.github_url === 'string' ? links.github_url : ''
      } catch {
        docsUrl.value = DEFAULT_DOCS_URL
        githubUrl.value = ''
      } finally {
        loaded.value = true
        loadingPromise = null
      }
    })()
    return loadingPromise
  }

  return { docsUrl, githubUrl, loaded, ensureLoaded }
})
