// Check relative links between docs pages. Splitting a section easily leaves links
// to moved content, and vitepress build does not fail on dead links.
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs'
import { join, relative, dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    if (['node_modules', 'homepage', 'shared', 'scripts', 'deploy', 'static-site', 'releases'].includes(name) || name.startsWith('.')) continue
    const path = join(dir, name)
    if (statSync(path).isDirectory()) walk(path, out)
    else if (name.endsWith('.md')) out.push(path)
  }
  return out
}

const files = walk(root)
const failures = []
let checked = 0

for (const file of files) {
  const lines = readFileSync(file, 'utf-8').split('\n')
  lines.forEach((line, index) => {
    for (const match of line.matchAll(/\[[^\]]*\]\(([^)\s]+)\)/g)) {
      const target = match[1]
      if (/^(https?:|#|mailto:)/.test(target)) continue
      const [path] = target.split('#')
      if (!path || !path.endsWith('.md')) continue
      checked++
      if (!existsSync(resolve(dirname(file), path))) {
        failures.push({ file: relative(root, file), line: index + 1, target })
      }
    }
  })
}

console.log(`Checked ${checked} internal links in ${files.length} pages, ${failures.length} broken`)
for (const failure of failures) {
  console.log(`  ${failure.file}:${failure.line} -> ${failure.target}`)
}
process.exit(failures.length === 0 ? 0 : 1)
