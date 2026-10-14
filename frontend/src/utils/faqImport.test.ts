import { test } from 'node:test'
import assert from 'node:assert/strict'
import * as XLSX from 'xlsx'
import { parseFAQImportCSV, parseFAQImportJSON, parseFAQImportWorkbook } from './faqImport'

// Byte-for-byte what GET /knowledge-bases/:id/faq/entries/export writes (buildFAQCSV plus the handler's BOM).
const exportedCSV =
  '\ufefftag_name(required),standard_question(required),' +
  'similar_questions(optional - separate multiple with ##),negative_questions(optional - separate multiple with ##),' +
  'answers(required - separate multiple with ##),answer_all(optional - default FALSE),' +
  'is_disabled(optional - default FALSE),not_recommended(optional - default FALSE means recommendable)\n' +
  'Billing,How do I pay?,Payment methods##Pay by card,,"Use a card, or PayPal##Ask for an invoice",TRUE,FALSE,TRUE\n' +
  'General,Opening hours,,When are you closed,9 to 5,FALSE,TRUE,FALSE\n'

// What ?format=json writes (types.FAQExportEntry).
const exportedJSON = JSON.stringify([
  {
    id: 1, tag_name: 'Billing', standard_question: 'How do I pay?',
    similar_questions: ['Payment methods', 'Pay by card'],
    answers: ['Use a card, or PayPal', 'Ask for an invoice'],
    answer_strategy: 'all', is_enabled: true, is_recommended: false,
  },
  {
    id: 2, tag_name: 'General', standard_question: 'Opening hours',
    negative_questions: ['When are you closed'], answers: ['9 to 5'],
    answer_strategy: 'random', is_enabled: false, is_recommended: true,
  },
])

const expected = [
  {
    standard_question: 'How do I pay?',
    answers: ['Use a card, or PayPal', 'Ask for an invoice'],
    similar_questions: ['Payment methods', 'Pay by card'],
    negative_questions: [],
    answer_strategy: 'all',
    tag_id: undefined,
    tag_name: 'Billing',
    is_enabled: true,
    is_recommended: false,
  },
  {
    standard_question: 'Opening hours',
    answers: ['9 to 5'],
    similar_questions: [],
    negative_questions: ['When are you closed'],
    answer_strategy: 'random',
    tag_id: undefined,
    tag_name: 'General',
    is_enabled: false,
    is_recommended: true,
  },
]

test('a CSV export re-imports with its answer strategy and recommendation flags', () => {
  assert.deepEqual(parseFAQImportCSV(exportedCSV), expected)
})

test('a JSON export re-imports with its answer strategy and recommendation flags', () => {
  assert.deepEqual(parseFAQImportJSON(exportedJSON), expected)
})

test('a CSV export saved as Excel re-imports with its flags', async () => {
  const workbook = XLSX.read(exportedCSV, { type: 'string' })
  const bytes: ArrayBuffer = XLSX.write(workbook, { type: 'array', bookType: 'xlsx' })
  assert.deepEqual(await parseFAQImportWorkbook(bytes), expected)
})

test('a template row without the optional flag columns leaves them to the server defaults', () => {
  const [entry] = parseFAQImportCSV('standard_question,answers\nHi,Hello\n')
  assert.equal(entry.answer_strategy, undefined)
  assert.equal(entry.is_enabled, undefined)
  assert.equal(entry.is_recommended, undefined)
})

test('a JSON document that is not an array is rejected', () => {
  assert.equal(parseFAQImportJSON('{"entries": []}'), null)
})
