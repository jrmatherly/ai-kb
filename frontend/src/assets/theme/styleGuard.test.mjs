import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

/**
 * Style guard (ratchet): counts patterns in views/ components/ assets/ that bypass design tokens or fight TDesign,
 * and only lets the counts go down. Any new occurrence fails the test; after a cleanup, lower the baseline.
 *
 * The tokens are defined in assets/theme/theme.css (--app-radius-* / --app-text-* / --app-space-* /
 * --app-motion-* / --z-*); global TDesign overrides live in assets/theme/tdesign-overrides.less.
 * It is .mjs because `npm test` (tsx --test) only discovers .mjs test files automatically under Node 20.
 */
const SRC_ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const SCAN_DIRS = ['views', 'components', 'assets']
const EXTS = new Set(['.vue', '.less', '.css'])
const EXEMPT_FILES = new Set(['assets/theme/theme.css'])

const RULES = [
  {
    name: 'brand-rgba',
    why: 'For a translucent brand color use color-mix(in srgb, var(--td-brand-color) N%, transparent), otherwise dark mode does not follow',
    pattern: /rgba\(\s*7\s*,\s*192\s*,\s*95\s*,/g,
    baseline: 0,
  },
  {
    name: 'legacy-blue-rgba',
    why: 'The legacy TDesign blue rgba(0,82,217) is no longer the brand color',
    pattern: /rgba\(\s*0\s*,\s*82\s*,\s*217\s*,/g,
    baseline: 18,
  },
  {
    name: 'td-token-fallback',
    why: 'theme.css guarantees the --td-* tokens exist, so a fallback never applies and easily carries the wrong color',
    pattern: /var\(\s*--td-(?!purple-5|cyan-6|font-family-code)[A-Za-z0-9-]+\s*,/g,
    baseline: 0,
  },
  {
    name: 'radius-literal',
    why: 'Use var(--app-radius-xs|sm|md|lg|xl|pill) (4/6/8/10/12/999px) for radii',
    pattern: /border(?:-[a-z]+)*-radius\s*:\s*\d+(?:\.\d+)?px/g,
    baseline: 162,
  },
  {
    name: 'font-size-literal',
    why: 'Use var(--app-text-2xs ... 4xl) (10 to 24px) for font sizes',
    pattern: /font-size\s*:\s*\d+(?:\.\d+)?px/g,
    baseline: 28,
  },
  {
    name: 'motion-literal',
    why: 'Use var(--app-motion-instant|fast|base|slow) (120/150/200/300ms) for transition durations',
    pattern: /transition[^;{]*?(?<![\d.])(?:0?\.\d+s|\d+ms)/g,
    baseline: 65,
  },
  {
    name: 'transition-all',
    why: 'transition: all animates unrelated properties too (layout properties included); list the specific properties',
    pattern: /transition\s*:\s*all\b/g,
    baseline: 78,
  },
  {
    name: 'important',
    why: '!important usually means fighting TDesign or your own styles; global intent overrides go in tdesign-overrides.less',
    pattern: /!important/g,
    baseline: 510,
  },
  {
    name: 'z-index-important',
    why: 'z-index should not win through !important; use t-popup attach="body" or the --z-* layer tokens',
    pattern: /z-index\s*:\s*-?\d+\s*!important/g,
    baseline: 26,
  },
  {
    name: 'raster-icon',
    why: 'Replace the more.png / circle.png bitmap icons with t-icon',
    pattern: /(more|circle)\.png/g,
    baseline: 10,
  },
]

function* walk(dir) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (name === 'node_modules') continue
    if (statSync(full).isDirectory()) {
      yield* walk(full)
    } else if (EXTS.has(full.slice(full.lastIndexOf('.'))) && !/\.test\./.test(name)) {
      yield full
    }
  }
}

function countMatches(rule) {
  const byFile = new Map()
  let total = 0
  for (const dir of SCAN_DIRS) {
    for (const file of walk(join(SRC_ROOT, dir))) {
      const rel = relative(SRC_ROOT, file).replaceAll('\\', '/')
      if (EXEMPT_FILES.has(rel)) continue
      const text = readFileSync(file, 'utf8')
      const n = (text.match(rule.pattern) ?? []).length
      if (n > 0) {
        byFile.set(rel, n)
        total += n
      }
    }
  }
  return { total, byFile }
}

for (const rule of RULES) {
  test(`style guard: ${rule.name} does not exceed baseline (${rule.baseline})`, () => {
    const { total, byFile } = countMatches(rule)
    const top = [...byFile.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 8)
      .map(([f, n]) => `  ${n}\t${f}`)
      .join('\n')
    assert.ok(
      total <= rule.baseline,
      `${rule.name}: ${total} occurrences > baseline ${rule.baseline}.\n${rule.why}\n${top}`,
    )
  })
}
