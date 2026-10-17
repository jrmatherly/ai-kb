import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { JSDOM } from 'jsdom';
import { parseHTML } from 'linkedom';

const read = (name: string) => readFileSync(new URL(`../../test/fixtures/${name}`, import.meta.url), 'utf8');

// Defuddle's selectors exceed jsdom's hard-coded cap, so its tests run on linkedom, the DOM Defuddle is
// tested against. The picker tests need Range, Selection and real events, which jsdom provides.
const linked = (name: string, url: string) => {
  const { document } = parseHTML(read(name));
  Object.defineProperty(document, 'location', { value: { href: url } });
  return document as unknown as Document;
};
const fixture = (name: string, url: string) => new JSDOM(read(name), { url });

// Defuddle's Markdown converter reads the global document, as a content script would.
const base = parseHTML('<!doctype html><html><body></body></html>');
Object.assign(globalThis, { window: base.window, document: base.document });

const { extractArticle, extractSelect, htmlToMarkdown, startRegionPicker } = await import('./extract');

test('extractArticle keeps the body, drops the chrome, and leaves the live page alone', () => {
  const doc = linked('article.html', 'https://fixture.example/post');
  const draft = extractArticle(doc);
  assert.equal(draft.kind, 'article');
  assert.equal(draft.title, 'Fixture Title');
  assert.equal(draft.url, 'https://fixture.example/post');
  assert.ok(draft.markdown.includes('Body paragraph one.'), draft.markdown);
  assert.ok(!draft.markdown.includes('Skip to content'), draft.markdown);
  assert.equal(draft.meta.author, 'Fixture Author');
  assert.ok(doc.querySelector('nav'), 'the live nav survived');
});

test('htmlToMarkdown converts inline markup', () => {
  assert.equal(htmlToMarkdown('<p>a <strong>b</strong></p>'), 'a **b**');
});

test('extractSelect with a real selection returns it without an overlay', async () => {
  const dom = fixture('selection.html', 'https://fixture.example/sel');
  const doc = dom.window.document;
  const range = doc.createRange();
  range.selectNodeContents(doc.getElementById('s')!);
  dom.window.getSelection()!.addRange(range);
  const draft = await extractSelect(doc);
  assert.equal(draft?.kind, 'select');
  assert.equal(draft?.markdown, 'Selected text');
  assert.equal(draft?.title, 'Selection Fixture');
  assert.equal(doc.getElementById('aikb-region-picker'), null);
});

test('extractSelect with nothing selected opens the picker, and Escape cancels it cleanly', async () => {
  const dom = fixture('selection.html', 'https://fixture.example/sel');
  const doc = dom.window.document;
  const pending = extractSelect(doc, () => []);
  await Promise.resolve();
  assert.ok(doc.getElementById('aikb-region-picker'), 'overlay present');
  doc.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape' }));
  assert.equal(await pending, null);
  assert.equal(doc.getElementById('aikb-region-picker'), null);
});

test('a drag over the text collects it in document order and removes the overlay', async () => {
  const dom = fixture('selection.html', 'https://fixture.example/sel');
  const doc = dom.window.document;
  const target = doc.getElementById('s')!.firstChild as Text;
  const rectsFor = (node: Text) => (node === target ? [new dom.window.DOMRect(10, 10, 100, 20)] : []);
  const pending = startRegionPicker(doc, rectsFor);
  const overlay = doc.getElementById('aikb-region-picker')!;
  overlay.dispatchEvent(new dom.window.MouseEvent('mousedown', { clientX: 0, clientY: 0, bubbles: true }));
  overlay.dispatchEvent(new dom.window.MouseEvent('mousemove', { clientX: 200, clientY: 50, bubbles: true }));
  overlay.dispatchEvent(new dom.window.MouseEvent('mouseup', { clientX: 200, clientY: 50, bubbles: true }));
  const draft = await pending;
  assert.equal(draft?.markdown, 'Selected text');
  assert.equal(doc.getElementById('aikb-region-picker'), null);
});

test('a drag over nothing resolves null', async () => {
  const dom = fixture('selection.html', 'https://fixture.example/sel');
  const doc = dom.window.document;
  const pending = startRegionPicker(doc, () => []);
  const overlay = doc.getElementById('aikb-region-picker')!;
  overlay.dispatchEvent(new dom.window.MouseEvent('mousedown', { clientX: 0, clientY: 0, bubbles: true }));
  overlay.dispatchEvent(new dom.window.MouseEvent('mouseup', { clientX: 5, clientY: 5, bubbles: true }));
  assert.equal(await pending, null);
  assert.equal(doc.getElementById('aikb-region-picker'), null);
});

test('a drag over inline markup keeps the sentence on one line', async () => {
  const dom = fixture('selection.html', 'https://fixture.example/sel');
  const doc = dom.window.document;
  const inline = doc.getElementById('inline')!;
  const rectsFor = (node: Text) => (inline.contains(node) ? [new dom.window.DOMRect(10, 10, 100, 20)] : []);
  const pending = startRegionPicker(doc, rectsFor);
  const overlay = doc.getElementById('aikb-region-picker')!;
  overlay.dispatchEvent(new dom.window.MouseEvent('mousedown', { clientX: 0, clientY: 0, bubbles: true }));
  overlay.dispatchEvent(new dom.window.MouseEvent('mouseup', { clientX: 200, clientY: 50, bubbles: true }));
  assert.equal((await pending)?.markdown, 'Pay now or later.');
});

test('a mouseup outside the overlay still finishes the drag', async () => {
  const dom = fixture('selection.html', 'https://fixture.example/sel');
  const doc = dom.window.document;
  const target = doc.getElementById('s')!.firstChild as Text;
  const pending = startRegionPicker(doc, (node) => (node === target ? [new dom.window.DOMRect(10, 10, 100, 20)] : []));
  doc.getElementById('aikb-region-picker')!.dispatchEvent(new dom.window.MouseEvent('mousedown', { clientX: 0, clientY: 0, bubbles: true }));
  doc.dispatchEvent(new dom.window.MouseEvent('mouseup', { clientX: 200, clientY: 50, bubbles: true }));
  assert.equal((await pending)?.markdown, 'Selected text');
  assert.equal(doc.getElementById('aikb-region-picker'), null);
});

test('a document without a body resolves null instead of throwing', async () => {
  const dom = new JSDOM('<svg xmlns="http://www.w3.org/2000/svg"></svg>', { contentType: 'image/svg+xml', url: 'https://fixture.example/x.svg' });
  assert.equal(await startRegionPicker(dom.window.document, () => []), null);
});
