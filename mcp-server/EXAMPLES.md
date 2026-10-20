# AI KB MCP Server usage examples

> **Deprecated.** AI KB ships a [built-in MCP server](https://ai-kb.matherly.net/docs/03-features/08-mcp); prefer it. This package may be removed in a future release.

Install, configure and start the server as [README.md](./README.md) describes. The server does not read a `.env` file; export the variables or set them in the MCP client's `env` block.

## MCP tool examples

Examples for common tools. The full tool list is in [README.md](./README.md#features).

### Workspace management

#### Create a workspace

This needs an API key with the platform capability `system_tenants_manage`. A workspace key from Settings → API Keys is refused.

```json
{
  "tool": "create_tenant",
  "arguments": {
    "name": "My Company",
    "description": "Company knowledge management system",
    "business": "technology",
    "retriever_engines": {
      "engines": [
        {"retriever_type": "keywords", "retriever_engine_type": "postgres"},
        {"retriever_type": "vector", "retriever_engine_type": "postgres"}
      ]
    }
  }
}
```

#### List all workspaces

```json
{
  "tool": "list_tenants",
  "arguments": {}
}
```

### Knowledge base management

#### Create a knowledge base

```json
{
  "tool": "create_knowledge_base",
  "arguments": {
    "name": "Product Docs",
    "description": "Product documents and materials",
    "embedding_model_id": "<embedding model ID from list_models>",
    "summary_model_id": "<chat model ID from list_models>"
  }
}
```

Both model IDs are AI KB model record IDs, not provider model names.

#### List knowledge bases

```json
{
  "tool": "list_knowledge_bases",
  "arguments": {}
}
```

#### Get knowledge base details

```json
{
  "tool": "get_knowledge_base",
  "arguments": {
    "kb_id": "kb_123456"
  }
}
```

#### Hybrid search

```json
{
  "tool": "hybrid_search",
  "arguments": {
    "kb_id": "kb_123456",
    "query": "How do I use the API",
    "vector_threshold": 0.7,
    "keyword_threshold": 0.5,
    "match_count": 10
  }
}
```

### Knowledge management

#### Create knowledge from a URL

```json
{
  "tool": "create_knowledge_from_url",
  "arguments": {
    "kb_id": "kb_123456",
    "url": "https://docs.example.com/api-guide",
    "enable_multimodel": true
  }
}
```

#### Create knowledge from a local file

```json
{
  "tool": "create_knowledge_from_file",
  "arguments": {
    "kb_id": "kb_123456",
    "file_path": "/srv/docs/spec/design.pdf",
    "file_name": "spec/design.pdf"
  }
}
```

The file must be on the MCP server's filesystem, inside the allowed upload roots ([README.md](./README.md#local-upload-directory-boundary)). `file_name` is optional; a path-qualified name puts the entry in that knowledge folder.

#### Create knowledge from text

```json
{
  "tool": "create_knowledge_from_text",
  "arguments": {
    "kb_id": "my-knowledge-base",
    "title": "Attention mechanism summary",
    "content": "# Attention mechanism\n\nThe attention mechanism lets a model assign weights dynamically while processing a sequence...",
    "status": "publish"
  }
}
```

#### Update manual knowledge

```json
{
  "tool": "update_knowledge_from_text",
  "arguments": {
    "knowledge_id": "know_789012",
    "title": "Attention mechanism summary (revised)",
    "content": "# Attention mechanism\n\nThe revised Markdown content goes here...",
    "status": "publish"
  }
}
```

Omitting `title` or passing an empty string keeps the original title. Set `status` to `draft` to save the content without re-indexing.

#### List knowledge

```json
{
  "tool": "list_knowledge",
  "arguments": {
    "kb_id": "kb_123456",
    "page": 1,
    "page_size": 20
  }
}
```

#### Get knowledge details

```json
{
  "tool": "get_knowledge",
  "arguments": {
    "knowledge_id": "know_789012"
  }
}
```

### Model management

#### Create a model

```json
{
  "tool": "create_model",
  "arguments": {
    "name": "GPT-4 Chat Model",
    "type": "KnowledgeQA",
    "source": "openai",
    "description": "OpenAI GPT-4 model for knowledge Q&A",
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-..."
  }
}
```

#### List models

```json
{
  "tool": "list_models",
  "arguments": {}
}
```

### Session management

#### Create a chat session

```json
{
  "tool": "create_session",
  "arguments": {
    "kb_id": "kb_123456",
    "title": "Product Q&A"
  }
}
```

The tool requires `kb_id`, but the server keeps only `title` and `description`: sessions are not bound to a knowledge base. Pass the knowledge bases to `chat` instead.

#### Get session details

```json
{
  "tool": "get_session",
  "arguments": {
    "session_id": "sess_345678"
  }
}
```

#### List sessions

```json
{
  "tool": "list_sessions",
  "arguments": {
    "page": 1,
    "page_size": 10
  }
}
```

### Chat

#### Send a chat message

```json
{
  "tool": "chat",
  "arguments": {
    "session_id": "sess_345678",
    "query": "Describe the product's main features",
    "knowledge_base_ids": ["kb_123456"]
  }
}
```

Without `knowledge_base_ids` nothing is retrieved and the answer comes from the model alone.

#### Call an agent

```json
{
  "tool": "agent_chat",
  "arguments": {
    "session_id": "sess_345678",
    "agent_id": "<agent name or ID from list_agents>",
    "query": "Compare the last two release notes",
    "knowledge_base_ids": ["kb_123456"]
  }
}
```

Pass `knowledge_base_ids` when the agent has no knowledge bases of its own (check with `get_agent`); otherwise the call fails with "no search targets available".

### Chunk management

#### List knowledge chunks

```json
{
  "tool": "list_chunks",
  "arguments": {
    "knowledge_id": "know_789012",
    "page": 1,
    "page_size": 50
  }
}
```

#### Delete a knowledge chunk

```json
{
  "tool": "delete_chunk",
  "arguments": {
    "knowledge_id": "know_789012",
    "chunk_id": "chunk_456789"
  }
}
```

## End-to-end workflow

### Scenario: build a complete knowledge Q&A system

```bash
# 1. Start the server
python main.py --verbose

# 2. Run these steps in the MCP client:
```

#### Step 1: create a knowledge base

```json
{
  "tool": "create_knowledge_base",
  "arguments": {
    "name": "API Docs",
    "description": "All API documentation"
  }
}
```

#### Step 2: add knowledge

```json
{
  "tool": "create_knowledge_from_url",
  "arguments": {
    "kb_id": "the returned knowledge base ID",
    "url": "https://docs.company.com/api",
    "enable_multimodel": true
  }
}
```

#### Step 3: create a chat session

```json
{
  "tool": "create_session",
  "arguments": {
    "kb_id": "knowledge base ID",
    "title": "API questions"
  }
}
```

#### Step 4: start chatting

```json
{
  "tool": "chat",
  "arguments": {
    "session_id": "session ID",
    "query": "How do I use the user authentication API?",
    "knowledge_base_ids": ["knowledge base ID"]
  }
}
```

## Error handling

A failed call returns a tool error whose text carries the HTTP error, for example `401 Client Error: Unauthorized`.

- Connection refused: check that `AIKB_BASE_URL` is correct and the service is running.
- 401 Unauthorized: check that `AIKB_API_KEY` is set and valid.
- 403 Forbidden: the API key lacks the capability the route needs (see the workspace example above).
- 404 Not Found: check the ID, or create the resource first.

## Advanced configuration

### Custom retrieval settings

```json
{
  "tool": "hybrid_search",
  "arguments": {
    "kb_id": "kb_123456",
    "query": "search query",
    "vector_threshold": 0.8,
    "keyword_threshold": 0.6,
    "match_count": 15
  }
}
```

## Performance tips

1. **Search thresholds**: set search thresholds to balance accuracy and performance
2. **Session management**: clean up sessions you no longer need to save resources
3. **Debug logs**: use the `--verbose` option to see debug logs

## Integration

### Claude Desktop and other MCP clients

See [MCP_CONFIG.md](./MCP_CONFIG.md) for client configuration.

Project repository: <https://github.com/jrmatherly/ai-kb/tree/main/mcp-server>

## Troubleshooting

If you run into problems:

1. Run `python main.py --check-only` to check the environment
2. Run `python main.py --verbose` to see debug logs
3. Check that the AI KB service is running
4. Check the network connection and firewall settings
