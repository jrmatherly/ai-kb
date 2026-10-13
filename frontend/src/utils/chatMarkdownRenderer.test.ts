import assert from 'node:assert/strict'
import test from 'node:test'

import {
  applyStreamingTailFade,
  closeDanglingStreamingEmphasis,
  createChatMarkdownRenderer,
  markStandaloneStrongParagraphs,
  normalizeFullwidthMarkdownImageParentheses,
  normalizeLegacyImageContextMarkup,
  preprocessMathDelimiters,
  renderChatMarkdown,
  repairFlankingEmphasis,
  replaceIncompleteImageWithPlaceholder,
  stripTrailingStreamingHorizontalRule,
  stripTrailingStreamingListMarker,
} from './chatMarkdownRenderer.ts'
import {
  collapseStandaloneCitationParagraphs,
  joinCitationTagsToPreviousLine,
  resolveCitationChunkId,
  stripIncompleteCitationTag,
} from './citationMarkdown.ts'

const SAMPLE_DOC = 'example-report.docx'
const SAMPLE_CHUNK_A = '00000001-0000-4000-8000-000000000001'
const SAMPLE_CHUNK_B = '00000002-0000-4000-8000-000000000002'
const SAMPLE_CHUNK_C = '00000003-0000-4000-8000-000000000003'
const SAMPLE_CHUNK_PRESERVE = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'

/** Remove the streaming tail-fade wrapper so structural assertions stay focused. */
function stripFadeTail(html: string): string {
  return html.replace(/<span class="stream-fade-tail">([\s\S]*?)<\/span>/g, '$1')
}

test('preprocessMathDelimiters converts escaped math delimiters for marked-katex', () => {
  assert.equal(
    preprocessMathDelimiters('inline \\(a+b\\) and block \\[x^2\\]'),
    'inline $a+b$ and block $$x^2$$',
  )
})

test('replaceIncompleteImageWithPlaceholder hides an unfinished streaming image', () => {
  assert.equal(
    replaceIncompleteImageWithPlaceholder('before ![chart](local://bucket/path'),
    'before <span class="streaming-image-loading"><span class="streaming-image-loading__skeleton"></span></span>',
  )
})

test('normalizeFullwidthMarkdownImageParentheses repairs localized image delimiters', () => {
  const ref = 'resource://yB7V7wE1gls7h9WonCDq5Q'
  assert.equal(
    // Chinese inputs exercise fullwidth punctuation and CJK emphasis flanking.
    normalizeFullwidthMarkdownImageParentheses(`![]\uff08${ref}\uff09`),
    `![](${ref})`,
  )
  assert.equal(
    normalizeFullwidthMarkdownImageParentheses(`![\u6d41\u7a0b\u56fe]\uff08${ref}\uff09`),
    `![\u6d41\u7a0b\u56fe](${ref})`,
  )
  assert.equal(
    normalizeFullwidthMarkdownImageParentheses(`\u666e\u901a\u4e2d\u6587\uff08\u8bf4\u660e\uff09\u548c ![\u793a\u4f8b]\uff08unknown://value\uff09`),
    `\u666e\u901a\u4e2d\u6587\uff08\u8bf4\u660e\uff09\u548c ![\u793a\u4f8b]\uff08unknown://value\uff09`,
  )
})

test('normalizeFullwidthMarkdownImageParentheses preserves literal examples in code', () => {
  const code = '`![]\uff08resource://example\uff09`\n```md\n![]\uff08resource://example\uff09\n```'
  assert.equal(normalizeFullwidthMarkdownImageParentheses(code), code)
})

test('renderChatMarkdown safely renders an image with fullwidth parentheses', () => {
  const renderer = createChatMarkdownRenderer({
    imageRenderer: ({ href, text }) => `<img src="${href}" alt="${text}">`,
    isValidImageUrl: (href) => href.startsWith('resource://'),
  })
  const html = renderChatMarkdown('![]\uff08resource://yB7V7wE1gls7h9WonCDq5Q\uff09', {
    renderer,
    escapeMarkdown: (text) => text,
    sanitizeHtml: (value) => value,
    streaming: false,
  })

  assert.match(html, /<img src="resource:\/\/yB7V7wE1gls7h9WonCDq5Q" alt="">/)
  assert.doesNotMatch(html, /\uff08|\uff09/)
})

test('renderChatMarkdown skips an image with an empty destination', () => {
  const renderer = createChatMarkdownRenderer({
    imageRenderer: ({ href, text }) => `<img src="${href}" alt="${text}">`,
    invalidImageHtml: () => '<p>invalid</p>',
    isValidImageUrl: (href) => Boolean(href),
  })
  const html = renderChatMarkdown('![\u6839\u76ee\u5f55\u793a\u4f8b\u6587\u4ef6]()', {
    renderer,
    escapeMarkdown: (text) => text,
    sanitizeHtml: (value) => value,
    streaming: false,
  })

  assert.doesNotMatch(html, /<img/)
  assert.doesNotMatch(html, /invalid/)
})

test('renderChatMarkdown hides an unfinished fullwidth-parenthesis image while streaming', () => {
  const renderer = createChatMarkdownRenderer()
  const html = renderChatMarkdown('before ![\u6d41\u7a0b\u56fe]\uff08resource://yB7V7wE1gls7h9WonCDq5Q', {
    renderer,
    escapeMarkdown: (text) => text,
    sanitizeHtml: (value) => value,
    streaming: true,
  })

  assert.match(html, /streaming-image-loading/)
  assert.doesNotMatch(html, /resource:\/\/|\uff08/)
})

test('normalizeLegacyImageContextMarkup converts copied image XML to Markdown', () => {
  const input = [
    'before',
    '<image url="resource://AbCdEfGhIjKlMnOpQrStUv">',
    '<image_caption>\u76ee\u6807\u8bf4\u8bdd\u4eba\u63d0\u53d6\u7684\u6d41\u7a0b\u56fe [\u6d4b\u8bd5]</image_caption>',
    '<image_ocr>\u76ee\u6807\u8bf4\u8bdd\u4eba\u63d0\u53d6</image_ocr>',
    '</image>',
    'after',
  ].join('\n')

  const output = normalizeLegacyImageContextMarkup(input)
  assert.ok(
    output.includes('![\u76ee\u6807\u8bf4\u8bdd\u4eba\u63d0\u53d6\u7684\u6d41\u7a0b\u56fe \\[\u6d4b\u8bd5\\]](resource://AbCdEfGhIjKlMnOpQrStUv)'),
  )
  assert.doesNotMatch(output, /<image|image_caption|image_ocr/)
  assert.match(output, /before[\s\S]*after/)
})

test('normalizeLegacyImageContextMarkup keeps original Markdown when present', () => {
  const input = [
    '<images>',
    '<image url="resource://AbCdEfGhIjKlMnOpQrStUv">',
    '<image_original>![\u539f\u56fe](resource://AbCdEfGhIjKlMnOpQrStUv)</image_original>',
    '<image_caption>description</image_caption>',
    '</image>',
    '</images>',
  ].join('\n')

  assert.equal(
    normalizeLegacyImageContextMarkup(input).trim(),
    '![\u539f\u56fe](resource://AbCdEfGhIjKlMnOpQrStUv)',
  )
})

test('normalizeLegacyImageContextMarkup hides an unfinished XML block while streaming', () => {
  const prefix = '\u6d4b\u8bd5\u9636\u6bb5\u4e3b\u8981\u6d41\u7a0b\n\n'
  for (const partial of [
    '<ima',
    '<image url="resource://AbCdEfGhIjKlMnOpQrStUv">',
    '<image url="resource://AbCdEfGhIjKlMnOpQrStUv">\n<image_caption>\u6d41\u7a0b\u56fe',
  ]) {
    const output = normalizeLegacyImageContextMarkup(prefix + partial, true)
    assert.equal(output, prefix + '<span class="streaming-image-loading"><span class="streaming-image-loading__skeleton"></span></span>')
    assert.doesNotMatch(output, /resource:\/\/|image_caption|<ima/)
  }
})

test('normalizeLegacyImageContextMarkup preserves literal image XML in code', () => {
  const code = '```xml\n<image url="resource://example">\n</image>\n```'
  assert.equal(normalizeLegacyImageContextMarkup(code, true), code)
  assert.equal(normalizeLegacyImageContextMarkup(code, false), code)
  assert.equal(normalizeLegacyImageContextMarkup('ordinary <input', true), 'ordinary <input')
})

test('renderChatMarkdown renders leaked legacy image XML through the safe image renderer', () => {
  const renderer = createChatMarkdownRenderer({
    imageRenderer: ({ href, text }) => `<img src="${href}" alt="${text}">`,
    isValidImageUrl: (href) => href.startsWith('resource://'),
  })
  const html = renderChatMarkdown(
    '<image url="resource://AbCdEfGhIjKlMnOpQrStUv"><image_caption>\u6d41\u7a0b\u56fe</image_caption></image>',
    {
      renderer,
      escapeMarkdown: (text) => text,
      sanitizeHtml: (value) => value,
      streaming: false,
    },
  )

  assert.match(html, /<img src="resource:\/\/AbCdEfGhIjKlMnOpQrStUv" alt="\u6d41\u7a0b\u56fe">/)
  assert.doesNotMatch(html, /image_caption|&lt;image/)
})

test('stripIncompleteCitationTag hides only an unfinished streaming citation tail', () => {
  const prefix = 'Source '
  const complete = '<kb doc="2.jpg" chunk_id="3c67efd5-f2ff-4e26-9032-9e44e6861178" />'

  for (const partial of ['<', '<k', '<kb', '<kb ', '<kb doc="2.jpg"', '<w', '<we', '<web url="https://example.com"']) {
    assert.equal(stripIncompleteCitationTag(prefix + partial), prefix)
  }

  assert.equal(stripIncompleteCitationTag(prefix + complete), prefix + complete)
  assert.equal(stripIncompleteCitationTag('Value < 5'), 'Value < 5')
})

test('stripTrailingStreamingHorizontalRule hides an ambiguous trailing rule only mid-stream', () => {
  for (const rule of ['---', '* * *', '___']) {
    assert.equal(stripTrailingStreamingHorizontalRule(`- item\n\n${rule}`), '- item\n\n')
  }
  assert.equal(stripTrailingStreamingHorizontalRule('- item\n\n---\nnext'), '- item\n\n---\nnext')

  const renderer = createChatMarkdownRenderer()
  const options = {
    renderer,
    escapeMarkdown: (text: string) => text,
    sanitizeHtml: (html: string) => html,
  }
  const source = '- item\n\n---'
  assert.doesNotMatch(renderChatMarkdown(source, { ...options, streaming: true }), /<hr>/)
  assert.match(renderChatMarkdown(source, { ...options, streaming: false }), /<hr>/)
})

test('closeDanglingStreamingEmphasis closes unfinished inline markers without false positives', () => {
  assert.equal(closeDanglingStreamingEmphasis('**\u5e73\u53f0\u5730\u5740\uff1aexample.com'), '**\u5e73\u53f0\u5730\u5740\uff1aexample.com**')
  assert.equal(closeDanglingStreamingEmphasis('\u524d\u6587 *\u659c\u4f53'), '\u524d\u6587 *\u659c\u4f53*')
  assert.equal(closeDanglingStreamingEmphasis('\u524d\u6587 ~~\u5220\u9664'), '\u524d\u6587 ~~\u5220\u9664~~')
  assert.equal(closeDanglingStreamingEmphasis('***\u53c8\u7c97\u53c8\u659c'), '***\u53c8\u7c97\u53c8\u659c***')
  assert.equal(closeDanglingStreamingEmphasis('\u8fd0\u884c `npm run'), '\u8fd0\u884c `npm run`')

  // A trailing marker run with no content after it is ambiguous (e.g. the start
  // of the next `**` in a bold list) and is hidden until content arrives, rather
  // than rendered as a literal `*`/`**` that flickers a frame later.
  assert.equal(closeDanglingStreamingEmphasis('4. **\u671d\u9633\u5b9e\u9a8c**\n5. *'), '4. **\u671d\u9633\u5b9e\u9a8c**\n5. ')
  assert.equal(closeDanglingStreamingEmphasis('5. **'), '5. ')
  assert.equal(closeDanglingStreamingEmphasis('3. **\u6cf3\u71c3\u6e38\u6cf3*'), '3. **\u6cf3\u71c3\u6e38\u6cf3**')

  // No dangling markers / structural markers must be left untouched.
  assert.equal(closeDanglingStreamingEmphasis('\u6b63\u6587 **\u52a0\u7c97** \u6536\u5c3e'), '\u6b63\u6587 **\u52a0\u7c97** \u6536\u5c3e')
  assert.equal(closeDanglingStreamingEmphasis('* \u5217\u8868\u9879\u4e00'), '* \u5217\u8868\u9879\u4e00')
  assert.equal(closeDanglingStreamingEmphasis('\u666e\u901a\u6587\u672c\u6ca1\u6709\u6807\u8bb0'), '\u666e\u901a\u6587\u672c\u6ca1\u6709\u6807\u8bb0')
  // Markers inside an open fenced code block are literal, not emphasis.
  assert.equal(
    closeDanglingStreamingEmphasis('```js\nconst x = **y'),
    '```js\nconst x = **y',
  )
})

test('renderChatMarkdown renders an unfinished bold line as bold immediately while streaming', () => {
  const renderer = createChatMarkdownRenderer()
  const options = {
    renderer,
    escapeMarkdown: (text: string) => text,
    sanitizeHtml: (html: string) => html,
  }
  const partial = '**\u5e73\u53f0\u8bbf\u95ee\u5730\u5740\uff1achatbot.weixin.qq.com'
  // Streaming: optimistically bold so no raw `**` and no late layout jump.
  assert.match(
    stripFadeTail(renderChatMarkdown(partial, { ...options, streaming: true })),
    /<p class="md-strong-title"><strong>\u5e73\u53f0\u8bbf\u95ee\u5730\u5740\uff1achatbot\.weixin\.qq\.com<\/strong><\/p>/,
  )
  // Completed: a genuinely unterminated marker stays literal (we never invent
  // content for the final, authoritative render).
  assert.match(
    renderChatMarkdown(partial, { ...options, streaming: false }),
    /<p>\*\*\u5e73\u53f0\u8bbf\u95ee\u5730\u5740/,
  )
})

test('repairFlankingEmphasis bolds punctuation-adjacent emphasis CommonMark would drop', () => {
  // Closing delimiter sits between punctuation and a letter -> CommonMark rejects
  // it; we convert exactly that blocked pattern to explicit HTML.
  assert.equal(repairFlankingEmphasis('**XBRL\uff08\u8bed\u8a00\uff09**\u662f\u4e00\u79cd'), '<strong>XBRL\uff08\u8bed\u8a00\uff09</strong>\u662f\u4e00\u79cd')
  assert.equal(repairFlankingEmphasis('**a)**b'), '<strong>a)</strong>b')
  assert.equal(repairFlankingEmphasis('~~\u5220\u9664\uff09~~\u6587\u5b57'), '<del>\u5220\u9664\uff09</del>\u6587\u5b57')
  assert.equal(repairFlankingEmphasis('*\u659c\u4f53\uff09*\u6587\u5b57'), '<em>\u659c\u4f53\uff09</em>\u6587\u5b57')

  // Opening delimiter sits between a letter/number and punctuation -> CommonMark
  // refuses to open emphasis even though the closer is valid; we bold it too.
  assert.equal(
    repairFlankingEmphasis('\u77e5\u8bc6\u5e93\u4e2d**\u300axxx\u300b\u5b66\u7a0b\u624b\u518c**\u6574\u7406'),
    '\u77e5\u8bc6\u5e93\u4e2d<strong>\u300axxx\u300b\u5b66\u7a0b\u624b\u518c</strong>\u6574\u7406',
  )
  assert.equal(repairFlankingEmphasis('\u5728**\u300a\u4e66\u540d\u300b**'), '\u5728<strong>\u300a\u4e66\u540d\u300b</strong>')
  assert.equal(repairFlankingEmphasis('\u770b\u5230~~\u300a\u5220\u300b\u6587\u5b57~~\u540e\u9762'), '\u770b\u5230<del>\u300a\u5220\u300b\u6587\u5b57</del>\u540e\u9762')

  // Cases marked already handles, literal markers, and code must be untouched.
  assert.equal(repairFlankingEmphasis('**\u4e2d\u6587**\u540e\u9762'), '**\u4e2d\u6587**\u540e\u9762')
  assert.equal(repairFlankingEmphasis('**\u4e2d\u6587\uff09** \u540e\u9762'), '**\u4e2d\u6587\uff09** \u540e\u9762')
  assert.equal(repairFlankingEmphasis('\u6b63\u6587 **\u52a0\u7c97** \u6536\u5c3e'), '\u6b63\u6587 **\u52a0\u7c97** \u6536\u5c3e')
  assert.equal(repairFlankingEmphasis('`**a)**b`'), '`**a)**b`')
  assert.equal(repairFlankingEmphasis('2 ** 3 ** 4'), '2 ** 3 ** 4')
  assert.equal(repairFlankingEmphasis('\u4ef7\u683c 3 * 5 * 7 \u5143'), '\u4ef7\u683c 3 * 5 * 7 \u5143')
  // Exponent / glob markers stay literal: the opener is followed by a number or
  // path content, not an emphasis-opening punctuation run.
  assert.equal(repairFlankingEmphasis('x**2 + y**2'), 'x**2 + y**2')
  assert.equal(repairFlankingEmphasis('2**3**4'), '2**3**4')
})

test('renderChatMarkdown bolds punctuation-adjacent emphasis both mid-stream and when complete', () => {
  const renderer = createChatMarkdownRenderer()
  const options = {
    renderer,
    escapeMarkdown: (text: string) => text,
    sanitizeHtml: (html: string) => html,
  }
  const text = '**XBRL\uff08\u8bed\u8a00\uff09**\u662f\u4e00\u79cd\u6807\u51c6'
  for (const streaming of [true, false]) {
    assert.match(
      stripFadeTail(renderChatMarkdown(text, { ...options, streaming })),
      /<strong>XBRL\uff08\u8bed\u8a00\uff09<\/strong>\u662f\u4e00\u79cd\u6807\u51c6/,
    )
  }
})

test('stripTrailingStreamingListMarker hides a content-less trailing list/underline marker', () => {
  assert.equal(stripTrailingStreamingListMarker('1. **AAAAA**\n   - '), '1. **AAAAA**\n')
  assert.equal(stripTrailingStreamingListMarker('\u6587\u672c\n1. '), '\u6587\u672c\n')
  assert.equal(stripTrailingStreamingListMarker('\u6807\u9898\n=='), '\u6807\u9898\n')
  // A lone trailing number is the start of an ordered marker (no `.` yet).
  assert.equal(stripTrailingStreamingListMarker('\u6587\u672c\n1'), '\u6587\u672c\n')
  // A marker with content after it is a real list item and must stay.
  assert.equal(stripTrailingStreamingListMarker('- \u9879\u76ee'), '- \u9879\u76ee')
  assert.equal(stripTrailingStreamingListMarker('1. \u5185\u5bb9'), '1. \u5185\u5bb9')
  // An in-sentence number must not be touched.
  assert.equal(stripTrailingStreamingListMarker('\u503c\u662f 1'), '\u503c\u662f 1')
})

test('renderChatMarkdown does not flash a setext heading when a nested bullet dash streams in', () => {
  const renderer = createChatMarkdownRenderer()
  const options = {
    renderer,
    escapeMarkdown: (text: string) => text,
    sanitizeHtml: (html: string) => html,
    streaming: true,
  }
  // A must stay a plain bold item through the whole lead-up to B: the bare dash
  // must not flash <h2>, and the next bullet's `*`/`**` (before its text) must
  // not flash an empty nested <li> under A.
  for (const partial of ['1. **AAAAA**\n   - ', '1. **AAAAA**\n   - *', '1. **AAAAA**\n   - **']) {
    const html = stripFadeTail(renderChatMarkdown(partial, options))
    assert.doesNotMatch(html, /<h2/)
    assert.doesNotMatch(html, /<ul>/)
    assert.match(html, /<ol>\s*<li><strong>AAAAA<\/strong><\/li>\s*<\/ol>/)
  }
})

test('renderChatMarkdown streams a bold ordered list without literal-marker flicker', () => {
  const renderer = createChatMarkdownRenderer()
  const options = {
    renderer,
    escapeMarkdown: (text: string) => text,
    sanitizeHtml: (html: string) => html,
    streaming: true,
  }
  // Next item's marker has arrived but its content/closing ** has not.
  const html = stripFadeTail(renderChatMarkdown('1. **\u901f\u6e38**\n2. *', options))
  assert.doesNotMatch(html, /<li>\*+<\/li>/)
  assert.match(html, /<li><strong>\u901f\u6e38<\/strong><\/li>/)
})

test('markStandaloneStrongParagraphs tags only paragraphs that are entirely one bold run', () => {
  // A model emitting **Section title** as a pseudo-heading: should be tagged.
  assert.equal(
    markStandaloneStrongParagraphs('<p><strong>\u5c0f\u8282\u6807\u9898\uff1a</strong></p>'),
    '<p class="md-strong-title"><strong>\u5c0f\u8282\u6807\u9898\uff1a</strong></p>',
  )

  // Mid-stream body paragraph with one completed bold run plus body text must
  // NOT be tagged (this is the streaming spacing-jump regression).
  for (const html of [
    '<p>\u6b63\u6587 <strong>\u52a0\u7c97\u90e8\u5206</strong> \u4e0e **</p>',
    '<p>\u6b63\u6587 <strong>A</strong> \u4e0e <strong>B</strong> \u6536\u5c3e</p>',
    '<p><strong>\u52a0\u7c97</strong> \u540e\u7eed\u6587\u5b57</p>',
  ]) {
    assert.equal(markStandaloneStrongParagraphs(html), html)
  }

  // A bold list item (loose list wraps content in <p>) is list text, not a
  // subtitle: it must keep the list's own margins, not gain a 1.25em top margin.
  assert.equal(
    markStandaloneStrongParagraphs('<ol><li><p><strong>AAA</strong></p></li></ol>'),
    '<ol><li><p><strong>AAA</strong></p></li></ol>',
  )
})

test('renderChatMarkdown does not give a text-heavy paragraph the strong-title margin mid-stream', () => {
  const renderer = createChatMarkdownRenderer()
  const options = {
    renderer,
    escapeMarkdown: (text: string) => text,
    sanitizeHtml: (html: string) => html,
    streaming: true,
  }
  const head = '#### **\u603b\u7ed3\u4e0e\u5efa\u8bae**\n\n'
  // One bold run closed inside an otherwise text-heavy body paragraph.
  const midStream = head + '\u603b\u4f53\u6765\u770b\uff0c\u4f53\u73b0\u5728 **"\u8865\u77ed\u677f"\uff08\u9ad8\u7aef\u6838\u5fc3\u90e8\u4ef6\uff09** \u4e0e **'
  const html = renderChatMarkdown(midStream, options)
  assert.doesNotMatch(html, /<p class="md-strong-title">\u603b\u4f53\u6765\u770b/)
  assert.match(html, /<h4><strong>\u603b\u7ed3\u4e0e\u5efa\u8bae<\/strong><\/h4>/)
})

test('renderChatMarkdown preserves citations, math, and sanitized output through one shared pipeline', () => {
  const renderer = createChatMarkdownRenderer({
    imageRenderer: ({ href, title, text }) =>
      `<img src="${href}" alt="${text}" title="${title || ''}" class="markdown-image">`,
    isValidImageUrl: (href) => href.startsWith('https://'),
  })

  const html = renderChatMarkdown(
    [
      'See <kb doc="sample-product-guide.pdf" chunk_id="chunk-1" kb_id="kb-1"/>',
      '',
      'Formula: \\(E=mc^2\\)',
      '',
      '| A | B |',
      '| --- | --- |',
      '| 1 | 2 |',
      '',
      '![ok](https://example.com/a.png "\u56fe")',
      '',
      '![bad](javascript:alert(1))',
    ].join('\n'),
    {
      renderer,
      escapeMarkdown: (text) => text,
      sanitizeHtml: (html) => html.replace(/javascript:alert\(1\)/g, ''),
    },
  )

  assert.match(html, /class="citation citation-kb"/)
  assert.match(html, /data-chunk-id="chunk-1"/)
  assert.match(html, /katex/)
  assert.match(html, /<div class="chat-markdown-table"><table>/)
  assert.match(html, /<img src="https:\/\/example\.com\/a\.png"/)
  assert.doesNotMatch(html, /javascript:alert/)
})

test('resolveCitationChunkId maps context index to retrieval chunk UUID', () => {
  const refs = [
    { id: 'uuid-chunk-1', knowledge_title: 'Doc A', chunk_type: 'faq' },
    { id: 'uuid-chunk-2', knowledge_title: 'FAQ TEST - FAQ', chunk_type: 'faq' },
  ]

  assert.equal(
    resolveCitationChunkId('2', { doc: 'FAQ TEST - FAQ' }, refs),
    'uuid-chunk-2',
  )
  assert.equal(
    resolveCitationChunkId('FAQ-2', { doc: 'FAQ TEST - FAQ' }, refs),
    'uuid-chunk-2',
  )
  assert.equal(
    resolveCitationChunkId('uuid-chunk-1', { doc: 'Doc A' }, refs),
    'uuid-chunk-1',
  )
})

test('renderChatMarkdown preserves chunk UUIDs when escapeMarkdown strips UUIDs from prose', () => {
  const renderer = createChatMarkdownRenderer({
    imageRenderer: ({ href, text }) => `<img src="${href}" alt="${text}">`,
    isValidImageUrl: () => true,
  })
  const chunkId = SAMPLE_CHUNK_PRESERVE
  const stripUuids = (text: string) => text.replace(
    /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi,
    '',
  )

  const html = renderChatMarkdown(
    `Sample text <kb doc="sample-topic.pdf" chunk_id="${chunkId}" />`,
    {
      renderer,
      escapeMarkdown: stripUuids,
      sanitizeHtml: (html) => html,
    },
  )

  assert.match(html, /class="citation citation-kb"/)
  assert.match(html, new RegExp(`data-chunk-id="${chunkId}"`))
})

test('renderChatMarkdown resolves indexed chunk_id when knowledge references are provided', () => {
  const renderer = createChatMarkdownRenderer({
    imageRenderer: ({ href, text }) => `<img src="${href}" alt="${text}">`,
    isValidImageUrl: () => true,
  })

  const html = renderChatMarkdown(
    'See <kb doc="Sample FAQ" chunk_id="2" />',
    {
      renderer,
      escapeMarkdown: (text) => text,
      sanitizeHtml: (html) => html,
      knowledgeReferences: [
        { id: 'resolved-chunk-id', knowledge_title: 'Sample FAQ', chunk_type: 'faq' },
        { id: 'other-chunk', knowledge_title: 'Other FAQ', chunk_type: 'faq' },
      ],
    },
  )

  assert.match(html, /data-chunk-id="resolved-chunk-id"/)
  assert.doesNotMatch(html, /data-chunk-id="2"/)
})

test('joinCitationTagsToPreviousLine removes blank lines before citation tags', () => {
  const input = 'Setup is complete.\n\n<kb doc="faq.pdf" chunk_id="1" />'
  assert.equal(joinCitationTagsToPreviousLine(input), 'Setup is complete. <kb doc="faq.pdf" chunk_id="1" />')
})

test('joinCitationTagsToPreviousLine inlines consecutive citation tags across single newlines', () => {
  const tag1 = `<kb doc="${SAMPLE_DOC}" chunk_id="${SAMPLE_CHUNK_A}" />`
  const tag2 = `<kb doc="${SAMPLE_DOC}" chunk_id="${SAMPLE_CHUNK_B}" />`
  const tag3 = `<kb doc="${SAMPLE_DOC}" chunk_id="${SAMPLE_CHUNK_C}" />`
  const input = `${tag1}\n${tag2}\n${tag3}`
  assert.equal(joinCitationTagsToPreviousLine(input), `${tag1} ${tag2} ${tag3}`)
})

test('renderChatMarkdown inlines consecutive citation tags across newlines', () => {
  const renderer = createChatMarkdownRenderer({
    imageRenderer: ({ href, text }) => `<img src="${href}" alt="${text}">`,
    isValidImageUrl: () => true,
  })
  const html = renderChatMarkdown(
    [
      `<kb doc="${SAMPLE_DOC}" chunk_id="${SAMPLE_CHUNK_A}" />`,
      `<kb doc="${SAMPLE_DOC}" chunk_id="${SAMPLE_CHUNK_B}" />`,
      `<kb doc="${SAMPLE_DOC}" chunk_id="${SAMPLE_CHUNK_C}" />`,
    ].join('\n'),
    {
      renderer,
      escapeMarkdown: (text) => text,
      sanitizeHtml: (html) => html,
    },
  )

  assert.equal((html.match(/citation-kb/g) || []).length, 3)
  assert.doesNotMatch(html, /<\/p>\s*<p>\s*<span class="citation citation-kb"/)
})

test('joinCitationTagsToPreviousLine appends an indented citation to the preceding list item', () => {
  const tag = '<kb doc="\u9605\u8bfb\u4e4b\u661f\u5168\u56fd\u9752\u5c11\u5e74\u9605\u8bfb\u98ce\u91c7\u5c55\u793a\u6d3b\u52a8.pdf" chunk_id="chunk-1" />'
  const input = [
    '#### 5️⃣ \u9605\u8bfb\u4e4b\u661f\u57f9\u517b\u57fa\u5730',
    '- \u6bcf\u4e2a\u7ec4\u522b\u51a0\u4e9a\u5b63\u519b\u53ca\u524d\u5341\u5f3a\u6240\u5728\u7684\u5b66\u6821\uff0c\u5c06\u83b7\u5f97 **"\u9605\u8bfb\u4e4b\u661f\u57f9\u517b\u57fa\u5730"** \u5956\u724c',
    '',
    `  ${tag}`,
  ].join('\n')
  assert.equal(
    joinCitationTagsToPreviousLine(input),
    [
      '#### 5️⃣ \u9605\u8bfb\u4e4b\u661f\u57f9\u517b\u57fa\u5730',
      `- \u6bcf\u4e2a\u7ec4\u522b\u51a0\u4e9a\u5b63\u519b\u53ca\u524d\u5341\u5f3a\u6240\u5728\u7684\u5b66\u6821\uff0c\u5c06\u83b7\u5f97 **"\u9605\u8bfb\u4e4b\u661f\u57f9\u517b\u57fa\u5730"** \u5956\u724c ${tag}`,
    ].join('\n'),
  )
})

test('renderChatMarkdown renders a citation after a list item inline in that item', () => {
  const renderer = createChatMarkdownRenderer({
    imageRenderer: ({ href, text }) => `<img src="${href}" alt="${text}">`,
    isValidImageUrl: () => true,
  })
  const tag = '<kb doc="\u9605\u8bfb\u4e4b\u661f\u5168\u56fd\u9752\u5c11\u5e74\u9605\u8bfb\u98ce\u91c7\u5c55\u793a\u6d3b\u52a8.pdf" chunk_id="chunk-1" />'
  const html = renderChatMarkdown(`- \u57f9\u517b\u57fa\u5730\u5956\u724c\n\n  ${tag}`, {
    renderer,
    escapeMarkdown: (text) => text,
    sanitizeHtml: (value) => value,
  })

  assert.match(html, /<li>\u57f9\u517b\u57fa\u5730\u5956\u724c <span class="citation citation-kb"/)
  assert.doesNotMatch(html, /<\/ul>\s*<p>\s*<span class="citation citation-kb"/)
})

test('joinCitationTagsToPreviousLine does not merge citations onto fenced code closing delimiter', () => {
  const tag = '<kb doc="guide.pdf" chunk_id="1" />'
  const input = '```bash\nunzip setup.zip\n```\n\n' + tag
  assert.equal(joinCitationTagsToPreviousLine(input), '```bash\nunzip setup.zip\n```\n\n' + tag)
})

test('joinCitationTagsToPreviousLine does not merge citations onto an unlabeled closing fence on a single newline', () => {
  const tag = '<kb doc="guide.pdf" chunk_id="1" />'
  const input = '```\nAPR = principal\n```\n' + tag
  assert.equal(joinCitationTagsToPreviousLine(input), '```\nAPR = principal\n```\n' + tag)
})

test('applyStreamingTailFade wraps the trailing text run', () => {
  const out = applyStreamingTailFade('<p>Great, that narrows it down a lot. Two more quick</p>')
  assert.match(out, /<span class="stream-fade-tail">[^<]*Two more quick<\/span><\/p>$/)
})

test('applyStreamingTailFade skips whitespace-only runs and fades the last list item', () => {
  const out = applyStreamingTailFade('<ol>\n<li>\u7b2c\u4e00\u9879</li>\n<li>\u6b63\u5728\u751f\u6210\u7684\u7b2c\u4e8c\u9879</li>\n</ol>')
  assert.match(out, /<li><span class="stream-fade-tail">\u6b63\u5728\u751f\u6210\u7684\u7b2c\u4e8c\u9879<\/span><\/li>/)
})

test('applyStreamingTailFade is a no-op for empty content', () => {
  assert.equal(applyStreamingTailFade(''), '')
  assert.equal(applyStreamingTailFade('<p></p>'), '<p></p>')
})

test('renderChatMarkdown adds the tail fade only while streaming', () => {
  const renderer = createChatMarkdownRenderer()
  const opts = {
    renderer,
    escapeMarkdown: (text: string) => text,
    sanitizeHtml: (html: string) => html,
  }
  const streamed = renderChatMarkdown('\u6b63\u5728\u751f\u6210\u4e2d\u7684\u56de\u7b54\u5185\u5bb9', { ...opts, streaming: true })
  assert.match(streamed, /stream-fade-tail/)
  const settled = renderChatMarkdown('\u6b63\u5728\u751f\u6210\u4e2d\u7684\u56de\u7b54\u5185\u5bb9', { ...opts, streaming: false })
  assert.doesNotMatch(settled, /stream-fade-tail/)
})

test('renderChatMarkdown keeps an unlabeled fenced code block closed when a citation immediately follows', () => {
  const renderer = createChatMarkdownRenderer()
  const tag = '<kb doc="guide.pdf" chunk_id="1" />'
  const html = renderChatMarkdown(
    ['```', 'APR = principal', '```', tag, '', '### \u91cd\u8981\u6027'].join('\n'),
    {
      renderer,
      escapeMarkdown: (text) => text,
      sanitizeHtml: (html) => html,
    },
  )

  assert.doesNotMatch(html, /### \u91cd\u8981\u6027/)
  assert.match(html, /<h3>\u91cd\u8981\u6027<\/h3>/)
  assert.equal((html.match(/<pre>/g) || []).length, 1)
})

test('renderChatMarkdown keeps fenced code blocks closed when citations follow', () => {
  const renderer = createChatMarkdownRenderer({
    imageRenderer: ({ href, text }) => `<img src="${href}" alt="${text}">`,
    isValidImageUrl: () => true,
  })
  const tag = '<kb doc="guide.pdf" chunk_id="1" />'
  const html = renderChatMarkdown(
    ['```bash', 'unzip setup.zip', '```', '', tag, '', '#### Next step'].join('\n'),
    {
      renderer,
      escapeMarkdown: (text) => text,
      sanitizeHtml: (html) => html,
    },
  )

  assert.doesNotMatch(html, /#### Next step/)
  assert.match(html, /<h4>Next step<\/h4>/)
  assert.equal((html.match(/<pre>/g) || []).length, 1)
})

test('collapseStandaloneCitationParagraphs merges citations across empty paragraphs', () => {
  const html = '<p>Steps:</p><p></p><p><span class="citation citation-kb" data-chunk-id="x">doc</span></p>'
  const out = collapseStandaloneCitationParagraphs(html)
  assert.match(out, /Steps:.*citation-kb/s)
  assert.doesNotMatch(out, /<p><\/p>/)
})

test('renderChatMarkdown keeps ~~strikethrough~~ while leaving single tildes literal', () => {
  const renderer = createChatMarkdownRenderer()
  const options = {
    renderer,
    escapeMarkdown: (text: string) => text,
    sanitizeHtml: (html: string) => html,
    streaming: false,
  }
  const html = renderChatMarkdown('~~a~~ and ~b~ and 2020~2035, 8:00~12:00.', options)
  assert.equal((html.match(/<del>/g) || []).length, 1)
  assert.match(html, /<del>a<\/del>/)
  assert.match(html, /~b~/)
  assert.match(html, /2020~2035/)
  assert.match(html, /8:00~12:00/)
})
