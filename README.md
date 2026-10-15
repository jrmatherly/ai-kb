<p align="center">
  <a href="https://ai-kb-docs.matherly.net">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="./docs/images/readme/hero-en-dark.svg">
      <img src="./docs/images/readme/hero-en-light.svg" alt="AI KB: find the answers, and put knowledge to work. A self-hosted, open-source knowledge base and agent platform for Q&A, tasks and wikis." width="100%">
    </picture>
  </a>
</p>

<p align="center">
  <a href="https://ai-kb-docs.matherly.net"><img alt="Website" src="https://img.shields.io/badge/website-ai--kb--docs.matherly.net-b8863b?style=flat-square&labelColor=101f38"></a>
  <a href="https://ai-kb-docs.matherly.net/docs/"><img alt="Docs" src="https://img.shields.io/badge/docs-read-b8863b?style=flat-square&labelColor=101f38"></a>
  <a href="./CHANGELOG.md"><img alt="Version" src="https://img.shields.io/badge/release-v2026.10.15.1-b8863b?style=flat-square&labelColor=101f38"></a>
  <a href="./LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-b8863b?style=flat-square&labelColor=101f38"></a>
  <a href="https://github.com/jrmatherly/ai-kb/stargazers"><img alt="Stars" src="https://img.shields.io/github/stars/jrmatherly/ai-kb?style=flat-square&labelColor=101f38&color=b8863b"></a>
  <br/>
  <a href="https://clawhub.ai/lyingbug/weknora"><img alt="ClawHub Skill" src="https://img.shields.io/badge/ClawHub-skill-ff6b35?style=flat-square&labelColor=101f38"></a>
  <a href="https://www.npmjs.com/package/@jrmatherly/dsh-ai-kb"><img alt="npm @jrmatherly/dsh-ai-kb" src="https://img.shields.io/npm/v/@jrmatherly/dsh-ai-kb?style=flat-square&label=dsh-ai-kb&labelColor=101f38&color=cb3837&logo=npm&logoColor=white"></a>
</p>

<p align="center">
  <a href="#overview">Overview</a> ·
  <a href="#quick-start">Quick Start</a> ·
  <a href="#whats-new">What's New</a> ·
  <a href="#features">Features</a> ·
  <a href="#clients-and-integrations">Clients</a> ·
  <a href="#documentation">Docs</a> ·
  <a href="#development">Development</a>
</p>

## Overview

[AI KB](https://ai-kb-docs.matherly.net) is an open-source, LLM-powered knowledge framework for enterprise document understanding, semantic retrieval and reasoning. It brings a team's documents together so they can be searched, reasoned over and kept up to date. AI KB is an independent copy of [Tencent/WeKnora](https://github.com/Tencent/WeKnora), maintained by [jrmatherly](https://github.com/jrmatherly).

<!-- markdownlint-disable-next-line no-bare-urls -->
https://github.com/user-attachments/assets/5722b10d-d04d-49ed-a6cc-635a8c77d91f

<p align="center"><sub>1:52 · 1080p · No narration, English on-screen text</sub></p>

Use RAG to look things up, the agent for multi-step tasks, and the wiki to organize knowledge. All three work on the same knowledge bases.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/images/readme/capabilities-en-dark.svg">
  <img src="./docs/images/readme/capabilities-en-light.svg" alt="01 RAG: answers you can check, with hybrid search, multimodal parsing and citations. 02 Agent: tasks done with knowledge and tools, with multi-step reasoning, skills and sandbox, the local browser, MCP tools and memory. 03 Wiki: documents organized into a wiki, with a knowledge graph and rollback." width="100%">
</picture>

**The agent's toolbox.** Skills installed from ClawHub / SkillHub / Git / ZIP run in session-persistent Docker / E2B / Cube sandboxes, with an interactive terminal and graphical desktop beside the chat. Through the BrowserSkill extension the agent operates the user's own Chrome or Edge, and external MCP services (OAuth included) can be connected and enabled tool by tool.

Beyond the three modes:

- **Memory and curation**: cross-session long-term memory keeps the profile, preferences and facts a user has confirmed. Folder uploads keep their directory tree, and retrieval chunks can be edited, diffed and rolled back.
- **Data sources and formats**: auto-sync from Feishu wiki / Feishu Drive / Confluence / GitLab / Tencent IMA / Notion / Yuque / DingTalk Docs / Seafile / RSS, with more on the way. 10+ document formats including PDF, Word, images, Excel and XMind; Office files are parsed in-process by anydoc.
- **Channels and integrations**: Q&A in WeCom, Feishu, Slack, Telegram and other IM apps; an embed widget for external websites; a built-in MCP Server for Cursor, Claude and other AI tools; scoped API keys with a principal model for programmatic access.
- **Models**: 29 built-in vendors with a generated model catalog, including OpenAI, DeepSeek, Qwen (Alibaba Cloud), Zhipu, Hunyuan, Gemini, MiniMax, NVIDIA, LiteLLM and Ollama.
- **Permissions and operations**: multi-workspace RBAC (four roles, per-resource ownership, per-workspace audit log), several storage instances per workspace, a runtime task-queue dashboard with worker-pool governance, and Langfuse tracing for agent steps, token usage and pipelines.
- **Deployment**: LLMs, vector databases and storage backends are all swappable. Deploy locally or on a private cloud and keep the data in your own environment.

## Quick Start

Deploy AI KB in your own environment with Docker or Kubernetes, and configure models, storage and networking yourself.

### Run with Docker Compose

Requires [Docker](https://www.docker.com/), [Docker Compose](https://docs.docker.com/compose/) and [Git](https://git-scm.com/).

```bash
git clone https://github.com/jrmatherly/ai-kb.git
cd ai-kb
cp .env.example .env    # Replace JWT_SECRET and SYSTEM_AES_KEY (see the note below)
docker compose build    # Build the app, docreader and frontend images locally
docker compose up -d    # Start core services
```

> [!NOTE]
> No release images are published under `ghcr.io/jrmatherly/ai-kb-*` yet, so `docker compose pull` fails with `denied`. Build the images from the checkout as above; the first build takes a while.
>
> `.env.example` ships default `JWT_SECRET` and `SYSTEM_AES_KEY` values so the first start works, and the server logs a warning for each until you replace them. Replace both before you store real credentials: `openssl rand -hex 32` for `JWT_SECRET`, `openssl rand -hex 16` for `SYSTEM_AES_KEY` (exactly 32 characters). The server refuses to start if `JWT_SECRET` is empty or `SYSTEM_AES_KEY` is not exactly 32 bytes. The shipped `JWT_SECRET` is never used to sign: the server stores a per-install secret instead. Embed sessions and presigned file links stay off until you set `SYSTEM_SIGNING_KEY` or a real `SYSTEM_AES_KEY`.

Then open **<http://localhost>** and follow the onboarding guide. A walkthrough with sample data is in the [Quickstart](https://ai-kb-docs.matherly.net/docs/01-getting-started/03-quickstart).

> [!TIP]
> To use a local Ollama model, run `ollama serve > /dev/null 2>&1 &` first. For the Ollama embedding model name, `OLLAMA_BASE_URL`, and RAM notes, see [Configuration](https://ai-kb-docs.matherly.net/docs/01-getting-started/04-configuration).

| Service | URL |
| --------- | ----- |
| Web UI | `http://localhost` |
| Backend API | `http://localhost:8080` |
| Langfuse Tracing (`langfuse` or `full` profile) | `http://localhost:3000` |

### Optional services

Add `--profile` flags to enable additional components; multiple profiles can be combined.

| Profile | Adds |
| --------- | ------ |
| *(default)* | Core services |
| `full` | Every profile below except `milvus`, `weaviate`, `doris` and `odl-hybrid`, plus the sandbox image and the deprecated Python MCP server (port 8082) |
| `neo4j` | Knowledge Graph (Neo4j) |
| `minio` | Object Storage (MinIO) |
| `langfuse` | Tracing (Langfuse) |
| `qdrant` / `milvus` / `weaviate` / `doris` | Vector databases |
| `searxng` | Web search (SearXNG) |
| `odl-hybrid` | OpenDataLoader hybrid PDF parsing |
| `dex` | OIDC test provider (Dex) |

```bash
docker compose --profile neo4j --profile minio up -d
docker compose down     # Stop services
```

### Upgrading

If you already have AI KB running and want a newer version:

```bash
git pull                # Update the checkout
docker compose build    # Rebuild the images from it
docker compose up -d    # Recreate containers with the new images
```

> [!NOTE]
> `docker compose up -d` alone reuses the images already built and may leave the UI version out of sync with the checkout.

### Other ways to deploy

| Option | When to use it |
| -------- | ---------------- |
| **Docker Compose** | The standard deployment above: all features, multiple services |
| **Kubernetes (Helm)** | Production clusters; the chart is in [`helm/`](./helm) |
| **Lite single binary** | Local or low-resource use with no external dependencies (SQLite + in-memory queue); see [Lite vs. standard](./docs/LITE.md) |
| **Desktop app** | The Lite runtime with a GUI, login-free start and a macOS host sandbox; no installer is published yet, so build it from source |

All options, hardware requirements and deployment topologies: [Installation guide](https://ai-kb-docs.matherly.net/docs/01-getting-started/02-installation).

> [!WARNING]
> AI KB ships with login authentication, but for production deployments we strongly recommend that you:
>
> - deploy it in an internal / private network rather than on the public internet;
> - avoid exposing the service directly to public networks, to prevent information leakage;
> - configure proper firewall rules and access controls for the deployment environment;
> - regularly update to the latest version for security patches and improvements.

## What's New

### 2026.10.15.1 <sub>· [changelog](./CHANGELOG.md)</sub>

Security fixes for spreadsheet parsing (16 excelize advisories) and the remaining Go and katex advisories, Lite and desktop builds that start (2026.10.15 embeds gojieba's dictionaries), a Windows installer and Linux tarball on every release, and the `ai-kb-mcp` package on PyPI.

### 2026.10.13 <sub>· [changelog](./CHANGELOG.md)</sub>

The first AI KB release: the rename (images `ghcr.io/jrmatherly/ai-kb-*`, CLI `aikb`, env prefix `AIKB_`), English-only UI, dated CalVer versions, a uniform `{success, error}` error envelope, hash-only API keys, `.xls` / `.xlsb` workbooks and held-back mass deletions in data-source syncs.

The releases below are upstream [Tencent/WeKnora](https://github.com/Tencent/WeKnora) history that AI KB starts from.

### Upstream v0.8.2 <sub>· 2026-09-24 · [release notes](https://ai-kb-docs.matherly.net/docs/07-releases/v0.8.2)</sub>

Agents can operate the browser on your computer, knowledge bases can be published to other AI tools over MCP, and a running conversation can be steered, forked or rewound.

- **[Local Browser (BrowserSkill)](https://ai-kb-docs.matherly.net/docs/07-releases/v0.8.2#local-browser)**: agents drive the user's own Chrome / Edge through the open-source BrowserSkill extension, with a live task preview, pause / resume and hand-off for logins and CAPTCHAs.
- **[Built-in MCP Server](https://ai-kb-docs.matherly.net/docs/07-releases/v0.8.2#mcp-server)**: per-workspace `/mcp/<endpoint_id>` endpoints over Streamable HTTP, each with its own token, knowledge-base scope, rate limit and tool groups. The Python `mcp-server/` is deprecated.
- **[Conversation control](https://ai-kb-docs.matherly.net/docs/07-releases/v0.8.2#conversation-control)**: append requirements to a running turn, fork from any earlier question, rewind in place with sandbox checkpoints, and pick reasoning effort per session. Generated files are collected in a new artifacts library.
- **[Sandbox](https://ai-kb-docs.matherly.net/docs/07-releases/v0.8.2#sandbox)**: interactive terminal and graphical desktop; macOS Lite host sandbox with project folders; a sidebar Toolbox for skills, MCP services and the browser connection.
- **[Models](https://ai-kb-docs.matherly.net/docs/07-releases/v0.8.2#models)**: rebuilt model catalog (27 built-in vendors with generated context-window, max-output, reasoning and vision metadata). Agent retrieval tools are consolidated into `search_knowledge` / `read_document` / `list_documents`.
- **[Knowledge and platform](https://ai-kb-docs.matherly.net/docs/07-releases/v0.8.2#knowledge)**: Confluence and DingTalk Docs data sources; Bocha and Serply web search; Japanese UI (not in AI KB, which is English only); per-channel IM reply language; whitelist-only outbound mode.

### Upstream v0.8.0 <sub>· [release notes](https://ai-kb-docs.matherly.net/docs/07-releases/v0.8.0)</sub>

- **Skill sandbox runtime**: session-persistent Docker / E2B / Cube backends with per-tenant network policy; the Local host-process backend is removed and Docker is opt-in.
- **Tenant skill catalog**: install from ClawHub / SkillHub / git / zip, with per-sandbox snapshots, live progress, file browse/edit, and personal and workspace env vars.
- **Cross-session long-term memory**: profile / preference / fact / task / interest, auto-extracted with user confirmation, plus `search_memory`.
- **Parsing and sources**: in-process anydoc office parser; GitLab and Tencent IMA data sources; XMind parsing.
- **Ecosystem**: official DeepSeek Harness plugin `@jrmatherly/dsh-ai-kb`; LiteLLM; Exa and Metaso web search.
- **Chat**: chat artifacts, question outline and timestamps; context compaction and provider prompt-cache markers.
- **Security**: OIDC JWKS verification, optional complex passwords, document auto-tagging, and broad sandbox/security hardening.

<details>
<summary><b>Earlier upstream releases (v0.2.0 – v0.7.2)</b></summary>

<br/>

- **v0.7.2**: Product documentation site; knowledge base folder tree; chunk editing with revision history; wiki page revisions; directly loadable file URLs (`resource_urls=public`); Feishu Drive data source; batch tagging; MCP Server 1.1 (29 tools); AWS S3 default credential chain.
- **v0.7.1**: Yunzhijia IM; Volcengine rerank; Zhipu AI web search; platform-scoped API keys; per-KB activity audit; FAQ filtering, tagging and export; Langfuse OTLP tracing; one-click Markdown export.
- **v0.7.0**: Scoped API keys and principal model; task-queue dashboard and worker-pool governance; multiple storage instances per workspace; temporary chat attachments; `@Skill / @MCP` mentions; mid-conversation MCP OAuth; QQBot and Lark IM; Redis TLS; `aikb` CLI v0.10.
- **v0.6.3**: Website embed widget & Integrations Center (secure-mode token exchange + rate limits); chat experience overhaul (citation popovers, RAG pipeline progress, streaming markdown); document multi-tag & batch reparse; Wiki folders & hierarchy navigation; RSS data source; MCP OAuth2; EPUB / MHTML parsing; agent model-readiness checks; model test debugger; session source filter; workspace deletion UI.
- **v0.6.2**: Per-upload process configuration with upload-confirm dialog; document reparse with `process_config`; `aikb` CLI v0.9 (bundled Agent Skills, `session stop`, auth/profile harmonization); KB marquee multi-select; HNSW index for 1024-dim pgvector embeddings; chat resources store refactor; Langfuse-only tracing (Jaeger removed).
- **v0.6.1**: Document parsing trace timeline (Langfuse-style span tree with stage-by-stage progress + stop-parse); OpenSearch vector store driver; declarative built-in models via YAML; system admin & consolidated platform settings + audit log; new-user onboarding guide; settings UI redesign; `aikb` CLI v0.7 / v0.8 (agent-first wire contract, NDJSON, `--dry-run`); OpenDataLoader + PaddleOCR-VL parsers; MCP server multi-transport (stdio / SSE / HTTP); per-model thinking-mode config; Tencent LKEAP rerank + native Gemini embeddings + MiniMax-M3.
- **v0.6.0**: Workspace RBAC (4-tier role matrix `Owner` / `Admin` / `Contributor` / `Viewer` + per-KB ownership + per-workspace audit log), workspace member management & multi-workspace UX, self-service workspaces; `aikb` CLI v0.4 GA with `mcp serve`; KB retrieval fan-out across vector stores; AES-256-GCM credential encryption + docreader gRPC TLS + Token; Zhipu embedder + Huawei OBS; server-side user preferences; Go 1.26.0. See [Tenants & auth](https://ai-kb-docs.matherly.net/docs/03-features/01-tenant-auth).
- **v0.5.2**: Wiki ingest scales to 40k-document KBs (task queue + DLQ); MCP human-in-the-loop tool approval; Anthropic / Apache Doris / Tencent VectorDB / KS3 / SearXNG backends; adaptive 3-tier chunking with live preview; global ⌘K command palette; Yuque connector + WeChat Mini Program; `aikb` CLI preview.
- **v0.5.1**: Knowledge-base batch management; workspace-wide IM channels overview; session search + user-scoped pinning; unified Model / Web Search / MCP settings cards; per-agent LLM timeout; desktop workspace switching.
- **v0.5.0**: Wiki Mode GA. Agents auto-generate structured, interlinked Markdown wiki pages with a knowledge graph; wiki browser + visual graph in the UI.
- **v0.4.0**: WeKnoraCloud (hosted LLM + parsing); Chrome Extension; ClawHub Skill; WeChat IM; attachment processing; Azure OpenAI / Alibaba OSS; Notion connector; Baidu + Ollama web search; VectorStore management.
- **v0.3.6**: ASR (audio); Feishu data-source auto-sync; OIDC; IM quote-reply context + thread-based sessions; document summarization; Tavily search; parallel tool calling; agent @mention scope restriction.
- **v0.3.5**: Telegram / DingTalk / Mattermost IM; IM slash commands + QA queue; suggested questions; VLM auto-describe MCP tool images; Novita AI; channel tracking.
- **v0.3.4**: WeCom / Feishu / Slack IM; multimodal image support; NVIDIA model API; Weaviate; AWS S3; AES-256-GCM API-key encryption; built-in MCP service; hybrid-search optimization; `final_answer` tool.
- **v0.3.3**: Parent-child chunking; KB pinning; fallback response; passage cleaning for rerank; storage auto-creation; Milvus.
- **v0.3.2**: Knowledge Search entry; per-source parser & storage engine config; image rendering in local storage; document preview; Volcengine TOS; Mermaid rendering; batch session management; memory graph preview.
- **v0.3.0**: Shared Space; Agent Skills + sandboxed execution; custom agents; Data Analyst agent; thinking mode; Bing / Google web search; API Key auth; Helm chart; Korean i18n (not in AI KB); Qdrant.
- **v0.2.0**: Agent Mode (ReACT); multi-type knowledge bases (FAQ + document); conversation strategy config; DuckDuckGo web search; MCP tool integration; new UI with agent mode switching; MQ async task management.

Full history: [`CHANGELOG.md`](./CHANGELOG.md).

</details>

## Product Tour

### Quick Q&A and smart reasoning

**Two ways to ask.** Quick Q&A answers from the knowledge base with RAG and cites the sources it used. In smart reasoning the agent plans multi-step work, searching, reading documents and calling tools and skills, and shows each step in the conversation. [Docs →](https://ai-kb-docs.matherly.net/docs/03-features/18-chat-experience)

<a href="./docs/images/readme/spotlight-qa-light.webp">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/images/readme/spotlight-qa-dark.webp">
  <img src="./docs/images/readme/spotlight-qa-light.webp" alt="Quick Q&A and smart reasoning" width="100%">
</picture>
</a>

### Local browser

**Operate the browser on your computer.** Through Tencent's open-source BrowserSkill extension, the agent opens pages and fills in forms in your own Chrome or Edge, and hands over to you for logins and CAPTCHAs. [Docs →](https://ai-kb-docs.matherly.net/docs/05-clients/09-local-browser)

<a href="./docs/images/readme/spotlight-browser-light.webp">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/images/readme/spotlight-browser-dark.webp">
  <img src="./docs/images/readme/spotlight-browser-light.webp" alt="Local browser" width="100%">
</picture>
</a>

### Skills and sandbox

**Run skills and produce files.** Docker, E2B and Cube backends are supported. Turns in the same session share one workspace, and generated files can be previewed and downloaded. Open the graphical desktop or interactive terminal beside the chat to follow each step and take over when needed. [Docs →](https://ai-kb-docs.matherly.net/docs/03-features/22-skills-sandbox)

<a href="./docs/images/readme/spotlight-sandbox-light.webp">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/images/readme/spotlight-sandbox-dark.webp">
  <img src="./docs/images/readme/spotlight-sandbox-light.webp" alt="Skills and sandbox" width="100%">
</picture>
</a>

### Toolbox: MCP services and skills

**Tools the agent can use.** Connect external MCP services and choose, tool by tool, which are enabled and which calls need approval. Install skills from ClawHub, SkillHub, Git or ZIP, manage them per workspace, and reuse them across sandboxes. [Docs →](https://ai-kb-docs.matherly.net/docs/03-features/22-skills-sandbox)

<a href="./docs/images/readme/spotlight-toolbox-light.webp">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/images/readme/spotlight-toolbox-dark.webp">
  <img src="./docs/images/readme/spotlight-toolbox-light.webp" alt="Toolbox: MCP services and skills" width="100%">
</picture>
</a>

### Automatic wiki

**Documents organized into a browsable wiki.** With Wiki enabled, AI KB extracts people, products and concepts from knowledge-base documents into pages with source citations, organized by directory. The knowledge graph shows how pages relate; pages can be edited directly and every change can be rolled back. [Docs →](https://ai-kb-docs.matherly.net/docs/03-features/14-wiki)

<a href="./docs/images/readme/spotlight-wiki-light.webp">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/images/readme/spotlight-wiki-dark.webp">
  <img src="./docs/images/readme/spotlight-wiki-light.webp" alt="Automatic wiki" width="100%">
</picture>
</a>

### Observability

**Tracing and runtime monitoring.** Langfuse traces the reasoning, tool calls and token usage of each agent step. The document parsing timeline shows progress stage by stage, and the task-queue dashboard lists queued and failed tasks. [Docs →](https://ai-kb-docs.matherly.net/docs/03-features/16-observability)

<a href="./docs/images/readme/spotlight-observability-light.webp">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/images/readme/spotlight-observability-dark.webp">
  <img src="./docs/images/readme/spotlight-observability-light.webp" alt="Observability" width="100%">
</picture>
</a>

## Architecture

<a href="./docs/images/readme/architecture-en-light.svg">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/images/readme/architecture-en-dark.svg">
  <img src="./docs/images/readme/architecture-en-light.svg" alt="AI KB architecture: clients and channels connect to the AI KB app, where RAG Q&A, agent reasoning and auto wiki share one knowledge pipeline; the app calls runtime services and stores data in PostgreSQL, Redis and optional stores" width="100%">
</picture>
</a>

A modular pipeline from document parsing, vectorization and retrieval to LLM inference, in which every component can be replaced or extended. It runs locally or on a private cloud, and the Web UI needs no setup to get started. More: [Architecture overview](https://ai-kb-docs.matherly.net/docs/02-architecture/01-overview) · [RAG pipeline](https://ai-kb-docs.matherly.net/docs/02-architecture/04-rag-pipeline) · [Extension points](https://ai-kb-docs.matherly.net/docs/06-development/03-extension-points).

## Features

| Area | Highlights |
| ------ | ------------ |
| [Q&A and agent](https://ai-kb-docs.matherly.net/docs/03-features/07-agent) | Quick Q&A answers from knowledge bases with citations; smart reasoning runs a ReAct agent over knowledge bases, web search, MCP tools, skills and the local browser. Steer, fork or rewind a running conversation, and keep [long-term memory](https://ai-kb-docs.matherly.net/docs/03-features/23-memory) across sessions |
| [Wiki](https://ai-kb-docs.matherly.net/docs/03-features/14-wiki) | Agent-generated, interlinked wiki pages with a knowledge graph; in-browser editing, revision diff and rollback |
| [Skills and sandbox](https://ai-kb-docs.matherly.net/docs/03-features/22-skills-sandbox) | Skill catalog installed from ClawHub / SkillHub / Git / ZIP; session-persistent Docker / E2B / Cube sandboxes with network policy; terminal and graphical desktop beside the chat |
| [Knowledge bases](https://ai-kb-docs.matherly.net/docs/03-features/02-knowledge-base) | FAQ, document and wiki bases; folder tree; chunk editing with revisions; per-upload parsing, chunking and multimodal settings; auto-tagging |
| [Retrieval](https://ai-kb-docs.matherly.net/docs/03-features/05-retrieval-engines) | Keyword + vector hybrid search, rerank, parent-child chunking and GraphRAG ([Neo4j](https://ai-kb-docs.matherly.net/docs/03-features/09-knowledge-graph)); end-to-end evaluation with recall and BLEU / ROUGE |
| [Access and security](https://ai-kb-docs.matherly.net/docs/03-features/01-tenant-auth) | Workspace RBAC with four roles and an audit log; scoped API keys; OIDC; AES-256-GCM credential encryption; SSRF-safe outbound requests with a whitelist-only mode |
| [Operations](https://ai-kb-docs.matherly.net/docs/03-features/16-observability) | Langfuse tracing for agent steps, tokens and pipelines; document parsing timeline; task-queue dashboard with worker pools; automatic migrations on upgrade |

### Supported backends

| Component | Options |
| ----------- | --------- |
| [LLMs](https://ai-kb-docs.matherly.net/docs/03-features/06-models) | 29 built-in vendors, including OpenAI / Azure OpenAI / Anthropic / DeepSeek / Qwen / Zhipu / Hunyuan / Doubao / Gemini / MiniMax / NVIDIA / SiliconFlow / OpenRouter / LiteLLM / Ollama |
| Embeddings | Ollama / BGE / GTE / Zhipu / OpenAI-compatible APIs |
| Vector databases | PostgreSQL (pgvector) / Elasticsearch / OpenSearch / Milvus / Weaviate / Qdrant / Apache Doris / Tencent VectorDB |
| [Object storage](https://ai-kb-docs.matherly.net/docs/03-features/19-storage-backends) | Local / Tencent Cloud COS / MinIO / AWS S3 / Volcengine TOS / Alibaba Cloud OSS / Kingsoft Cloud KS3 / Huawei Cloud OBS |
| [Document formats](https://ai-kb-docs.matherly.net/docs/03-features/03-document-parsing) | PDF / Word / PPT / Excel / CSV / TXT / Markdown / HTML / EPUB / MHTML / JSON / XMind / images |
| [Data sources](https://ai-kb-docs.matherly.net/docs/03-features/10-datasource) | Feishu wiki / Feishu Drive / Lark / Confluence / GitLab / Tencent IMA / Notion / Yuque / DingTalk Docs / Seafile / RSS |
| [IM channels](https://ai-kb-docs.matherly.net/docs/03-features/12-im-integration) | WeCom / Feishu / Lark / QQBot / Slack / Telegram / DingTalk / Mattermost / WeChat / Yunzhijia |
| [Web search](https://ai-kb-docs.matherly.net/docs/03-features/11-web-search) | DuckDuckGo / Bing / Google / Tavily / Baidu / Ollama / SearXNG / Keenable / Zhipu AI / Exa / Metaso / Bocha / Serply |
| Deployment | Docker Compose / Kubernetes (Helm) / Lite single binary / desktop app; offline and private-cloud installs; English UI |

## Clients and Integrations

| | Client | What it does |
| :-: | -------- | -------------- |
| <img src="./docs/images/readme/icons/terminal.svg" width="22" height="22" alt=""> | [**CLI `aikb`**](./cli/README.md) | Agent-first command line for the full API, with a curated MCP tool surface and bundled Agent Skills |
| <img src="./docs/images/readme/icons/plug.svg" width="22" height="22" alt=""> | [**Built-in MCP Server**](https://ai-kb-docs.matherly.net/docs/03-features/08-mcp) | Publishes knowledge bases to Cursor, Claude and other MCP clients over Streamable HTTP; the Python [`mcp-server/`](./mcp-server/MCP_CONFIG.md) is deprecated |
| <img src="./website-docs/homepage/public/docs/_home/brands/browserskill.png" width="22" height="22" alt=""> | [**Local Browser (BrowserSkill)**](https://ai-kb-docs.matherly.net/docs/05-clients/09-local-browser) | Lets agents operate the user's own Chrome / Edge |
| <img src="./docs/images/readme/icons/phone.svg" width="22" height="22" alt=""> | [**WeChat Mini Program**](./miniprogram/README.md) | Lightweight mobile client: configure API access, select knowledge bases, import URLs, and ask knowledge chat from WeChat |
| <img src="./docs/images/readme/icons/skills.svg" width="22" height="22" alt=""> | [**ClawHub Skill**](https://clawhub.ai/lyingbug/weknora) | A third-party skill on ClawHub for document import, hybrid search and knowledge management via the REST API |
| <img src="./website-docs/homepage/public/docs/_home/brands/deepseek-color.svg" width="22" height="22" alt=""> | [**DeepSeek Harness plugin**](https://www.npmjs.com/package/@jrmatherly/dsh-ai-kb) | Gives `dsh` coding agents four read-only tools: search, read document, ask and list knowledge bases |
| <img src="./docs/images/readme/icons/code.svg" width="22" height="22" alt=""> | [**Website Embed Widget**](https://ai-kb-docs.matherly.net/docs/03-features/13-embed-channel) | Publishes agents on external sites |
| <img src="./docs/images/readme/icons/braces.svg" width="22" height="22" alt=""> | [**Go SDK**](https://ai-kb-docs.matherly.net/docs/05-clients/03-go-sdk) | CRUD for knowledge bases, documents and sessions, plus SSE streaming Q&A |

### Command-Line Interface

`aikb` is the official CLI for driving the API from a terminal or an AI agent. It is **agent-first**: every command emits a stable JSON envelope by default (with typed error codes mapped to exit codes), and `--format text` renders for humans. It also serves a curated MCP tool surface (`aikb mcp serve`) and ships bundled Agent Skills.

```bash
aikb profile add prod --host https://kb.example.com --use
aikb auth login
aikb kb list
aikb link --kb my-knowledge-base    # bind the current directory
aikb doc upload notes.md
aikb chat "summarise the design doc"
```

For headless / CI use, set `AIKB_API_KEY` + `AIKB_HOST` and skip `auth login`; no credentials are written to disk. See [`cli/README.md`](./cli/README.md) for install + 5-minute quickstart and [`cli/AGENTS.md`](./cli/AGENTS.md) for the operational contract AI agents rely on.

## Documentation

The full product documentation lives at **[ai-kb-docs.matherly.net/docs](https://ai-kb-docs.matherly.net/docs/)**, organized as Getting Started → Architecture → Features → API → Clients → Development and covering ~360 API endpoints and ~150 environment variables.

| Start here | |
| ------------ | --- |
| [Introduction](https://ai-kb-docs.matherly.net/docs/01-getting-started/01-introduction) | Capabilities overview |
| [Installation](https://ai-kb-docs.matherly.net/docs/01-getting-started/02-installation) | Docker Compose, Helm, Lite and desktop |
| [Configuration](https://ai-kb-docs.matherly.net/docs/01-getting-started/04-configuration) | Environment variables and models |
| [Troubleshooting FAQ](https://ai-kb-docs.matherly.net/docs/01-getting-started/05-troubleshooting) | Common problems and fixes |
| [API reference](https://ai-kb-docs.matherly.net/docs/04-api/01-api-overview) | REST API overview |
| [Release notes](https://ai-kb-docs.matherly.net/docs/07-releases/v0.8.2) | What changed in each release |

## Development

If you need to frequently modify code, you don't need to rebuild Docker images every time. Use fast development mode:

```bash
make dev-start      # Start infrastructure
make dev-app        # Start backend (new terminal)
make dev-frontend   # Start frontend (new terminal)
```

- Frontend modifications auto hot-reload (no restart needed)
- Backend modifications quick restart (5-10 seconds, supports Air hot-reload)
- No need to rebuild Docker images
- Supports IDE breakpoint debugging

See the [Development guide](https://ai-kb-docs.matherly.net/docs/06-development/01-dev-guide) for details.

The documentation site and product homepage are built from [`website-docs/`](./website-docs/README.md). With Node.js 24, run `cd website-docs && npm run setup && npm run build && npm run preview` to preview both together; the unified static output serves the homepage at `/` and documentation at `/docs/`. See the directory's README for Nginx and Docker deployment.

## Contributing

Welcome to submit [Issues](https://github.com/jrmatherly/ai-kb/issues) or Pull Requests.

- **Process:** Fork → Create branch → Commit changes → Open PR
- **Standards:** Format code with `gofmt`, follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:` / `fix:` / `docs:` / `test:` / `refactor:`)

<details>
<summary><b>Validating your change</b></summary>

<br/>

`scripts/validate.sh` runs the checks for every area your change touches (Go, CLI, client, frontend, docreader, Helm, docs site, Markdown, shell scripts and more) and ends with the English-only and em dash gates:

```bash
scripts/validate.sh                 # working tree and untracked files against HEAD
scripts/validate.sh --base origin/main
scripts/validate.sh --list          # show which areas would run
```

`scripts/install-git-hooks.sh` installs the pre-commit and pre-push hooks that run the same checks. The [Development guide](https://ai-kb-docs.matherly.net/docs/06-development/01-dev-guide#validation-and-contribution-workflow) lists each area's commands and the tools they need.

</details>

### Contributors

Thanks to everyone who has contributed:

[![Contributors](https://contrib.rocks/image?repo=jrmatherly/ai-kb)](https://github.com/jrmatherly/ai-kb/graphs/contributors)

## License

AI KB is maintained by [jrmatherly](https://github.com/jrmatherly) and based on [Tencent/WeKnora](https://github.com/Tencent/WeKnora), which Tencent publishes under the MIT License. AI KB is not a Tencent product. It is licensed under the [MIT License](./LICENSE), which keeps Tencent's copyright notice for the upstream code.
