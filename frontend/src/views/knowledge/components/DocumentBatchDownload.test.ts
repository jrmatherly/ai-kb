import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'

const knowledgeBase = readFileSync(new URL('../KnowledgeBase.vue', import.meta.url), 'utf8')
const batchBar = readFileSync(new URL('./DocumentBatchBar.vue', import.meta.url), 'utf8')
const listView = readFileSync(new URL('./DocumentListView.vue', import.meta.url), 'utf8')
const knowledgeApi = readFileSync(new URL('../../../api/knowledge-base/index.ts', import.meta.url), 'utf8')
const request = readFileSync(new URL('../../../utils/request.ts', import.meta.url), 'utf8')

test('batch download uses an authenticated Blob request and supports cancel', () => {
  assert.match(knowledgeApi, /knowledge-bases\/\$\{encodeURIComponent\(kbId\)\}\/knowledge\/batch-download/)
  assert.match(knowledgeApi, /responseType:\s*'blob'/)
  assert.match(knowledgeApi, /signal,/)
  assert.match(knowledgeBase, /new AbortController\(\)/)
  assert.match(knowledgeBase, /batchDownloadController\?\.abort\(\)/)
})

test('the batch download UI caps a batch at 200 items and keeps the read-only permission boundary', () => {
  assert.match(batchBar, /v-if="canDownload"/)
  assert.match(batchBar, /count > 200/)
  assert.match(batchBar, /batch-download-trigger/)
  assert.match(batchBar, /v-if="canMutate"/)
  assert.match(knowledgeBase, /isBatchDownloadableKnowledge/)
  assert.match(knowledgeBase, /ids\.length > MAX_BATCH_DOWNLOAD_FILES/)
  assert.match(knowledgeBase, /:can-download="canDownloadKnowledge"/)
  assert.match(knowledgeBase, /@select-loaded="toggleSelectAll\(true\)"/)
  assert.match(listView, /v-if="canEdit \|\| canDownload"/)
})

test('a JSON error in Blob form is restored to a readable message', () => {
  assert.match(request, /error\.response\.data instanceof Blob/)
  assert.match(request, /JSON\.parse\(text\)/)
  assert.match(request, /text\.startsWith\('\{'\)/)
})
