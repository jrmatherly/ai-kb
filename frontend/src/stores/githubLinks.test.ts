import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))

// Sample text, not a link the UI renders.
const allowed = new Set(['views/knowledge/settings/chunkingSamples.ts'])

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) return sources(path)
    return /\.(ts|vue)$/.test(entry.name) && !/\.test\.ts$/.test(entry.name) ? [path] : []
  })
}

// A literal repository URL skips AIKB_GITHUB_LINK_ENABLED; build it from useSiteLinksStore().githubUrl.
test('app sources hard-code no link to the AI KB repository', () => {
  const hits = sources(root).filter(path => !allowed.has(relative(root, path))).flatMap(path =>
    readFileSync(path, 'utf8').split('\n').flatMap((line, i) =>
      /github\.com\/jrmatherly\/ai-kb(?![-\w])/.test(line) ? [`${relative(root, path)}:${i + 1}`] : []))
  assert.deepEqual(hits, [])
})
