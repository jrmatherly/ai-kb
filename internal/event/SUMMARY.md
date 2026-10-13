# AI KB event system summary

## Overview

An in-process publish and subscribe bus for events raised while a query is handled.

## What it provides

1. **Event bus (`EventBus`)**
   - `Emit(ctx, event)` publishes an event. A missing `ID` is filled with a new UUID.
   - `On(eventType, handler)` registers a listener. Several listeners per type are allowed.
   - `Off(eventType)` removes every listener for a type.
   - `EmitAndWait(ctx, event)` runs every listener in its own goroutine and waits for all of them.
   - Sync mode (`NewEventBus`) runs listeners in order and stops at the first error. Async mode (`NewAsyncEventBus`) starts each listener in a goroutine, returns at once and drops listener errors.

2. **Event types** (`event.go`)
   - Query processing (received, validated, preprocess, rewrite, rewritten)
   - Retrieval (start, vector, keyword, entity, complete)
   - Rerank (start, complete)
   - Merge (start, complete)
   - Chat generation (start, complete, stream)
   - Agent stages and agent streaming events (`thought`, `tool_call`, `tool_result`, `references`, `final_answer` and others)
   - Tool approval, MCP OAuth prompts, memory recall, context compaction, injected user messages, session title, stop and error

3. **Event data structures** (`event_data.go`)
   - `QueryData`, `RetrievalData`, `RerankData`, `MergeData`, `ChatData`, `ErrorData`
   - Agent and streaming payloads (`AgentThoughtData`, `AgentToolCallData`, `AgentFinalAnswerData` and others)

4. **Middleware** (`middleware.go`)
   - `WithLogging()` logs each event and the handler result.
   - `WithTiming()` logs the handler duration and writes `duration_ms` to the event metadata after the handler returns.
   - `WithRecovery()` turns a handler panic into a `*PanicError`.
   - `Chain()` and `ApplyMiddleware()` compose middleware.

5. **Global event bus** (`global.go`)
   - A lazily created sync bus, `GetGlobalEventBus()`, replaceable with `SetGlobalEventBus()`.
   - Package-level helpers (`On`, `Off`, `Emit`, `EmitAndWait`, `HasHandlers`, `Clear`).

6. **Interface adapter** (`adapter.go`)
   - `EventBusAdapter` exposes a bus as `types.EventBusInterface`, so `internal/types` callers avoid an import cycle.

7. **Examples and tests** (`example_test.go`)
   - Runnable examples, unit tests for multiple handlers, async mode and `EmitAndWait`, and an emit benchmark.

## How it is used today

The QA handler, the agent engine, the IM service, skill installation and the MCP `ask` tool each create a bus per request with `event.NewEventBus()` and carry the agent streaming events on it. Nothing outside this package emits the query, retrieval, rerank, merge or chat stage events. The global bus receives one emit (`EventAgentQuery` in `internal/handler/session/qa.go`) and no listener registers on it.

## Files

```
internal/event/
├── event.go           # event types and the bus
├── event_data.go      # event payload structs
├── middleware.go      # middleware
├── global.go          # global bus and package-level helpers
├── adapter.go         # types.EventBusInterface adapter
├── example_test.go    # examples, tests and benchmark
├── usage_example.md   # integration sketches
└── SUMMARY.md         # this file
```

## Performance

- `BenchmarkEventBus_Emit` (one sync listener) measured about 320 ns per emit on an Apple M3 Pro. The benchmark event has no ID, so each emit also generates a UUID.
- Listener registration and lookup are guarded by a `sync.RWMutex`.

## Use cases

### 1. Monitoring and metrics

```go
bus.On(event.EventRetrievalComplete, func(ctx context.Context, e event.Event) error {
    data := e.Data.(event.RetrievalData)
    // Send to Prometheus or another monitoring system
    metricsCollector.RecordRetrievalDuration(data.Duration)
    return nil
})
```

### 2. Logging

```go
bus.On(event.EventQueryRewritten, func(ctx context.Context, e event.Event) error {
    data := e.Data.(event.QueryData)
    logger.Infof(ctx, "Query rewritten: %s -> %s", 
        data.OriginalQuery, data.RewrittenQuery)
    return nil
})
```

### 3. User behavior analytics

```go
bus.On(event.EventQueryReceived, func(ctx context.Context, e event.Event) error {
    data := e.Data.(event.QueryData)
    // Send to an analytics platform
    analytics.TrackQuery(data.UserID, data.OriginalQuery)
    return nil
})
```

### 4. Error tracking

```go
bus.On(event.EventError, func(ctx context.Context, e event.Event) error {
    data := e.Data.(event.ErrorData)
    // Send to an error tracking system
    sentry.CaptureException(data.Error)
    return nil
})
```

## Integration

These steps are a plan. "How it is used today" says what is wired.

### Step 1: initialize the event system

At application startup (`main.go` or `container.go`):

```go
import "github.com/jrmatherly/ai-kb/internal/event"

func Initialize() {
    // Get the global event bus
    bus := event.GetGlobalEventBus()
    
    // Set up monitoring and analytics (these handlers do not exist yet)
    event.NewMonitoringHandler(bus)
    event.NewAnalyticsHandler(bus)
}
```

### Step 2: emit events from each processing stage

Add emits to the chat pipeline plugins (`internal/application/service/chat_pipeline`):

```go
// In search.go
event.Emit(ctx, event.NewEvent(event.EventRetrievalStart, event.RetrievalData{
    Query:           chatManage.ProcessedQuery,
    KnowledgeBaseID: chatManage.KnowledgeBaseID,
    TopK:            chatManage.EmbeddingTopK,
}).WithSessionID(chatManage.SessionID))

// In rerank.go
event.Emit(ctx, event.NewEvent(event.EventRerankComplete, event.RerankData{
    Query:       chatManage.ProcessedQuery,
    InputCount:  len(chatManage.SearchResult),
    OutputCount: len(rerankResults),
    Duration:    time.Since(startTime).Milliseconds(),
}).WithSessionID(chatManage.SessionID))
```

### Step 3: register custom handlers

```go
event.On(event.EventQueryRewritten, func(ctx context.Context, e event.Event) error {
    // Custom handling
    return nil
})
```

## Design properties

1. **Loose coupling.** Emitters and listeners do not know about each other.
2. **Flexibility.** Sync or async, one or many listeners per type.
3. **Extensibility.** New event types and handlers need no change to the bus.
4. **Typed payloads.** One predefined data struct per event; listeners type-assert `Data`.
5. **Middleware.** Cross-cutting concerns (logging, timing, panic recovery) wrap handlers.
6. **Testability.** A test can create its own bus and inspect what was emitted.

## Possible extensions

1. **Persistence.** Store key events in a database or message queue.
2. **Replay.** Replay events for debugging or analysis.
3. **Filtering.** Richer filtering and routing.
4. **Priorities.** Handle events by priority.
5. **Distributed events.** Cross-service events over a message queue.
6. **Integrations.** Prometheus metrics, structured logging, tracing, event-based alerts.
