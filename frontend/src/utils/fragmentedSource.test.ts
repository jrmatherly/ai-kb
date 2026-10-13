import test from 'node:test'
import assert from 'node:assert/strict'
import { findSourceFragments } from './fragmentedSource'
import { resolvePdfSource } from './pdfSourceLocate'
import { sourceImageContext, sourceQuoteText } from './sourceLocator'
import { mergeDocumentReferences, resolveReferenceSource } from './referenceSources'
import { preprocessCitationTags } from './citationMarkdown'

test('Agent grouping retains every cited chunk identity and resolves the exact document', () => {
  const refs = mergeDocumentReferences([
    // Chinese inputs exercise CJK fragment matching and file names.
    { id: 'first', knowledge_id: 'doc', knowledge_title: '\u5fae\u4fe1&\u4f01\u5fae.docx', content: 'first evidence' },
    { id: 'second', knowledge_id: 'doc', knowledge_title: '\u5fae\u4fe1&\u4f01\u5fae.docx', content: 'second evidence' },
  ])
  assert.deepEqual(refs[0]?.chunk_ids, ['first', 'second'])
  const source = resolveReferenceSource(refs, { chunkId: 'second', openSource: true })
  assert.equal(source?.knowledgeId, 'doc')
  assert.equal(source?.chunkId, 'second')
  assert.equal(source?.content, undefined, 'grouped content must be fetched for the cited chunk')
})

test('encoded citation titles are decoded once before HTML escaping', () => {
  const html = preprocessCitationTags('<kb doc="\u5fae\u4fe1&amp;\u4f01\u5fae.docx" id="kb" chunk_id="chunk"/>')
  assert.ok(!html.includes('&amp;amp;'))
  assert.ok(html.includes('\u5fae\u4fe1&amp;\u4f01\u5fae.docx'))
})

test('column-serialized table identifies a row-serialized page by independent cells', async () => {
  const pages = ['\u673a\u5668\u4eba\u7814\u7a76\u793e\uff08\u5468\u56db\uff09 \u5f20\u8001\u5e08 \u6559\u5ba4A\n\u5929\u6587\u89c2\u6d4b\u793e\uff08\u5468\u56db\uff09 \u674e\u8001\u5e08 \u6559\u5ba4B', '\u673a\u5668\u4eba\u7814\u7a76\u793e\uff08\u5468\u56db\uff09 \u5f20\u8001\u5e08 \u64cd\u573a']
  const scope = '|\u8bfe\u7a0b|\u8001\u5e08|\u5730\u70b9|\n|\u673a\u5668\u4eba\u7814\u7a76\u793e\uff08\u5468\u56db\uff09 \u5929\u6587\u89c2\u6d4b\u793e\uff08\u5468\u56db\uff09|\u5f20\u8001\u5e08 \u674e\u8001\u5e08|\u6559\u5ba4A \u6559\u5ba4B|'
  const result = await resolvePdfSource({ token: 1, quotes: [], locators: [], scope, sentence: '\u5929\u6587\u89c2\u6d4b\u793e\u6bcf\u5468\u56db\u5f00\u8bfe\u3002' }, pages.length, async p => pages[p - 1]!)
  assert.equal(result.reason, 'partial')
  assert.equal(result.targets[0]?.page, 1)
  assert.ok(result.targets[0]?.quote?.includes('\u5929\u6587\u89c2\u6d4b\u793e'))
  assert.ok(result.targets.every(t => t.page === 1))
})

test('a conjunction of repeated cells must identify a unique page', () => {
  const scope = '|\u673a\u5668\u4eba\u7814\u7a76\u793e\uff08\u5468\u56db\uff09 \u5929\u6587\u89c2\u6d4b\u793e\uff08\u5468\u56db\uff09|'
  const pages = ['\u673a\u5668\u4eba\u7814\u7a76\u793e\uff08\u5468\u56db\uff09 \u5f20\u8001\u5e08 \u5929\u6587\u89c2\u6d4b\u793e\uff08\u5468\u56db\uff09 \u674e\u8001\u5e08', '\u673a\u5668\u4eba\u7814\u7a76\u793e\uff08\u5468\u56db\uff09', '\u5929\u6587\u89c2\u6d4b\u793e\uff08\u5468\u56db\uff09']
  assert.equal(findSourceFragments(pages, scope).length, 2)
  assert.deepEqual(findSourceFragments([pages[0]!, pages[0]!], scope), [])
})

test('fragment recovery cannot claim a missing numeric value or a coincidental opening', () => {
  assert.deepEqual(findSourceFragments(['Shared opening. Different conclusion.'], 'Shared opening. Missing conclusion.'), [])
  assert.deepEqual(findSourceFragments(['\u7532\u9879\u76ee\u6e29\u5ea615\u5ea6\n\u4e59\u9879\u76ee\u9650\u503c20\u6beb\u7c73'], '|\u7532\u9879\u76ee\u6e29\u5ea61.5\u5ea6|\u4e59\u9879\u76ee\u9650\u503c2.0\u6beb\u7c73|'), [])
})

test('parser artifacts retain only verified sentences and explicitly report partial coverage', async () => {
  const scope = '\u7ed1\u5b9a\u624b\u673a\uff1a\u70b9\u51fb\u8d26\u53f7à\u9009\u62e9\u4e2a\u4eba\u4e2d\u5fc3\u3002\n\n\u5b66\u751f\u9009\u8bfe\u5f00\u59cb\u65f6\u95f4\u4e3a\u5468\u4e00\u4e0a\u5348\u5341\u4e00\u70b9\u3002\u5c0f\u5b66\u90e8\u9ad8\u5e74\u7ea7\u9009\u8bfe\u5728\u5468\u4e00\u4e0b\u5348\u4e00\u70b9\u5f00\u59cb\u3002'
  const pages = ['\u7ed1\u5b9a\u624b\u673a\uff1a\u70b9\u51fb\u8d26\u53f7→\u9009\u62e9\u4e2a\u4eba\u4e2d\u5fc3\u3002\u5b66\u751f\u9009\u8bfe\u5f00\u59cb\u65f6\u95f4\u4e3a\u5468\u4e00\u4e0a\u5348\u5341\u4e00\u70b9\u3002\u5c0f\u5b66\u90e8\u9ad8\u5e74\u7ea7\u9009\u8bfe\u5728\u5468\u4e00\u4e0b\u5348\u4e00\u70b9\u5f00\u59cb\u3002']
  const result = await resolvePdfSource({ token: 1, quotes: [], locators: [], scope }, 1, async () => pages[0]!)
  assert.equal(result.reason, 'partial')
  assert.equal(result.targets.length, 3)
  assert.ok(result.targets.every(t => !t.quote?.includes('à')))
  assert.ok(result.targets.some(t => t.quote === '\u7ed1\u5b9a\u624b\u673a\uff1a\u70b9\u51fb\u8d26\u53f7'))
})

test('OCR image context requires exact resource identity and at least one neighboring paragraph', () => {
  const md = '\u524d\u4e00\u6bb5\u660e\u786e\u6765\u6e90\u3002\n\n![descript](resource://image)\n\n\u540e\u4e00\u6bb5\u660e\u786e\u6765\u6e90\u3002'
  assert.deepEqual(sourceImageContext(md, ['resource://image']), { before: '\u524d\u4e00\u6bb5\u660e\u786e\u6765\u6e90\u3002', after: '\u540e\u4e00\u6bb5\u660e\u786e\u6765\u6e90\u3002' })
  assert.equal(sourceImageContext(md, ['resource://another-image']), undefined)
  assert.deepEqual(sourceImageContext(md.replace('\u540e\u4e00\u6bb5\u660e\u786e\u6765\u6e90\u3002', ''), ['resource://image']), { before: '\u524d\u4e00\u6bb5\u660e\u786e\u6765\u6e90\u3002', after: '' })
  assert.equal(sourceImageContext(md + '\n\n' + md, ['resource://image']), undefined)
  assert.equal(sourceQuoteText(md).includes('descript'), false)
})

test('title fallback rejects duplicate document titles even within one knowledge base', () => {
  const refs = ['doc-one', 'doc-two'].map(knowledge_id => ({ id: knowledge_id, knowledge_id, knowledge_title: 'Same.docx', knowledge_base_id: 'kb' }))
  assert.equal(resolveReferenceSource(refs, { chunkId: 'missing-chunk', documentTitle: 'Same.docx', knowledgeBaseId: 'kb' }), null)
})

test('decoding a citation attribute never turns encoded markup into active HTML', () => {
  const html = preprocessCitationTags('<kb doc="&lt;img src=x onerror=alert(1)&gt;.docx" chunk_id="chunk"/>')
  assert.ok(html.includes('data-doc="&lt;img'))
  assert.ok(!html.includes('<img'))
})
