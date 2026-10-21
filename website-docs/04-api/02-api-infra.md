# API reference: infrastructure and data sources

Register and manage vector stores, file storage backends, web search providers and data sources, with connection tests and sync operations.

Permission summary: reads require Viewer+; writes and connection tests require Admin+ (they probe external systems with stored credentials). API key capabilities: vector stores `manage_vector_stores`, storage backends `manage_storage_backends`, web search providers `manage_web_search`, data sources `manage_datasources` (a full-access key covers all four).

## Vector stores (/api/v1/vector-stores)

### GET /api/v1/vector-stores/types

Purpose: available engine types with their connection and index field schemas. Permission: Viewer+.

Response: 200 `{"success":true,"data":[{type,display_name,connection_fields,index_fields}]}`

```bash
curl $BASE/api/v1/vector-stores/types -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/vector-stores/test

Purpose: test a connection with a raw configuration (nothing is saved). The engine type allow-list, required fields and the SSRF policy are checked before any dial. Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `engine_type` | string | yes (`binding:"required"`) | Engine type |
| `connection_config` | object | yes (`binding:"required"`) | Connection configuration |

Response: 200 `{"success":true|false,"version":"...","error":"..."}`

```bash
curl -X POST $BASE/api/v1/vector-stores/test -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"engine_type":"qdrant","connection_config":{"addr":"qdrant:6334"}}'
```

### POST /api/v1/vector-stores

Purpose: create a vector store configuration. Permission: Admin+. Fields: `name` (required), `engine_type` (required), `connection_config` (required), `index_config` (optional). The service runs a connection test before saving (a failure returns 400) and stores the detected server version; a store with the same engine, endpoint and index as an existing or environment-configured store returns 409.

Response: 201 `{"success":true,"data":{VectorStoreResponse}}` (`id,tenant_id,name,engine_type,connection_config,index_config,source,readonly,...`; sensitive connection fields are masked)

```bash
curl -X POST $BASE/api/v1/vector-stores -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"qdrant-main","engine_type":"qdrant","connection_config":{"addr":"qdrant:6334"}}'
```

### GET /api/v1/vector-stores

Purpose: list vector stores (the `__env_*` stores injected from environment variables come first, with `source: "env"` and `readonly: true`). Permission: Viewer+.

Response: 200 `{"success":true,"data":[VectorStoreResponse]}`

```bash
curl $BASE/api/v1/vector-stores -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/vector-stores/:id

Purpose: vector store detail (`__env_*` IDs are supported). Permission: Viewer+.

Response: 200 `{"success":true,"data":{VectorStoreResponse}}`

```bash
curl $BASE/api/v1/vector-stores/vs-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/vector-stores/:id

Purpose: update (rename only; `engine_type`, `connection_config` and `index_config` are immutable, and env stores cannot be modified, 400). Permission: Admin+. Request body: `{"name":"..."}` (`binding:"required"`).

Response: 200 `{"success":true,"data":{VectorStoreResponse}}`

```bash
curl -X PUT $BASE/api/v1/vector-stores/vs-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"qdrant-prod"}'
```

### DELETE /api/v1/vector-stores/:id

Purpose: delete (env stores cannot be deleted, 400; a store still bound to a knowledge base is refused, unbind or delete those knowledge bases first). Permission: Admin+.

Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/vector-stores/vs-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/vector-stores/:id/test

Purpose: test a saved or env vector store. A newly detected version is written back to the saved store. Permission: Admin+.

Response: 200 `{"success":true|false,"version","error"}`

```bash
curl -X POST $BASE/api/v1/vector-stores/vs-1/test -H "Authorization: Bearer $TOKEN"
```

## Storage backends (/api/v1/storage-backends)

Request body (`storageBackendRequest`, shared by Create, Update and TestRaw):

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes (`binding:"required"`) | Name |
| `provider` | string | yes (`binding:"required"`) | Provider: `local`/`minio`/`cos`/`tos`/`s3`/`oss`/`ks3`/`obs`, restricted by `STORAGE_ALLOW_LIST` |
| `config` | object | No | Provider configuration; fields in [Storage backends](../03-features/19-storage-backends.md#connection-parameters) (credentials are masked in responses) |
| `status` | string | no | `active` (default) / `disabled` |

### GET /api/v1/storage-backends/types

Purpose: the provider names allowed by `STORAGE_ALLOW_LIST` (all eight when it is unset). Permission: Viewer+. Response: 200 `{"success":true,"data":["local","minio",...]}`

```bash
curl $BASE/api/v1/storage-backends/types -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/storage-backends/test

Purpose: connection test with a raw configuration. Permission: Admin+. Response: 200 `{"success":bool,"error"}`

```bash
curl -X POST $BASE/api/v1/storage-backends/test -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"t","provider":"minio","config":{"endpoint":"minio:9000"}}'
```

### POST /api/v1/storage-backends

Purpose: create a storage backend. The service runs a connection test before saving (a failure returns 400); a duplicate name returns 409. Permission: Admin+. Response: 201 `{"success":true,"data":{StorageBackend}}` (`id,tenant_id,name,provider,config,source,status,legacy_alias,...`)

```bash
curl -X POST $BASE/api/v1/storage-backends -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"minio-main","provider":"minio","config":{"endpoint":"minio:9000"}}'
```

### GET /api/v1/storage-backends

Purpose: list (with `default_storage_backend_id`). Permission: Viewer+. Response: 200 `{"success":true,"data":[...],"default_storage_backend_id":"..."}`

```bash
curl $BASE/api/v1/storage-backends -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/storage-backends/:id

Purpose: detail (credentials masked). Permission: Viewer+. Response: 200 `{"success":true,"data":{StorageBackend}}`

```bash
curl $BASE/api/v1/storage-backends/sb-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/storage-backends/:id

Purpose: update the mutable fields (name, credentials, status). `provider` is ignored and kept from the stored row; endpoint, region, bucket and path prefix are immutable (400, use storage migration instead); backends with `source: "env"` are read-only (400); masked credential placeholders keep the stored secrets; a backend that is the workspace default or is bound to a knowledge base or active resources cannot be set to `disabled` (400). The connection test runs again before saving. Permission: Admin+. Response: 200 `{"success":true,"data":{StorageBackend}}`

```bash
curl -X PUT $BASE/api/v1/storage-backends/sb-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"minio-prod","provider":"minio"}'
```

### DELETE /api/v1/storage-backends/:id

Purpose: delete. Refused (400) for the workspace default, a backend still bound to knowledge bases or active resources, an env backend, or a legacy alias that old file paths may still reference. Permission: Admin+. Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/storage-backends/sb-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/storage-backends/:id/test

Purpose: test a saved backend. Permission: Admin+. Response: 200 `{"success":bool,"error"}`

```bash
curl -X POST $BASE/api/v1/storage-backends/sb-1/test -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/storage-backends/:id/default

Purpose: set the default backend (only an `active` backend can be the default). Permission: Admin+. Response: 200 `{"success":true}`

```bash
curl -X PUT $BASE/api/v1/storage-backends/sb-1/default -H "Authorization: Bearer $TOKEN"
```

## Web search (/api/v1/web-search and /api/v1/web-search-providers)

14 search providers are registered, including Metaso, Exa, Bocha, Brave and Serply. The `api_key` and `extra_config` parameters of each one are in [Web search](../03-features/11-web-search.md).

### GET /api/v1/web-search/providers

Purpose: the built-in provider catalog (read-only). Permission: Viewer+, JWT only (no API key policy is declared). Handler: `internal/handler/web_search.go`

Response: 200 `{"success":true,"data":[...]}`

```bash
curl $BASE/api/v1/web-search/providers -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/web-search-providers/types

Purpose: provider types and their parameter schema (the same catalog as above). Permission: Viewer+. Handler: `internal/handler/web_search_provider.go`

Response: 200 `{"success":true,"data":[...]}`

```bash
curl $BASE/api/v1/web-search-providers/types -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/web-search-providers/test

Purpose: test raw credentials (nothing is saved) by running one sample search; an empty result set counts as a failure. Permission: Admin+. Request body: `provider` (`binding:"required"`), `parameters` (optional).

Response: 200 `{"success":bool,"error"}`

```bash
curl -X POST $BASE/api/v1/web-search-providers/test -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"provider":"tavily","parameters":{"api_key":"tvly-..."}}'
```

### POST /api/v1/web-search-providers

Purpose: create a provider configuration. Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes (`binding:"required"`) | Name |
| `provider` | string | yes (`binding:"required"`) | Type (bing/tavily/google and so on) |
| `description` | string | no | Description |
| `parameters` | object | no | Parameters (`api_key`, `engine_id`, `base_url`, `proxy_url`, `extra_config`; set `api_key` through the credentials subresource) |
| `is_default` | bool | no | Default provider (clears the flag on the others) |

Response: 201 `{"success":true,"data":{WebSearchProviderResponse}}`

An unknown `provider` type or missing required parameters (such as the API key a provider needs) returns 400 with the reason.

```bash
curl -X POST $BASE/api/v1/web-search-providers -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"tavily-main","provider":"tavily"}'
```

### GET /api/v1/web-search-providers

Purpose: list providers. Permission: Viewer+. Response: 200 `{"success":true,"data":[...]}`

```bash
curl $BASE/api/v1/web-search-providers -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/web-search-providers/:id

Purpose: detail. Permission: Viewer+. Response: 200 `{"success":true,"data":{...}}`

```bash
curl $BASE/api/v1/web-search-providers/wsp-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/web-search-providers/:id

Purpose: update. An empty `name` or `description` and an omitted `extra_config` keep the stored values; the stored `api_key` is always kept (one in the body is ignored); `provider` is immutable; `is_default` is taken from the body as sent (omitted means false). Permission: Admin+. Request body: `name/description/parameters/is_default` (all optional).

Response: 200 `{"success":true,"data":{...}}`

Invalid parameters return 400 with the reason, and the other providers keep their default flag.

```bash
curl -X PUT $BASE/api/v1/web-search-providers/wsp-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"is_default":true}'
```

### DELETE /api/v1/web-search-providers/:id

Purpose: delete. Permission: Admin+. Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/web-search-providers/wsp-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/web-search-providers/:id/credentials

Purpose: set the API key (`{"api_key":"..."}`; with the field omitted the call only returns the current status). Permission: Admin+. Handler: `internal/handler/web_search_provider_credentials.go`

Response: 200 `{"success":true,"data":{"fields":{"api_key":{"configured":bool}}}}`

```bash
curl -X PUT $BASE/api/v1/web-search-providers/wsp-1/credentials -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"api_key":"tvly-..."}'
```

### DELETE /api/v1/web-search-providers/:id/credentials/:field

Purpose: clear a credential field (`field` must be `api_key`, anything else returns 400). Permission: Admin+. Response: 204.

```bash
curl -X DELETE $BASE/api/v1/web-search-providers/wsp-1/credentials/api_key -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/web-search-providers/:id/test

Purpose: test a saved provider. Permission: Admin+. Response: 200 `{"success":bool,"error"}`

```bash
curl -X POST $BASE/api/v1/web-search-providers/wsp-1/test -H "Authorization: Bearer $TOKEN"
```

## Data sources (/api/v1/datasource)

External content connectors (Feishu, Notion, Yuque and others) whose sync jobs write into a knowledge base. Handler: `internal/handler/datasource.go`. Most successful responses in this group are raw objects or arrays (no `success` envelope); errors use the standard error envelope (`{"success":false,"error":{"code":...,"message":"..."}}`, see [Response format and error codes](./01-api-overview.md#response-format-and-error-codes)). Every route checks that the data source's knowledge base belongs to the caller's workspace (404 when the data source is unknown, 403 when the knowledge base belongs to another workspace or is outside the API key's allow-list).

Errors from the connected service (validating a connection or credentials, listing resources, resolving ancestors) keep the connector's cause in the message: 400 when it rejects the configuration or credentials, denies access or cannot find a selected resource, 502 (code 1011) when it fails or cannot be reached. `details.code` classifies the cause as `permission_denied`, `auth_failed`, `not_found`, `invalid_config` or `upstream_error`. A database or queue failure returns the generic 500.

12 connector types are registered: feishu, lark, feishu_drive, lark_drive, notion, confluence, yuque, dingtalk, ima, rss, gitlab and seafile. The credentials, resource selection and sync limits of each connector are in [Data sources](../03-features/10-datasource.md). With `sync_deletions` on, knowledge items that belong to the data source and were deleted at the source are really deleted; `source_created_at` and `source_updated_at` are stored in the knowledge item metadata.

### GET /api/v1/datasource/types

Purpose: the whole connector catalog, sorted by `priority`. `enabled` is false for a type `AIKB_DATASOURCE_TYPES` excludes (unset enables Notion and Confluence only, `all` enables every type); excluded types stay listed so the capabilities of saved sources of that type remain known. `POST /datasource` answers 400 for an excluded type; existing sources of that type keep syncing, can be edited and can have their credentials replaced. Permission: Viewer+.

Response: 200 `[{type,name,description,icon,priority,auth_type,capabilities,enabled}]`

```bash
curl $BASE/api/v1/datasource/types -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/datasource/validate-credentials

Purpose: validate raw credentials (the 'Test connection' button; nothing is saved). Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `type` | string | yes (`binding:"required"`) | Connector type |
| `credentials` | map | yes (`binding:"required"`) | Credentials |

Response: 200 `{"status":"connected"}`; failure 400 or 502 with the error envelope (see the connector errors above)

```bash
curl -X POST $BASE/api/v1/datasource/validate-credentials -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"type":"notion","credentials":{"api_key":"ntn_xxx"}}'
```

### POST /api/v1/datasource/list-resources

Purpose: browse the external resource tree from an unsaved configuration (the creation wizard's resource step; nothing is saved). Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `type` | string | yes (`binding:"required"`) | Connector type |
| `credentials` | map | yes (`binding:"required"`) | Credentials (an empty map is accepted) |
| `settings` | map | no | Connector settings, for example RSS `feed_urls` |
| `resource_ids` | []string | no | Feishu and Lark Drive read the root `folder_token` from the first entry |
| `parent_id` | string | no | Empty lists the top level, a resource `external_id` lists its children |

Response: 200 `[{external_id,name,type,description,url,modified_at,parent_id,has_children,metadata}]`; failure 400 or 502 with the error envelope (see the connector errors above)

```bash
curl -X POST $BASE/api/v1/datasource/list-resources -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"type":"notion","credentials":{"api_key":"ntn_xxx"}}'
```

### POST /api/v1/datasource

Purpose: create a data source. Permission: Admin+. Request body (`types.DataSource`):

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `knowledge_base_id` | string | yes | Target knowledge base (must belong to this workspace) |
| `name` | string | yes | Name |
| `type` | string | yes | Connector type |
| `config` | object | yes | Credentials (stored encrypted) plus resource selection and settings |
| `sync_schedule` | string | no | Cron expression |
| `sync_mode` | string | no | `incremental` (default) / `full` |
| `conflict_strategy` | string | no | `overwrite` (default) / `skip` |
| `sync_deletions` | bool | no | Default true |
| `sync_log_retention_days` | int | no | Default 30 |

Response: 201 `DataSourceResponse` (credentials stripped, see `internal/handler/dto/datasource.go`); a validation failure returns 400 with the error envelope.

```bash
curl -X POST $BASE/api/v1/datasource -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"knowledge_base_id":"kb-1","name":"notion sync","type":"notion","config":{}}'
```

### GET /api/v1/datasource

Purpose: list data sources. Permission: Viewer+. Query parameter: `kb_id` (required; missing returns 400).

Response: 200 `[DataSourceResponse]` (each item carries `latest_sync_log` when one exists)

```bash
curl "$BASE/api/v1/datasource?kb_id=kb-1" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/datasource/:id

Purpose: detail. Permission: Viewer+. Response: 200 `DataSourceResponse`; 404 with the error envelope, message `data source not found`

```bash
curl $BASE/api/v1/datasource/ds-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/datasource/:id

Purpose: update (`id`, `tenant_id` and `knowledge_base_id` are locked to their stored values). Permission: Admin+. Request body: same as create.

Send the whole object: GET the detail first, change it, and send it back (`name`, `type`, `config`, `sync_schedule`, `sync_mode`, `conflict_strategy`, `sync_deletions`, `sync_log_retention_days`, `status` and so on). `sync_schedule` and `sync_deletions` are always written from the request body: omitting `sync_schedule` clears it to an empty string (manual sync only; an existing cron entry is removed), and omitting `sync_deletions` sets it to false. Other fields that are omitted or zero-valued keep their stored value (the repository update skips zero values). Credentials are not changed through this route: `config.credentials` in the body is ignored and the stored credentials are kept, while `config.type`, `config.resource_ids` and `config.settings` are applied. When the non-credential configuration changed and credentials are stored, the connector validates the new configuration against the live service (400 or 502 on failure, see above).

Response: 200 `DataSourceResponse`

```bash
curl -X PUT $BASE/api/v1/datasource/ds-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"notion sync v2","type":"notion","knowledge_base_id":"kb-1",
       "config":{"type":"notion","resource_ids":["page-1"]},
       "sync_schedule":"0 0 */6 * * *","sync_mode":"incremental","conflict_strategy":"overwrite",
       "sync_deletions":true,"sync_log_retention_days":30,"status":"active"}'
```

### DELETE /api/v1/datasource/:id

Purpose: delete. Permission: Admin+. Response: 204.

```bash
curl -X DELETE $BASE/api/v1/datasource/ds-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/datasource/:id/credentials

Purpose: replace the credentials as a whole (data source credentials are one atomic map behind the single logical field `credentials`). Permission: Admin+. Request body: `{"credentials":{...}}` (required, non-empty map; an empty map returns 400). Handler: `internal/handler/datasource_credentials.go`

Response: 200 `{"success":true,"data":{"fields":{"credentials":{"configured":bool}}}}`

```bash
curl -X PUT $BASE/api/v1/datasource/ds-1/credentials -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"credentials":{"api_key":"ntn_xxx"}}'
```

### DELETE /api/v1/datasource/:id/credentials/:field

Purpose: clear the credentials (`field` must be `credentials`). Permission: Admin+. Response: 204.

```bash
curl -X DELETE $BASE/api/v1/datasource/ds-1/credentials/credentials -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/datasource/:id/validate

Purpose: validate the connection of a saved data source. Permission: Admin+. Response: 200 `{"status":"connected"}`; failure 400 or 502 with the error envelope (see the connector errors above)

```bash
curl -X POST $BASE/api/v1/datasource/ds-1/validate -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/datasource/:id/resources

Purpose: browse the external resource tree (lazy loading). Permission: Admin+. Query parameter: `parent_id` (optional; empty lists the top level).

Response: 200 `[{external_id,name,type,description,url,modified_at,parent_id,has_children,metadata}]`

```bash
curl "$BASE/api/v1/datasource/ds-1/resources?parent_id=" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/datasource/:id/resource-ancestors

Purpose: resolve the ancestor chain of resources (to expand the picker to an existing selection). Permission: Admin+. Request body: `{"resource_ids":["..."]}` (required).

Response: 200 `{"ancestors":[...]}`

```bash
curl -X POST $BASE/api/v1/datasource/ds-1/resource-ancestors -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"resource_ids":["page-1"]}'
```

### POST /api/v1/datasource/:id/sync

Purpose: trigger a sync now. Permission: Admin+. Request body (optional): `{"allow_mass_deletion": true}` lets this one run apply its own deletions and those the mass-deletion guard held back (see [Data sources](../03-features/10-datasource.md)); it is never stored on the data source. A body that is not valid JSON of that shape returns 400. Response: 200 `SyncLog` (`id,status,started_at,items_total,items_created,items_updated,items_deleted,items_skipped,items_failed,error_message,result,...`). `result.reconcile` reports the run's deletions: `deselected` and `missing` counts, `held` (`code`, `params`, `message`) when it deleted nothing on purpose, `overridable` when `allow_mass_deletion` would apply them, and `pending`, up to 100 titles of the held-back items.

```bash
curl -X POST $BASE/api/v1/datasource/ds-1/sync -H "Authorization: Bearer $TOKEN"
curl -X POST $BASE/api/v1/datasource/ds-1/sync -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"allow_mass_deletion":true}'
```

### POST /api/v1/datasource/:id/pause and POST /api/v1/datasource/:id/resume

Purpose: pause / resume scheduled syncs. Permission: Admin+.

Response: 200 `{"status":"paused"}` / `{"status":"active"}`

```bash
curl -X POST $BASE/api/v1/datasource/ds-1/pause -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/datasource/:id/logs

Purpose: list sync logs. Permission: Viewer+. Query parameters: `limit` (default 10, 1 to 100, otherwise 400), `offset` (default 0).

Response: 200 `[SyncLog]`

```bash
curl "$BASE/api/v1/datasource/ds-1/logs?limit=10" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/datasource/logs/:log_id

Purpose: one sync log. Permission: Viewer+. Response: 200 `SyncLog`; 404 with the error envelope, message `sync log not found`

```bash
curl $BASE/api/v1/datasource/logs/log-1 -H "Authorization: Bearer $TOKEN"
```

## Implementation reference

Route registration: `RegisterVectorStoreRoutes`, `RegisterStorageBackendRoutes`, `RegisterWebSearchRoutes`, `RegisterWebSearchProviderRoutes` and `RegisterDataSourceRoutes` in `internal/router/routes_infra.go`. Handlers: `internal/handler/vectorstore.go`, `internal/handler/storagebackend.go`, `internal/handler/web_search.go`, `internal/handler/web_search_provider.go`, `internal/handler/web_search_provider_credentials.go`, `internal/handler/datasource.go`, `internal/handler/datasource_credentials.go`.
