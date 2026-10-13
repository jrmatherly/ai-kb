# DeepSeek Harness plugin

The DeepSeek Harness (dsh) plugin connects to an existing AI KB deployment over the REST API. It retrieves source passages, reads full documents and produces answers with citations. The plugin source is in `packages/dsh-ai-kb`.

## Install and connect

This section installs the plugin into a dsh profile and points it at your deployment.

Install the published `@jrmatherly/dsh-ai-kb` package, then point it at your deployment.

```sh
dsh plugin --profile web add @jrmatherly/dsh-ai-kb

export AIKB_BASE_URL=https://kb.example.com
export AIKB_API_KEY='<api-key>'
export AIKB_KNOWLEDGE_BASE_IDS=kb-123,kb-456
export AIKB_RESOURCE_URLS=public
dsh web
```

To run unreleased changes, install from a checkout instead: `(cd packages/dsh-ai-kb && npm ci)` builds the entry point `dist/index.js` through the `prepare` script, then `dsh plugin --profile web add ./packages/dsh-ai-kb` from the repository root. The plugin itself requires Node 20.11 or later. The dsh runtime the repository tested needs a Node build with zstd (22.15 or later, or 24 or later) and pnpm 10 or later. The tested versions are dsh 0.1.0-rc.8 and AI KB 0.8.0. For other versions, confirm compatibility against the dsh plugin interface.

The service address may include `/api/v1`. When it does not, the plugin appends it. The API key is sent in `X-API-Key` and needs the `retrieve` capability. Calling `ask` also needs `chat`. With a platform key, set `tenantId`, which requests send as `X-Tenant-ID`.

## The four tools

This section describes what each tool returns, so you know which one an agent should use.

| Tool | Purpose |
| --- | --- |
| `aikb_list_knowledge_bases` | Lists the visible knowledge bases and their IDs |
| `aikb_search` | Hybrid retrieval of passages that also matches document titles. Returns the knowledge item ID, the chunk index and the score |
| `aikb_read_document` | Reads a document's text in order. The first page includes the title and summary, and long documents can be paged |
| `aikb_ask` | Creates or continues a session in AI KB and returns the answer, the citations and the server-side tool steps |

`search` returns retrieval evidence for dsh to write its own answer from. `ask` calls the model on the AI KB server and creates a session and messages, which suits questions that draw on many documents. Neither one changes knowledge base content.

When no knowledge base scope is set, the plugin resolves every knowledge base the credential can see, caches that list in the process and shares it between search and ask. With `agentId` set, ask runs the agent flow and the agent resolves its knowledge scope on the server.

## Profile configuration

This section sets the plugin options for one dsh profile.

Set them in `$DSH_HOME/profiles/<name>/cordis.patch.yml`:

```yaml
- id: aikb
  config:
    baseUrl: https://kb.example.com
    apiKey: !!js process.env.AIKB_API_KEY
    knowledgeBaseIds: [kb-product-docs]
    agentId: ''
    maxResults: 8
    maxChunkChars: 1200
    requestTimeoutMs: 30000
    chatTimeoutMs: 300000
    resourceUrls: public
    toolPrefix: aikb
    tools:
      listKnowledgeBases: true
      search: true
      readDocument: true
      ask: true
```

An update replaces the row's whole `config`, so include every field you want to keep. All `tools.*` entries are on by default. To connect several deployments, give each one its own `toolPrefix`. A prefix must start with a lowercase letter and contain only lowercase letters, digits and underscores.

## Cited images and troubleshooting

This section explains how image links in answers are produced and how to fix common failures.

`resourceUrls` defaults to `public`, which asks the server to turn resource handles into loadable links. When an API key limited to certain knowledge bases gets a 403 for this, the plugin switches to `handle` and keeps that mode for later calls. A browser cannot load a `resource://` handle directly. The requirements for external links are in [External access to images and files](../03-features/21-file-access.md).

| Symptom | Check |
| --- | --- |
| Configuration error while loading | Fix baseUrl, out-of-range parameters or toolPrefix according to the per-field errors |
| 401/403 on retrieval | Whether the key is valid, has the retrieve capability, and is authorized for the target workspace or knowledge base |
| ask is refused | Whether the key also has chat, and whether a platform key comes with tenantId |
| No direct image links | Whether the key is limited to certain knowledge bases, or the deployment has no externally reachable link configured |
| Long questions time out | Raise chatTimeoutMs and check the model calls on the AI KB server |

## Implementation reference

This section lists the files to read when changing the plugin or checking its API contract.

`packages/dsh-ai-kb/src/`, `packages/dsh-ai-kb/test/fixtures/api-contract.json`, `packages/dsh-ai-kb/contract/contract_test.go`.
