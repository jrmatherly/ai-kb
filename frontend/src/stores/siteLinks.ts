import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getSystemLinks } from '@/api/system'

/** Public deployment of website-docs (VitePress, base `/docs/`); AIKB_DOCS_URL overrides it. */
export const DEFAULT_DOCS_URL = 'https://ai-kb.matherly.net/docs/'

export const useSiteLinksStore = defineStore('siteLinks', () => {
  const docsUrl = ref(DEFAULT_DOCS_URL)
  const githubUrl = ref('')
  let p: Promise<void> | undefined

  const ensureLoaded = (): Promise<void> =>
    (p ??= getSystemLinks()
      .then((links) => {
        docsUrl.value = links?.docs_url || DEFAULT_DOCS_URL
        githubUrl.value = links?.github_url || ''
      })
      // The defaults stay (a backend older than this route answers 401); a later call retries.
      .catch(() => {
        p = undefined
      }))

  return { docsUrl, githubUrl, ensureLoaded }
})
