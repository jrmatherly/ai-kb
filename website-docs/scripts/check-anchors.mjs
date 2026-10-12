// Validate `#fragment` targets in internal markdown links against the headings of the target page.
// check-links.mjs only checks that the file exists; a renamed heading leaves the fragment dangling
// and vitepress build does not fail on it.
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs'
import { join, relative, dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const SKIP = new Set(['node_modules', 'homepage', 'shared', 'scripts', 'deploy', 'static-site', 'releases'])

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    if (SKIP.has(name) || name.startsWith('.')) continue
    const path = join(dir, name)
    if (statSync(path).isDirectory()) walk(path, out)
    else if (name.endsWith('.md')) out.push(path)
  }
  return out
}

// Same algorithm as @mdit-vue/shared slugify, which vitepress uses for heading ids.
const rControl = /[\u0000-\u001f]/g
const rSpecial = /[\s~`!@#$%^&*()\-_+=[\]{}|\\;:"'“”‘’<>,.?/]+/g
const rCombining = /[̀-ͯ]/g
export function slugify(str) {
  return str.normalize('NFKD').replace(rCombining, '').replace(rControl, '').replace(rSpecial, '-')
    .replace(/-{2,}/g, '-').replace(/^-+|-+$/g, '').replace(/^(\d)/, '_$1').toLowerCase()
}

function headingIds(file) {
  const ids = new Set()
  const counts = new Map()
  let fenced = false
  for (const raw of readFileSync(file, 'utf-8').split('\n')) {
    if (/^\s*(```|~~~)/.test(raw)) { fenced = !fenced; continue }
    if (fenced) continue
    const m = /^#{1,6}\s+(.*?)\s*$/.exec(raw)
    if (!m) continue
    let text = m[1]
    const custom = /\{#([^}]+)\}\s*$/.exec(text)
    if (custom) { ids.add(custom[1]); continue }
    text = text.replace(/`([^`]*)`/g, '$1').replace(/\[([^\]]*)\]\([^)]*\)/g, '$1').replace(/\*/g, '')
    let id = slugify(text)
    const n = counts.get(id) ?? 0
    counts.set(id, n + 1)
    if (n > 0) id = `${id}-${n}`
    ids.add(id)
  }
  return ids
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const files = walk(root)
  const cache = new Map()
  const idsOf = (f) => { if (!cache.has(f)) cache.set(f, headingIds(f)); return cache.get(f) }
  const failures = []
  let checked = 0
  for (const file of files) {
    let fenced = false
    readFileSync(file, 'utf-8').split('\n').forEach((line, index) => {
      if (/^\s*(```|~~~)/.test(line)) { fenced = !fenced; return }
      if (fenced) return
      for (const match of line.matchAll(/\[[^\]]*\]\(([^)\s]+)\)/g)) {
        const target = match[1]
        if (/^(https?:|mailto:)/.test(target) || !target.includes('#')) continue
        const [path, fragment] = target.split('#')
        const dest = path ? resolve(dirname(file), path) : file
        if (!existsSync(dest) || !dest.endsWith('.md')) continue
        checked++
        let id
        try { id = decodeURIComponent(fragment) } catch {}
        if (!idsOf(dest).has(id)) {
          failures.push({ file: relative(root, file), line: index + 1, target })
        }
      }
    })
  }
  console.log(`Checked ${checked} anchored links in ${files.length} pages, ${failures.length} unresolved`)
  for (const f of failures) console.log(`  ${f.file}:${f.line}  ${f.target}`)
  process.exit(failures.length ? 1 : 0)
}
