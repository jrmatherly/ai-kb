import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { JSDOM } from 'jsdom';

// jsdom caps selectors at 2048 characters with no public knob, and Defuddle's selectors are longer.
// Widen it on the document implementation's prototype so the clone Defuddle parses is covered too.
const require = createRequire(import.meta.url);
const idlUtils = require('jsdom/lib/generated/idl/utils.js');
const { DOMSelector } = require('@asamuzakjp/dom-selector');
const base = new JSDOM('');
const docImplProto = Object.getPrototypeOf(idlUtils.implForWrapper(base.window.document));
docImplProto._getDOMSelector = function () {
  this.__wideSelector ??= new DOMSelector(this._globalObject, this, { idlUtils, maxLength: 1_000_000 });
  return this.__wideSelector;
};
// Defuddle's Markdown converter reads the global document, as a content script would.
Object.assign(globalThis, { window: base.window, document: base.window.document });

const { extractArticle, extractSelect, htmlToMarkdown, startRegionPicker } = await import('./extract');

const fixture = (name: string, url: string) => new JSDOM(readFileSync(new URL(`../../test/fixtures/${name}`, import.meta.url), 'utf8'), { url });

test('extractArticle keeps the body, drops the chrome, and leaves the live page alone', () => {
  const dom = fixture('article.html', 'https://fixture.example/post');
  const draft = extractArticle(dom.window.document);
  assert.equal(draft.kind, 'article');
  assert.equal(draft.title, 'Fixture Title');
  assert.equal(draft.url, 'https://fixture.example/post');
  assert.ok(draft.markdown.includes('Body paragraph one.'), draft.markdown);
  assert.ok(!draft.markdown.includes('Skip to content'), draft.markdown);
  assert.equal(draft.meta.author, 'Fixture Author');
  assert.ok(dom.window.document.querySelector('nav'), 'the live nav survived');
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
