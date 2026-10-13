import assert from 'node:assert/strict'
import test from 'node:test'
import { findTableSource } from './tableSource'
import { resolvePdfSource } from './pdfSourceLocate'

// Chinese inputs exercise CJK table-cell matching.
const courses = '\u6570\u636e\u5b9e\u9a8c \u6570\u636e\u5b9e\u9a8c \u6570\u636e\u5b9e\u9a8c \u97f3\u4e50\u521b\u7f16\uff08\u521d\u7ea7\uff09 \u821e\u8e48 A \u56e2\uff08\u5468\u4e00\uff09 \u5929\u6587\u89c2\u6d4b'
const teachers = '\u674e\u6b23 \u4f55\u9038\u7fa4 \u5f20\u5065 \u8881\u4ea6\u4f73 \u5468\u5b5f\u5f71 \u5218\u4f73\u6587'
const scope = `|\u4e94 \u5e74 \u7ea7|\u5468\u4e00 \u5468\u4e8c|${courses}|${teachers}|`
const original = '\u4e94\u5e74\u7ea7 \u7efc\u5408\u8bfe\u7a0b\u8868\n\u6570\u636e\u5b9e\u9a8c \u674e\u6b23 \u6559\u5ba4\n\u6570\u636e\u5b9e\u9a8c \u4f55\u9038\u7fa4 \u6559\u5ba4\n\u6570\u636e\u5b9e\u9a8c \u5f20\u5065 \u6559\u5ba4\n\u97f3\u4e50\u521b\u7f16\uff08\u521d\u7ea7\uff09 \u8881\u4ea6\u4f73 \u97f3\u4e50\u5ba4\n\u821e\u8e48 A \u56e2\uff08\u5468\u4e00\uff09 \u5468\u5b5f\u5f71 \u793c\u5802\n\u5929\u6587\u89c2\u6d4b \u5218\u4f73\u6587 \u79d1\u5b66\u5ba4'

test('paired column sequences resolve repeated courses to their original rows', () => {
  const nearbyGrade = original.replace('\u4e94\u5e74\u7ea7', '\u516d\u5e74\u7ea7').replace('\u4f55\u9038\u7fa4', '\u6768\u7fa4\u82f1')
  const hits = findTableSource([nearbyGrade, original, nearbyGrade], scope)
  assert.equal(hits.length, 12)
  assert.ok(hits.every(h => h.page === 2))
  assert.equal(hits.filter(h => h.quote === '\u6570\u636e\u5b9e\u9a8c').length, 3)
  assert.ok(hits.some(h => h.quote.includes('\u821e\u8e48 A \u56e2')))
})

test('identical tables in two places remain unresolved', () => {
  assert.deepEqual(findTableSource([original, original], scope), [])
})

test('matching names in the wrong rows cannot satisfy paired columns', () => {
  assert.deepEqual(findTableSource([original.replace('\u6570\u636e\u5b9e\u9a8c \u674e\u6b23', '\u6570\u636e\u5b9e\u9a8c \u8881\u4ea6\u4f73').replace('\u521d\u7ea7\uff09 \u8881\u4ea6\u4f73', '\u521d\u7ea7\uff09 \u674e\u6b23')], scope), [])
})

test('a spanning label cannot resolve to another grade with the same courses and teachers', () => {
  assert.deepEqual(findTableSource([original.replace('\u4e94\u5e74\u7ea7', '\u516d\u5e74\u7ea7')], scope), [])
})

test('individual cell fallback cannot bypass the table grade constraint', async () => {
  const wrong = original.replace('\u4e94\u5e74\u7ea7', '\u516d\u5e74\u7ea7')
  const result = await resolvePdfSource({ token: 1, locators: [], quotes: [], scope }, 1, async () => wrong)
  assert.deepEqual(result.targets, [])
  assert.equal(result.reason, 'missing')
})

test('a sequence cannot splice rows across an intervening mismatched table row', () => {
  const lines = original.split('\n')
  lines.splice(3, 0, '\u6570\u636e\u5b9e\u9a8c \u674e\u6b23 \u6559\u5ba4')
  assert.deepEqual(findTableSource([lines.join('\n')], scope), [])
})

test('ordered rows can cross a PDF page boundary', () => {
  const lines = original.split('\n')
  const hits = findTableSource([lines.slice(0, 4).join('\n'), lines.slice(4).join('\n')], scope)
  assert.equal(hits.length, 12)
  assert.deepEqual([...new Set(hits.map(h => h.page))], [1, 2])
})

test('a changed numeric cell is not accepted as a prefix of the original', () => {
  const numerical = '|\u68c0\u6d4b\u4e00\u6279\uff081.5\uff09 \u68c0\u6d4b\u4e8c\u6279\uff082.5\uff09 \u68c0\u6d4b\u4e09\u6279\uff083.5\uff09 \u68c0\u6d4b\u56db\u6279\uff084.5\uff09|\u8d1f\u8d23\u4eba\u7532 \u8d1f\u8d23\u4eba\u4e59 \u8d1f\u8d23\u4eba\u4e19 \u8d1f\u8d23\u4eba\u4e01|'
  const page = '\u68c0\u6d4b\u4e00\u6279\uff0815\uff09 \u8d1f\u8d23\u4eba\u7532\n\u68c0\u6d4b\u4e8c\u6279\uff082.5\uff09 \u8d1f\u8d23\u4eba\u4e59\n\u68c0\u6d4b\u4e09\u6279\uff083.5\uff09 \u8d1f\u8d23\u4eba\u4e19\n\u68c0\u6d4b\u56db\u6279\uff084.5\uff09 \u8d1f\u8d23\u4eba\u4e01'
  assert.deepEqual(findTableSource([page], numerical), [])
})

test('image-heavy steps recover exact instructions around font arrows and page gaps', async () => {
  const text = '\u7b2c\u4e8c\u6b65\uff1a\u8bbe\u7f6e\u9ed8\u8ba4\u9875\u9762\u5e76\u70b9\u51fb\u786e\u8ba4\u6309\u94ae\u3002\n\u7b2c\u4e09\u6b65\uff1a\u8fdb\u5165\u684c\u9762\u6253\u5f00\u9009\u4fee\u8bfe\u5e94\u7528\u3002\na) \u9996\u9875\u8fdb\u5165\u3010\u9009\u4fee\u8bfe\u3011à②\u5de6\u4e0a\u89d2\u3010\u9009\u8bfe\u62a5\u540d\u3011à③\u6ed1\u52a8\u627e\u5230\u5f85\u9009\u8bfe\u7a0b\uff0c\u5355\u51fb\u3010\u7533\u8bf7\u9009\u4fee\u3011\u6309\u94ae\n' + '\u56fe\u7247\u8bc6\u522b\u4ea7\u751f\u7684\u989d\u5916\u8bf4\u660e\u6587\u5b57'.repeat(30)
  const pages = ['\u7b2c\u4e8c\u6b65\uff1a\u8bbe\u7f6e\u9ed8\u8ba4\u9875\u9762\u5e76\u70b9\u51fb\u786e\u8ba4\u6309\u94ae\u3002', '\u56fe\u7247\u5185\u5bb9\n\u7b2c\u4e09\u6b65\uff1a\u8fdb\u5165\u684c\u9762\u6253\u5f00\u9009\u4fee\u8bfe\u5e94\u7528\u3002\na) \u9996\u9875\u8fdb\u5165\u3010\u9009\u4fee\u8bfe\u3011→②\u5de6\u4e0a\u89d2\u3010\u9009\u8bfe\u62a5\u540d\u3011→③\u6ed1\u52a8\u627e\u5230\u5f85\u9009\u8bfe\u7a0b\uff0c\u5355\u51fb\u3010\u7533\u8bf7\u9009\u4fee\u3011\u6309\u94ae']
  const r = await resolvePdfSource({token: 1, locators: [], quotes: [], scope: text, sentence: '\u8fdb\u5165\u3010\u9009\u4fee\u8bfe\u3011\uff0c\u627e\u5230\u8bfe\u7a0b\u540e\u70b9\u51fb\u3010\u7533\u8bf7\u9009\u4fee\u3011\u3002'}, 2, async p => pages[p-1]!)
  assert.equal(r.reason, 'partial')
  assert.equal(r.targets[0]?.page, 2)
  assert.ok(r.targets.some(t => t.quote?.includes('\u7533\u8bf7\u9009\u4fee')))
  assert.ok(r.targets.every(t => !t.quote?.includes('\u989d\u5916\u8bf4\u660e')))
})
