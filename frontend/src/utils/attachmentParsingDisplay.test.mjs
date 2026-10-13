import assert from 'node:assert/strict'
import test from 'node:test'

import {
  getAttachmentParsingSummaryHtml,
  resolveAttachmentParsingCounts,
} from './attachmentParsingDisplay.ts'

const t = (key, params) => {
  if (key === 'agentStream.attachmentParsing.parsedSummary') {
    return `Parsed ${params?.count} attachment(s)`
  }
  if (key === 'agentStream.attachmentParsing.parsedWithSkipped') {
    return `Parsed ${params?.parsed} attachment(s), ${params?.skipped} skipped (still processing)`
  }
  if (key === 'agentStream.attachmentParsing.noneReady') {
    return 'No parsed attachments available'
  }
  return key
}

test('resolveAttachmentParsingCounts prefers structured tool_data', () => {
  assert.deepEqual(
    resolveAttachmentParsingCounts({
      tool_data: { parsed_count: 2, skipped_count: 1 },
    }),
    { parsed: 2, skipped: 1 },
  )
})

test('resolveAttachmentParsingCounts falls back to the output text', () => {
  assert.deepEqual(
    resolveAttachmentParsingCounts({
      output: 'Parsed 3 attachment(s), 1 skipped (still processing)',
    }),
    { parsed: 3, skipped: 1 },
  )
})

test('getAttachmentParsingSummaryHtml renders parsed count', () => {
  const html = getAttachmentParsingSummaryHtml(t, {
    success: true,
    tool_data: { parsed_count: 1, skipped_count: 0 },
  })
  assert.equal(html, 'Parsed <strong>1</strong> attachment(s)')
})

test('getAttachmentParsingSummaryHtml renders skipped count', () => {
  const html = getAttachmentParsingSummaryHtml(t, {
    success: true,
    tool_data: { parsed_count: 2, skipped_count: 1 },
  })
  assert.equal(html, 'Parsed <strong>2</strong> attachment(s), <strong>1</strong> skipped (still processing)')
})
