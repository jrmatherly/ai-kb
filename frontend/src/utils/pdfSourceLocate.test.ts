import test from 'node:test'
import assert from 'node:assert/strict'
import { resolvePdfSource, mergeSourceRects } from './pdfSourceLocate.ts'
import { findInText, findTextMatches, normalizeForMatch, selectQuoteForSentence, parseSourceLocators, sourceQuoteText } from './sourceLocator.ts'

const resolve = (pages: string[], request: Partial<Parameters<typeof resolvePdfSource>[0]>) =>
  resolvePdfSource({ locators: [], quotes: [], token: 1, ...request }, pages.length, async (p) => pages[p - 1]!)

test('decimal, negative, percentage and comparison differences never match', () => {
  for (const [source, quote] of [
    // Chinese inputs exercise CJK text matching against PDF text.
    ['\u9650\u503c\u4e3a15\u6beb\u7c73', '\u9650\u503c\u4e3a1.5\u6beb\u7c73'], ['\u6e29\u5ea6\u4e3a5\u5ea6', '\u6e29\u5ea6\u4e3a-5\u5ea6'],
    ['\u589e\u957f\u4e3a5', '\u589e\u957f\u4e3a5%'], ['\u503c\u5c0f\u4e8e<5', '\u503c\u5c0f\u4e8e>5'], ['\u65f6\u95f412:30', '\u65f6\u95f41230'],
    ['\u6bd4\u4f8b\u4e3a1/2', '\u6bd4\u4f8b\u4e3a12'], ['\u9608\u503c≤5', '\u9608\u503c≥5'],
  ]) assert.equal(findInText(source!, quote!), null, `${source} / ${quote}`)
  assert.ok(findInText('\u9650\u503c\u4e3a\uff11.\uff15\u6beb\u7c73', '\u9650\u503c\u4e3a1.5\u6beb\u7c73'))
})

test('partial common opening does not manufacture a full quote', () => {
  const common = '\u8bbe\u5907\u8fdb\u5165\u65bd\u5de5\u73b0\u573a\u4ee5\u540e\u5fc5\u987b\u6309\u7167\u89c4\u5b9a\u68c0\u67e5\u6240\u6709\u5b89\u5168\u63aa\u65bd'
  assert.equal(findInText(common + '\u672c\u6b21\u9a8c\u6536\u6ca1\u6709\u901a\u8fc7', common + '\u672c\u6b21\u9a8c\u6536\u5df2\u7ecf\u901a\u8fc7'), null)
})

test('duplicates are ambiguous and supplementary characters have correct UTF-16 ranges', () => {
  assert.equal(findInText('\u91cd\u590d\u539f\u6587\u3002\u91cd\u590d\u539f\u6587\u3002', '\u91cd\u590d\u539f\u6587'), null)
  assert.equal(findTextMatches('\u91cd\u590d\u539f\u6587\u3002\u91cd\u590d\u539f\u6587\u3002', '\u91cd\u590d\u539f\u6587').length, 2)
  const text = '\u524d\u6587\u{20000}\u539f\u6587\u7247\u6bb5\u{20001}\u540e\u6587'
  const hit = findInText(text, '\u{20000}\u539f\u6587\u7247\u6bb5\u{20001}')!
  assert.equal(text.slice(hit.start, hit.end), '\u{20000}\u539f\u6587\u7247\u6bb5\u{20001}')
})

test('a paraphrase retains full source context rather than an unrelated opening', () => {
  const source = '\u7b2c\u4e00\u90e8\u5206\u8ba8\u8bba\u80cc\u666f\u3002\u7b2c\u4e8c\u90e8\u5206\u8bd5\u9a8c\u5931\u8d25\u3002'
  assert.deepEqual(selectQuoteForSentence(source, 'The experiment failed.'), [source])
})

test('malformed pages and boxes cannot be drawn', () => {
  assert.deepEqual(parseSourceLocators([{ type: 'pdf', page: 0 }, { type: 'pdf', page: 1.5 }]), [])
  for (const bbox of [[NaN, 0, 1, 1], [0, 0, Infinity, 1], [0, 0, 2, 1], [0.8, 0, 0.2, 1]]) {
    assert.equal(parseSourceLocators([{ type: 'pdf', page: 1, bbox }])[0]?.bbox, undefined)
  }
})

test('all pages are considered, including page 501', async () => {
  const pages = Array.from({ length: 501 }, (_, i) => `Page ${i + 1} unrelated.`)
  pages[500] = 'The uniquely cited passage.'
  const result = await resolve(pages, { scope: pages[500] })
  assert.equal(result.targets[0]?.page, 501)
})

test('duplicate passages on different pages are not silently selected', async () => {
  const result = await resolve(['\u8bbe\u5907\u9a8c\u6536\u5408\u683c\u540e\u65b9\u53ef\u4f7f\u7528\u3002', '\u8bbe\u5907\u9a8c\u6536\u5408\u683c\u540e\u65b9\u53ef\u4f7f\u7528\u3002'], { scope: '\u8bbe\u5907\u9a8c\u6536\u5408\u683c\u540e\u65b9\u53ef\u4f7f\u7528\u3002' })
  assert.deepEqual(result, { targets: [], reason: 'ambiguous' })
})

test('full chunk context disambiguates a repeated sentence and retains occurrence', async () => {
  const result = await resolve(['\u7b2c\u4e00\u7ae0\u3002\u8bbe\u5907\u9a8c\u6536\u5408\u683c\u540e\u65b9\u53ef\u4f7f\u7528\u3002\u7b2c\u4e8c\u7ae0\u3002\u8bbe\u5907\u9a8c\u6536\u5408\u683c\u540e\u65b9\u53ef\u4f7f\u7528\u3002'], {
    scope: '\u7b2c\u4e8c\u7ae0\u3002\u8bbe\u5907\u9a8c\u6536\u5408\u683c\u540e\u65b9\u53ef\u4f7f\u7528\u3002', sentence: '\u8bbe\u5907\u9a8c\u6536\u5408\u683c\u540e\u65b9\u53ef\u4f7f\u7528\u3002',
  })
  assert.equal(result.targets[0]?.page, 1)
  assert.equal(result.targets[0]?.occurrence, 1)
})

test('legacy wrong-page bbox is revalidated against original text', async () => {
  const result = await resolve(['\u7b2c\u4e00\u6bb5\u6b63\u786e\u6587\u672c\u3002', '\u7b2c\u4e8c\u6bb5\u5b9e\u9645\u5185\u5bb9\u3002'], {
    scope: '\u7b2c\u4e8c\u6bb5\u5b9e\u9645\u5185\u5bb9\u3002', locators: [{ type: 'pdf', page: 1, bbox: [0, 0, 1, 1], quote: '\u7b2c\u4e8c\u6bb5\u5b9e\u9645\u5185\u5bb9\u3002' }],
  })
  assert.equal(result.targets[0]?.page, 2)
  assert.equal(result.targets[0]?.bbox, undefined)
})

test('legacy matching page does not override ambiguity elsewhere', async () => {
  const result = await resolve(['\u76f8\u540c\u7684\u539f\u6587\u6bb5\u843d\u3002', '\u76f8\u540c\u7684\u539f\u6587\u6bb5\u843d\u3002'], {
    locators: [{ type: 'pdf', page: 1, quote: '\u76f8\u540c\u7684\u539f\u6587\u6bb5\u843d\u3002' }], scope: '\u76f8\u540c\u7684\u539f\u6587\u6bb5\u843d\u3002',
  })
  assert.equal(result.reason, 'ambiguous')
})

test('mixed bbox and page targets are both resolved', async () => {
  const result = await resolve(['', '\u7b2c\u4e8c\u9875\u7684\u5f15\u7528\u6587\u672c\u3002'], { locators: [
    { type: 'pdf', page: 1, mapping: 'exact', bbox: [0.1, 0.2, 0.9, 0.4], quote: '\u626b\u63cf\u9875\u7684\u539f\u6587' },
    { type: 'pdf', page: 2, mapping: 'exact', quote: '\u7b2c\u4e8c\u9875\u7684\u5f15\u7528\u6587\u672c\u3002' },
  ] })
  assert.deepEqual(result.targets.map((t) => [t.page, t.granularity]), [[1, 'block'], [2, 'text']])
})

test('scanned page without text retains verified block geometry', async () => {
  const result = await resolve([''], { locators: [{ type: 'pdf', page: 1, mapping: 'exact', bbox: [0.1, 0.2, 0.8, 0.5], quote: 'OCR \u6587\u672c' }] })
  assert.equal(result.targets[0]?.granularity, 'block')
})

test('cross-page quote produces separate source fragments', async () => {
  const result = await resolve(['The first part of', 'the cited passage ends here.'], { scope: 'The first part of the cited passage ends here.' })
  assert.deepEqual(result.targets.map((t) => t.page), [1, 2])
  assert.equal(normalizeForMatch(result.targets.map((t) => t.quote).join('')), normalizeForMatch('The first part of the cited passage ends here.'))
})

test('missing full chunk cannot fall back to a coincidental opening', async () => {
  const result = await resolve(['Shared opening. Completely different conclusion.'], { scope: 'Shared opening. Unique result absent.', quotes: ['Shared opening.'] })
  assert.deepEqual(result, { targets: [], reason: 'missing' })
})

test('unavailable revision does not read or highlight a replacement document', async () => {
  const result = await resolvePdfSource({ token: 1, quotes: ['old quote'], locators: [], unavailable: true }, 2, async () => { throw new Error('must not read') })
  assert.equal(result.reason, 'unavailable')
})

test('rapid citation switch cancels before consuming a stale page result', async () => {
  let cancelled = false
  const result = await resolvePdfSource({ token: 1, quotes: [], locators: [], scope: 'old citation' }, 20, async () => { cancelled = true; return 'old citation' }, () => cancelled)
  assert.deepEqual(result, { targets: [], reason: 'cancelled' })
})

test('same-height columns remain distinct while adjacent glyph runs merge', () => {
  const rects = mergeSourceRects([
    { left: 10, top: 10, width: 30, height: 12 }, { left: 42, top: 10, width: 30, height: 12 },
    { left: 250, top: 10, width: 80, height: 12 },
  ])
  assert.equal(rects.length, 2)
  assert.equal(rects[0]?.width, 62)
})

test('native DOMRect prototype getters survive rectangle merging', () => {
  // Browser DOMRect coordinates are getters on the prototype, not own fields.
  const nativeLike = Object.create({ left: 20, top: 30, width: 40, height: 12 })
  assert.deepEqual(mergeSourceRects([nativeLike]), [{ left: 20, top: 30, width: 40, height: 12 }])
})

test('a number cannot match part of a different numeric value', () => {
  for (const [source, quote] of [['\u589e\u957f\u4e3a5%', '\u589e\u957f\u4e3a5'], ['115', '15'], ['1.5', '15'], ['15.5', '15'], ['-15', '15']]) {
    assert.equal(findInText(source!, quote!), null, `${source} / ${quote}`)
  }
})

test('partially mapped chunks cannot claim complete precise evidence', async () => {
  const result = await resolve(['Known paragraph.', 'Actual important conclusion.'], {
    scope: 'Known paragraph. Missing important conclusion.',
    locators: [{ type: 'pdf', page: 1, mapping: 'exact', partial: true, quote: 'Known paragraph.' }],
  })
  assert.equal(result.reason, 'missing')
})


test('HTML entities preserve comparisons and are decoded only once', () => {
  assert.equal(sourceQuoteText('<td>\u9650\u503c&#x3c;1.5</td>'), '\u9650\u503c<1.5')
  assert.equal(sourceQuoteText('&amp;lt;'), '&lt;')
  assert.equal(findInText('temperature 5 C', 'temperature - 5 C'), null)
  assert.ok(findInText('temperature - 5 C', 'temperature -5 C'))
})
