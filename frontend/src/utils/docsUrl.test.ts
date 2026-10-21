import { strict as assert } from 'node:assert'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { after, before, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createPinia, setActivePinia } from 'pinia'
import { createServer, type ViteDevServer } from 'vite'

const WEBSITE_DOCS = resolve(dirname(fileURLToPath(import.meta.url)), '../../../website-docs')

type LinksReply = () => Promise<unknown>
const globals = globalThis as unknown as { __systemLinksReply?: LinksReply }

let server: ViteDevServer
let docs: typeof import('./docsUrl')
let useSiteLinksStore: typeof import('../stores/siteLinks').useSiteLinksStore

before(async () => {
  server = await createServer({
    configFile: false,
    plugins: [{ name: 'stub-request', enforce: 'pre',
      resolveId(id) { if (id.endsWith('/utils/request')) return '\0stub-request' },
      load(id) {
        if (id === '\0stub-request') {
          return 'const request = (url) => url === "/api/v1/system/links" ? globalThis.__systemLinksReply() : Promise.reject(new Error(url)); export { request as get, request as post, request as put, request as del, request as patch, request as postUpload };'
        }
      },
    }],
    optimizeDeps: { noDiscovery: true, entries: [] },
    resolve: { alias: { '@': fileURLToPath(new URL('../', import.meta.url)) } },
    server: { middlewareMode: true, hmr: false }, appType: 'custom',
  })
  docs = await server.ssrLoadModule('/src/utils/docsUrl.ts') as typeof docs
  ;({ useSiteLinksStore } = await server.ssrLoadModule('/src/stores/siteLinks.ts'))
})
after(async () => {
  await server?.close()
  delete globals.__systemLinksReply
})

// VitePress default slugify for the headings we link to: explicit `{#id}`
// wins, otherwise lowercase with whitespace and punctuation collapsed to `-`.
function headingIds(markdown: string): Set<string> {
  const ids = new Set<string>()
  for (const line of markdown.split('\n')) {
    const m = /^#{1,6}\s+(.*?)\s*$/.exec(line)
    if (!m) continue
    const explicit = /\{#([^}]+)\}$/.exec(m[1])
    if (explicit) {
      ids.add(explicit[1])
      continue
    }
    ids.add(
      m[1]
        .toLowerCase()
        .replace(/[\s~`!@#$%^&*()+=[\]{}|\\;:'",.<>/?]+/g, '-')
        .replace(/^-+|-+$/g, ''),
    )
  }
  return ids
}

test('every linked doc page and anchor exists in website-docs', () => {
  for (const [key, target] of Object.entries(docs.DOC_PAGES)) {
    const [page, anchor] = target.split('#')
    const file = resolve(WEBSITE_DOCS, page ? `${page}.md` : 'index.md')
    assert.ok(existsSync(file), `${key}: ${file} not found`)
    if (anchor) {
      assert.ok(headingIds(readFileSync(file, 'utf8')).has(anchor), `${key}: #${anchor} not found in ${file}`)
    }
  }
})

test('docsUrl falls back to the public docs site before the links load', () => {
  assert.equal(docs.docsUrl('home'), 'https://ai-kb.matherly.net/docs/')
  setActivePinia(createPinia())
  assert.equal(docs.docsUrl('models'), 'https://ai-kb.matherly.net/docs/03-features/06-models')
})

test('docsUrl follows the deployment docs URL once loaded', async () => {
  globals.__systemLinksReply = async () => ({
    docs_url: 'https://kb.example.com/docs/',
    github_url: 'https://github.com/jrmatherly/ai-kb',
  })
  setActivePinia(createPinia())
  const links = useSiteLinksStore()
  await links.ensureLoaded()
  assert.equal(docs.docsUrl('models'), 'https://kb.example.com/docs/03-features/06-models')
  assert.equal(links.githubUrl, 'https://github.com/jrmatherly/ai-kb')
  assert.equal(links.loaded, true)
})

test('a failed links request keeps the default docs and no GitHub link', async () => {
  globals.__systemLinksReply = async () => { throw new Error('404') }
  setActivePinia(createPinia())
  const links = useSiteLinksStore()
  await links.ensureLoaded()
  assert.equal(docs.docsUrl('home'), 'https://ai-kb.matherly.net/docs/')
  assert.equal(links.githubUrl, '')
  assert.equal(links.loaded, true)
})
