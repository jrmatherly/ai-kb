import test from 'node:test'
import assert from 'node:assert/strict'

import {
  findInText,
  findNormalized,
  locatorPages,
  normalizeForMatch,
  parseSourceLocators,
  selectLocatorsForSentence,
  selectQuoteForSentence,
  textFragmentUrl,
} from './sourceLocator.ts'
import { anchorWithContext, lastSentence } from './citationAnchor.ts'
import { resolveReferenceSource } from './referenceSources.ts'
import { resolveEpubPath } from './epubPreview.ts'
import { resolvePreviewKind } from './filePreview.ts'

test('normalization preserves numeric symbols while folding width and case', () => {
  // Chinese inputs exercise CJK normalization and sentence matching.
  assert.equal(normalizeForMatch('\uff18.\uff11\uff14 \u7d22\u5939\uff0c\uff28ello!'), '8.14\u7d22\u5939hello')
})

test('findInText maps a normalized match back to the raw text', () => {
  const text = '8.14.11 \u7d22\u5939\u4e0e\u540a\u7d22\u65bd\u5de5\u5e94\u7b26\u5408\u4e0b\u5217\u89c4\u5b9a\uff1a\n1 \u5728\u6ee1\u8db3\u65bd\u5de5\u9700\u8981\u7684\u524d\u63d0\u4e0b\uff0c\u5e94\u51cf\u5c0f\u732b\u9053\u9762\u5c42\u5f00\u5b54\u9762\u79ef\u3002'
  const hit = findInText(text, '\u5728\u6ee1\u8db3\u65bd\u5de5\u9700\u8981\u7684\u524d\u63d0\u4e0b,\u5e94\u51cf\u5c0f\u732b\u9053\u9762\u5c42\u5f00\u5b54\u9762\u79ef')
  assert.ok(hit)
  assert.equal(text.slice(hit.start, hit.end), '\u5728\u6ee1\u8db3\u65bd\u5de5\u9700\u8981\u7684\u524d\u63d0\u4e0b\uff0c\u5e94\u51cf\u5c0f\u732b\u9053\u9762\u5c42\u5f00\u5b54\u9762\u79ef')
})

test('findNormalized tolerates only an omitted leading list label', () => {
  const hay = normalizeForMatch('\u524d\u6587\u3002\u7d27\u56fa\u540c\u4e00\u7d22\u5939\u7684\u87ba\u6813\u65f6\uff0c\u5404\u87ba\u6813\u53d7\u529b\u5e94\u5747\u5300\uff0c\u5e76\u5e94\u6309\u89c4\u5b9a\u987a\u5e8f\u65bd\u62e7\u3002\u540e\u6587')
  const m = findNormalized(hay, '\uff08\u4e8c\uff09\u7d27\u56fa\u540c\u4e00\u7d22\u5939\u7684\u87ba\u6813\u65f6\uff0c\u5404\u87ba\u6813\u53d7\u529b\u5e94\u5747\u5300\uff0c\u5e76\u5e94\u6309\u89c4\u5b9a\u987a\u5e8f\u65bd\u62e7')
  assert.ok(m)
  assert.ok(m.score >= 0.5)
  assert.equal(findNormalized(hay, '\u5b8c\u5168\u65e0\u5173\u7684\u4e00\u6bb5\u5185\u5bb9\u4e0d\u5e94\u8be5\u88ab\u5339\u914d\u5230\u8fd9\u91cc\u6765\u5427'), null)
})



test('selectLocatorsForSentence narrows to the locator that supports the sentence', () => {
  const locs = [
    { type: 'pdf', page: 57, quote: '8.14.11 \u7d22\u5939\u4e0e\u540a\u7d22\u65bd\u5de5\u5e94\u7b26\u5408\u4e0b\u5217\u89c4\u5b9a' },
    { type: 'pdf', page: 57, quote: '1 \u5728\u6ee1\u8db3\u65bd\u5de5\u9700\u8981\u7684\u524d\u63d0\u4e0b\uff0c\u5e94\u51cf\u5c0f\u732b\u9053\u9762\u5c42\u5f00\u5b54\u9762\u79ef\uff0c\u5e76\u5e94\u5728\u5f00\u5b54\u4f4d\u7f6e\u56db\u5468\u7ed1\u624e\u9632\u6ed1\u6728\u6761\uff0c\u8bbe\u7acb\u8b66\u793a\u6807\u5fd7\u3002' },
    { type: 'pdf', page: 57, quote: '2 \u7d22\u5939\u5728\u4e3b\u7f06\u4e0a\u5b9a\u4f4d\u540e\uff0c\u5e94\u7d27\u56fa\u87ba\u6813\u3002' },
  ]
  const picked = selectLocatorsForSentence(locs, '\u5728\u6ee1\u8db3\u65bd\u5de5\u9700\u8981\u7684\u524d\u63d0\u4e0b\uff0c\u5e94\u51cf\u5c0f\u732b\u9053\u9762\u5c42\u5f00\u5b54\u9762\u79ef\uff0c\u5e76\u5e94\u5728\u5f00\u5b54\u4f4d\u7f6e\u56db\u5468\u7ed1\u624e\u9632\u6ed1\u6728\u6761\uff0c\u8bbe\u7acb\u8b66\u793a\u6807\u5fd7\u3002')
  assert.equal(picked.length, 1)
  assert.equal(picked[0], locs[1])
  assert.equal(selectLocatorsForSentence(locs, '\u6beb\u4e0d\u76f8\u5e72\u7684\u4e00\u53e5\u8bdd').length, 3)
})

test('selectQuoteForSentence prefers the supporting sentence of the chunk', () => {
  const content = '\u7d22\u5939\u5728\u4e3b\u7f06\u4e0a\u5b9a\u4f4d\u540e\uff0c\u5e94\u7d27\u56fa\u87ba\u6813\u3002\u540a\u8fd0\u7269\u4f53\u65f6\uff0c\u4f5c\u4e1a\u4eba\u5458\u4e0d\u5f97\u6cbf\u4e3b\u7f06\u9876\u9762\u884c\u8d70\u3002'
  const [first] = selectQuoteForSentence(content, '\u540a\u8fd0\u7269\u4f53\u65f6\u4f5c\u4e1a\u4eba\u5458\u4e0d\u5f97\u6cbf\u4e3b\u7f06\u9876\u9762\u884c\u8d70')
  assert.equal(first, '\u540a\u8fd0\u7269\u4f53\u65f6\uff0c\u4f5c\u4e1a\u4eba\u5458\u4e0d\u5f97\u6cbf\u4e3b\u7f06\u9876\u9762\u884c\u8d70')
  assert.equal(selectQuoteForSentence(content, '')[0], content)
})

test('parseSourceLocators and locatorPages', () => {
  const locs = parseSourceLocators([{ type: 'pdf', page: 3 }, null, { page: 2 }, { type: 'pdf', page: 3 }, { type: 'pdf', page: 1 }])
  assert.equal(locs.length, 3)
  assert.deepEqual(locatorPages(locs), [3, 1])
  assert.deepEqual(parseSourceLocators('nope'), [])
})

test('textFragmentUrl builds a scroll-to-text link', () => {
  assert.equal(
    textFragmentUrl('https://example.com/a#old', 'safety rules - part 1'),
    'https://example.com/a#:~:text=safety%20rules%20%2D%20part%201',
  )
  const long = textFragmentUrl('https://example.com/a', 'x'.repeat(50) + ' ' + 'y'.repeat(50))
  assert.match(long, /#:~:text=x+,.*y+$/)
})





test('lastSentence keeps the cited sentence and extends short ones', () => {
  assert.equal(lastSentence('\u7b2c\u4e00\u53e5\u5f88\u957f\u5f88\u957f\u7684\u5185\u5bb9\u3002\u7b2c\u4e8c\u53e5\u4e5f\u662f\u5b8c\u6574\u7684\u53e5\u5b50\u3002'), '\u7b2c\u4e8c\u53e5\u4e5f\u662f\u5b8c\u6574\u7684\u53e5\u5b50\u3002')
  assert.equal(lastSentence('\u524d\u4e00\u53e5\u8db3\u591f\u957f\u7684\u5185\u5bb9\u3002\u77ed\u53e5\u3002'), '\u524d\u4e00\u53e5\u8db3\u591f\u957f\u7684\u5185\u5bb9\u3002\u77ed\u53e5\u3002')
})

test('anchorWithContext borrows the previous block for a marker on a label line', () => {
  const para = '\u5728\u7d22\u5939\u540a\u88c5\u4f5c\u4e1a\u5f00\u59cb\u524d\uff0c\u5fc5\u987b\u5bf9\u4e3b\u7f06\u8868\u9762\u8fdb\u884c\u6e05\u7406\u3002'
  assert.equal(anchorWithContext('\u4f9d\u636e\uff1a', [para, '2. \u6e05\u7406\u4e3b\u7f06\u8868\u9762']), `${para}\u4f9d\u636e\uff1a`)
  assert.equal(anchorWithContext('\u7d22\u5939\u5c31\u4f4d\u540e\u5e94\u7acb\u5373\u8fdb\u884c\u521d\u62e7\u3002', [para]), '\u7d22\u5939\u5c31\u4f4d\u540e\u5e94\u7acb\u5373\u8fdb\u884c\u521d\u62e7\u3002')
  assert.equal(anchorWithContext('\u4f9d\u636e\uff1a', []), '\u4f9d\u636e\uff1a')
})

test('resolveReferenceSource picks the cited chunk of a document reference', () => {
  const refs = [
    { id: 'c1', knowledge_id: 'k1', knowledge_filename: 'a.pdf', knowledge_title: 'A', content: 'x', source_locators: [{ type: 'pdf', page: 2 }] },
    { id: 'c2', chunk_ids: ['c2', 'c3'], knowledge_id: 'k2', knowledge_title: 'B' },
    { id: 'w', chunk_type: 'web_search', knowledge_id: 'k3' },
    { id: 'f', chunk_type: 'faq', knowledge_id: 'k4' },
  ]
  const exact = resolveReferenceSource(refs, { chunkId: 'c1', anchorText: 's' })
  assert.equal(exact?.knowledgeId, 'k1')
  assert.equal(exact?.locators?.[0].page, 2)
  assert.equal(exact?.anchorText, 's')
  const grouped = resolveReferenceSource(refs, { chunkId: 'c3' })
  assert.equal(grouped?.knowledgeId, 'k2')
  assert.equal(grouped?.chunkId, 'c3')
  assert.equal(grouped?.locators, undefined, 'a grouped match belongs to another chunk')
  assert.equal(resolveReferenceSource(refs, { chunkId: 'w' }), null)
  assert.equal(resolveReferenceSource(refs, { chunkId: 'f' }), null)
  assert.equal(resolveReferenceSource(refs, { url: 'https://x' }), null)
})

test('EPUB paths resolve against their document and preview as epub', () => {
  assert.equal(resolveEpubPath('OEBPS/text/ch1.xhtml', '../images/a%20b.png#x'), 'OEBPS/images/a b.png')
  assert.equal(resolveEpubPath('OEBPS/content.opf', 'ch1.xhtml'), 'OEBPS/ch1.xhtml')
  assert.equal(resolvePreviewKind('epub'), 'epub')
})
