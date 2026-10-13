# Go SDK

The Go SDK wraps create, read, update and delete calls for the main resources (knowledge bases, documents, sessions and more) and SSE streaming chat. The source lives in `client/` and ships as its own Go module. The official CLI is built on this SDK.

## Installation

Install the module with `go get` and import the `client` package. The module path is defined in `client/go.mod`:

```text
module github.com/jrmatherly/ai-kb/client

go 1.24.2
```

Install it:

```bash
go get github.com/jrmatherly/ai-kb/client
```

Import it:

```go
import "github.com/jrmatherly/ai-kb/client"
```

## Initialization and authentication

This section covers creating a client and choosing how it authenticates. The core types and the constructor are defined in `client/client.go`.

### The Client type

```go
type Client struct {
    baseURL       string
    httpClient    *http.Client
    streamTimeout time.Duration
    apiKey        string
    bearerToken   string
    tenantID      *uint64
}
```

Create an instance with `NewClient(baseURL string, options ...ClientOption) *Client`. Ordinary requests time out after 30 seconds by default. Streaming (SSE) requests have **no timeout** by default; their lifetime is controlled by the `context`, unless you call `WithTimeout`.

### ClientOption reference

| Option | Description |
|---|---|
| `WithAPIKey(key string)` | Sets a long-lived API key, sent as the `X-API-Key` header |
| `WithBearerToken(token string)` | Sets a short-lived JWT, sent as `Authorization: Bearer <token>` (usually after a successful `Login`) |
| `WithToken(token string)` | **Deprecated**. The v0.x compatibility alias for `WithAPIKey`; it will be removed in the next major version |
| `WithTimeout(timeout time.Duration)` | Sets the upper bound for both ordinary and streaming requests |
| `WithTransport(rt http.RoundTripper)` | Replaces the underlying `http.RoundTripper` (for retry, instrumentation or signing middleware); pass `nil` to restore `http.DefaultTransport` |
| `WithTenantID(tenantID uint64)` | Adds an `X-Tenant-ID` header to every request. Use it only for explicit cross-workspace access by callers with the `CanAccessAllTenants` permission |

### Authentication methods

The SDK supports two credential kinds. Both can be set at once; `X-API-Key` takes precedence at the HTTP layer.

- **API key** (long-lived): `WithAPIKey`, header `X-API-Key`.
- **Bearer JWT** (short-lived): `WithBearerToken`, header `Authorization: Bearer <token>`. Use it with `Login`, `RefreshToken` and `GetCurrentUser` in `client/auth.go`.

A typical JWT login (`POST /api/v1/auth/login`). The login response carries the access token at the top level as `token` (`LoginResponse.Token`), not inside `data`:

```go
c := client.NewClient("http://localhost:8080")
loginResp, err := c.Login(ctx, client.LoginRequest{Email: "user@example.com", Password: "..."})
// Then build an authenticated client with the returned access token
authed := client.NewClient("http://localhost:8080",
    client.WithBearerToken(loginResp.Token))
```

`LoginResponse.ActiveTenant` is the workspace encoded in the JWT. `RefreshToken` returns a new pair as `AccessToken` and `RefreshToken`.

### Workspace (tenant) and injected headers

`applyAuthHeaders` (`client/client.go`) adds these headers to every request:

- `X-API-Key` and `Authorization`, as configured.
- `X-Request-ID`: read from `ctx.Value("RequestID")` (a string), for request tracing.
- `X-Tenant-ID`: the `"TenantID"` value in the context (`uint64`, `*uint64` or a numeric string) wins over the client-wide default from `WithTenantID`.

Override the workspace for one request:

```go
tenantID := uint64(10000)
ctx := context.WithValue(context.Background(), "TenantID", &tenantID)
kb, err := apiClient.GetKnowledgeBase(ctx, kbID)
```

JWTs and workspace API keys already carry the workspace identity, so ordinary users **should not** set `X-Tenant-ID`. The server auth middleware runs the cross-workspace check on any bearer request that carries this header, and ordinary users get 403.

### The Raw escape hatch

`Client.Raw(ctx, method, path, body)` (Experimental) sends any HTTP request with the client's configured auth headers. It exists for one-off integrations and the `aikb api` CLI passthrough. Prefer a typed method when one exists.

## Resources and methods

This section lists every public `Client` method by resource, so you can find the call for a task. Internal methods (`buildRequest`, `doRequest`, `doRequestStream` and others) are not listed.

### Auth: `client/auth.go`

| Method | Description |
|---|---|
| `Login` | Email and password login; returns the JWT access and refresh tokens |
| `GetCurrentUser` | Returns the signed-in principal and workspace information (`GET /api/v1/auth/me`) |
| `RefreshToken` | Exchanges a refresh token for a new access token |
| `SwitchTenant` | Switches to the given workspace and issues a new token pair; also records it as the default workspace for the next login (`POST /api/v1/auth/switch-tenant`) |
| `ChangePassword` | Changes the current user's password. On success the server revokes all of the user's sessions; discard local tokens |
| `GetAuthConfig` | Reads the public auth settings: registration mode and whether complex passwords are enforced (`GET /api/v1/auth/config`, no authentication needed) |

### Knowledge bases: `client/knowledgebase.go`

| Method | Description |
|---|---|
| `CreateKnowledgeBase` | Creates a knowledge base |
| `GetKnowledgeBase` | Returns knowledge base details |
| `ListKnowledgeBases` | Lists knowledge bases |
| `UpdateKnowledgeBase` | Updates a knowledge base |
| `DeleteKnowledgeBase` | Deletes a knowledge base |
| `ClearKnowledgeBaseContents` | Removes all content from a knowledge base |
| `HybridSearch` / `HybridSearchDetailed` | Hybrid search (vector + keyword) inside a knowledge base; pass `ResourceURLOptions` to get direct file URLs. The `Detailed` variant also returns the rerank diagnostics in `Meta` |
| `TogglePinKnowledgeBase` | Pins or unpins a knowledge base |
| `ListMoveTargets` | Lists the knowledge bases that documents can be moved to |
| `CopyKnowledgeBase` | Copies a knowledge base asynchronously and returns the task information |
| `DuplicateKnowledgeBase` | Creates a settings-only duplicate of a knowledge base |
| `GetKBCloneProgress` | Returns the progress of a copy task |

### Knowledge: `client/knowledge.go`

| Method | Description |
|---|---|
| `CreateKnowledgeFromFile` | Uploads a local file as a knowledge item (multipart; supports metadata, the multimodal switch, a custom file name, the channel and parse config overrides) |
| `CreateKnowledgeFromURL` | Creates a knowledge item from a URL |
| `GetKnowledge` | Returns knowledge item details |
| `GetKnowledgeBatch` | Returns several knowledge items at once |
| `ListKnowledge` | Lists knowledge items with pagination |
| `ListKnowledgeWithFilter` | Lists knowledge items with filters (`KnowledgeListFilter`: tag, keyword, file type, parse status, source, time range, folder) |
| `DeleteKnowledge` | Deletes a knowledge item |
| `DownloadKnowledgeFile` | Downloads the original file of a knowledge item to a local path |
| `OpenKnowledgeFile` | Opens the original file as a stream (returns the file name and an `io.ReadCloser`) |
| `DownloadKnowledgeFiles` / `OpenKnowledgeFilesArchive` | Packs the original files of several documents in one knowledge base into a ZIP, saved to a local path or read as a stream (`POST /api/v1/knowledge-bases/{id}/knowledge/batch-download`; at most 200 IDs and 512 MiB in total per request; requires Contributor and write access to the knowledge base). Uses the streaming HTTP client, so the 30-second default timeout does not apply |
| `ListKnowledgeFolders` / `MoveKnowledgeToFolder` / `RenameKnowledgeFolder` | The knowledge base folder tree; moving documents into a folder (an empty `FolderPath` moves them back to the root); renaming a folder and every path below it |
| `UpdateKnowledge` | Updates a knowledge item |
| `ReparseKnowledge` | Parses a knowledge item again |
| `CancelKnowledgeParse` | Cancels a parse task |
| `GetKnowledgeProcessingSpans` | Returns the processing trace spans of a knowledge item |
| `UpdateImageInfo` | Updates image information |
| `CreateManualKnowledge` | Creates a hand-written (manual) knowledge item |
| `UpdateManualKnowledge` | Updates a hand-written knowledge item |
| `FilterKnowledge` | Filters knowledge items by keyword, file type or agent |
| `MoveKnowledge` | Moves knowledge items to another knowledge base |
| `GetKnowledgeMoveProgress` | Returns the progress of a move task |
| `PreviewKnowledgeFile` | Previews a knowledge file (returns the raw `*http.Response`) |
| `BatchUpdateKnowledgeTags` | Updates the tags of several knowledge items |

Since v0.8.2 the server's knowledge list endpoint accepts `sort_by` (`updated_at` / `created_at` / `file_name`) and `sort_order` (`asc` / `desc`); the default is still `created_at desc`. `KnowledgeListFilter` has no fields for them yet. For a custom sort order call `Raw`; the parameters are described in the [knowledge API](../04-api/02-api-knowledge.md).

### Chunks: `client/chunk.go`

| Method | Description |
|---|---|
| `ListKnowledgeChunks` | Lists the chunks of a knowledge item with pagination |
| `UpdateChunk` | Updates a chunk's content or enabled state |
| `DeleteChunk` | Deletes a chunk |
| `GetChunkByIDOnly` | Returns a chunk by its chunk ID alone |
| `DeleteGeneratedQuestion` | Deletes a question generated for a chunk |
| `DeleteChunksByKnowledgeID` | Deletes every chunk of a knowledge item |

`DeleteGeneratedQuestion` calls `DELETE /api/v1/chunks/by-id/{id}/questions` with `{"question_id":"..."}`.

### Sessions: `client/session.go`

| Method | Description |
|---|---|
| `CreateSession` | Creates a session |
| `GetSession` | Returns a session |
| `GetSessionsByTenant` | Lists the workspace's sessions with pagination |
| `UpdateSession` | Updates a session |
| `DeleteSession` | Deletes a session |
| `BatchDeleteSessions` | Deletes several sessions |
| `GenerateTitle` | Generates a session title |
| `KnowledgeQAStream` | Knowledge Q&A (SSE streaming; see below) |
| `ContinueStream` | Resumes a stream still in progress (after a disconnect) |
| `StopSession` | Stops generation of one assistant message |
| `SearchKnowledge` / `SearchKnowledgeDetailed` | Knowledge retrieval without LLM summarization; the `Detailed` variant also returns the rerank diagnostics in `Meta` |

### Messages: `client/message.go`, `client/message_suggestion.go`

| Method | Description | Source file |
|---|---|---|
| `LoadMessages` | Loads messages by time | `client/message.go` |
| `GetRecentMessages` | Returns the latest N messages | `client/message.go` |
| `GetMessagesBefore` | Returns messages before a point in time | `client/message.go` |
| `SearchMessages` | Searches message history | `client/message.go` |
| `GetChatHistoryKBStats` | Chat history statistics per knowledge base | `client/message.go` |
| `DeleteMessage` | Deletes a message | `client/message.go` |
| `EnsureMessageSuggestions` | Makes sure suggested questions exist (optionally forces regeneration) | `client/message_suggestion.go` |
| `GetMessageSuggestions` | Returns a message's suggested questions | `client/message_suggestion.go` |
| `RecordMessageSuggestionEvent` | Reports a click or impression event for a suggested question | `client/message_suggestion.go` |

### Agent chat (streaming): `client/agent.go`

| Method | Description |
|---|---|
| `AgentQAStream` | Agent-mode streaming Q&A (Deprecated, simplified entry point) |
| `AgentQAStreamWithRequest` | Agent-mode streaming Q&A with the full `AgentQARequest` payload |
| `NewAgentSession` | Creates an `AgentSession` wrapper (with `Ask`, `AskWithRequest` and `GetSessionID`) |

### Agent management: `client/agent_manage.go`

| Method | Description |
|---|---|
| `CreateAgent` | Creates a custom agent |
| `ListAgents` | Lists agents |
| `GetAgent` | Returns an agent |
| `UpdateAgent` | Updates an agent |
| `DeleteAgent` | Deletes an agent |
| `CopyAgent` | Copies an agent |
| `GetAgentPlaceholders` | Returns the agent configuration placeholders |
| `GetSuggestedQuestions` | Returns an agent's suggested questions |

Two request fields changed in v0.8.2:

- `UpdateAgentRequest.Avatar` changed from `string` to `*string`. With `nil` the field is not sent and the stored avatar is kept; a pointer to an empty string clears the avatar. After upgrading the SDK, adjust assignments (for example `Avatar: &avatar`).
- The suggested question type `SuggestedQuestion` gained `KnowledgeID` (the source document). When a user picks a suggested question, send its source back in `QuestionOrigin` (`KnowledgeBaseID`, `KnowledgeID`) of `KnowledgeQARequest` or `AgentQARequest`. The server searches that source first before answering. It is only a hint and never widens the request's own retrieval scope.

### Models: `client/model.go`

| Method | Description |
|---|---|
| `CreateModel` | Creates a model |
| `GetModel` | Returns a model |
| `ListModels` | Lists models |
| `UpdateModel` | Updates a model |
| `DeleteModel` | Deletes a model |
| `ListModelProviders` | Lists model providers, optionally filtered by model type |

### Workspaces (Tenant): `client/tenant.go`

| Method | Description |
|---|---|
| `CreateTenant` | Creates a workspace |
| `GetTenant` | Returns a workspace |
| `UpdateTenant` | Updates a workspace |
| `DeleteTenant` | Deletes a workspace |
| `ListTenants` | Lists workspaces |
| `ListAllTenants` | Lists all workspaces (administrators) |
| `SearchTenants` | Searches workspaces (paginated) |
| `ListTenantAPIKeys` | Lists the workspace's API keys |
| `CreateTenantAPIKey` | Creates a workspace API key |
| `DeleteTenantAPIKey` | Deletes a workspace API key |
| `GetTenantKV` | Reads a workspace-level KV setting |
| `UpdateTenantKV` | Updates a workspace-level KV setting |
| `GetAPIPrincipalConfig` | Returns the API principal configuration |
| `UpdateAPIPrincipalConfig` | Updates the API principal configuration |
| `CreateAPIPrincipalTestToken` | Creates an API principal test token |

### Organizations and sharing: `client/organization.go`

| Method | Description |
|---|---|
| `CreateOrganization` / `ListMyOrganizations` / `GetOrganization` / `UpdateOrganization` / `DeleteOrganization` | Organization CRUD |
| `SearchOrganizations` / `PreviewOrganizationByInviteCode` | Search organizations; preview an organization by invite code |
| `JoinOrganizationByInviteCode` / `SubmitJoinRequest` / `JoinByOrganizationID` / `LeaveOrganization` / `RequestRoleUpgrade` | Join, leave, request a role upgrade |
| `GenerateInviteCode` / `SearchTenantsForInvite` / `InviteTenant` / `InviteMember` | Invite members (see the note on `SearchTenantsForInvite` below) |
| `ListOrgMembers` / `UpdateMemberRole` / `RemoveMember` | Member management |
| `ListJoinRequests` / `ReviewJoinRequest` | Join request review |
| `ShareKnowledgeBase` / `ListKBShares` / `UpdateSharePermission` / `RemoveKBShare` | Knowledge base sharing |
| `ShareAgent` / `ListAgentShares` / `RemoveAgentShare` | Agent sharing |
| `ListOrgShares` / `ListOrgAgentShares` / `ListSharedKnowledgeBases` / `ListSharedAgents` | Shared resource queries |

Since v0.8.2 the server resolves invitation candidates only by **full workspace ID** and no longer searches by workspace name, user name or email. `SearchTenantsForInvite(ctx, orgID, workspaceID)` calls `GET /api/v1/organizations/{id}/search-tenants?q=<workspace ID>` and returns a `[]TenantInviteCandidate` (`tenant_id`, `tenant_name`). The list is empty when the ID is unknown or the workspace is already a member. `InviteTenant(ctx, orgID, tenantID, role)` adds a candidate's workspace directly (`POST /api/v1/organizations/{id}/invite` with `tenant_id` and `role`). `InviteMember(ctx, orgID, userID, role)` sends `user_id` instead, and the server adds that user's workspace.

### FAQ: `client/faq.go`

| Method | Description |
|---|---|
| `ListFAQEntries` | Lists FAQ entries with pagination |
| `UpsertFAQEntries` | Adds or updates FAQ entries in bulk |
| `CreateFAQEntry` | Creates one FAQ entry |
| `GetFAQEntry` | Returns one FAQ entry |
| `UpdateFAQEntry` | Updates one FAQ entry |
| `AddSimilarQuestions` | Adds similar questions |
| `UpdateFAQEntryFieldsBatch` | Updates fields in bulk |
| `UpdateFAQEntryTagBatch` | Updates tags in bulk |
| `DeleteFAQEntries` | Deletes in bulk |
| `SearchFAQEntries` | Searches FAQ entries |
| `ExportFAQEntries` | Exports as CSV (returns `[]byte`) |
| `GetFAQImportProgress` | Returns the progress of an asynchronous import task (including dry runs) |
| `UpdateLastFAQImportResultDisplayStatus` | Updates the display status of the latest import result |

### Tags: `client/tag.go`

| Method | Description |
|---|---|
| `ListTags` | Lists tags |
| `CreateTag` | Creates a tag |
| `UpdateTag` / `UpdateTagBySeqID` | Updates a tag (by ID / by seq ID) |
| `DeleteTag` / `DeleteTagBySeqID` | Deletes a tag (by ID / by seq ID) |

### MCP services: `client/mcp_service.go`

| Method | Description |
|---|---|
| `CreateMCPService` / `ListMCPServices` / `GetMCPService` / `UpdateMCPService` / `DeleteMCPService` | MCP service CRUD |
| `TestMCPService` | Connectivity test |
| `GetMCPServiceTools` / `GetMCPServiceResources` | Lists MCP tools and resources |
| `GetMCPMetadata` | Reads the saved tool directory without connecting upstream. Returns `nil` if the service has never been synchronized; `Stale=true` means the saved connection no longer matches the current configuration |
| `RefreshMCPMetadata` | Connects upstream and replaces the saved tool directory as a whole. OAuth services save a snapshot per caller (Viewer+); static-auth services write a workspace-wide snapshot (requires Admin, or an API key that can manage MCP services) |
| `ResolveToolApproval` | Resolves a tool call approval |

`MCPService` list items carry `Catalog` (tool count, whether stale, sync time) and `UsageInstructions`.

### MCP endpoints (AI KB as an MCP server): `client/mcp_endpoint.go`

| Method | Description |
|---|---|
| `ListMCPEndpoints` / `GetMCPEndpoint` / `CreateMCPEndpoint` / `UpdateMCPEndpoint` / `DeleteMCPEndpoint` | CRUD for the MCP endpoints the workspace publishes. `Token` appears only in the create and rotate responses |
| `RotateMCPEndpointToken` | Rotates the endpoint token; the old token stops working immediately |
| `GetMCPEndpointToolCatalog` | The tools an endpoint can expose, with groups and default selection |

### Initialization and model checks: `client/initialization.go`

| Method | Description |
|---|---|
| `GetInitializationConfig` / `InitializeByKB` / `UpdateKBConfig` / `SetKBModelConfig` | Knowledge base initialization and model configuration |
| `CheckOllamaStatus` / `ListOllamaModels` / `CheckOllamaModels` | Ollama status and model detection |
| `DownloadOllamaModel` / `GetOllamaDownloadProgress` / `ListOllamaDownloadTasks` | Ollama model download tasks |
| `CheckRemoteModel` / `TestEmbeddingModel` / `CheckRerankModel` / `TestMultimodalFunction` | Connectivity checks for remote LLM, embedding, rerank and multimodal models |
| `ExtractTextRelations` | Text relation extraction test |

### System: `client/system.go`

| Method | Description |
|---|---|
| `GetSystemInfo` | Returns system information (version and more) |
| `GetDeploymentCapabilities` | Returns the deployment capability snapshot (`GET /api/v1/system/capabilities`: the edition, and `supported` / `reason` for each capability) |
| `ListParserEngines` / `CheckParserEngines` | Lists and checks document parser engines |
| `ReconnectDocReader` | Reconnects the DocReader service |
| `GetStorageEngineStatus` / `CheckStorageEngine` | Storage engine status and check |

### Other

| Method | Description | Source file |
|---|---|---|
| `StartEvaluation` / `GetEvaluationResult` | Starts an evaluation task / returns its result | `client/evaluation.go` |
| `ListSkills(ctx, sandboxConfigID)` | Lists the skills a sandbox configuration can invoke; returns the skill list and an availability flag | `client/skill.go` |
| `GetWebSearchProviders` | Lists the available web search providers | `client/web_search.go` |
| `Raw` | Raw HTTP escape hatch (Experimental) | `client/client.go` |

In total there are about 230 public `Client` methods across more than 20 resource groups.

### Memory, sandbox skills and personal variables

| File | Methods and purpose |
| --- | --- |
| `client/memory.go` | GetMemorySettings / UpdateMemorySettings; List/Create/Update/DeleteMemoryItem; Confirm/RejectMemoryItem; ClearMemoryItems |
| `client/memory.go` | ListMemoryTopics / PromoteMemoryTopic / DeleteMemoryTopic; ListMemoryDocuments / DeleteMemoryDocument; ExportMemory / ConsolidateMemory |
| `client/skill.go` | InstallSandboxSkillFromSource / UploadSandboxSkill / ReinstallSandboxSkill / StopSandboxSkill, for the install flow |
| `client/skill.go` | UpdateSandboxSkill / SetSandboxSkillEnabled / SetSandboxSkillEnvValues; ListSandboxSkillFiles / GetSandboxSkillFile |
| `client/env_var.go` | ListMyEnvVars; SetMySkillEnvVar / DeleteMySkillEnvVar; SetMySandboxEnvVar / DeleteMySandboxEnvVar |
| `client/tenant.go` | UpdateTenantAPIKey, which replaces the full authorization configuration of an existing key without rotating its token |

Workspace skill variables are set by administrators; personal variables apply only to the caller. The list never returns values in plain text. The backend endpoints still enforce permissions for memory and skill methods; the SDK does not bypass them.

```go
items, total, err := c.ListMemoryItems(ctx, "active", 50, 0)
_ = items
_ = total
_ = err

skills, available, err := c.ListSkills(ctx, "sandbox-config-id")
_ = skills
_ = available
_ = err
```

A system administrator creates users with `POST /api/v1/system/admin/users/create`. The SDK has no dedicated method for this endpoint; use the `Raw` escape hatch or another HTTP client. The response is described in the [system API](../04-api/02-api-system.md). The full contracts are in the [long-term memory API](../04-api/02-api-memory.md) and the [sandbox and skills API](../04-api/02-api-sandbox-skills.md).

## Streaming chat (SSE)

This section shows how to consume streamed answers. The streaming methods use a **callback**, not a channel. The SDK parses SSE line by line with `bufio.Scanner` (`event:` and `data:` prefixes, a blank line ends a frame) and calls the callback once per parsed frame. If the callback returns a non-nil error, the stream stops. The SSE line buffer is raised to 4 MiB (`scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)`) so that large references frames do not fail with "token too long".

Streaming requests go through `doRequestStream` (`client/client.go`). By default the 30-second timeout does not apply; the stream's lifetime is controlled by the `ctx` you pass.

### Knowledge Q&A stream: `KnowledgeQAStream` (`client/session.go`)

```go
func (c *Client) KnowledgeQAStream(
    ctx context.Context,
    sessionID string,
    request *KnowledgeQARequest,
    callback func(*StreamResponse) error,
    opts ...ResourceURLOptions,
) error
```

Each `StreamResponse` frame carries `ResponseType` (`answer`, `references`, `thinking`, `tool_call`, `tool_result`, `error`, `reflection`, `session_title`, `agent_query`, `complete`), an incremental `Content` and the end marker `Done`. Sources arrive in `KnowledgeReferences` on `references` frames. Pass `ResourceURLOptions` to receive public HTTP(S) file URLs in the stream.

### Agent Q&A stream: `AgentQAStreamWithRequest` (`client/agent.go`)

```go
type AgentEventCallback func(*AgentStreamResponse) error

func (c *Client) AgentQAStreamWithRequest(ctx context.Context,
    sessionID string, request *AgentQARequest, callback AgentEventCallback,
    opts ...ResourceURLOptions,
) error
```

`AgentQARequest` supports `KnowledgeBaseIDs`, `AgentID`, `WebSearchEnabled`, `MentionedItems` (@mentions of knowledge bases, files, tags, MCP services or skills), `Images` (multimodal images) and more. You can also use the convenience wrapper:

```go
as := apiClient.NewAgentSession(session.ID)
err := as.Ask(ctx, "Give me an introduction to AI KB", func(ev *client.AgentStreamResponse) error {
    if ev.ResponseType == client.AgentResponseTypeAnswer {
        fmt.Print(ev.Content)
    }
    return nil
})
```

### Resuming after a disconnect: `ContinueStream` (`client/session.go`)

`ContinueStream(ctx, sessionID, messageID, callback)` resumes a stream the server is still generating, through `GET /api/v1/sessions/continue-stream/{sessionID}?message_id=...`. The callback works as in `KnowledgeQAStream`. Call `StopSession(ctx, sessionID, messageID)` to stop the generation.

## Error handling

This section shows how to tell HTTP errors and stream errors apart.

### HTTP layer: `APIError` (`client/client.go`)

Every non-2xx response is wrapped as `*APIError`. Use `errors.As` to branch on the HTTP status code or the server's structured error code:

```go
var apiErr *client.APIError
if errors.As(err, &apiErr) {
    switch {
    case apiErr.StatusCode == 404:
        // Resource not found
    case apiErr.Code == client.ServerErrUnauthorized: // 1001
        // Trigger a new login
    }
}
```

`Code` is the structured error code from the `{"code":N}` response body. The package defines the constants `ServerErrBadRequest` (1000) through `ServerErrValidation` (1010). `Error()` keeps the old `"HTTP error <status>: <body>"` format for consumers that match on the string.

### Stream layer: `SSEStreamError` (`client/stream_errors.go`)

When the server sends a terminal error frame on an SSE stream (`response_type=error, done=true`), the SDK **first passes that frame to the callback**, then returns `*SSEStreamError`:

```go
type SSEStreamError struct {
    Content string // Content of the error frame
}
```

Two equivalent checks (the first is recommended):

```go
// Option 1: the sentinel error (SSEStreamError.Unwrap() returns it)
if errors.Is(err, client.ErrSSEStreamTerminal) { ... }

// Option 2: the helper (also matches older fmt.Errorf("SSE stream error: ...") chains)
if client.IsSSEStreamError(err) { ... }
```

## Logging and tracing

This section shows how to turn on SDK debug output and pass a request ID. `client/log.go` provides an internal debug logger based on `log/slog`. By default it writes to `io.Discard`, so the SDK is silent for its users. An embedding application can call this at startup:

```go
client.SetDebugLevel("debug") // "debug"/"info"/"warn"; any other value (including "error" and "") is silent
```

Output goes to stderr and includes per-line SSE parsing and request failures. The function is **not safe for concurrent use**; call it once before any SDK call.

For tracing, put `"RequestID"` (a string) in the context and the SDK sends it as the `X-Request-ID` header (see `applyAuthHeaders` in `client/client.go`):

```go
ctx := context.WithValue(context.Background(), "RequestID", "req-20260727-0001")
```

## Complete examples

These examples are adapted from the code in `client/example.go`.

### Example 1: create a knowledge base and upload a file

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/jrmatherly/ai-kb/client"
)

func main() {
    apiClient := client.NewClient(
        "http://localhost:8080",
        client.WithAPIKey("your-api-key"),
        client.WithTimeout(30*time.Second),
    )

    // Create a knowledge base
    kb := &client.KnowledgeBase{
        Name:        "Test Knowledge Base",
        Description: "This is a test knowledge base",
        ChunkingConfig: client.ChunkingConfig{
            ChunkSize:    500,
            ChunkOverlap: 50,
            Separators:   []string{"\n\n", "\n", ". ", "? ", "! "},
        },
        EmbeddingModelID: "embedding_model_id",
        SummaryModelID:   "summary_model_id",
    }
    createdKB, err := apiClient.CreateKnowledgeBase(context.Background(), kb)
    if err != nil {
        fmt.Printf("Failed to create knowledge base: %v\n", err)
        return
    }
    fmt.Printf("Knowledge base created: ID=%s, Name=%s\n", createdKB.ID, createdKB.Name)

    // Upload a file as a knowledge item
    metadata := map[string]string{"source": "local", "type": "document"}
    knowledge, err := apiClient.CreateKnowledgeFromFile(
        context.Background(), createdKB.ID, "path/to/sample.pdf",
        metadata, nil, "", "", nil)
    if err != nil {
        fmt.Printf("Failed to upload knowledge file: %v\n", err)
        return
    }
    fmt.Printf("File uploaded: Knowledge ID=%s, Title=%s\n", knowledge.ID, knowledge.Title)
}
```

### Example 2: create a session and stream a knowledge Q&A

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "strings"

    "github.com/jrmatherly/ai-kb/client"
)

func main() {
    apiClient := client.NewClient("http://localhost:8080",
        client.WithAPIKey("your-api-key"))

    // Create a session
    session, err := apiClient.CreateSession(context.Background(), &client.CreateSessionRequest{
        Title:       "Test Session",
        Description: "A test session for knowledge Q&A",
    })
    if err != nil {
        fmt.Printf("Failed to create session: %v\n", err)
        return
    }

    // Streaming Q&A: collect the answer and the sources
    question := "What is artificial intelligence?"
    var answer strings.Builder
    var references []*client.SearchResult

    err = apiClient.KnowledgeQAStream(context.Background(),
        session.ID,
        &client.KnowledgeQARequest{Query: question},
        func(response *client.StreamResponse) error {
            if response.ResponseType == client.ResponseTypeAnswer {
                answer.WriteString(response.Content)
            }
            if response.ResponseType == client.ResponseTypeReferences {
                references = response.KnowledgeReferences
            }
            return nil
        })
    if err != nil {
        // Tell a terminal SSE error frame apart from other errors
        if errors.Is(err, client.ErrSSEStreamTerminal) {
            fmt.Printf("Stream terminated by server error: %v\n", err)
        } else {
            fmt.Printf("Q&A failed: %v\n", err)
        }
        return
    }
    fmt.Printf("Answer: %s\n", answer.String())
    for i, ref := range references {
        fmt.Printf("Reference %d: %s\n", i+1, ref.Content)
    }
}
```

### Example 3: message history, chunk management and cleanup

```go
package main

import (
    "context"
    "fmt"

    "github.com/jrmatherly/ai-kb/client"
)

func main() {
    apiClient := client.NewClient("http://localhost:8080",
        client.WithAPIKey("your-api-key"))
    ctx := context.Background()

    // Get the 10 most recent session messages
    sessionID := "your-session-id"
    messages, err := apiClient.GetRecentMessages(ctx, sessionID, 10)
    if err != nil {
        fmt.Printf("Failed to get session messages: %v\n", err)
    } else {
        for i, msg := range messages {
            fmt.Printf("%d. Role: %s, Content: %s\n", i+1, msg.Role, msg.Content)
        }
    }

    // Manage knowledge chunks: list one page and update the first chunk
    knowledgeID := "your-knowledge-id"
    chunks, total, err := apiClient.ListKnowledgeChunks(ctx, knowledgeID, 1, 10)
    if err != nil {
        fmt.Printf("Failed to get knowledge chunks: %v\n", err)
    } else {
        fmt.Printf("Knowledge has %d chunks, retrieved %d\n", total, len(chunks))
        if len(chunks) > 0 {
            updated, err := apiClient.UpdateChunk(ctx, knowledgeID, chunks[0].ID,
                &client.UpdateChunkRequest{
                    Content:   "Updated chunk content - " + chunks[0].Content,
                    IsEnabled: true,
                })
            if err != nil {
                fmt.Printf("Failed to update chunk: %v\n", err)
            } else {
                fmt.Printf("Chunk updated: ID=%s\n", updated.ID)
            }
        }
    }

    // Clean up
    if err := apiClient.DeleteSession(ctx, sessionID); err != nil {
        fmt.Printf("Failed to delete session: %v\n", err)
    }
    if err := apiClient.DeleteKnowledge(ctx, knowledgeID); err != nil {
        fmt.Printf("Failed to delete knowledge: %v\n", err)
    }
}
```

## Source reference

These files are the starting points for reading the SDK code.

- Client core and error types: `client/client.go`
- Authentication: `client/auth.go`
- Streaming Q&A: `client/session.go`, `client/agent.go`
- Stream errors: `client/stream_errors.go`
- Logging: `client/log.go`
- Full usage example: `client/example.go`
