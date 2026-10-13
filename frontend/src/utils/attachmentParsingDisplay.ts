import type { ComposerTranslation } from 'vue-i18n'

type AttachmentParsingEvent = {
  success?: boolean
  output?: string
  error?: string
  tool_data?: Record<string, unknown> | null
}

export function resolveAttachmentParsingCounts(event: AttachmentParsingEvent): {
  parsed: number
  skipped: number
} {
  const toolData = event.tool_data
  if (toolData && toolData.parsed_count !== undefined) {
    return {
      parsed: Number(toolData.parsed_count) || 0,
      skipped: Number(toolData.skipped_count) || 0,
    }
  }

  return parseAttachmentOutput(event.output)
}

function parseAttachmentOutput(output?: string): { parsed: number; skipped: number } {
  if (!output) return { parsed: 0, skipped: 0 }

  // Matches the attachment_parsing summary built in internal/handler/session/qa.go.
  const parsedMatch = output.match(/Parsed\s*(\d+)\s*attachment/i)
  const skippedMatch = output.match(/(\d+)\s*skipped/i)

  return {
    parsed: Number(parsedMatch?.[1] ?? 0) || 0,
    skipped: Number(skippedMatch?.[1] ?? 0) || 0,
  }
}

export function getAttachmentParsingSummaryHtml(
  t: ComposerTranslation,
  event: AttachmentParsingEvent,
): string {
  if (event.success === false) {
    const err = String(event.error || event.output || '').trim()
    if (!err) return ''
    const normalized = err.replace(/^Attachment parsing failed:\s*/i, '').trim()
    return normalized || err
  }

  const { parsed, skipped } = resolveAttachmentParsingCounts(event)
  if (parsed === 0 && skipped === 0) {
    return t('agentStream.attachmentParsing.noneReady')
  }
  if (skipped > 0) {
    return t('agentStream.attachmentParsing.parsedWithSkipped', {
      parsed: `<strong>${parsed}</strong>`,
      skipped: `<strong>${skipped}</strong>`,
    })
  }
  return t('agentStream.attachmentParsing.parsedSummary', {
    count: `<strong>${parsed}</strong>`,
  })
}
