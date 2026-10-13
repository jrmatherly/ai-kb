import { test } from 'node:test'
import assert from 'node:assert/strict'
import { quotedSourceExcerpt, selectLocatorsForSentence, selectQuoteForSentence } from './sourceLocator.ts'
import { sourceImageDigest } from './sourceImage.ts'
test('a paraphrase with a verbatim quotation selects only supporting evidence', () => {
 // Chinese quotes exercise CJK quotation-mark matching.
 const quote = '\u5c0f\u5bb6\u56e2\u5706\u7f8e\u6ee1\uff0c\u5927\u56fd\u7e41\u8363\u660c\u76db'
 const sentence = `\u7269\u4e1a\u5728\u53cc\u8282\u671f\u95f4\u5411\u4e1a\u4e3b\u81f4\u4ee5\u8282\u65e5\u795d\u798f\uff0c\u5021\u5bfc\u201c${quote}\u201d\u7684\u5bb6\u56fd\u60c5\u6000`
 const source = `\u5f00\u5934\u95ee\u5019\u3002\n\n\u613f\u5bb6\u56fd\u540c\u8f89\uff1a${quote}\uff0c\u53cc\u559c\u4e34\u95e8\u3002`
 assert.equal(quotedSourceExcerpt(source, sentence), quote)
 assert.deepEqual(selectQuoteForSentence(source, sentence), [quote])
 const locs = [{ type: 'docx', quote: '\u5f00\u5934\u95ee\u5019\u3002' }, { type: 'docx', quote: source.split('\n\n')[1] }]
 assert.deepEqual(selectLocatorsForSentence(locs, sentence), [locs[1]])
 assert.equal(quotedSourceExcerpt('\u5176\u4ed6\u5185\u5bb9\u3002', sentence), undefined)
 assert.equal(quotedSourceExcerpt(`${source}\n${source}`, sentence), undefined)
})
test('quotation does not relax numeric matching', () => {
 assert.equal(quotedSourceExcerpt('The cited limit is 1.5 millimeters.', 'Claim: "The cited limit is 15 millimeters."'), undefined)
})
test('source image SHA256 identifies exact bytes', async () => {
 assert.equal(await sourceImageDigest(new TextEncoder().encode('abc').buffer), 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad')
 assert.notEqual(await sourceImageDigest(new TextEncoder().encode('abd').buffer), await sourceImageDigest(new TextEncoder().encode('abc').buffer))
})
