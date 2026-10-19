# Event system usage examples

## Wiring the event system into the chat pipeline

These are sketches, not current code. Nothing outside `internal/event` emits the query, retrieval, rerank, merge or chat stage events today: the chat pipeline emits only the agent streaming events (`thought`, `tool_call`, `final_answer` and the rest), `error` and `memory_recalled` through the per-request `chatManage.EventBus`. `event.NewMonitoringHandler` and `event.NewAnalyticsHandler` do not exist, and `PluginRewriteQuery`, `ErrRewrite` and the plugin helpers called below (`performSearch`, `rewriteQuery`, `performRerank`) are placeholders.

### 1. Set up the event bus at service startup

```go
// internal/container/container.go or main.go

import (
    "github.com/jrmatherly/ai-kb/internal/event"
)

func InitializeEventSystem() {
    // Get the global event bus
    bus := event.GetGlobalEventBus()
    
    // Register a monitoring handler (does not exist yet)
    event.NewMonitoringHandler(bus)
    
    // Register an analytics handler (does not exist yet)
    event.NewAnalyticsHandler(bus)
    
    // Or register a custom handler
    bus.On(event.EventQueryReceived, func(ctx context.Context, e event.Event) error {
        // Custom handling
        return nil
    })
}
```

### 2. Emit events from the query pipeline

#### Example: events in search.go

```go
// internal/application/service/chat_pipeline/search.go

import (
    "strings"
    "time"

    "github.com/jrmatherly/ai-kb/internal/event"
)

func (p *PluginSearch) OnEvent(
    ctx context.Context,
    eventType types.EventType,
    chatManage *types.ChatManage,
    next func() *PluginError,
) *PluginError {
    // Emit retrieval start
    startTime := time.Now()
    event.Emit(ctx, event.NewEvent(event.EventRetrievalStart, event.RetrievalData{
        Query:           chatManage.RewriteQuery,
        KnowledgeBaseID: strings.Join(chatManage.KnowledgeBaseIDs, ","),
        TopK:            chatManage.EmbeddingTopK,
        RetrievalType:   "vector",
    }).WithSessionID(chatManage.SessionID))
    
    // Run the retrieval
    results, err := p.performSearch(ctx, chatManage)
    if err != nil {
        // Emit an error event
        event.Emit(ctx, event.NewEvent(event.EventError, event.ErrorData{
            Error:     err.Error(),
            Stage:     "retrieval",
            SessionID: chatManage.SessionID,
            Query:     chatManage.RewriteQuery,
        }).WithSessionID(chatManage.SessionID))
        return ErrSearch.WithError(err)
    }
    
    // Emit retrieval complete
    event.Emit(ctx, event.NewEvent(event.EventRetrievalComplete, event.RetrievalData{
        Query:           chatManage.RewriteQuery,
        KnowledgeBaseID: strings.Join(chatManage.KnowledgeBaseIDs, ","),
        TopK:            chatManage.EmbeddingTopK,
        RetrievalType:   "vector",
        ResultCount:     len(results),
        Duration:        time.Since(startTime).Milliseconds(),
        Results:         results,
    }).WithSessionID(chatManage.SessionID))
    
    chatManage.SearchResult = results
    return next()
}
```

#### Example: events in a query rewrite plugin

```go
// The pipeline has no rewrite.go; rewriting lives in query_understand.go.

func (p *PluginRewriteQuery) OnEvent(
    ctx context.Context,
    eventType types.EventType,
    chatManage *types.ChatManage,
    next func() *PluginError,
) *PluginError {
    // Emit rewrite start
    event.Emit(ctx, event.NewEvent(event.EventQueryRewrite, event.QueryData{
        OriginalQuery: chatManage.Query,
        SessionID:     chatManage.SessionID,
    }).WithSessionID(chatManage.SessionID))
    
    // Rewrite the query
    rewrittenQuery, err := p.rewriteQuery(ctx, chatManage)
    if err != nil {
        return ErrRewrite.WithError(err)
    }
    
    // Emit rewrite complete
    event.Emit(ctx, event.NewEvent(event.EventQueryRewritten, event.QueryData{
        OriginalQuery:  chatManage.Query,
        RewrittenQuery: rewrittenQuery,
        SessionID:      chatManage.SessionID,
    }).WithSessionID(chatManage.SessionID))
    
    chatManage.RewriteQuery = rewrittenQuery
    return next()
}
```

#### Example: events in rerank.go

```go
// internal/application/service/chat_pipeline/rerank.go

func (p *PluginRerank) OnEvent(
    ctx context.Context,
    eventType types.EventType,
    chatManage *types.ChatManage,
    next func() *PluginError,
) *PluginError {
    // Emit rerank start
    startTime := time.Now()
    inputCount := len(chatManage.SearchResult)
    
    event.Emit(ctx, event.NewEvent(event.EventRerankStart, event.RerankData{
        Query:      chatManage.RewriteQuery,
        InputCount: inputCount,
        ModelID:    chatManage.RerankModelID,
    }).WithSessionID(chatManage.SessionID))
    
    // Run the rerank
    rerankResults, err := p.performRerank(ctx, chatManage)
    if err != nil {
        return ErrRerank.WithError(err)
    }
    
    // Emit rerank complete
    event.Emit(ctx, event.NewEvent(event.EventRerankComplete, event.RerankData{
        Query:       chatManage.RewriteQuery,
        InputCount:  inputCount,
        OutputCount: len(rerankResults),
        ModelID:     chatManage.RerankModelID,
        Duration:    time.Since(startTime).Milliseconds(),
        Results:     rerankResults,
    }).WithSessionID(chatManage.SessionID))
    
    chatManage.RerankResult = rerankResults
    return next()
}
```

#### Example: events in chat_completion.go

```go
// internal/application/service/chat_pipeline/chat_completion.go

func (p *PluginChatCompletion) OnEvent(
    ctx context.Context,
    eventType types.EventType,
    chatManage *types.ChatManage,
    next func() *PluginError,
) *PluginError {
    // Emit chat start
    startTime := time.Now()
    event.Emit(ctx, event.NewEvent(event.EventChatStart, event.ChatData{
        Query:    chatManage.Query,
        ModelID:  chatManage.ChatModelID,
        IsStream: false,
    }).WithSessionID(chatManage.SessionID))
    
    // Prepare the model and messages
    chatModel, opt, err := prepareChatModel(ctx, p.modelService, chatManage)
    if err != nil {
        return ErrGetChatModel.WithError(err)
    }
    
    chatMessages := prepareMessagesWithHistory(chatManage)
    
    // Call the model
    chatResponse, err := chatModel.Chat(ctx, chatMessages, opt)
    if err != nil {
        event.Emit(ctx, event.NewEvent(event.EventError, event.ErrorData{
            Error:     err.Error(),
            Stage:     "chat_completion",
            SessionID: chatManage.SessionID,
            Query:     chatManage.Query,
        }).WithSessionID(chatManage.SessionID))
        return ErrModelCall.WithError(err)
    }
    
    // Emit chat complete
    event.Emit(ctx, event.NewEvent(event.EventChatComplete, event.ChatData{
        Query:      chatManage.Query,
        ModelID:    chatManage.ChatModelID,
        Response:   chatResponse.Content,
        TokenCount: chatResponse.Usage.TotalTokens,
        Duration:   time.Since(startTime).Milliseconds(),
        IsStream:   false,
    }).WithSessionID(chatManage.SessionID))
    
    chatManage.ChatResponse = chatResponse
    return next()
}
```

### 3. Emit the query-received event from the handler layer

The QA entry point is `executeQA` in `internal/handler/session/qa.go`, which already emits `EventAgentQuery` on the global bus in agent mode. A query-received event would go next to it:

```go
// internal/handler/session/qa.go, inside executeQA

// Emit query received
if err := event.Emit(ctx, event.NewEvent(event.EventQueryReceived, event.QueryData{
    OriginalQuery: reqCtx.query,
    SessionID:     sessionID,
}).WithSessionID(sessionID).WithRequestID(reqCtx.requestID)); err != nil {
    logger.Errorf(ctx, "Failed to emit query received event: %v", err)
}
```

### 4. Custom monitoring handler

This sketch needs `github.com/prometheus/client_golang`, which is not in `go.mod` today.

```go
// internal/monitoring/event_monitor.go

package monitoring

import (
    "context"
    "github.com/jrmatherly/ai-kb/internal/event"
    "github.com/prometheus/client_golang/prometheus"
)

var (
    retrievalDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "retrieval_duration_milliseconds",
            Help: "Duration of retrieval operations",
        },
        []string{"knowledge_base_id", "retrieval_type"},
    )
    
    rerankDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "rerank_duration_milliseconds",
            Help: "Duration of rerank operations",
        },
        []string{"model_id"},
    )
)

func init() {
    prometheus.MustRegister(retrievalDuration)
    prometheus.MustRegister(rerankDuration)
}

func SetupEventMonitoring() {
    bus := event.GetGlobalEventBus()
    
    // Track retrieval performance
    bus.On(event.EventRetrievalComplete, func(ctx context.Context, e event.Event) error {
        data := e.Data.(event.RetrievalData)
        retrievalDuration.WithLabelValues(
            data.KnowledgeBaseID,
            data.RetrievalType,
        ).Observe(float64(data.Duration))
        return nil
    })
    
    // Track rerank performance
    bus.On(event.EventRerankComplete, func(ctx context.Context, e event.Event) error {
        data := e.Data.(event.RerankData)
        rerankDuration.WithLabelValues(data.ModelID).Observe(float64(data.Duration))
        return nil
    })
}
```

### 5. Logging handler

```go
// internal/logging/event_logger.go

package logging

import (
    "context"
    "encoding/json"
    "github.com/jrmatherly/ai-kb/internal/event"
    "github.com/jrmatherly/ai-kb/internal/logger"
)

func SetupEventLogging() {
    bus := event.GetGlobalEventBus()
    
    // Structured logging for every event.
    // WithTiming writes duration_ms to the metadata after this handler returns,
    // so the log line below never contains it.
    logHandler := event.ApplyMiddleware(
        func(ctx context.Context, e event.Event) error {
            data, _ := json.Marshal(e.Data)
            logger.Infof(ctx, "Event: type=%s, session=%s, request=%s, data=%s",
                e.Type, e.SessionID, e.RequestID, string(data))
            return nil
        },
        event.WithTiming(),
    )
    
    // Register on the key events
    bus.On(event.EventQueryReceived, logHandler)
    bus.On(event.EventQueryRewritten, logHandler)
    bus.On(event.EventRetrievalComplete, logHandler)
    bus.On(event.EventRerankComplete, logHandler)
    bus.On(event.EventChatComplete, logHandler)
    bus.On(event.EventError, logHandler)
}
```

### 6. Full initialization

```go
// cmd/server/main.go or internal/container/container.go

func Initialize() {
    // 1. Initialize the event system
    eventBus := event.GetGlobalEventBus()
    
    // 2. Set up monitoring (handler does not exist yet)
    event.NewMonitoringHandler(eventBus)
    
    // 3. Set up analytics (handler does not exist yet)
    event.NewAnalyticsHandler(eventBus)
    
    // 4. Set up Prometheus monitoring (if needed)
    // monitoring.SetupEventMonitoring()
    
    // 5. Set up structured logging (if needed)
    // logging.SetupEventLogging()
    
    // 6. Other initialization...
}
```

## Testing with the event system

```go
// Use a dedicated event bus in tests
func TestMyService(t *testing.T) {
    ctx := context.Background()
    
    // Create a bus for this test
    testBus := event.NewEventBus()
    
    // Register a test listener
    var receivedEvents []event.Event
    testBus.On(event.EventQueryReceived, func(ctx context.Context, e event.Event) error {
        receivedEvents = append(receivedEvents, e)
        return nil
    })
    
    // Run the test...
    testBus.Emit(ctx, event.NewEvent(event.EventQueryReceived, event.QueryData{
        OriginalQuery: "test",
    }))
    
    // Check the events
    if len(receivedEvents) != 1 {
        t.Errorf("Expected 1 event, got %d", len(receivedEvents))
    }
}
```

## Async example

```go
// Use async mode for events the main flow does not depend on
func SetupAsyncAnalytics() {
    asyncBus := event.NewAsyncEventBus()
    
    asyncBus.On(event.EventQueryReceived, func(ctx context.Context, e event.Event) error {
        // Send to the analytics platform without blocking the main flow
        // sendToAnalyticsPlatform(e)
        return nil
    })
    
    // Emit on the async bus
    // asyncBus.Emit(ctx, event)
}
```

## Performance tips

1. **Keep the sync bus off the hot path.** Use async mode for monitoring and logging that business logic does not depend on. An async handler runs in its own goroutine with no ordering and its error is dropped.
2. **Use middleware only where needed.** Each layer adds overhead.
3. **Keep event data small**, especially in async mode.
4. **Keep each listener to one job.**
