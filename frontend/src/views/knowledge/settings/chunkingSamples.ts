// Curated sample texts for the chunking debug drawer. Each preset is sized
// to ≈3000–4500 characters and shaped to exercise a distinct tier of the
// chunker, so users can quickly see how their config behaves on realistic
// content without preparing their own sample.

export interface ChunkingSample {
  id: string
  // i18n key under knowledgeEditor.chunking.debug.samples.<id>
  labelKey: string
  text: string
}

const MARKDOWN_SAMPLE = `# The AI KB Knowledge Framework

AI KB is an open-source, LLM-based enterprise knowledge framework that combines RAG question answering, ReAct agents and a wiki knowledge graph. This document covers its motivation, architecture and typical usage.

## Motivation

Internal company knowledge is scattered across Confluence, Feishu, Notion, Git repositories and other systems. Traditional full-text search struggles with meaning, and an LLM on its own lacks a trustworthy source of context. AI KB aims for:

- **Multi-source ingestion**: extract, clean and vectorize scattered content in one pipeline
- **Multi-strategy retrieval**: dense, sparse and knowledge graph recall paths + RRF fusion
- **Explainable reasoning**: ReAct agents make the reasoning chain traceable and steerable
- **Observability**: native Langfuse integration, with a trace for every reasoning step and every tool call

## Core features

### Multi-model support

Compatible with 20+ mainstream LLMs: OpenAI, Anthropic, DeepSeek, Qwen, Zhipu, Hunyuan, Gemini, and local Ollama. The model layer abstracts a \`Provider\` interface, so a new vendor only needs to implement the \`Chat()\` method and register it.

### Multiple vector stores

Switch with the \`RETRIEVE_DRIVER\` environment variable:

| Driver | Best for |
|------|---------|
| pgvector | Existing PostgreSQL, under 10 million chunks |
| Elasticsearch | Hybrid retrieval + keyword highlighting |
| Milvus | Large scale, low latency |
| Weaviate | Hybrid BM25 + GraphQL queries |
| Qdrant | Recommended default, simple to deploy, balanced performance |

### Data source connectors

- **Feishu**: syncs knowledge bases and document trees automatically, with incremental updates
- **Notion**: extracts Pages / Databases through the API
- **Yuque**: pulls by Group, keeping the multi-level directory

Every connector implements the shared \`DataSource\` interface; a new data source takes about 200 lines of code.

## Quick start

### Requirements

- Docker 20.10+ and Docker Compose v2
- At least 8GB of memory and 20GB of disk space
- To run an LLM locally, an NVIDIA GPU with 24GB or more of VRAM is recommended

### Start commands

\`\`\`bash
git clone https://github.com/jrmatherly/ai-kb && cd ai-kb
cp .env.example .env       # set your model and database config
make dev-start             # start postgres / redis / qdrant
make dev-app               # start the backend with hot reload
make dev-frontend          # start the frontend with auto refresh
\`\`\`

Then open http://localhost:5173 in a browser.

## Architecture overview

The AI KB backend uses a clean layered architecture:

\`\`\`
┌────────────────────────┐
│   HTTP layer (Gin)     │  request routing / auth / rate limiting
├────────────────────────┤
│   Service layer        │  core algorithms / orchestration
├────────────────────────┤
│   Repository layer     │  database access / caching
├────────────────────────┤
│   Domain model (types) │  KnowledgeBase / Chunk / Agent ...
└────────────────────────┘
\`\`\`

The dependency injection container (go/dig) wires every dependency in \`cmd/server/main.go\`: handlers get services through interfaces, services get repositories through interfaces, and every replaceable implementation is bound at startup.

### Retrieval pipeline

Document upload → parsed by the docreader service → chunking (parent-child chunks included) → embedding → written to the vector store and graph database → multi-path recall at query time → RRF fusion → rerank → response.

### Agent orchestration

The agent engine runs the classic ReAct loop:

1. **Reason**: the LLM outputs its reasoning from the system prompt and the current observation
2. **Act**: parse the tool call JSON and dispatch it to a built-in tool or an MCP tool
3. **Observe**: put the tool's return value back into the context
4. Repeat 1-3 until a final answer is produced or the iteration limit is reached

Every step streams to the frontend over SSE and is written to a Langfuse trace for troubleshooting.

## Further reading

- API docs: \`website-docs/04-api/01-api-overview.md\`
- Config reference: \`config/config.yaml\` and \`.env.example\`
- Troubleshooting: \`website-docs/01-getting-started/05-troubleshooting.md\`
- Product introduction: \`website-docs/01-getting-started/01-introduction.md\``

const FAQ_SAMPLE = `# AI KB Deployment and Usage FAQ

This document collects the questions community and internal users ask most, grouped as "install / config / retrieval / models / performance".

## Install and start

### Q1: Where do I download the Docker images?
The official images are distributed through a registry mirror:

\`\`\`
docker pull ghcr.io/jrmatherly/ai-kb-app:v0.5.0
docker pull ghcr.io/jrmatherly/ai-kb-docreader:v0.5.0
docker pull ghcr.io/jrmatherly/ai-kb-ui:v0.5.0
\`\`\`

### Q2: Port 5173 shows the old UI after startup?
The browser cached the old frontend assets. Force a refresh with Ctrl+Shift+R, and clear the site storage if needed.

### Q3: Registering a new user returns 500 Internal Server Error
Usually the backend container did not start properly. Check \`make dev-logs | grep app\` first; the common causes are a failed database connection or migrations that did not run.

### Q4: Uploading a document reports column "xxx" does not exist
The database migrations are incomplete. Run \`make migrate-up\`, confirm every migration succeeded, then restart the app container.

### Q5: A document stays in processing after upload
Check that the docreader container is running (\`docker compose ps\`) and look for parse errors in its logs. Common causes: a missing Python dependency, an encrypted PDF, or OCR timing out on a scan.

## Configuration

### Q6: How do I switch the vector store?
Change \`RETRIEVE_DRIVER\` in \`.env\`. Values: \`qdrant\` (default) / \`pgvector\` / \`elasticsearch\` / \`milvus\` / \`weaviate\`. Existing data must be re-embedded after switching.

### Q7: How do I change the default chunk size?
The global default is in the \`knowledge.chunking\` section of \`config/config.yaml\`; a single knowledge base can override it on the "Chunking settings" page in the UI. Changes apply only to newly uploaded documents.

### Q8: How do I enable multimodal?
Turn it on in the knowledge base's "Multimodal config" and pick a VLM (such as GPT-4o or Qwen2.5-VL). Once on, images in documents are described by the VLM and take part in retrieval.

### Q9: How do I switch the logs to JSON?
Set \`LOG_FORMAT=json LOG_LEVEL=info\`, which makes it easy to feed a log stack such as ELK / Loki.

## Retrieval and recall

### Q10: Retrieval returns nothing although the document is indexed
Check in this order:

1. Confirm the knowledge base status is \`completed\`, not \`indexing\` or \`failed\`
2. Check the vector store connection (qdrant:6334 by default); \`docker compose ps\` shows whether qdrant is healthy
3. Run "Rebuild index" on the admin page and confirm the embedding model works
4. Look for a dimension mismatch from the retriever in the backend logs

### Q11: Retrieval hits but the answer misses the question
Usually a chunk boundary cut through a key sentence. Try:

- Enable parent-child chunking, so vectors recall small chunks and the context uses large ones
- Raise the chunk overlap to 15%-20% of the chunk size
- Switch to the "split by heading" or "structure-aware" strategy

### Q12: The ranking looks wrong
The rerank model may not be enabled. Configure BGE-Reranker or Cohere Rerank in "Model settings", and turn on rerank in the knowledge base config.

## Models and tokens

### Q13: Calls to OpenAI time out
Check the proxy network and the \`OPENAI_BASE_URL\` config; without internet access, switch to Ollama or a regionally hosted model.

### Q14: Where do I see token usage?
With Langfuse on, every session, every reasoning step and every tool call has a full trace of its token use, which can be aggregated by time, workspace and model.

### Q15: How do I connect a private LLM?
Implement the \`internal/models/chat/provider/Provider\` interface, register it with the provider factory and add one entry to the config file; about 100-200 lines of code.

## Performance

### Q16: Large document uploads are slow
docreader is CPU-bound, so scale it out: raise \`deploy.replicas\` for the docreader service in docker-compose, or deploy it separately on a stronger machine.

### Q17: Embedding throughput is the bottleneck
Switch to local Ollama with GPU acceleration, or use a commercial API's batch endpoint (such as OpenAI's batch API). Setting \`EMBED_BATCH_SIZE=64\` raises throughput noticeably.

### Q18: Slow database queries
Enable \`pg_stat_statements\` to find the slow SQL; the common bottleneck is JSONB metadata queries on the chunks table, which an index on the hot fields can fix.`

const CHAPTER_SAMPLE = `Chapter 1 Introduction

1.1 Purpose of this document

This document describes the overall architecture, component breakdown and deployment plan of a distributed knowledge retrieval platform, for operations engineers, SREs and system architects. Readers should know foundational components such as Docker, Kubernetes and PostgreSQL.

1.2 Terminology

- Gateway layer (Gateway): schedules ingress traffic and converts protocols
- Service layer (Service): carries the domain logic and orchestration
- Storage layer (Storage): encapsulates persistence and caching details
- Inference layer (Inference): the isolation layer for talking to external LLMs / embedding models

1.3 Document version

This is version v1.4, matching the v0.5.x series of the platform code. The document is updated with the code; see Appendix A for the main changes.

Chapter 2 System architecture

2.1 Overall design

The platform uses a classic microservice architecture but stays restrained about boundaries, splitting out a service only where it truly needs to scale on its own. There are currently 4 long-running services: the gateway, the business backend, document parsing and the vector index.

2.2 Module breakdown

2.2.1 Users and permissions

Users, organizations, shared spaces, roles and permission rules live together in user-service. The RBAC model supports inheritance and overrides, and external OIDC is connected through an adapter layer.

2.2.2 Content and indexing

The content side owns the document lifecycle: upload, parsing, chunking, embedding, storage, retrieval and citation tracking. A chunk is the finest-grained retrieval unit, and every chunk carries the source document metadata and position.

2.2.3 Inference and orchestration

The agent orchestrator runs the ReAct loop, and tool calls are dispatched through the MCP protocol or the built-in registry. Every LLM call goes through the model proxy layer, which makes switching, rate limiting and billing easy.

2.3 Data flow

After upload a document goes through: parsing → cleaning → chunking → embedding → storage. The query path: rewrite → multi-path recall → fusion → rerank → context assembly → LLM generation.

Chapter 3 Deployment guide

3.1 Requirements

Physical resources: start at 8 vCPU / 16 GB memory / 100 GB SSD; for production, 16 vCPU / 32 GB / 500 GB is recommended. A GPU is required only for local inference.

3.2 Container orchestration

3.2.1 Single-host Docker Compose

Suited to PoCs and small to midsize teams (under 100 people, under 1 million chunks). One command brings up every service:

docker compose up -d

3.2.2 Kubernetes Helm

Suited to deployment at scale. The Helm chart is in the helm/ directory, with statefulset, config secret and ingress templates. It can connect to existing PG / Redis clusters.

3.3 Configuration best practices

- Keep external dependencies (database, object storage, vector store) in a config center rather than hard-coding them
- Manage model API keys as Secrets, isolated per environment
- Send logs to stdout/stderr for the orchestration platform to collect, rather than writing them to local disk

Chapter 4 Common operations scenarios

4.1 Upgrade process

Blue-green or rolling both work. Before upgrading, always check the CHANGELOG for schema changes; migrations run before the business image is replaced.

4.2 Backup and restore

Daily full PG backups + WAL increments; MinIO/object storage relies on cloud versioning. Qdrant snapshots are saved to object storage on a schedule through the snapshot API.

4.3 Troubleshooting

Work through four layers in order: "gateway → business → storage → model". Every layer has a health check endpoint and a Langfuse trace entry, and together they locate 80% of problems within 5 minutes.`

const PLAIN_SAMPLE = `Retrieval quality in a knowledge base depends on several factors, and the most direct is how well the splitting strategy matches the embedding model. Splitting too coarsely mixes meanings within one passage and dilutes relevance; splitting too finely loses context, so a single retrieved passage cannot answer a question that spans passages. A common recommendation is a chunk size of 50%–80% of the embedding model's recommended window, which keeps meaning intact while leaving headroom. Common embedding models such as BGE and Cohere's embed-v3 recommend a 512-token window, roughly 300–800 characters, since a CJK character is about 0.5–1.2 tokens and an English word about 1.3 tokens.

Besides chunk size, overlap also strongly affects recall completeness. With zero overlap, a question that spans passages often recalls only half a sentence; 10%–20% overlap usually covers most boundary cases; overlap above 30% makes neighboring chunks largely redundant, raising indexing cost without improving recall. A simple rule of thumb: for your target queries, start the overlap at one third of the average answer length, then tune it with A/B tests.

Separators should follow the real structure of the document. Plain text can keep the default double newline as a strong separator; Markdown documents should keep the heading levels, splitting by heading first and then by paragraph inside large sections; content that mixes code and tables needs a finer strategy, such as extracting code structure with Tree-sitter, detecting table boundaries, or using a script to separate the two kinds of content before text splitting. If a document mixes Chinese and English heavily, the separators must include both fullwidth CJK punctuation and ASCII punctuation (. ! ?), or they work for only one language and the results suffer.

The choice of embedding model is often underrated. Among open-source models the BGE-large series leads steadily on Chinese benchmarks, and BGE-M3 supports dense, sparse and ColBERT multi-vector retrieval at once; the commercial OpenAI text-embedding-3-large is versatile and not expensive, and Voyage-3-large is slightly ahead on English but average on Chinese. For long documents, mind the embedding model's maximum input length: OpenAI allows 8191 tokens and BGE defaults to 512, and overlong input is either truncated or embedded in segments and averaged, neither of which works well.

Finally, do not neglect post-processing. Rerank improves end-to-end results by 5%–15% in almost every scenario, at the cost of 100–300ms of extra latency per query. Common rerankers include BGE-Reranker and Cohere Rerank; the former is open source, the latter is paid but slightly better. If your retrieval path includes keyword recall, a reranker as a second filter is strongly recommended, or BM25's purely literal hits will pollute the context.`

export const CHUNKING_SAMPLES: ChunkingSample[] = [
  { id: "markdown", labelKey: "samples.markdown", text: MARKDOWN_SAMPLE },
  { id: "faq", labelKey: "samples.faq", text: FAQ_SAMPLE },
  { id: "chapter", labelKey: "samples.chapter", text: CHAPTER_SAMPLE },
  { id: "plain", labelKey: "samples.plain", text: PLAIN_SAMPLE },
];

export const DEFAULT_SAMPLE_ID = "markdown";
