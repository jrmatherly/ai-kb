package datasource

import (
	"cmp"
	"context"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/jrmatherly/ai-kb/internal/envlist"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// Connector is the interface that all external data source connectors must implement.
// Each connector (Feishu, Notion, Confluence, etc.) provides an implementation of this interface.
type Connector interface {
	// Type returns the connector type identifier (e.g., "feishu", "notion")
	Type() string

	// Validate verifies that the provided configuration is valid by testing connectivity
	// and checking credentials. Returns error if validation fails.
	Validate(ctx context.Context, config *types.DataSourceConfig) error

	// ListResources lists available resources that can be synced (documents, spaces, folders, etc.)
	// Returns a list of Resource objects that the user can select for syncing.
	//
	// parentID controls lazy (on-demand) loading of hierarchical resources:
	//   - parentID == "" → return the top-level resources (e.g. Feishu wiki spaces).
	//   - parentID != "" → return only the direct children of that resource.
	// Connectors whose listing is already flat or returns the full tree in a single
	// call may ignore parentID for the root call and return an empty slice for any
	// non-empty parentID.
	ListResources(ctx context.Context, config *types.DataSourceConfig, parentID string) ([]types.Resource, error)

	// ResolveResourceAncestors resolves, for each of the given resource IDs, the
	// ExternalIDs of every ancestor whose direct children must be loaded so a
	// lazily-loaded picker can reveal a pre-existing (possibly deeply nested)
	// selection. The returned set is deduplicated and unordered.
	//
	// It exists so connectors that load their tree one level at a time (e.g. the
	// Feishu wiki) can expose, in O(depth) per selection, the path back to the
	// root without re-traversing the whole tree. Connectors that already return
	// the full tree (Notion) or a flat list (Yuque) have nothing to reveal and
	// return an empty slice.
	ResolveResourceAncestors(
		ctx context.Context, config *types.DataSourceConfig, resourceIDs []string,
	) ([]string, error)

	// FetchAll performs a full sync of the specified resources.
	// Returns all items from the given resource IDs.
	FetchAll(ctx context.Context, config *types.DataSourceConfig, resourceIDs []string) ([]types.FetchedItem, error)

	// FetchIncremental performs an incremental sync based on the provided cursor.
	// Returns items that have changed since the last sync, a new cursor for the next sync,
	// and an error if the operation fails.
	FetchIncremental(ctx context.Context, config *types.DataSourceConfig, cursor *types.SyncCursor) ([]types.FetchedItem, *types.SyncCursor, error)
}

// StreamHandler receives items and progress checkpoints emitted during a
// streaming fetch. The service implements it to ingest each item as it arrives
// (bounding memory to one item instead of the whole wiki) and to persist the
// connector cursor at page boundaries, so a sync that times out mid-traversal
// resumes from the last checkpoint instead of restarting from scratch
// (Tencent/WeKnora#2136).
type StreamHandler interface {
	// Emit ingests a single fetched item. Returning an error aborts the
	// stream: the connector stops fetching and propagates the error, since a
	// failed ingest means the sync is failing and further API calls are wasted.
	Emit(ctx context.Context, item types.FetchedItem) error

	// Checkpoint persists the cursor reached so far. The cursor is only valid
	// for the duration of the call (the connector may keep mutating its backing
	// maps afterwards), so implementations must serialize it synchronously.
	//
	// The cursor MUST be a complete resumable snapshot, not a delta: resuming
	// from it must reproduce all progress so far. This is what lets the service
	// treat a checkpoint as a safe restart point and, for a full sync, drop the
	// prior baseline without losing already-synced state.
	Checkpoint(ctx context.Context, cursor *types.SyncCursor) error
}

// StreamingConnector is an optional interface. Connectors that implement it let
// the service interleave fetch→ingest→checkpoint so a large sync persists
// incrementally and resumes after a timeout, rather than holding every item in
// memory and losing all progress on retry. Connectors that do not implement it
// fall back to FetchAll / FetchIncremental unchanged.
type StreamingConnector interface {
	Connector

	// FetchStream walks the configured resources starting from cursor (nil =
	// from the beginning / full sync), calling h.Emit for each changed item and
	// h.Checkpoint at page boundaries. It returns the final cursor for the next
	// sync. Nodes already recorded in cursor at their current edit time are
	// skipped, which is what makes a resumed sync converge.
	FetchStream(
		ctx context.Context, config *types.DataSourceConfig,
		cursor *types.SyncCursor, h StreamHandler,
	) (*types.SyncCursor, error)
}

// FullStreamingConnector lets a streaming connector re-fetch every item while
// retaining the previous cursor exclusively for safe deletion reconciliation.
type FullStreamingConnector interface {
	StreamingConnector

	FetchFullStream(
		ctx context.Context, config *types.DataSourceConfig,
		cursor *types.SyncCursor, h StreamHandler,
	) (*types.SyncCursor, error)
}

// FullSyncWithCursor is optional. The batch sync path uses it for ForceFull and
// sync_mode=full so a connector can re-fetch every document while still
// reconciling deletions against the previous cursor. Connectors that omit it
// keep FetchAll's no-cursor behaviour and therefore cannot emit deletions on a
// full sync.
type FullSyncWithCursor interface {
	FetchAllFromCursor(
		ctx context.Context,
		config *types.DataSourceConfig,
		resourceIDs []string,
		cursor *types.SyncCursor,
	) ([]types.FetchedItem, *types.SyncCursor, error)
}

// ConnectorRegistry manages the registration and lookup of available connectors
type ConnectorRegistry struct {
	connectors map[string]Connector
}

// NewConnectorRegistry creates a new connector registry
func NewConnectorRegistry() *ConnectorRegistry {
	return &ConnectorRegistry{
		connectors: make(map[string]Connector),
	}
}

// Register registers a connector with the registry
func (r *ConnectorRegistry) Register(connector Connector) error {
	if connector == nil {
		return ErrConnectorNil
	}
	if connector.Type() == "" {
		return ErrConnectorTypeEmpty
	}
	r.connectors[connector.Type()] = connector
	return nil
}

// Get retrieves a connector by type
func (r *ConnectorRegistry) Get(connectorType string) (Connector, error) {
	connector, exists := r.connectors[connectorType]
	if !exists {
		return nil, ErrConnectorNotFound
	}
	return connector, nil
}

// List returns all registered connector types
func (r *ConnectorRegistry) List() []string {
	types := make([]string, 0, len(r.connectors))
	for t := range r.connectors {
		types = append(types, t)
	}
	return types
}

// ConnectorMetadata provides metadata about available connectors
type ConnectorMetadata struct {
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Icon         string   `json:"icon,omitempty"`
	Priority     int      `json:"priority"`     // Priority order for UI display (lower = higher priority)
	AuthType     string   `json:"auth_type"`    // "oauth2", "api_key", "token", etc.
	Capabilities []string `json:"capabilities"` // "incremental", "webhook", "deletion_sync", etc.
	// Enabled is false for a type AIKB_DATASOURCE_TYPES excludes. The registry leaves it unset.
	Enabled bool `json:"enabled"`
}

// CapabilityFullSyncReconcile marks a connector type that can set
// types.SyncCursor.CompleteListing, so a full sync with deletion sync on
// deletes knowledge items missing from a complete listing. Its connectors also
// delete the items of a resource taken out of the selection.
const CapabilityFullSyncReconcile = "full_sync_reconcile"

// CapabilitySelectionTracksSource marks a connector whose items store, as
// source_resource_id, the selected resource ID they were listed under, so an
// item whose ID is no longer selected was deselected. Seafile lacks it: it
// stores the library root while the selection may name folders inside it.
const CapabilitySelectionTracksSource = "selection_tracks_source"

// HasCapability reports whether a connector type declares capability.
func HasCapability(connectorType, capability string) bool {
	return slices.Contains(ConnectorMetadataRegistry[connectorType].Capabilities, capability)
}

// ConfigDependentListing is implemented by a connector declaring
// CapabilityFullSyncReconcile that reports a complete listing only under some
// configurations (Confluence: Server/DC, not Cloud).
type ConfigDependentListing interface {
	ReportsCompleteListing(config *types.DataSourceConfig) bool
}

// ReconcilesFullSync reports whether a full sync of c under config can report
// a complete listing and so reconcile deletions.
func ReconcilesFullSync(c Connector, config *types.DataSourceConfig) bool {
	if !HasCapability(c.Type(), CapabilityFullSyncReconcile) {
		return false
	}
	l, ok := c.(ConfigDependentListing)
	return !ok || l.ReportsCompleteListing(config)
}

// GetConnectorMetadata returns metadata for all available connectors
// This is used by the frontend to display connector options
var ConnectorMetadataRegistry = map[string]ConnectorMetadata{
	types.ConnectorTypeFeishu: {
		Type:        types.ConnectorTypeFeishu,
		Name:        "Feishu",
		Description: "Sync documents, wikis, and content from Feishu",
		Priority:    0,
		AuthType:    "oauth2",
		Capabilities: []string{
			"incremental", "deletion_sync", CapabilityFullSyncReconcile, CapabilitySelectionTracksSource,
		},
	},
	types.ConnectorTypeLark: {
		Type:        types.ConnectorTypeLark,
		Name:        "Lark",
		Description: "Sync documents, wikis, and content from Lark (Feishu international)",
		Priority:    0,
		AuthType:    "oauth2",
		Capabilities: []string{
			"incremental", "deletion_sync", CapabilityFullSyncReconcile, CapabilitySelectionTracksSource,
		},
	},
	types.ConnectorTypeFeishuDrive: {
		Type:        types.ConnectorTypeFeishuDrive,
		Name:        "Feishu Drive",
		Description: "Sync documents and files from a Feishu Drive folder",
		Priority:    0,
		AuthType:    "oauth2",
		Capabilities: []string{
			"incremental", "deletion_sync", CapabilityFullSyncReconcile, CapabilitySelectionTracksSource,
		},
	},
	types.ConnectorTypeLarkDrive: {
		Type:        types.ConnectorTypeLarkDrive,
		Name:        "Lark Drive",
		Description: "Sync documents and files from a Lark Drive folder",
		Priority:    0,
		AuthType:    "oauth2",
		Capabilities: []string{
			"incremental", "deletion_sync", CapabilityFullSyncReconcile, CapabilitySelectionTracksSource,
		},
	},
	types.ConnectorTypeNotion: {
		Type:         types.ConnectorTypeNotion,
		Name:         "Notion",
		Description:  "Sync pages and databases from Notion",
		Priority:     1,
		AuthType:     "api_key",
		Capabilities: []string{"incremental"},
	},
	types.ConnectorTypeConfluence: {
		Type:        types.ConnectorTypeConfluence,
		Name:        "Confluence",
		Description: "Sync spaces and pages from Atlassian Confluence",
		Priority:    2,
		AuthType:    "api_key",
		Capabilities: []string{
			"incremental", "deletion_sync", CapabilityFullSyncReconcile, CapabilitySelectionTracksSource,
		},
	},
	types.ConnectorTypeYuque: {
		Type:         types.ConnectorTypeYuque,
		Name:         "Yuque",
		Description:  "Sync knowledge bases and documents from Yuque",
		Priority:     3,
		AuthType:     "api_key",
		Capabilities: []string{"incremental"},
	},
	types.ConnectorTypeIMA: {
		Type:         types.ConnectorTypeIMA,
		Name:         "Tencent IMA (ima.qq.com)",
		Description:  "Sync knowledge bases and documents from Tencent IMA",
		Priority:     3,
		AuthType:     "api_key",
		Capabilities: []string{"incremental", "deletion_sync"},
	},
	types.ConnectorTypeGitHub: {
		Type:         types.ConnectorTypeGitHub,
		Name:         "GitHub",
		Description:  "Sync repositories, wikis, and issues from GitHub",
		Priority:     4,
		AuthType:     "oauth2",
		Capabilities: []string{"incremental"},
	},
	types.ConnectorTypeGoogleDrive: {
		Type:         types.ConnectorTypeGoogleDrive,
		Name:         "Google Drive",
		Description:  "Sync documents and files from Google Drive",
		Priority:     5,
		AuthType:     "oauth2",
		Capabilities: []string{"incremental"},
	},
	types.ConnectorTypeOneDrive: {
		Type:         types.ConnectorTypeOneDrive,
		Name:         "OneDrive / SharePoint",
		Description:  "Sync documents and files from Microsoft OneDrive",
		Priority:     6,
		AuthType:     "oauth2",
		Capabilities: []string{"incremental"},
	},
	types.ConnectorTypeDingTalk: {
		Type:        types.ConnectorTypeDingTalk,
		Name:        "DingTalk",
		Description: "Sync online documents from DingTalk knowledge bases",
		Priority:    7,
		AuthType:    "oauth2",
		Capabilities: []string{
			"incremental", "deletion_sync", CapabilityFullSyncReconcile, CapabilitySelectionTracksSource,
		},
	},
	types.ConnectorTypeWebCrawler: {
		Type:         types.ConnectorTypeWebCrawler,
		Name:         "Web Crawler (Sitemap)",
		Description:  "Crawl websites via Sitemap.xml",
		Priority:     9,
		AuthType:     "none",
		Capabilities: []string{},
	},
	types.ConnectorTypeSlack: {
		Type:         types.ConnectorTypeSlack,
		Name:         "Slack",
		Description:  "Sync channel messages and files from Slack",
		Priority:     10,
		AuthType:     "oauth2",
		Capabilities: []string{"incremental"},
	},
	types.ConnectorTypeIMAP: {
		Type:         types.ConnectorTypeIMAP,
		Name:         "Email (IMAP)",
		Description:  "Sync email content from IMAP servers",
		Priority:     11,
		AuthType:     "password",
		Capabilities: []string{},
	},
	types.ConnectorTypeRSS: {
		Type:         types.ConnectorTypeRSS,
		Name:         "RSS / Atom Feed",
		Description:  "Sync articles from RSS/Atom feeds",
		Priority:     12,
		AuthType:     "custom",
		Capabilities: []string{"incremental"},
	},
	types.ConnectorTypeGitLab: {
		Type:         types.ConnectorTypeGitLab,
		Name:         "GitLab",
		Description:  "Sync files from GitLab projects",
		Priority:     8,
		AuthType:     "token",
		Capabilities: []string{"incremental", "hierarchical"},
	},
	types.ConnectorTypeSeafile: {
		Type:         types.ConnectorTypeSeafile,
		Name:         "Seafile",
		Description:  "Sync folders and files from Seafile libraries",
		Priority:     13,
		AuthType:     "token",
		Capabilities: []string{"incremental", "hierarchical", "deletion_sync", CapabilityFullSyncReconcile},
	},
}

// ListAvailableConnectors returns the whole connector catalog sorted by
// priority, each entry's Enabled set from the allow-list. Excluded types stay
// listed because saved sources of those types still sync and need their
// capabilities.
func ListAvailableConnectors() []ConnectorMetadata {
	enabled := enabledConnectorTypes()
	metadata := slices.Collect(maps.Values(ConnectorMetadataRegistry))
	for i := range metadata {
		metadata[i].Enabled = enabled[metadata[i].Type]
	}
	slices.SortFunc(metadata, func(a, b ConnectorMetadata) int {
		return cmp.Or(cmp.Compare(a.Priority, b.Priority), cmp.Compare(a.Type, b.Type))
	})
	return metadata
}

// ConnectorTypesEnv is the deployment allow-list of connector types (keys of
// ConnectorMetadataRegistry). Unset or blank means every type; a list that
// names only unknown types means none. Parsed by internal/envlist.
const ConnectorTypesEnv = "AIKB_DATASOURCE_TYPES"

var unknownConnectorTypeWarning sync.Once

// ConnectorTypeEnabled reports whether the deployment allow-list admits connectorType.
func ConnectorTypeEnabled(connectorType string) bool {
	return enabledConnectorTypes()[connectorType]
}

// enabledConnectorTypes is not cached: tests set the env per case (t.Setenv).
func enabledConnectorTypes() map[string]bool {
	known := slices.Collect(maps.Keys(ConnectorMetadataRegistry))
	enabled, unknown := envlist.Parse(os.Getenv(ConnectorTypesEnv), known)
	if len(unknown) > 0 {
		unknownConnectorTypeWarning.Do(func() {
			logger.Warnf(context.Background(), "[DataSource] %s names unknown connector types, ignored: %s",
				ConnectorTypesEnv, strings.Join(unknown, ", "))
		})
	}
	return enabled
}
