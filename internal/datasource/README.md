# AI KB Data Source Sync Framework

## Overview

The data source sync framework enables AI KB to automatically import and synchronize content from external platforms (Feishu, Notion, Confluence, etc.) into knowledge bases. This is the foundational layer upon which all specific connectors are built.

Implemented connectors, registered in `initConnectorRegistry()` (`internal/container/container.go`): Feishu, Lark, Feishu Drive, Lark Drive, Notion, Confluence, Yuque, DingTalk, IMA, RSS, GitLab and Seafile. GitHub, Google Drive, OneDrive, Web Crawler, Slack and IMAP have metadata entries in `connector.go` only, so `GET /api/v1/datasource/types` lists them but they cannot sync yet.

## Architecture

### Core Components

```text
┌─────────────────────────────────────────────────────────────┐
│                     External Data Sources                  │
│   (Feishu/Lark, Notion, Confluence, Yuque, DingTalk, ...)   │
└─────────────┬───────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Connector Registry & Adapters              │
│    Each platform (feishu/, notion/, confluence/, etc)     │
│           implements the Connector interface               │
└─────────────┬───────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────┐
│            DataSourceService (Business Logic)               │
│  ├─ CreateDataSource / UpdateDataSource                    │
│  ├─ ManualSync / ListAvailableResources                    │
│  ├─ ValidateConnection / PauseDataSource                   │
│  └─ ProcessSync (asynq task handler)                       │
└─────────────┬───────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────┐
│        HTTP Handler & API Routes (/api/v1/datasource)      │
│      REST endpoints for UI/programmatic access             │
└─────────────┬───────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────┐
│              Database (PostgreSQL / SQLite)                 │
│  ├─ data_sources (configuration & state)                   │
│  └─ sync_logs (history & metadata)                         │
└─────────────────────────────────────────────────────────────┘
```

### Task Flow

```text
Manual Trigger or Scheduled Job (cron)
        ↓
  Scheduler enqueues asynq Task
        ↓
  ProcessSync (task handler) executes
        ↓
  Connector.Fetch* methods pull data
        ↓
  Diff engine identifies changes (new/update/delete)
        ↓
  KnowledgeService creates/updates Knowledge entries
        ↓
  Documents parsed → chunks → vectors → indexed
```

## File Structure

```text
internal/datasource/
├── connector.go          # Connector interface, registry & metadata
├── errors.go            # Error definitions
├── scheduler.go         # Cron scheduler for periodic syncs
├── httpclient.go        # SSRF-guarded HTTP client for connectors
├── filename.go          # File name sanitizing for synced items
├── connector/           # Connector implementations (one package per platform)
├── syncreason/          # Test-only parity check of sync reason codes vs. frontend keys
└── README.md            # This file

internal/types/
├── datasource.go        # Data models (DataSource, SyncLog, etc)
└── interfaces/datasource.go  # Service interfaces

internal/application/
├── repository/datasource_repo.go  # Database access layer
└── service/datasource_service.go  # Business logic

internal/handler/
└── datasource.go        # HTTP request handlers

internal/router/
├── routes_infra.go     # RegisterDataSourceRoutes (/api/v1/datasource)
├── task.go             # Asynq task registration
└── sync_task.go        # Lite mode task registration

migrations/versioned/
└── 000029_datasource_tables.up/down.sql  # Database schema (PostgreSQL)

migrations/sqlite/
└── 000000_init.up.sql   # Database schema (SQLite, Lite mode)
```

## Data Models

### DataSource

Represents a configured external data source:

- **ID**: Unique identifier
- **Type**: Connector type (feishu, notion, confluence, etc)
- **Config**: Credentials and configuration (JSON); credential strings are encrypted when `SYSTEM_AES_KEY` is set
- **SyncSchedule**: Cron expression (e.g., `0 */6 * * *`)
- **SyncMode**: "incremental" or "full"
- **Status**: active | paused | error | deleted
- **ConflictStrategy**: overwrite | skip
- **SyncDeletions**: Whether to sync deletions from source
- **LastSyncAt**: Timestamp of last successful sync
- **LastSyncCursor**: State for incremental sync
- **LastSyncResult**: Summary of last sync

### SyncLog

Tracks execution of each sync operation:

- **Status**: running | success | partial | failed | canceled
- **StartedAt/FinishedAt**: Execution timestamps
- **Items***: Counters (total, created, updated, deleted, skipped, failed)
- **ErrorMessage**: Error details if failed
- **Result**: Detailed sync result (JSON)

### Connector Interface

All external data sources implement this interface:

```go
type Connector interface {
    Type() string
    Validate(ctx context.Context, config *types.DataSourceConfig) error
    ListResources(ctx context.Context, config *types.DataSourceConfig, parentID string) ([]types.Resource, error)
    ResolveResourceAncestors(
        ctx context.Context, config *types.DataSourceConfig, resourceIDs []string,
    ) ([]string, error)
    FetchAll(ctx context.Context, config *types.DataSourceConfig, resourceIDs []string) ([]types.FetchedItem, error)
    FetchIncremental(ctx context.Context, config *types.DataSourceConfig, cursor *types.SyncCursor) ([]types.FetchedItem, *types.SyncCursor, error)
}
```

`ListResources` with an empty `parentID` returns the top-level resources and otherwise the direct children of that resource. `ResolveResourceAncestors` returns the ancestors a lazily loaded picker must expand to show an existing selection; connectors with a flat or full-tree listing return an empty slice.

Optional interfaces in `connector.go` add behavior on top: `StreamingConnector` (`FetchStream`, ingest and checkpoint as items arrive), `FullStreamingConnector` (`FetchFullStream`), `FullSyncWithCursor` (`FetchAllFromCursor`, full sync that still reconciles deletions) and `ConfigDependentListing` (`ReportsCompleteListing`).

## API Endpoints

### Data Source Management

```text
POST   /api/v1/datasource              # Create
GET    /api/v1/datasource              # List (by kb_id)
GET    /api/v1/datasource/:id          # Get
PUT    /api/v1/datasource/:id          # Update
DELETE /api/v1/datasource/:id          # Delete
PUT    /api/v1/datasource/:id/credentials         # Replace credentials
DELETE /api/v1/datasource/:id/credentials/:field  # Clear a credential field
```

### Operations

```text
POST   /api/v1/datasource/validate-credentials  # Test credentials without saving
POST   /api/v1/datasource/list-resources    # List resources for an unsaved config
POST   /api/v1/datasource/:id/validate      # Test connection
GET    /api/v1/datasource/:id/resources     # List available resources
POST   /api/v1/datasource/:id/resource-ancestors  # Resolve ancestors of selected resources
POST   /api/v1/datasource/:id/sync          # Trigger manual sync
POST   /api/v1/datasource/:id/pause         # Pause
POST   /api/v1/datasource/:id/resume        # Resume
```

### Logs

```text
GET    /api/v1/datasource/:id/logs          # List sync logs
GET    /api/v1/datasource/logs/:log_id      # Get specific log
```

### Metadata

```text
GET    /api/v1/datasource/types             # Metadata for every connector type, implemented or not
```

## Implementation Guide

### Adding a New Connector

Follow [CONNECTOR_IMPLEMENTATION_GUIDE.md](CONNECTOR_IMPLEMENTATION_GUIDE.md). It covers the package, the SSRF-guarded HTTP client, error sentinels, registration, metadata, sync reason codes and the frontend form and icon.

## Database Schema

### data_sources table

- Stores data source configurations
- Indexes: tenant_id, knowledge_base_id, type, status
- Config field with credential strings encrypted (AES-256-GCM) when `SYSTEM_AES_KEY` is set

### sync_logs table

- Stores sync operation history
- Indexes: data_source_id, tenant_id, status, started_at
- Foreign key to data_sources with cascade delete

## Key Design Decisions

1. **Adapter Pattern**: Each connector is a separate implementation, easy to add new ones
2. **Async Task Queue**: Syncs are non-blocking, using asynq (or sync mode in Lite)
3. **Incremental Sync**: Supports cursor-based incremental syncs to minimize API calls
4. **Encryption**: API keys/tokens stored encrypted with AES-256-GCM when `SYSTEM_AES_KEY` is set
5. **Multi-tenant**: All operations are tenant-isolated
6. **Channel Tracking**: Knowledge.Channel field tracks data source type for origin tracking

## Feishu request pacing and rate-limit retries

Feishu export creation, export status polling, export downloads, and Wiki node
listing each use a separate request budget. Clients using the same API host and
App ID share these budgets within one backend process, including concurrent data
sources and task retries. Document tokens, export tickets, and query parameters
do not create additional budgets.

| Environment variable | Default | Behavior |
| --- | --- | --- |
| `FEISHU_API_REQUESTS_PER_MINUTE` | `80` | Positive integer; requests to each paced API are evenly spaced (750 ms by default), without accumulating burst capacity. |
| `FEISHU_RATE_LIMIT_RETRY_WAIT` | `10s` | Positive Go duration used when the response supplies no valid reset/retry hint. |
| `FEISHU_RATE_LIMIT_MAX_WAIT` | `5m` | Positive Go duration capping each cooldown, including response-header hints and the fallback. |
| `FEISHU_EXPORT_MAX_POLLS` | `30` | Positive integer bounding status polls of the same export task; API retries do not consume additional polls. |
| `FEISHU_EXPORT_POLL_INTERVAL` | `2s` | Positive Go duration between status polls while the export is still processing. |

Invalid values use the defaults. Other Feishu APIs retain their existing request
frequency, with reactive cooldowns scoped to the HTTP method and normalized API
path template. Document tokens, table IDs, sheet ranges, and query parameters do
not split a known API's cooldown. Media downloads, file downloads, docx blocks,
raw content, bitable fields/record searches, sheet values, folder metadata, Wiki
space listing, and Wiki node lookup use independent cooldowns. Unknown endpoints
use their service/version/resource prefix. Check the application's actual API
quotas before increasing the configured budget.

JSON requests retry HTTP 429 and HTTP 400/200 responses containing Feishu error
code `99991400`. Downloads retry HTTP 429 and HTTP 400 with that error code.
The wait is the longer valid value from `x-ogw-ratelimit-reset` and `Retry-After`;
`Retry-After` also accepts an HTTP date. All hints are clamped to the configured
maximum so an unreasonable gateway header cannot indefinitely block later syncs.
Without a valid hint, a rate-limit response imposes a 10-second cooldown by
default. The three retries add a nominal 30 seconds of cooldown; shared pacing
or further rate-limit responses from other clients can extend the total wait.
This replaces the original 2/4/8-second backoff with a shared cooldown scoped
to the affected API.
Only callers of the same API share that cooldown. Requests retain the existing
limit of three retries. Even when retries are exhausted, subsequent documents and other
clients sharing that API wait for the cooldown. Waiting honors task cancellation.
Ordinary permission/validation errors still fail immediately, and download error
bodies retain their existing size bounds.

Export polling uses a bounded number of polls rather than a 60-second wall-clock
deadline. Shared pacing and cooldown waits therefore do not cause false export
timeouts or create replacement tasks within the operation. With the defaults,
an export that remains processing is checked up to 30 times, with 2 seconds
between polls. The caller's context bounds total elapsed time, including waits
and retries. Exhausting the poll budget retains the `feishu_timeout` failure
classification. Tokens are checked again after gate waits; requests needing a
token refresh rejoin the gate afterwards, so both JSON requests and downloads
remain paced and honor any cooldown imposed during the refresh.

These budgets are process-local. Multiple backend instances or other applications
using the same Feishu credentials may still hit the upstream quota; configure
their combined traffic accordingly.

## Future Enhancements

1. **Webhook Support**: Real-time push from platforms that support webhooks
2. **Conflict Resolution**: Advanced merge/conflict strategies for overlapping content
3. **Rate Limiting**: Extend request pacing to other connectors and coordinate quotas across backend instances
4. **Scheduling**: Full cron scheduler with time zone support
5. **Monitoring**: Metrics, alerting, and sync health dashboards
6. **Filtering**: User-defined filters for selective syncing (by title, date, tags, etc)
7. **Transformation**: Content transformation pipelines (e.g., extract tables, summarize)
8. **Deduplication**: Smart deduplication across multiple sources

## Testing

The framework is designed to be testable:

- Mock connectors can be created for testing
- No external dependencies required for core logic
- Database operations isolated and mockable

## Error Handling

All operations return detailed error information:

- Connection failures are captured and status updated
- Partial failures are logged (some items succeed, some fail)
- Error messages stored in DataSource for user troubleshooting
