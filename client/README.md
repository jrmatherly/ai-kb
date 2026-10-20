# AI KB HTTP client

This package is a client library for the AI KB service. It wraps the server's REST API under `/api/v1`, so other modules can integrate with AI KB without writing HTTP requests by hand.

## Features

The client has these feature areas:

1. **Sessions**: create, get, update and delete sessions
2. **Knowledge bases**: create, get, update and delete knowledge bases
3. **Knowledge items**: add, get and delete knowledge content
4. **Workspaces**: workspace CRUD
5. **Knowledge Q&A**: streaming Q&A
6. **Agent Q&A**: agent-based Q&A with thinking, tool calls and reflection
7. **Chunks**: query, update and delete knowledge chunks
8. **Messages**: get and delete session messages
9. **Models**: create, get, update and delete models
10. **Evaluation**: start evaluation tasks and get evaluation results
11. **Sandbox skills**: install skills into a sandbox configuration (zip upload, or from sources such as ClawHub / SkillHub / GitHub) and configure the environment variables the skills need
12. **Long-term memory**: the current user's cross-session memory (on/off setting, add/edit/delete entries, confirm/reject, topics, document affinity, export, consolidate now)
13. **Authentication**: log in, refresh tokens, switch the active workspace (`SwitchTenant` records the last active tenant preference)

## Usage

### Create a client

```go
import (
    "context"
    "github.com/jrmatherly/ai-kb/client"
    "time"
)

// Create a client
apiClient := client.NewClient(
    "http://api.example.com",
    client.WithToken("your-auth-token"),
    client.WithTimeout(30*time.Second),
)
```

### Workspace

`WithTenantID` sets a default workspace; every request then carries the `X-Tenant-ID` header:

```go
tenantID := uint64(10000)
apiClient := client.NewClient(
    "http://api.example.com",
    client.WithToken("your-auth-token"),
    client.WithTenantID(tenantID),
)
```

To switch workspace for a single request, set `TenantID` in the `context`. The value can be a `uint64`, a `*uint64` or a number as a string, and the client uses it in preference to the default:

```go
ctx := context.WithValue(context.Background(), "TenantID", uint64(10000))
// Pass ctx to any client method to use workspace 10000
```

### Example: create a knowledge base and upload a file

```go
// Create a knowledge base
kb := &client.KnowledgeBase{
    Name:        "Test knowledge base",
    Description: "A knowledge base for testing",
    ChunkingConfig: client.ChunkingConfig{
        ChunkSize:    500,
        ChunkOverlap: 50,
        Separators:   []string{"\n\n", "\n", ". ", "? ", "! "},
    },
    ImageProcessingConfig: client.ImageProcessingConfig{
        ModelID: "image_model_id",
    },
    EmbeddingModelID: "embedding_model_id",
    SummaryModelID:   "summary_model_id",
}

kb, err := apiClient.CreateKnowledgeBase(context.Background(), kb)
if err != nil {
    // handle the error
}

// Upload a knowledge file with metadata
metadata := map[string]string{
    "source": "local",
    "type":   "document",
}
knowledge, err := apiClient.CreateKnowledgeFromFile(context.Background(), kb.ID, "path/to/file.pdf", metadata, nil, "", "", nil)
if err != nil {
    // handle the error
}

// Download original files from the same knowledge base as one ZIP (at most 200 IDs, 512 MiB in total)
err = apiClient.DownloadKnowledgeFiles(context.Background(), kb.ID, []string{knowledge.ID}, "knowledge-files.zip")
if err != nil {
    // handle the error
}
```

### Example: create a session and ask questions

```go
// Create a session. A session is only a conversation container: the knowledge
// bases and retrieval settings come with each question.
sessionRequest := &client.CreateSessionRequest{
    Title:       "Test session",
    Description: "A session for testing",
}

session, err := apiClient.CreateSession(context.Background(), sessionRequest)
if err != nil {
    // handle the error
}

// Streaming Q&A
err = apiClient.KnowledgeQAStream(context.Background(), session.ID, &client.KnowledgeQARequest{
    Query:            "What is machine learning?",
    KnowledgeBaseIDs: []string{knowledgeBaseID}, // optional: restrict to these knowledge bases
    WebSearchEnabled: false,                      // optional: enable web search
}, func(response *client.StreamResponse) error {
    // handle each response chunk
    fmt.Print(response.Content)
    return nil
})
if err != nil {
    // handle the error
}
```

### Example: agent Q&A

Agent Q&A is a more capable conversation mode, with tool calls, visible thinking and self-reflection.

```go
// Create an agent session
agentSession := apiClient.NewAgentSession(session.ID)

// Ask the agent, handling every event type
err := agentSession.Ask(context.Background(), "Search for knowledge about machine learning and summarize the key points",
    func(resp *client.AgentStreamResponse) error {
        switch resp.ResponseType {
        case client.AgentResponseTypeThinking:
            // the agent is thinking
            if resp.Done {
                fmt.Printf("💭 Thinking: %s\n", resp.Content)
            }

        case client.AgentResponseTypeToolCall:
            // the agent calls a tool
            if resp.Data != nil {
                toolName := resp.Data["tool_name"]
                fmt.Printf("🔧 Tool call: %v\n", toolName)
            }

        case client.AgentResponseTypeToolResult:
            // tool result
            fmt.Printf("✓ Tool result: %s\n", resp.Content)

        case client.AgentResponseTypeReferences:
            // knowledge references
            if resp.KnowledgeReferences != nil {
                fmt.Printf("📚 Found %d relevant knowledge items\n", len(resp.KnowledgeReferences))
                for _, ref := range resp.KnowledgeReferences {
                    fmt.Printf("  - [%.3f] %s\n", ref.Score, ref.KnowledgeTitle)
                }
            }

        case client.AgentResponseTypeAnswer:
            // final answer (streamed)
            fmt.Print(resp.Content)
            if resp.Done {
                fmt.Println() // newline at the end
            }

        case client.AgentResponseTypeReflection:
            // the agent's self-reflection
            if resp.Done {
                fmt.Printf("🤔 Reflection: %s\n", resp.Content)
            }

        case client.AgentResponseTypeError:
            // error
            fmt.Printf("❌ Error: %s\n", resp.Content)
        }
        return nil
    })

if err != nil {
    // handle the error
}

// Simple version: only the final answer
var finalAnswer string
err = agentSession.Ask(context.Background(), "What is deep learning?",
    func(resp *client.AgentStreamResponse) error {
        if resp.ResponseType == client.AgentResponseTypeAnswer {
            finalAnswer += resp.Content
        }
        return nil
    })
```

### Agent event types

| Event type | Meaning | When it fires |
| --------- | ------ | --------- |
| `AgentResponseTypeThinking` | Agent thinking | While the agent analyzes the question and plans |
| `AgentResponseTypeToolCall` | Tool call | When the agent decides to use a tool |
| `AgentResponseTypeToolResult` | Tool result | After the tool finishes |
| `AgentResponseTypeReferences` | Knowledge references | When relevant knowledge is retrieved |
| `AgentResponseTypeAnswer` | Final answer | While the agent writes the answer (streamed) |
| `AgentResponseTypeArtifactsPending` | Generated files uploading | After the answer ends, before the files are written to object storage |
| `AgentResponseTypeReflection` | Self-reflection | When the agent evaluates its own answer |
| `AgentResponseTypeError` | Error | When an error occurs |

### Example: manage models

```go
// Create a model
modelRequest := &client.CreateModelRequest{
    Name:        "Test model",
    Type:        client.ModelTypeKnowledgeQA,
    Source:      client.ModelSourceLocal,
    Description: "A model for testing",
    Parameters: client.ModelParameters{
        "temperature": 0.7,
        "top_p":       0.9,
    },
    IsDefault: true,
}
model, err := apiClient.CreateModel(context.Background(), modelRequest)
if err != nil {
    // handle the error
}

// List all models
models, err := apiClient.ListModels(context.Background())
if err != nil {
    // handle the error
}
```

### Example: manage knowledge chunks

```go
// List knowledge chunks
chunks, total, err := apiClient.ListKnowledgeChunks(context.Background(), knowledgeID, 1, 10)
if err != nil {
    // handle the error
}

// Update a chunk
updateRequest := &client.UpdateChunkRequest{
    Content:   "Updated chunk content",
    IsEnabled: true,
}
updatedChunk, err := apiClient.UpdateChunk(context.Background(), knowledgeID, chunkID, updateRequest)
if err != nil {
    // handle the error
}
```

### Example: reparse a knowledge item

```go
// Reparse a knowledge item (deletes the existing content and parses again)
// Use it when:
// 1. The original parse failed and needs a retry
// 2. The parsing configuration changed (chunking strategy, multimodal settings, ...)
// 3. The knowledge content changed and the parse result needs a refresh

knowledge, err := apiClient.ReparseKnowledge(context.Background(), knowledgeID)
if err != nil {
    // handle the error
}

// The knowledge item goes to "pending" and is reparsed asynchronously
fmt.Printf("Knowledge ID: %s\n", knowledge.ID)
fmt.Printf("Parse Status: %s\n", knowledge.ParseStatus)      // "pending"
fmt.Printf("Enable Status: %s\n", knowledge.EnableStatus)    // "disabled"

// Poll the parse status
for {
    time.Sleep(5 * time.Second)
    knowledge, err := apiClient.GetKnowledge(context.Background(), knowledgeID)
    if err != nil {
        // handle the error
    }

    if knowledge.ParseStatus == "completed" {
        fmt.Println("Knowledge re-parsing completed!")
        break
    } else if knowledge.ParseStatus == "failed" {
        fmt.Printf("Knowledge re-parsing failed: %s\n", knowledge.ErrorMessage)
        break
    }
}
```

### Example: cancel parsing

```go
// Cancel a running parse task (when resources are tight or the wrong file was uploaded)
// - knowledge items already completed / failed cannot be cancelled
// - chunks and indexes already written are kept; call ReparseKnowledge later to parse again

knowledge, err := apiClient.CancelKnowledgeParse(context.Background(), knowledgeID)
if err != nil {
    // handle the error
}
fmt.Printf("Parse Status: %s\n", knowledge.ParseStatus) // "cancelled"
```

### Example: view the document parsing trace (span tree)

```go
// Get the span tree of the document parsing pipeline (root → stage → subspan)
// - attempt 0 means the latest parse attempt
// - always returns the 5 standard stages: docreader / chunking / embedding / multimodal / postprocess
trace, err := apiClient.GetKnowledgeProcessingSpans(context.Background(), knowledgeID, 0)
if err != nil {
    // handle the error
}
fmt.Printf("ParseStatus=%s CurrentStage=%s\n", trace.ParseStatus, trace.CurrentStage)
for _, stage := range trace.Trace.Children {
    fmt.Printf("- %s: %s (%dms)\n", stage.Name, stage.Status, stage.DurationMs)
}
```

### Example: get session messages

```go
// Get recent messages
messages, err := apiClient.GetRecentMessages(context.Background(), sessionID, 10)
if err != nil {
    // handle the error
}

// Get messages before a given time
beforeTime := time.Now().Add(-24 * time.Hour)
olderMessages, err := apiClient.GetMessagesBefore(context.Background(), sessionID, beforeTime, 10)
if err != nil {
    // handle the error
}
```

### Example: install a sandbox skill from a hosted registry

`source` must be explicit: `@owner/slug` for ClawHub, the full `https://clawhub.ai/skills-sh/owner/repo/slug` or `skills-sh:owner/repo/slug` for skills.sh entries on ClawHub, and the full URL for GitHub / SkillHub. Do not pass a bare `owner/slug`.

```go
skillID, err := apiClient.InstallSandboxSkillFromSource(
    context.Background(), sandboxConfigID, "@owner/slug")
if err != nil {
    // handle the error
}
_ = skillID // subscribe to /sandbox-configs/{id}/skills/{skillID}/install-events with skillID
```

### Example: stop a stuck installation

After a service restart an installation row can stay in `installing`, and the UI can neither retry nor uninstall it. Stopping rewrites the row at once (and cancels the goroutine if one is still running in the process); after that you can retry or uninstall.

```go
skill, err := apiClient.StopSandboxSkill(context.Background(), sandboxConfigID, skillID)
if err != nil {
    // handle the error
}
_ = skill
```

### Example: retry a failed installation

Installations often fail for reasons unrelated to the package (sandbox unreachable, dependency source timeout). The server keeps the original package, so a retry does not upload it again.

```go
skillID, err := apiClient.ReinstallSandboxSkill(context.Background(), sandboxConfigID, skillID)
if err != nil {
    // handle the error
}
```

### Example: view an installed skill's files

```go
files, err := apiClient.ListSandboxSkillFiles(context.Background(), sandboxConfigID, skillID)
if err != nil {
    // handle the error
}
content, err := apiClient.GetSandboxSkillFile(context.Background(), sandboxConfigID, skillID, "SKILL.md")
if err != nil {
    // handle the error
}
_ = files
_ = content
```

### Example: configure a skill's environment variables

A skill declares the environment variables it needs when it is installed. Values have two levels: workspace values are set by an administrator and apply to everyone; personal values apply only to the **calling identity** and override the workspace value. No endpoint reads back a saved value; they only report whether it is set.

An API key and a web login are two different identities: a personal value entered in the web UI does not apply to runs started with an API key. For integrations, prefer workspace values.

```go
// Workspace level: applies to everyone in the workspace, needs Admin or higher
skill, err := apiClient.SetSandboxSkillEnvValues(
    context.Background(), sandboxConfigID, skillID,
    map[string]string{"TAVILY_API_KEY": "tvly-xxxxx"})
if err != nil {
    // handle the error
}

// Personal level: applies only to the calling identity
err = apiClient.SetMySkillEnvVar(
    context.Background(), skillID, "TAVILY_API_KEY", "tvly-yyyyy")
if err != nil {
    // handle the error
}

// See which variables are still unset. Clear a value with Delete, not by writing an empty string
groups, err := apiClient.ListMyEnvVars(context.Background())
if err != nil {
    // handle the error
}
_ = skill
_ = groups
```

## Full example

See the `ExampleUsage` function in `example.go` for the full client flow.
