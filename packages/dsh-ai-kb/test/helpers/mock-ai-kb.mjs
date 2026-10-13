/**
 * A stand-in AI KB backend used by the unit tests and by the end-to-end run
 * inside dsh. It speaks the same routes, envelopes and SSE event types as the
 * real server (see internal/handler/session/qa.go and internal/types/chat.go),
 * so a test failure here means the plugin, not the fixture, drifted.
 */

import { createServer } from 'node:http'

const DOCUMENTS = [
  {
    knowledge_id: 'doc-retrieval-pipeline',
    knowledge_title: 'AI KB retrieval pipeline.md',
    knowledge_base_id: 'kb-product',
    // AI KB stores the generated summary in `description`, not `summary`.
    description: 'How AI KB hybrid retrieval recalls passages, its thresholds, and parent-child chunk expansion.',
    chunks: [
      'AI KB hybrid retrieval runs vector recall, then keyword recall, and hands both to the rerank model for one ranking.',
      'The default vector_threshold is 0.5 and keyword_threshold is 0.3; each knowledge base can override them in its settings.',
      'With parent-child chunking on, a matched child chunk expands to its parent, giving the generation model fuller context.',
    ],
  },
  {
    knowledge_id: 'doc-architecture',
    knowledge_title: 'System architecture diagram.md',
    knowledge_base_id: 'kb-product',
    description: 'A system architecture diagram of the retrieval and generation layers.',
    chunks: [
      'The system has a retrieval layer and a generation layer; the architecture diagram shows the overall structure.\n\n![System architecture](resource://AbCdEfGhIjKlMnOpQrStUv)',
    ],
  },
  {
    knowledge_id: 'doc-deployment',
    knowledge_title: 'AI KB deployment guide.md',
    knowledge_base_id: 'kb-ops',
    description: 'Single-node and Kubernetes deployment, and the vector store backends to choose from.',
    chunks: [
      'AI KB supports single-node deployment with docker compose and ships a Helm chart for Kubernetes.',
      'The vector store can be pgvector, Milvus, Qdrant, Weaviate, OpenSearch or another backend.',
    ],
  },
  {
    // Title and body share no wording on purpose: retrieving this document by
    // name is the only way to reach it, which is what the by-name search is for.
    knowledge_id: 'doc-release-checklist',
    knowledge_title: 'Release checklist.md',
    knowledge_base_id: 'kb-ops',
    description: 'What to confirm before going live, and rollback preparation.',
    chunks: [
      'Before going live, confirm that monitoring alerts, the rollback plan and database migration scripts are ready, and have the on-call engineer review them.',
    ],
  },
]

// Titled so a by-name query matches it while its text shares nothing with that
// query — the case passage retrieval alone cannot serve.
DOCUMENTS.push({
  knowledge_id: 'doc-release-checklist',
  knowledge_title: 'Release checklist.md',
  knowledge_base_id: 'kb-ops',
  description: 'What to confirm before going live, and the rollback plan.',
  chunks: ['This page lists the settings to confirm and the rollback plan before going live.'],
})

const KNOWLEDGE_BASES = [
  { id: 'kb-product', name: 'Product docs', description: 'AI KB product and retrieval docs' },
  { id: 'kb-ops', name: 'Ops runbooks', description: 'Deployment and operations runbooks' },
]

const ARCH_HANDLE = 'resource://AbCdEfGhIjKlMnOpQrStUv'
const ARCH_PUBLIC = 'https://cdn.example.com/architecture.png'

/** Mirror AI KB's `resource_urls=public` rewrite of `resource://` handles. */
function rewriteResources(value, publicMode) {
  if (!publicMode) return value
  return JSON.parse(JSON.stringify(value).replaceAll(ARCH_HANDLE, ARCH_PUBLIC))
}

/**
 * Score a passage by the share of query words (three letters or more) it
 * contains. The real backend scores with embeddings plus keyword match; word
 * overlap is enough to make the fixture rank plausibly. Character bigrams
 * would match nearly every English passage and leave no query unmatched.
 */
function scoreOf(query, content) {
  const words = new Set(query.toLowerCase().split(/[^\p{L}\p{N}_]+/u).filter(word => word.length >= 3))
  if (words.size === 0) return 0
  const text = content.toLowerCase()
  const hits = [...words].filter(word => text.includes(word)).length
  return Math.min(0.99, 0.4 + (hits / words.size) * 0.5)
}

function searchResults(query, knowledgeBaseIds, knowledgeIds) {
  const documents = DOCUMENTS.filter(document =>
    (knowledgeIds.length === 0 || knowledgeIds.includes(document.knowledge_id))
    && (knowledgeBaseIds.length === 0 || knowledgeBaseIds.includes(document.knowledge_base_id)))
  return documents
    .flatMap(document => document.chunks.map((content, index) => ({
      id: `${document.knowledge_id}-chunk-${index}`,
      content,
      knowledge_id: document.knowledge_id,
      knowledge_title: document.knowledge_title,
      chunk_index: index,
      score: scoreOf(query, content),
      match_type: 2,
      start_at: 0,
      end_at: content.length,
      metadata: {},
    })))
    .filter(result => result.score > 0.4)
    .sort((left, right) => right.score - left.score)
}

/** The document shape the by-name search and `GET /knowledge/:id` both return. */
function documentRecord(document) {
  return {
    id: document.knowledge_id,
    title: document.knowledge_title,
    file_name: document.knowledge_title,
    file_type: 'md',
    description: document.description,
    knowledge_base_id: document.knowledge_base_id,
    knowledge_base_name: KNOWLEDGE_BASES.find(kb => kb.id === document.knowledge_base_id)?.name ?? '',
    parse_status: 'completed',
    summary_status: 'completed',
  }
}

/** Assemble the answer the fake RAG/agent pipeline streams back. */
function answerFor(query, results) {
  if (results.length === 0) return `Nothing related to "${query}" was retrieved.`
  const cited = results.slice(0, 2).map(result => result.content).join(' ')
  return `From the knowledge base: ${cited} (question: ${query})`
}

/**
 * Start the mock backend.
 * @param options.apiKey - when set, requests must carry it as `X-API-Key`.
 * @param options.streamError - make the chat routes stream an `error` event.
 * @param options.streamTruncated - end the chat stream mid-answer, with no `complete`.
 * @param options.forbidPublicResourceUrls - reject `resource_urls=public` with the
 *   403 AI KB returns for a knowledge-base-restricted API key.
 * @returns the base URL, the recorded requests, and a close function.
 */
export async function startMockAikb(options = {}) {
  const requests = []
  // Paths forced to fail, so a test can prove the plugin degrades rather than
  // breaks when a deployment does not serve one of the optional routes.
  const failures = new Map()
  const server = createServer((request, response) => {
    const url = new URL(request.url, 'http://mock.invalid')
    const chunks = []
    request.on('data', chunk => chunks.push(chunk))
    request.on('end', () => {
      const rawBody = Buffer.concat(chunks).toString('utf8')
      let body = {}
      if (rawBody !== '') {
        try {
          body = JSON.parse(rawBody)
        } catch {
          body = { _unparsed: rawBody }
        }
      }
      requests.push({
        method: request.method,
        path: url.pathname,
        query: Object.fromEntries(url.searchParams),
        headers: request.headers,
        body,
      })

      const json = (status, payload) => {
        response.writeHead(status, { 'Content-Type': 'application/json' })
        response.end(JSON.stringify(payload))
      }

      if (options.apiKey !== undefined && request.headers['x-api-key'] !== options.apiKey) {
        json(401, { success: false, error: { message: 'invalid api key' } })
        return
      }

      if (options.forbidPublicResourceUrls === true && url.searchParams.get('resource_urls') === 'public') {
        json(403, {
          success: false,
          error: { message: 'resource_urls=public is not available for a knowledge-base-restricted API key' },
        })
        return
      }

      const publicMode = url.searchParams.get('resource_urls') === 'public'

      const forced = failures.get(url.pathname)
      if (forced !== undefined) {
        json(forced, { success: false, error: { message: `forced failure for ${url.pathname}` } })
        return
      }

      if (request.method === 'GET' && url.pathname === '/api/v1/knowledge-bases') {
        json(200, { success: true, data: KNOWLEDGE_BASES })
        return
      }

      // By-name document search. Spans every knowledge base and takes no
      // scope, matching the real /knowledge/search route.
      if (request.method === 'GET' && url.pathname === '/api/v1/knowledge/search') {
        const keyword = (url.searchParams.get('keyword') ?? url.searchParams.get('query') ?? '').trim()
        if (keyword === '') {
          json(400, { success: false, error: { message: 'missing search keyword: pass ?keyword=... or ?query=...' } })
          return
        }
        const limit = Number(url.searchParams.get('limit') ?? '20')
        json(200, {
          success: true,
          data: DOCUMENTS
            .filter(document => document.knowledge_title.toLowerCase().includes(keyword.toLowerCase()))
            .slice(0, limit)
            .map(document => documentRecord(document)),
        })
        return
      }

      if (request.method === 'GET' && url.pathname.startsWith('/api/v1/knowledge/')) {
        const knowledgeId = decodeURIComponent(url.pathname.slice('/api/v1/knowledge/'.length))
        const document = DOCUMENTS.find(entry => entry.knowledge_id === knowledgeId)
        if (document === undefined) {
          json(404, { success: false, error: { message: 'knowledge not found' } })
          return
        }
        json(200, { success: true, data: documentRecord(document) })
        return
      }

      if (request.method === 'POST' && url.pathname === '/api/v1/knowledge-search') {
        if (typeof body.query !== 'string' || body.query === '') {
          json(400, { success: false, error: { message: 'Query content cannot be empty' } })
          return
        }
        // Mirror the real handler, which refuses an unscoped retrieval
        // (internal/handler/session/qa.go). Answering one here would let the
        // plugin ship a call every deployment rejects.
        if ((body.knowledge_base_ids ?? []).length === 0
          && (body.knowledge_ids ?? []).length === 0
          && (body.tag_ids ?? []).length === 0) {
          json(400, {
            success: false,
            error: {
              message: 'At least one knowledge_base_id, knowledge_base_ids, knowledge_ids, or scoped tag must be provided',
            },
          })
          return
        }
        json(200, {
          success: true,
          data: rewriteResources(
            searchResults(body.query, body.knowledge_base_ids ?? [], body.knowledge_ids ?? []),
            publicMode,
          ),
        })
        return
      }

      if (request.method === 'GET' && url.pathname.startsWith('/api/v1/chunks/')) {
        const knowledgeId = decodeURIComponent(url.pathname.slice('/api/v1/chunks/'.length))
        const document = DOCUMENTS.find(entry => entry.knowledge_id === knowledgeId)
        if (document === undefined) {
          json(404, { success: false, error: { message: 'knowledge not found' } })
          return
        }
        const page = Number(url.searchParams.get('page') ?? '1')
        const pageSize = Number(url.searchParams.get('page_size') ?? '10')
        const start = (page - 1) * pageSize
        json(200, {
          success: true,
          data: document.chunks.slice(start, start + pageSize).map((content, index) => ({
            id: `${knowledgeId}-chunk-${start + index}`,
            knowledge_id: knowledgeId,
            knowledge_base_id: 'kb-product',
            content,
            chunk_index: start + index,
            is_enabled: true,
          })),
          total: document.chunks.length,
          page,
          page_size: pageSize,
        })
        return
      }

      if (request.method === 'POST' && url.pathname === '/api/v1/sessions') {
        json(201, { success: true, data: { id: 'session-mock-1', title: body.title ?? '' } })
        return
      }

      const chatMatch = /^\/api\/v1\/(knowledge-chat|agent-chat)\/(.+)$/.exec(url.pathname)
      if (request.method === 'POST' && chatMatch !== null) {
        const [, route, sessionId] = chatMatch
        response.writeHead(200, {
          'Content-Type': 'text/event-stream',
          'Cache-Control': 'no-cache',
          'Connection': 'keep-alive',
        })
        const send = payload => response.write(`event:message\ndata:${JSON.stringify(payload)}\n\n`)
        if (options.streamError === true) {
          send({ id: 'e1', response_type: 'error', content: 'model provider unavailable', done: true })
          response.end()
          return
        }
        // The RAG route retrieves only what the request scopes: a session holds
        // no knowledge base of its own (CreateSessionRequest carries none), so
        // AI KB has no default to fall back on and answers from nothing.
        // Retrieving here anyway would hide an unscoped ask from the tests. The
        // agent route does have a server-side default, its KBSelectionMode.
        const scoped = (body.knowledge_base_ids ?? []).length > 0 || (body.knowledge_ids ?? []).length > 0
        const results = rewriteResources(
          route === 'agent-chat' || scoped
            ? searchResults(body.query ?? '', body.knowledge_base_ids ?? [], [])
            : [],
          publicMode,
        )
        if (route === 'agent-chat') {
          send({
            id: 't1',
            response_type: 'tool_call',
            content: '',
            done: false,
            session_id: sessionId,
            tool_calls: [{ id: 'call-1', function: { name: 'knowledge_search', arguments: '{}' } }],
          })
        }
        send({ id: 'r1', response_type: 'references', content: '', done: false, knowledge_references: results })
        const pieces = answerFor(body.query ?? '', results).match(/.{1,24}/gs) ?? []
        // A stream cut off mid-answer, which is what a dropped connection or a
        // killed backend looks like: answer deltas but no `complete`.
        if (options.streamTruncated === true) {
          send({ id: 'a1', response_type: 'answer', content: pieces[0] ?? '', done: false, session_id: sessionId })
          response.end()
          return
        }
        for (const piece of pieces) {
          send({ id: 'a1', response_type: 'answer', content: piece, done: false, session_id: sessionId })
        }
        send({ id: 'c1', response_type: 'complete', content: '', done: true, session_id: sessionId })
        response.end()
        return
      }

      json(404, { success: false, error: { message: `no route for ${request.method} ${url.pathname}` } })
    })
  })

  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  const { port } = server.address()
  return {
    url: `http://127.0.0.1:${port}/api/v1`,
    requests,
    /** Force `path` to answer `status` from now on. */
    fail(path, status) {
      failures.set(path, status)
    },
    async close() {
      await new Promise(resolve => server.close(resolve))
    },
  }
}

export { ARCH_HANDLE, ARCH_PUBLIC, DOCUMENTS, KNOWLEDGE_BASES }
