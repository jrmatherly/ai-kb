import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) return sources(path)
    return /\.(ts|vue)$/.test(entry.name) && !/\.test\.ts$/.test(entry.name) ? [path] : []
  })
}

// en-US is the only UI locale; a removed locale left as a formatter or header fallback leaks its formats.
test('app sources name no removed UI locale as a fallback', () => {
  const hits = sources(root).flatMap(path =>
    readFileSync(path, 'utf8').split('\n').flatMap((line, i) =>
      /['"]zh-CN['"]/.test(line) ? [`${relative(root, path)}:${i + 1}`] : []))
  assert.deepEqual(hits, [])
})
