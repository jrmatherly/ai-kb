// Validate every ```mermaid block in the docs with mermaid's own parser.
// The site build does not check diagram syntax, and a broken diagram only errors
// in the browser, so this catches it in CI or locally first.
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { JSDOM } from 'jsdom'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')

// mermaid is a browser library, and parse() needs a DOM to initialize.
const dom = new JSDOM('<!DOCTYPE html><body></body>', { pretendToBeVisual: true })
globalThis.window = dom.window
globalThis.document = dom.window.document
// Node 22's globalThis.navigator is getter-only, so only defineProperty can override it.
Object.defineProperty(globalThis, 'navigator', {
  value: dom.window.navigator,
  configurable: true,
})

const { default: mermaid } = await import('mermaid')
mermaid.initialize({ startOnLoad: false, securityLevel: 'loose', layout: 'dagre', look: 'classic' })

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    if (['node_modules', 'homepage', 'shared', 'scripts', 'deploy', 'static-site', 'releases'].includes(name) || name.startsWith('.')) continue
    const path = join(dir, name)
    if (statSync(path).isDirectory()) walk(path, out)
    else if (name.endsWith('.md')) out.push(path)
  }
  return out
}

/** Extract each mermaid block with its start line, used to locate errors. */
function blocksOf(text) {
  const lines = text.split('\n')
  const blocks = []
  let start = -1
  let buffer = []
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (start === -1 && /^\s*```mermaid\s*$/.test(line)) {
      start = i + 1
      buffer = []
    } else if (start !== -1 && /^\s*```\s*$/.test(line)) {
      blocks.push({ line: start + 1, code: buffer.join('\n') })
      start = -1
    } else if (start !== -1) {
      buffer.push(line)
    }
  }
  return blocks
}

let total = 0
const failures = []

for (const file of walk(root)) {
  for (const block of blocksOf(readFileSync(file, 'utf-8'))) {
    total++
    try {
      await mermaid.parse(block.code)
    } catch (error) {
      failures.push({
        file: relative(root, file),
        line: block.line,
        message: String(error?.message ?? error).split('\n').slice(0, 6).join('\n'),
      })
    }
  }
}

console.log(`Checked ${total} mermaid diagrams, ${failures.length} failed`)
for (const failure of failures) {
  console.log(`\n--- ${failure.file}:${failure.line}\n${failure.message}`)
}
process.exit(failures.length === 0 ? 0 : 1)
