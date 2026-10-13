import Papa from 'papaparse'
import * as XLSX from 'xlsx'

export type FAQAnswerStrategy = 'all' | 'random'

export interface FAQEntryPayload {
  standard_question: string
  similar_questions: string[]
  negative_questions: string[]
  answers: string[]
  answer_strategy?: FAQAnswerStrategy
  tag_id?: number
  tag_name?: string
  is_enabled?: boolean
  is_recommended?: boolean
}

// Legacy Chinese export headers, still accepted on import.
const LEGACY = {
  question: '\u95ee\u9898',
  answers: '\u673a\u5668\u4eba\u56de\u7b54',
  similar: '\u76f8\u4f3c\u95ee\u9898',
  negative: '\u53cd\u4f8b\u95ee\u9898',
  tag: '\u6807\u7b7e',
  category: '\u5206\u7c7b',
  disabled: '\u662f\u5426\u505c\u7528',
}

// Multi-value cells are split on ## only, so commas and semicolons inside an answer survive.
export const splitByDelimiter = (value?: string): string[] => {
  const trimmed = value?.trim()
  if (!trimmed) return []
  return trimmed.split('##').map(item => item.trim()).filter(Boolean)
}

// TRUE/FALSE, true/false, 1/0, YES/NO and the legacy Chinese yes/no; anything else is false.
export const parseBooleanField = (value?: string): boolean | undefined => {
  if (!value) return undefined
  const normalized = value.trim().toUpperCase()
  return normalized === 'TRUE' || normalized === '1' || normalized === '\u662f' || normalized === 'YES'
}

export const normalizeFAQImportPayload = (payload: Partial<FAQEntryPayload>): FAQEntryPayload => ({
  standard_question: payload.standard_question || '',
  answers: payload.answers?.filter(Boolean) || [],
  similar_questions: payload.similar_questions?.filter(Boolean) || [],
  negative_questions: payload.negative_questions?.filter(Boolean) || [],
  answer_strategy:
    payload.answer_strategy === 'all' || payload.answer_strategy === 'random' ? payload.answer_strategy : undefined,
  tag_id: payload.tag_id || undefined,
  tag_name: payload.tag_name || '',
  is_enabled: typeof payload.is_enabled === 'boolean' ? payload.is_enabled : undefined,
  is_recommended: typeof payload.is_recommended === 'boolean' ? payload.is_recommended : undefined,
})

// Headers are matched with their parenthesized notes stripped; English names are case-insensitive.
const normalizeHeader = (header: string): string => {
  const cleaned = header.replace(/^\ufeff/, '').trim().replace(/\([^)]*\)/g, '').trim()
  return /[\u4e00-\u9fa5]/.test(cleaned) ? cleaned : cleaned.toLowerCase()
}

const invert = (value: boolean | undefined) => (value === undefined ? undefined : !value)

// One CSV or spreadsheet row in the template / export column layout.
export const mapFAQImportRow = (row: Record<string, unknown>): FAQEntryPayload => {
  const r: Record<string, string> = {}
  for (const key of Object.keys(row)) {
    r[normalizeHeader(key)] = String(row[key] ?? '').trim()
  }
  const answerAll = parseBooleanField(r['answer_all'])
  return normalizeFAQImportPayload({
    standard_question: r[LEGACY.question] || r['standard_question'] || r['question'] || '',
    answers: splitByDelimiter(r[LEGACY.answers] || r['answers']),
    similar_questions: splitByDelimiter(r[LEGACY.similar] || r['similar_questions']),
    negative_questions: splitByDelimiter(r[LEGACY.negative] || r['negative_questions']),
    answer_strategy: answerAll === undefined ? undefined : answerAll ? 'all' : 'random',
    tag_id: r['tag_id'] ? Number(r['tag_id']) : undefined,
    tag_name: r[LEGACY.tag] || r[LEGACY.category] || r['tag_name'] || '',
    is_enabled: invert(parseBooleanField(r[LEGACY.disabled] || r['is_disabled'])),
    is_recommended: invert(parseBooleanField(r['not_recommended'])),
  })
}

// Returns null when the document is not an array, so the caller can show its own message.
export const parseFAQImportJSON = (text: string): FAQEntryPayload[] | null => {
  const data = JSON.parse(text)
  return Array.isArray(data) ? data.map(normalizeFAQImportPayload) : null
}

export const parseFAQImportCSV = (text: string): FAQEntryPayload[] => {
  const results = Papa.parse<Record<string, string>>(text.replace(/^\ufeff/, ''), {
    header: true,
    skipEmptyLines: true,
    delimiter: '', // auto-detect comma or tab
    quoteChar: '"',
    escapeChar: '"',
  })
  if (results.errors.length && !results.data.length) {
    throw new Error(`CSV parse failed: ${results.errors[0].message}`)
  }
  return results.data.map(mapFAQImportRow)
}

export const parseFAQImportWorkbook = (data: ArrayBuffer): FAQEntryPayload[] => {
  const workbook = XLSX.read(data, { type: 'array' })
  const worksheet = workbook.Sheets[workbook.SheetNames[0]]
  return XLSX.utils.sheet_to_json<Record<string, string>>(worksheet, { defval: '', raw: false }).map(mapFAQImportRow)
}
