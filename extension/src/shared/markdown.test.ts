import { test } from 'node:test';
import assert from 'node:assert/strict';
import { JSDOM } from 'jsdom';

const dom = new JSDOM('');
Object.assign(globalThis, { window: dom.window, document: dom.window.document });
const { renderMarkdown } = await import('./markdown');

const parse = (md: string) => {
  const el = dom.window.document.createElement('div');
  el.innerHTML = renderMarkdown(md);
  return el;
};

test('renders a heading', () => {
  assert.equal(parse('# Hi').querySelector('h1')?.textContent, 'Hi');
});

test('drops script elements', () => {
  assert.equal(parse('<script>alert(1)</script>').querySelector('script'), null);
});

test('drops javascript: links', () => {
  const a = parse('[x](javascript:alert(1))').querySelector('a');
  assert.ok(a);
  assert.ok(!(a.getAttribute('href') ?? '').startsWith('javascript:'));
});

test('drops inline event handlers', () => {
  assert.equal(parse('<img src=x onerror=alert(1)>').querySelector('[onerror]'), null);
});

test('storage-scheme images become placeholders the panel resolves', () => {
  const img = parse('![a](minio://b/k.png)').querySelector('img');
  assert.equal(img?.getAttribute('data-storage-ref'), 'minio://b/k.png');
  assert.equal(img?.getAttribute('src'), '');
});

test('remote images become click-to-load buttons', () => {
  const body = parse('![a](https://e.com/i.png)');
  assert.equal(body.querySelector('img'), null);
  const button = body.querySelector('button.aikb-remote-image');
  assert.equal(button?.getAttribute('data-src'), 'https://e.com/i.png');
  assert.equal(button?.textContent, 'Load image from e.com');
});

test('links open in a new tab without an opener', () => {
  const a = parse('[x](https://e.com/)').querySelector('a');
  assert.equal(a?.getAttribute('target'), '_blank');
  assert.equal(a?.getAttribute('rel'), 'noopener');
});

test('renders a table', () => {
  assert.ok(parse('| a | b |\n| - | - |\n| 1 | 2 |').querySelector('table'));
});

test('raw HTML images with a storage src become placeholders too', () => {
  const img = parse('<img src="minio://b/k.png">').querySelector('img');
  assert.equal(img?.getAttribute('data-storage-ref'), 'minio://b/k.png');
});

test('raw HTML remote images become click-to-load buttons with their src', () => {
  const button = parse('<img src="https://e.com/raw.png">').querySelector('button.aikb-remote-image');
  assert.equal(button?.getAttribute('data-src'), 'https://e.com/raw.png');
});

test('images with a non-http scheme render no button and no data-src', () => {
  const body = parse('![x](javascript:alert(1))');
  assert.equal(body.querySelector('button'), null);
  assert.equal(body.querySelector('[data-src]'), null);
});

test('anchor and mailto links keep their href', () => {
  const links = [...parse('[a](#top) [m](mailto:a@b.c)').querySelectorAll('a')].map((a) => a.getAttribute('href'));
  assert.deepEqual(links, ['#top', 'mailto:a@b.c']);
});

test('every provider scheme and the storage backend form count as storage refs', () => {
  for (const src of ['oss://b/k.png', 'resource://x/y.png', 'storage://backend-1/minio://b/k.png']) {
    assert.equal(parse(`![a](${src})`).querySelector('img')?.getAttribute('data-storage-ref'), src, src);
  }
});

test('the button carries an explicit type attribute', () => {
  assert.equal(parse('![a](https://e.com/i.png)').querySelector('button')?.getAttribute('type'), 'button');
});

test('same-page anchors stay in the page', () => {
  const a = parse('[1](#cite-a1-1)').querySelector('a');
  assert.equal(a?.getAttribute('href'), '#cite-a1-1');
  assert.equal(a?.getAttribute('target'), null);
  assert.equal(a?.getAttribute('rel'), null);
});

test('an http image URL that does not parse falls back to its alt text instead of throwing', () => {
  assert.equal(renderMarkdown('![x](https://)'), '<p>x</p>\n');
  assert.equal(parse('<img src="https://[x" alt="y">').textContent, 'y');
});

// Each of these fetches a third-party URL on render without a click.
const REMOTE_LOADS = [
  '<p style="background:url(https://evil.example/px)">x</p>',
  '<picture><source srcset="https://evil.example/a.png"><img src="data:image/png;base64,AA"></picture>',
  '<video poster="https://evil.example/p.png" src="https://evil.example/v.mp4"></video>',
  '<audio src="https://evil.example/a.mp3"></audio>',
  '<video><track src="https://evil.example/t.vtt"></video>',
  '<img src="data:image/png;base64,AA" srcset="https://evil.example/2x.png 2x">',
  '<svg><image href="https://evil.example/s.png"></image></svg>',
  '<input type="image" src="https://evil.example/i.png">',
  '<table background="https://evil.example/bg.png"><tr><td>x</td></tr></table>',
  '<link rel="stylesheet" href="https://evil.example/s.css">',
  '<form action="https://evil.example/f"><input name="q"><button>Go</button></form>',
];

test('no markup fetches a remote URL on render', () => {
  for (const html of REMOTE_LOADS) assert.ok(!renderMarkdown(html).includes('evil.example'), html);
});

test('an inline image keeps its src but loses a supplied storage ref', () => {
  const img = parse('<img src="data:image/png;base64,AA" data-storage-ref="../../etc/passwd">').querySelector('img');
  assert.equal(img?.getAttribute('src'), 'data:image/png;base64,AA');
  assert.equal(img?.getAttribute('data-storage-ref'), null);
});

test('a click-to-load button cannot come from the input', () => {
  const body = parse('<button class="aikb-remote-image" data-src="https://evil.example/t.png">Load image from kb.example.com</button>');
  assert.equal(body.querySelector('button'), null);
  assert.equal(body.textContent?.trim(), 'Load image from kb.example.com');
});

test('GFM task lists keep their checkboxes', () => {
  const box = parse('- [x] done').querySelector('input');
  assert.deepEqual([box?.getAttribute('type'), box?.hasAttribute('checked')], ['checkbox', true]);
});
