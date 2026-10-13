import assert from 'node:assert/strict'
import test from 'node:test'
import { buildManualDraft, deriveManualTitle } from './manualKnowledgeDraft'

const labels = { emptyAnswer: '(No content)', sourcesHeading: 'Sources' }

test('a chat answer loses its <kb/> tags and gains a source list', () => {
  const answer = [
    '## A binary view of technology',
    '',
    '> **"People fall into a binary good-or-bad view."** <kb doc="Interview.md" chunk_id="230a27cb-45c8-4ba9-9ba1-babea70bcd95" kb_id="441ee604-a04d-4c9a-a4bd-1c4e198e6226" />',
    '',
    'He goes on to use Zig as an example <kb doc="Interview.md" chunk_id="ef18491c-34f0-4563-bb4a-062cacbdfd95" kb_id="441ee604-a04d-4c9a-a4bd-1c4e198e6226" />',
  ].join('\n')

  const draft = buildManualDraft(answer, labels)

  assert.ok(!draft.includes('<kb'), draft)
  assert.ok(!draft.includes('chunk_id'), draft)
  assert.ok(draft.includes('> **"People fall into a binary good-or-bad view."**'))
  assert.ok(draft.endsWith('**Sources**\n\n- Interview.md\n'), draft)
})

test('web citations survive as real Markdown links', () => {
  const draft = buildManualDraft('See the <web url="https://ziglang.org" title="Zig" /> website', labels)
  assert.equal(draft, 'See the [Zig](https://ziglang.org) website')
})

test('a tag alone on its line leaves no blank line behind', () => {
  const draft = buildManualDraft('First paragraph\n<kb doc="A.md" chunk_id="1" />\nSecond paragraph', labels)
  assert.equal(draft, 'First paragraph\nSecond paragraph\n\n---\n\n**Sources**\n\n- A.md\n')
})

test('code blocks keep their own spacing while citations are stripped', () => {
  const answer = '```js\nfn( 1 ) ;\n\n\nconst x = 2\n```\n\nNote <kb doc="A.md" chunk_id="1" />'
  const draft = buildManualDraft(answer, labels)
  assert.ok(draft.startsWith('```js\nfn( 1 ) ;\n\n\nconst x = 2\n```'), draft)
  assert.ok(draft.includes('\n\nNote\n'), draft)
})

test('an answer without citations is passed through untouched', () => {
  assert.equal(buildManualDraft('Plain text answer', labels), 'Plain text answer')
  assert.equal(buildManualDraft('   ', labels), '(No content)')
})

test('a long question is cut on a clause boundary, with no trailing ellipsis', () => {
  const title = deriveManualTitle(
    // Chinese questions exercise the CJK clause boundaries and question mark.
    '\u4e92\u8054\u7f51\u4e0a\u7684\u4eba\u4eec\u5e38\u5e38\u9677\u5165\u5bf9\u6280\u672f\u597d\u574f\u7684\u4e8c\u5143\u770b\u6cd5\uff0c\u8fd9\u79cd\u770b\u6cd5\u5982\u4f55\u5f71\u54cd\u4e86\u6280\u672f\u7684\u591a\u6837\u6027\u548c\u521b\u65b0\uff1f',
    'Chat excerpt',
  )
  assert.equal(title, '\u4e92\u8054\u7f51\u4e0a\u7684\u4eba\u4eec\u5e38\u5e38\u9677\u5165\u5bf9\u6280\u672f\u597d\u574f\u7684\u4e8c\u5143\u770b\u6cd5')
})

test('a short question keeps its wording, minus the question mark', () => {
  assert.equal(deriveManualTitle('Zig \u4e3a\u4ec0\u4e48\u503c\u5f97\u5173\u6ce8\uff1f', '\u5bf9\u8bdd\u6458\u5f55'), 'Zig \u4e3a\u4ec0\u4e48\u503c\u5f97\u5173\u6ce8')
  assert.equal(deriveManualTitle('  ', 'Chat excerpt'), 'Chat excerpt')
})

test('a long question with no boundary is cut at the limit', () => {
  const title = deriveManualTitle('\u957f'.repeat(60), '\u5bf9\u8bdd\u6458\u5f55')
  assert.equal(title, '\u957f'.repeat(32))
})

test('an English question is cut on a word boundary', () => {
  const title = deriveManualTitle('How does a binary view of technology hurt diversity?', 'Chat excerpt')
  assert.equal(title, 'How does a binary view of')
})
