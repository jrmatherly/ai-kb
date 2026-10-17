#!/usr/bin/env node
// Renders every AI KB brand asset from the marks below.
//
//   node scripts/brand/generate.mjs                  write SVG sources and every PNG/ICO in OUTPUTS
//   node scripts/brand/generate.mjs --compare <dir> [ref.svg]  write <dir>/index.html comparing all marks
//
// Needs Google Chrome (or CHROME=/path/to/chrome) to rasterize and ImageMagick `magick` to pack ICO files
// and drop alpha. The wordmark is live text in 'Helvetica Neue' Bold, so PNGs match byte for byte only
// where that font is installed (macOS). The marks share the ai-workspace logo system: 24-unit grid,
// black squircle tile (rx 5.3), white 3.8-unit round strokes, one 4.2-unit accent square.
import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '../..');

const ACCENTS = {
  teal: '#2FE3C4',
  amber: '#FFB13D',
  violet: '#9B8CFF',
  coral: '#FF6B57',
};

const STROKE = 'fill="none" stroke-width="3.8" stroke-linecap="round" stroke-linejoin="round"';
const square = (x, y, accent) => `<rect x="${x}" y="${y}" width="4.2" height="4.2" rx="1.1" fill="${accent}"/>`;

const MARKS = {
  // A K from the core mark's strokes: stem plus a sideways core chevron, the accent square in its mouth.
  kay: {
    label: 'Kay',
    body: ({ fg, accent }) =>
      `<path d="M6.5 4.5V19.5M18 4.5L7 12L18 19.5" stroke="${fg}" ${STROKE}/>${square(15, 9.9, accent)}`,
  },
  // A page whose highlighted passage is the retrieved, cited chunk.
  leaf: {
    label: 'Leaf',
    body: ({ fg, accent }) =>
      `<path d="M14 4.4H7.4a1.9 1.9 0 0 0-1.9 1.9v11.4a1.9 1.9 0 0 0 1.9 1.9h9.2a1.9 1.9 0 0 0 1.9-1.9V8.9Z" stroke="${fg}" ${STROKE}/>` +
      square(10.5, 10.7, accent),
  },
  // A citation: brackets around the source.
  cite: {
    label: 'Cite',
    body: ({ fg, accent }) =>
      `<path d="M9 4.8H5.8V19.2H9M15 4.8H18.2V19.2H15" stroke="${fg}" ${STROKE}/>${square(9.9, 9.9, accent)}`,
  },
};

const DEFAULT = { mark: 'kay', accent: 'amber' };

const TILE = '#000000';
const INK = { light: '#000000', dark: '#FFFFFF' };
const FONT = "'Helvetica Neue', Helvetica, Arial, sans-serif";

// The tile and mark on the 24-unit grid; `inset` shrinks the tile inside the canvas (macOS app icon margin).
const tiled = (mark = DEFAULT.mark) =>
  `<rect width="24" height="24" rx="5.3" fill="${TILE}"/>` + MARKS[mark].body({ fg: '#FFFFFF', accent: ACCENTS[DEFAULT.accent] });

function icon({ mark, size = 24, inset = 0 } = {}) {
  const g = `<g transform="translate(${inset} ${inset}) scale(${(24 - 2 * inset) / 24})">${tiled(mark)}</g>`;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24">${g}</svg>`;
}

// Icon plus "AI KB", centered in a w x h canvas. Content box is 900 x 216 units.
function lockup({ theme = 'light', w = 900, h = 216 } = {}) {
  const CW = 900, CH = 216;
  const s = Math.min((w * 0.96) / CW, (h * 0.92) / CH);
  const tx = (w - CW * s) / 2, ty = (h - CH * s) / 2;
  const ic = `<g transform="scale(9)">${tiled()}</g>`;
  const text = `<text x="268" y="186" font-family="${FONT}" font-weight="700" font-size="216" textLength="632" ` +
    `lengthAdjust="spacingAndGlyphs" fill="${INK[theme]}">AI KB</text>`;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}" role="img" aria-label="AI KB">` +
    `<g transform="translate(${tx} ${ty}) scale(${s})">${ic}${text}</g></svg>`;
}

// Every committed asset: path, the SVG it is rendered from, and how it is packed.
const OUTPUTS = [
  { path: 'docs/images/brand/ai-kb-icon.svg', svg: () => icon({ size: 512 }) },
  { path: 'docs/images/brand/ai-kb-lockup-light.svg', svg: () => lockup({ theme: 'light' }) },
  { path: 'docs/images/brand/ai-kb-lockup-dark.svg', svg: () => lockup({ theme: 'dark' }) },
  { path: 'docs/images/brand/ai-kb-icon-512.png', svg: () => icon({ size: 512 }) },
  { path: 'frontend/public/favicon.ico', ico: [16, 32] },
  { path: 'website-docs/public/favicon.ico', ico: [16, 32] },
  // Apple's icon grid: an 824 tile on a 1024 canvas.
  { path: 'cmd/desktop/build/appicon.png', svg: () => icon({ size: 1024, inset: 2.34375 }) },
  // The README logo keeps upstream's white 945 x 650 canvas.
  { path: 'docs/images/logo.png', svg: () => bandOnCanvas(), opaque: true },
  { path: 'website-docs/homepage/public/docs/_home/brand/ai-kb-lockup-light.png', svg: () => lockup({ theme: 'light', w: 945, h: 238 }) },
  { path: 'website-docs/homepage/public/docs/_home/brand/ai-kb-lockup-dark.png', svg: () => lockup({ theme: 'dark', w: 945, h: 238 }) },
  { path: 'docs/images/readme/src/wordmark-light.png', svg: () => lockup({ theme: 'light', w: 904, h: 208 }) },
  { path: 'docs/images/readme/src/wordmark-dark.png', svg: () => lockup({ theme: 'dark', w: 904, h: 208 }) },
  { path: 'misc/intro-video/assets/logo-light.png', svg: () => lockup({ theme: 'light', w: 945, h: 238 }) },
  { path: 'misc/intro-video/assets/logo-dark.png', svg: () => lockup({ theme: 'dark', w: 945, h: 238 }) },
  { path: 'frontend/src/assets/img/aikb.png', svg: () => lockup({ theme: 'light', w: 400, h: 126 }) },
  { path: 'frontend/src/assets/img/aikb-dark.png', svg: () => lockup({ theme: 'dark', w: 400, h: 126 }) },
  { path: 'extension/public/icons/16.png', svg: () => icon({ size: 16 }) },
  { path: 'extension/public/icons/32.png', svg: () => icon({ size: 32 }) },
  { path: 'extension/public/icons/48.png', svg: () => icon({ size: 48 }) },
  { path: 'extension/public/icons/128.png', svg: () => icon({ size: 128 }) },
];

function bandOnCanvas() {
  const band = lockup({ theme: 'light', w: 945, h: 238 }).replace(/^<svg[^>]*>|<\/svg>$/g, '');
  return `<svg xmlns="http://www.w3.org/2000/svg" width="945" height="650" viewBox="0 0 945 650">` +
    `<rect width="945" height="650" fill="#FFFFFF"/><g transform="translate(0 208)">${band}</g></svg>`;
}

function chrome() {
  const found = [process.env.CHROME, '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', '/usr/bin/google-chrome', '/usr/bin/chromium']
    .find((p) => p && existsSync(p));
  if (!found) throw new Error('Chrome not found; set CHROME=/path/to/chrome');
  return found;
}

// No timestamps in the output, so a rerun with unchanged marks leaves git clean.
const DETERMINISTIC = ['-strip', '-define', 'png:exclude-chunks=date,time'];
const TMP = mkdtempSync(join(tmpdir(), 'ai-kb-brand-'));

function rasterize(svg, out) {
  const [, w, h] = svg.match(/width="(\d+)" height="(\d+)"/);
  const src = join(TMP, 'in.html');
  writeFileSync(src, `<!doctype html><style>html,body{margin:0;background:transparent}svg{display:block}</style>${svg}`);
  execFileSync(chrome(), ['--headless', '--disable-gpu', '--hide-scrollbars', '--force-device-scale-factor=1',
    '--default-background-color=00000000', `--window-size=${w},${h}`, `--screenshot=${out}`, `file://${src}`], { stdio: 'ignore' });
  if (!existsSync(out)) throw new Error(`Chrome wrote no ${out}`);
  execFileSync('magick', [out, '-crop', `${w}x${h}+0+0`, '+repage', ...DETERMINISTIC, out]);
}

function write(o) {
  const dest = join(ROOT, o.path);
  mkdirSync(dirname(dest), { recursive: true });
  if (o.path.endsWith('.svg')) return writeFileSync(dest, o.svg() + '\n');
  if (o.ico) {
    const pngs = o.ico.map((n) => { const p = join(TMP, `ico-${n}.png`); rasterize(icon({ size: n }), p); return p; });
    return execFileSync('magick', [...pngs, '-type', 'TrueColorAlpha', dest]);
  }
  rasterize(o.svg(), dest);
  if (o.opaque) execFileSync('magick', [dest, '-background', 'white', '-alpha', 'remove', '-alpha', 'off', ...DETERMINISTIC, `PNG24:${dest}`]);
}

// `ref` is an optional SVG file shown as the last row, for comparing against a sibling product's mark.
function compare(dir, ref) {
  mkdirSync(dir, { recursive: true });
  const sizes = [16, 32, 64, 512];
  const row = (name, svg) => `<tr><th>${name}</th>${sizes.map((n) =>
    `<td><img width="${n}" height="${n}" src="data:image/svg+xml;base64,${Buffer.from(svg).toString('base64')}"></td>`).join('')}</tr>`;
  const table = (theme) => `<section class="${theme}"><h2>${theme}</h2><table><tr><th></th>${sizes.map((n) => `<th>${n}px</th>`).join('')}</tr>` +
    Object.entries(MARKS).map(([id, m]) => row(`${m.label}${id === DEFAULT.mark ? ' (applied)' : ''}`, icon({ mark: id, size: 512 }))).join('') +
    (ref ? row(ref.split('/').pop(), readFileSync(ref, 'utf8')) : '') + '</table>' +
    `<img class="lockup" src="data:image/svg+xml;base64,${Buffer.from(lockup({ theme })).toString('base64')}"></section>`;
  writeFileSync(join(dir, 'index.html'), `<!doctype html><meta charset="utf-8"><title>AI KB logo candidates</title>
<style>body{font:14px system-ui;margin:0}section{padding:24px}.light{background:#fdfcfa;color:#101f38}.dark{background:#0b1220;color:#e6ecf5}
table{border-collapse:collapse}td,th{padding:10px 18px;text-align:center;vertical-align:middle}th:first-child{text-align:left}
img{image-rendering:auto}.lockup{width:360px;margin-top:16px}</style>
<section><h1>AI KB marks</h1><p>Default accent: ${DEFAULT.accent} ${ACCENTS[DEFAULT.accent]}. Switch with DEFAULT in scripts/brand/generate.mjs.</p></section>
${table('light')}${table('dark')}`);
}

try {
  const i = process.argv.indexOf('--compare');
  if (i > 0) compare(resolve(process.argv[i + 1]), process.argv[i + 2]);
  else for (const o of OUTPUTS) { write(o); console.log('wrote', o.path); }
} finally {
  rmSync(TMP, { recursive: true, force: true });
}
