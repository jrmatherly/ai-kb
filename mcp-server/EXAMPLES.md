# AI KB MCP Server usage examples

Detailed usage examples for the AI KB MCP Server.

## Basic usage

### 1. Start the server

```bash
# Recommended: the main entry point
python main.py

# Check the environment configuration
python main.py --check-only

# Enable verbose logging
python main.py --verbose
```

### 2. Environment configuration

```bash
# Set environment variables
export AIKB_BASE_URL="http://localhost:8080/api/v1"
export AIKB_API_KEY="your_api_key_here"

# Or set them in a .env file
echo "AIKB_BASE_URL=http://localhost:8080/api/v1" > .env
echo "AIKB_API_KEY=your_api_key_here" >> .env
```

## MCP tool examples

Examples for each MCP tool:

### Workspace management

#### Create a workspace
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
    "embedding_model_id": "text-embedding-ada-002",
    "summary_model_id": "gpt-3.5-turbo"
  }
}
```

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
    "api_key": "sk-...",
    "is_default": true
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
    "max_rounds": 10,
    "enable_rewrite": true,
    "fallback_response": "Sorry, I can't answer that question.",
    "summary_model_id": "gpt-3.5-turbo"
  }
}
```

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
    "query": "Describe the product's main features"
  }
}
```

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

#### Step 1: create a workspace
```json
{
  "tool": "create_tenant",
  "arguments": {
    "name": "Engineering Docs Center",
    "description": "Company engineering documentation",
    "business": "technology"
  }
}
```

#### Step 2: create a knowledge base
```json
{
  "tool": "create_knowledge_base",
  "arguments": {
    "name": "API Docs",
    "description": "All API documentation"
  }
}
```

#### Step 3: add knowledge
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

#### Step 4: create a chat session
```json
{
  "tool": "create_session",
  "arguments": {
    "kb_id": "knowledge base ID",
    "max_rounds": 5,
    "enable_rewrite": true
  }
}
```

#### Step 5: start chatting
```json
{
  "tool": "chat",
  "arguments": {
    "session_id": "session ID",
    "query": "How do I use the user authentication API?"
  }
}
```

## Error handling

### Common errors and fixes

#### 1. Connection error
```json
{
  "error": "Connection refused",
  "solution": "Check that AIKB_BASE_URL is correct and the service is running"
}
```

#### 2. Authentication error
```json
{
  "error": "Unauthorized",
  "solution": "Check that AIKB_API_KEY is set correctly"
}
```

#### 3. Resource not found
```json
{
  "error": "Knowledge base not found",
  "solution": "Check the knowledge base ID, or create the knowledge base first"
}
```

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

### Custom session strategy
```json
{
  "tool": "create_session",
  "arguments": {
    "kb_id": "kb_123456",
    "max_rounds": 20,
    "enable_rewrite": true,
    "fallback_response": "Based on the available knowledge, I can't answer your question accurately. Try rephrasing it or contact technical support."
  }
}
```

## Performance tips

1. **Batch operations**: create and update knowledge in batches where possible
2. **Search thresholds**: set search thresholds to balance accuracy and performance
3. **Session management**: clean up sessions you no longer need to save resources
4. **Log monitoring**: use the `--verbose` option to watch performance metrics

## Integration

### Claude Desktop
Add this to the Claude Desktop configuration file:
```json
{
  "mcpServers": {
    "aikb": {
      "command": "python",
      "args": ["path/to/main.py"],
      "env": {
        "AIKB_BASE_URL": "http://localhost:8080/api/v1",
        "AIKB_API_KEY": "your_api_key"
      }
    }
  }
}
```

Project repository: https://github.com/jrmatherly/ai-kb/tree/main/mcp-server

### Other MCP clients
See each client's documentation to configure the server launch command and environment variables.

## Troubleshooting

If you run into problems:
1. Run `python main.py --check-only` to check the environment
2. Run `python main.py --verbose` to see detailed logs
3. Check that the AI KB service is running
4. Check the network connection and firewall settings
