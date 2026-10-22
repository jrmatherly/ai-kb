# Adding a New Connector - Step-by-Step Guide

This guide walks you through implementing a new connector (e.g., Feishu, Notion, Confluence) for the data source sync framework.

## Overview

A connector is an adapter that translates between AI KB's data model and an external platform's API. It handles:

- Connection validation (credentials, connectivity)
- Resource listing (documents, spaces, folders the user can choose from)
- Full sync (fetch all items from selected resources)
- Incremental sync (fetch only changed items since last sync)

Ten connector packages plus the shared `repofile` helper package live under `internal/datasource/connector/`: `confluence`, `dingtalk`, `feishu` (`core`, `wiki`, `drive`), `github`, `gitlab`, `ima`, `notion`, `rss`, `seafile` and `yuque`. `initConnectorRegistry()` in `internal/container/container.go` registers them as thirteen types: `feishu`, `lark`, `feishu_drive`, `lark_drive`, `notion`, `confluence`, `yuque`, `dingtalk`, `ima`, `rss`, `github`, `gitlab` and `seafile`.

## Step 1: Create Connector Package Structure

```bash
mkdir -p internal/datasource/connector/yourtype/
```

Create three files:

- `client.go` - API client wrapper
- `connector.go` - Implements Connector interface
- `types.go` - Platform-specific data structures

## Step 2: Define Platform Types (types.go)

```go
package yourtype

import "time"

// Platform-specific configuration
type Config struct {
    BaseURL   string `json:"base_url"`
    APIToken  string `json:"api_token"`
    
    // Or OAuth fields:
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresAt    time.Time `json:"expires_at"`
}

// Platform-specific resource representation
type YourResource struct {
    ID        string
    Name      string
    Type      string // "document", "folder", "space", etc.
    ModifiedAt time.Time
    URL       string
}

// Platform-specific item representation
type YourItem struct {
    ID          string
    Title       string
    Content     string
    ContentHTML string
    ModifiedAt  time.Time
    URL         string
    CreatedBy   string
}

// Platform-specific pagination/cursor
type YourCursor struct {
    Offset      int       `json:"offset,omitempty"`
    LastModified time.Time `json:"last_modified,omitempty"`
    PageToken   string    `json:"page_token,omitempty"`
}
```

## Step 3: Implement API Client (client.go)

```go
package yourtype

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/jrmatherly/ai-kb/internal/datasource"
)

type Client struct {
    baseURL    string
    apiToken   string
    httpClient *http.Client
}

// NewClient creates a new API client. BaseURL is user input, so it passes the
// SSRF policy first, and the HTTP client re-checks every dial and redirect.
func NewClient(config *Config) (*Client, error) {
    if err := datasource.ValidateConnectorBaseURL(config.BaseURL); err != nil {
        return nil, fmt.Errorf("%w: %w", datasource.ErrInvalidConfig, err)
    }
    return &Client{
        baseURL:    config.BaseURL,
        apiToken:   config.APIToken,
        httpClient: datasource.NewConnectorHTTPClient(30 * time.Second),
    }, nil
}

// apiError carries the HTTP status; Is maps it to the datasource sentinels so
// the service can tell bad credentials from a missing resource.
type apiError struct {
    status int
}

func (e *apiError) Error() string { return fmt.Sprintf("yourtype API: status %d", e.status) }

func (e *apiError) Is(target error) bool {
    switch target {
    case datasource.ErrInvalidCredentials:
        return e.status == http.StatusUnauthorized
    case datasource.ErrPermissionDenied:
        return e.status == http.StatusForbidden
    case datasource.ErrResourceNotFound:
        return e.status == http.StatusNotFound
    }
    return false
}

// Example methods
func (c *Client) GetResources(ctx context.Context) ([]YourResource, error) {
    // Call platform API
    // Parse response
    // Return resources
}

func (c *Client) GetDocument(ctx context.Context, docID string) (*YourItem, error) {
    // Fetch single document
}

func (c *Client) GetDocumentsFromResource(ctx context.Context, resourceID string) ([]YourItem, error) {
    // Fetch every document under one selected resource
}

func (c *Client) GetDocumentsModifiedSince(ctx context.Context, since time.Time) ([]YourItem, error) {
    // Fetch documents modified since timestamp
}
```

Every connector except `rss` (which calls `utils.NewSSRFSafeHTTPClient` directly) builds its client with `datasource.NewConnectorHTTPClient`. Do not use a bare `&http.Client{}`: it bypasses the SSRF guards. Return failures wrapped in the `datasource.Err*` sentinels from `internal/datasource/errors.go` (`ErrInvalidConfig`, `ErrInvalidCredentials`, `ErrPermissionDenied`, `ErrResourceNotFound`, ...), either with `fmt.Errorf("%w: ...", sentinel)` or through a typed error's `Is` method as above.

## Step 4: Implement Connector Interface (connector.go)

```go
package yourtype

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/jrmatherly/ai-kb/internal/datasource"
    "github.com/jrmatherly/ai-kb/internal/types"
)

type YourConnector struct {
    client *Client
}

// NewConnector creates a new connector
func NewConnector() *YourConnector {
    return &YourConnector{}
}

// Type returns the connector type identifier
func (c *YourConnector) Type() string {
    return types.ConnectorTypeYourType // Must match constant in types/datasource.go
}

// Validate verifies that the configuration is valid
func (c *YourConnector) Validate(ctx context.Context, config *types.DataSourceConfig) error {
    if config == nil {
        return datasource.ErrInvalidConfig
    }
    
    // Parse your type-specific config
    yourConfig := &Config{}
    if err := parseConfig(config, yourConfig); err != nil {
        return fmt.Errorf("%w: %w", datasource.ErrInvalidConfig, err)
    }
    
    client, err := NewClient(yourConfig)
    if err != nil {
        return err
    }
    
    // Test connection. A 401 surfaces as errors.Is(err, datasource.ErrInvalidCredentials).
    if _, err := client.GetResources(ctx); err != nil {
        return fmt.Errorf("connection failed: %w", err)
    }
    
    return nil
}

// ListResources lists available resources (documents, spaces, folders).
// An empty parentID asks for the top level; a flat listing returns an empty
// slice for any other parentID.
func (c *YourConnector) ListResources(
    ctx context.Context, config *types.DataSourceConfig, parentID string,
) ([]types.Resource, error) {
    if parentID != "" {
        return []types.Resource{}, nil
    }
    yourConfig := &Config{}
    if err := parseConfig(config, yourConfig); err != nil {
        return nil, err
    }
    
    client, err := NewClient(yourConfig)
    if err != nil {
        return nil, err
    }
    yourResources, err := client.GetResources(ctx)
    if err != nil {
        return nil, err
    }
    
    // Convert to AI KB Resource format
    resources := make([]types.Resource, len(yourResources))
    for i, yr := range yourResources {
        resources[i] = types.Resource{
            ExternalID: yr.ID,
            Name:       yr.Name,
            Type:       yr.Type,
            URL:        yr.URL,
            ModifiedAt: yr.ModifiedAt,
        }
    }
    
    return resources, nil
}

// ResolveResourceAncestors returns the ancestors a lazily loaded picker must
// expand to show the selected resources. A flat listing has none.
func (c *YourConnector) ResolveResourceAncestors(
    ctx context.Context, config *types.DataSourceConfig, resourceIDs []string,
) ([]string, error) {
    return []string{}, nil
}

// FetchAll performs a full sync
func (c *YourConnector) FetchAll(ctx context.Context, config *types.DataSourceConfig, resourceIDs []string) ([]types.FetchedItem, error) {
    yourConfig := &Config{}
    if err := parseConfig(config, yourConfig); err != nil {
        return nil, err
    }
    
    client, err := NewClient(yourConfig)
    if err != nil {
        return nil, err
    }
    
    var allItems []types.FetchedItem
    
    // Fetch all documents from specified resources
    for _, resourceID := range resourceIDs {
        // Get documents from this resource (implementation depends on platform)
        yourItems, err := client.GetDocumentsFromResource(ctx, resourceID)
        if err != nil {
            return nil, fmt.Errorf("failed to fetch resource %s: %w", resourceID, err)
        }
        
        // Convert to FetchedItem format
        for _, yi := range yourItems {
            item := types.FetchedItem{
                ExternalID:       yi.ID,
                Title:            yi.Title,
                Content:          []byte(yi.Content),
                ContentType:      "text/markdown",
                FileName:         fmt.Sprintf("%s.md", yi.Title),
                URL:              yi.URL,
                UpdatedAt:        yi.ModifiedAt,
                SourceResourceID: resourceID,
                Metadata: map[string]string{
                    "created_by":     yi.CreatedBy,
                    "platform":       "yourtype",
                },
            }
            allItems = append(allItems, item)
        }
    }
    
    return allItems, nil
}

// FetchIncremental performs an incremental sync
func (c *YourConnector) FetchIncremental(ctx context.Context, config *types.DataSourceConfig, cursor *types.SyncCursor) ([]types.FetchedItem, *types.SyncCursor, error) {
    yourConfig := &Config{}
    if err := parseConfig(config, yourConfig); err != nil {
        return nil, nil, err
    }
    
    client, err := NewClient(yourConfig)
    if err != nil {
        return nil, nil, err
    }
    
    // Determine start time for incremental fetch
    var sinceTime time.Time
    if cursor != nil && !cursor.LastSyncTime.IsZero() {
        sinceTime = cursor.LastSyncTime
    } else {
        sinceTime = time.Now().AddDate(0, 0, -7) // Default: last 7 days
    }
    
    // Fetch changed items
    yourItems, err := client.GetDocumentsModifiedSince(ctx, sinceTime)
    if err != nil {
        return nil, nil, fmt.Errorf("incremental fetch failed: %w", err)
    }
    
    // Convert to FetchedItem format
    items := make([]types.FetchedItem, len(yourItems))
    for i, yi := range yourItems {
        items[i] = types.FetchedItem{
            ExternalID:  yi.ID,
            Title:       yi.Title,
            Content:     []byte(yi.Content),
            ContentType: "text/markdown",
            FileName:    fmt.Sprintf("%s.md", yi.Title),
            URL:         yi.URL,
            UpdatedAt:   yi.ModifiedAt,
            Metadata: map[string]string{
                "created_by": yi.CreatedBy,
                "platform":   "yourtype",
            },
        }
    }
    
    // Create new cursor for next sync
    nextCursor := &types.SyncCursor{
        LastSyncTime: time.Now(),
        ConnectorCursor: map[string]interface{}{
            "last_modified": time.Now(),
        },
    }
    
    return items, nextCursor, nil
}

// Helper function to parse config
func parseConfig(config *types.DataSourceConfig, target interface{}) error {
    data, err := json.Marshal(config.Credentials)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, target)
}
```

Filter candidate files through `config.Importable(name)` (`(*types.DataSourceConfig).Importable`) before fetching them, in the sync and in `ListResources`, and never keep a private extension list. It accepts an extension in the shared import set (`internal/utils/import_filetypes.go`) unless the file is an image and the knowledge base has no vision model, or audio and it has no speech model. The service sets `config.Imports` from the knowledge base before every call; the create wizard's picker passes its knowledge base id, and without one, or when the knowledge base cannot be loaded, every capability is on. A file the filter rejects must stay out of your deletion detection too: a model removed later hides it, and it must not be reported as deleted. The service checks content items again before replacing an existing item and counts a rejected one as skipped, so a connector that cannot filter upstream loses nothing. Each persisted cursor records the capabilities of its run; when one was gained, the next streaming run starts from a nil cursor. A connector whose cursor resumes a change list mid-way must restart that list when `cursor.Imports` differs from `config.Imports`, as GitHub's `FetchStream` does. The batch path does not compare capabilities.

## Step 5: Register the Connector

Connectors are registered in `initConnectorRegistry()` in `internal/container/container.go`, which the container provides as the `*datasource.ConnectorRegistry`. Import your package and add a `registry.Register` call next to the existing ones; registration errors are aggregated with `errors.Join` so a broken connector fails startup:

```go
import (
    // ... existing imports
    yourconnector "github.com/jrmatherly/ai-kb/internal/datasource/connector/yourtype"
)

func initConnectorRegistry() (*datasource.ConnectorRegistry, error) {
    registry := datasource.NewConnectorRegistry()

    var errs error
    // ... existing registrations ...
    if err := registry.Register(yourconnector.NewConnector()); err != nil {
        errs = errors.Join(errs, fmt.Errorf("register yourtype connector: %w", err))
    }

    if errs != nil {
        return nil, errs
    }
    return registry, nil
}
```

## Step 6: Add Connector Type Constant

Edit `internal/types/datasource.go`:

```go
const (
    // ... existing types ...
    ConnectorTypeYourType = "yourtype"
)
```

## Step 7: Add Metadata

Edit `internal/datasource/connector.go`:

```go
var ConnectorMetadataRegistry = map[string]ConnectorMetadata{
    // ... existing entries ...
    types.ConnectorTypeYourType: {
        Type:         types.ConnectorTypeYourType,
        Name:         "Your Platform Name",
        Description:  "Sync documents from Your Platform",
        Priority:     X,  // Lower number = higher priority in UI
        AuthType:     "oauth2",  // or "api_key", "token", "password", "custom", "none"
        Capabilities: []string{"incremental", CapabilityFullSyncReconcile},
    },
}
```

Two capability values change behavior. `full_sync_reconcile` (`CapabilityFullSyncReconcile`) says the connector can set `types.SyncCursor.CompleteListing`, so a full sync with deletion sync on deletes knowledge items missing from a complete listing. `selection_tracks_source` (`CapabilitySelectionTracksSource`) says each item's `source_resource_id` is the selected resource it was listed under, so items of a deselected resource can be found. `incremental`, `deletion_sync` and `hierarchical` only describe the connector to API clients.

## Step 8: Sync Reason Codes

A per-item failure is a `types.FetchedItem` with no content and `error_reason_code` (plus English fallback text in `error_reason`) in `Metadata`. Declare the codes in a package-level `SyncReasonTexts map[string]string` (see `connector/seafile/types.go`), add that table to the list in `internal/datasource/syncreason/parity_test.go`, and add a `datasource.syncError.<code>` key for each code to `frontend/src/i18n/locales/en-US.ts`. The parity test fails when either side has a code the other lacks.

## Step 9: Frontend Form and Icon

- Add an entry to `connectorDefs` in `frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue` with the type, doc links and one field per credential key (`labelKey`, `placeholder`, `secret: true` for secrets), and add the label keys to `en-US.ts`.
- Add the icon under `frontend/src/assets/img/` and map the type to it in `frontend/src/views/knowledge/settings/datasourceIcons.ts`.

## Optional Interfaces

`internal/datasource/connector.go` defines optional interfaces the sync service checks for: `StreamingConnector` (`FetchStream`, ingest and checkpoint as items arrive), `FullStreamingConnector` (`FetchFullStream`), `FullSyncWithCursor` (`FetchAllFromCursor`, full sync that still reconciles deletions) and `ConfigDependentListing` (`ReportsCompleteListing`, for a `full_sync_reconcile` connector that reports a complete listing only under some configurations).

## Step 10: Test Your Connector

```go
// Example test
func TestYourConnectorValidate(t *testing.T) {
    connector := NewConnector()
    
    config := &types.DataSourceConfig{
        Type: types.ConnectorTypeYourType,
        Credentials: map[string]interface{}{
            "api_token": "test_token",
        },
    }
    
    err := connector.Validate(context.Background(), config)
    // assert no error
}

func TestYourConnectorFetchAll(t *testing.T) {
    connector := NewConnector()
    config := &types.DataSourceConfig{
        Type: types.ConnectorTypeYourType,
        Credentials: map[string]interface{}{
            "api_token": "test_token",
        },
        ResourceIDs: []string{"resource_1"},
    }
    
    items, err := connector.FetchAll(context.Background(), config, []string{"resource_1"})
    // assert results
}
```

## Checklist

- [ ] Created `internal/datasource/connector/yourtype/` package
- [ ] Implemented `types.go` with platform data structures
- [ ] Implemented `client.go` with `datasource.NewConnectorHTTPClient` and `ValidateConnectorBaseURL`
- [ ] Mapped API failures to the `datasource.Err*` sentinels
- [ ] Implemented `connector.go` with Connector interface
- [ ] Added connector type constant
- [ ] Registered in `initConnectorRegistry()`
- [ ] Added metadata entry
- [ ] Added `SyncReasonTexts` to the parity test and `datasource.syncError.*` keys to `en-US.ts`
- [ ] Added the form entry in `DataSourceEditorDialog.vue` and the icon in `datasourceIcons.ts`
- [ ] Added unit tests
- [ ] Tested manually with real API
- [ ] Documented any special requirements

## Common Patterns

### OAuth Flow

If using OAuth, store tokens in config:

```go
type Config struct {
    AccessToken  string
    RefreshToken string
    ExpiresAt    time.Time
}

// Refresh tokens when expired
func (c *Client) ensureValidToken(ctx context.Context) error {
    if time.Now().After(c.config.ExpiresAt) {
        return c.refreshToken(ctx)
    }
    return nil
}
```

### Pagination

For platforms with pagination:

```go
func (c *Client) GetDocumentsPage(ctx context.Context, pageToken string) (*Page, error) {
    // Returns {Items, NextPageToken}
}
```

### Incremental Sync

For timestamp-based incremental sync:

```go
func (c *Client) GetModifiedSince(ctx context.Context, since time.Time) ([]Item, error) {
    // Uses API parameter like &modified_after=2026-03-26T10:00:00Z
}
```

### Deletion Tracking

For platforms that report deletions:

```go
type Item struct {
    IsDeleted bool // Set when item is deleted
}
```

## Testing with Real API

1. Set up test credentials
2. Create small test resource (e.g., single document)
3. Test each method:

   ```go
   connector := NewConnector()
   config := &types.DataSourceConfig{...}
   
   // Test Validate
   err := connector.Validate(ctx, config)
   
   // Test ListResources
   resources, err := connector.ListResources(ctx, config, "")
   
   // Test FetchAll
   items, err := connector.FetchAll(ctx, config, []string{resources[0].ExternalID})
   
   // Test FetchIncremental
   items, cursor, err := connector.FetchIncremental(ctx, config, nil)
   ```

---

## Example: Feishu Connector Reference

The Feishu connector is the most complete reference implementation:

```text
internal/datasource/connector/feishu/
├── core/     (shared API client, regions, request pacing, Markdown export)
├── wiki/     (Wiki connector: feishu and lark types)
└── drive/    (Drive connector: feishu_drive and lark_drive types)
```

`wiki/` also shows the optional `StreamingConnector` and lazy `ListResources` with `ResolveResourceAncestors`. Smaller single-package connectors such as `notion/`, `yuque/` and `rss/` are a simpler starting point.
