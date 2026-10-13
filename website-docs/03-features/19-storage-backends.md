# Storage backends

A storage backend holds the original files, the images extracted during parsing and exported Artifacts. A workspace can register several instances, mark one as the default, and give each knowledge base its own storage location.

Multiple instances are useful when you want to:

- keep the material of different teams or projects in separate buckets, each with its own quota and permissions;
- choose the bucket region that a data residency requirement demands;
- move to cloud object storage gradually: new knowledge bases use the new backend while existing ones keep reading their original files.

<Screenshot
  src="/screenshots/settings-storage-backends.png"
  caption="Storage backend settings: the instance list, the default instance and the connection test"
  hint="The registered storage backend cards (provider, status, default badge) and the create/edit form with the result of 'Test connection'." />

## Registering and choosing a storage backend

This section is for a workspace Admin who wants to add a storage instance and decide which knowledge bases use it. Instances are managed under Settings → Storage Engine:

1. Create a backend: pick the provider (`local`, `minio`, `cos`, `tos`, `s3`, `oss`, `ks3` or `obs`, the same providers as in the [document pipeline](../02-architecture/03-document-pipeline.md)) and fill in the connection parameters (table below). The environment variable `STORAGE_ALLOW_LIST` restricts the providers on offer; leave it empty to allow all of them.
2. **Test connection**: the test really reads and writes the storage to verify the endpoint, the bucket and the credentials. Saving runs the same test, so a backend that cannot be reached is not saved.
3. Make the instance the workspace default. A new knowledge base that names no instance uses it.
4. To pick an instance for one knowledge base, open the "Storage Engine" tab of the knowledge base edit dialog.

The storage backend of a knowledge base can be changed while it is empty. Once it holds files, the UI disables the selector and points to storage migration: file paths depend on the backend they were imported with, so switching in place would break access to the existing files. Create a new knowledge base and migrate the content instead.

The same rule applies to a saved instance. Editing it may change its name, status and credentials, but the endpoint, region, bucket and path prefix are immutable; move the data with storage migration instead. A backend cannot be disabled or deleted while it is the workspace default or still bound to a knowledge base or an active resource, and a backend created from environment variables (`source: env`) is read-only.

## Connection parameters

All providers share one set of configuration fields. `access_key_id` and `secret_access_key` are stored encrypted and come back masked in API responses.

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `endpoint` | string | empty | Service address. Required for `minio` (`mode=remote`), `tos`, `s3`, `oss`, `ks3` and `obs`; `cos` does not use it. Saving runs an SSRF check, so a private address must be listed in `SSRF_WHITELIST` |
| `region` | string | empty | Region. Required for `cos`, `tos`, `s3`, `oss`, `ks3` and `obs` |
| `access_key_id` / `secret_access_key` | string | empty | Access credentials; for COS these are the SecretId / SecretKey. Not needed for `local` or for MinIO with `mode=docker` |
| `bucket_name` | string | empty | Bucket. Required for every provider except `local` |
| `path_prefix` | string | empty | Object key prefix. Must be a relative path: no leading `/` and no `..` |
| `mode` | string | `remote` | MinIO only. `docker` uses the MinIO shipped with the deployment (address and credentials come from `MINIO_ENDPOINT` and the related environment variables); `remote` connects to an external MinIO |
| `use_ssl` | bool | false | Whether MinIO, S3 and OBS use HTTPS |
| `force_path_style` | bool | false | S3 only. Use path-style addressing; most S3-compatible services need it |
| `app_id` | string | empty | COS only. The Tencent Cloud AppID |
| `temp_bucket_name` / `temp_region` | string | empty | COS, TOS and OSS only. The bucket and region for temporary files; OSS also needs `use_temp_bucket=true` to enable them |

- **OBS**: a domain endpoint uses virtual-hosted addressing (`<bucket>.<endpoint>`), because Huawei Cloud has rejected path-style requests on domain endpoints since 2023-12-30 ([Huawei Cloud notice](https://www.huaweicloud.com/intl/en-us/notice/20231106133906404.html)); an IP endpoint keeps path-style. Without a proxy domain, file URLs take the form `<scheme>://<bucket>.<endpoint-host>/<key>`.
- **Transfer timeouts**: for S3, COS, KS3, OBS and OSS a single upload or download may take up to 30 minutes. Connection setup, the TLS handshake and the wait for response headers have their own shorter timeouts, so an unreachable peer fails quickly.
- **KS3 redirects**: every hop of a redirect is SSRF-checked, and the request signature is not forwarded to the redirect target host.

## Storage backend versus vector store

File storage and vector storage are configured separately: one keeps the original files, the other the retrieval indexes.

| | Storage backend | Vector store |
| --- | --- | --- |
| What it holds | Original files, images, exported Artifacts | Vectors and retrieval indexes |
| Where it is configured | Settings → Storage Engine | Settings → Vector Store |
| Knowledge base field | `storage_backend_id` | `vector_store_id` |
| Related page | This page | [Retrieval engines and vector stores](05-retrieval-engines.md) |

Reading and serving the stored files (presigned URLs, `resource://` handles) is covered in [File access](21-file-access.md).

## API reference

| Method | Path | Permission |
| --- | --- | --- |
| GET | `/storage-backends/types` | Viewer+, returns the provider names allowed by `STORAGE_ALLOW_LIST` |
| GET | `/storage-backends`, `/storage-backends/:id` | Viewer+ |
| POST | `/storage-backends` | Admin+ |
| PUT / DELETE | `/storage-backends/:id` | Admin+ |
| POST | `/storage-backends/test` | Admin+, tests unsaved parameters |
| POST | `/storage-backends/:id/test` | Admin+, tests a saved instance |
| PUT | `/storage-backends/:id/default` | Admin+, makes the instance the workspace default |

An API key needs the `manage_storage_backends` capability or full access.

## Data model and compatibility rules

The `storage_backends` table is isolated by `tenant_id` and uses soft deletes. Key columns:

| Column | Description |
| --- | --- |
| `name` | Unique within the workspace (a partial unique index that ignores soft-deleted rows) |
| `provider` | Storage type |
| `config` | JSONB, with the credentials encrypted |
| `source` | `user` (registered through the UI or API) or `env` (generated from environment variable configuration) |
| `status` | `active` or `disabled` |
| `legacy_alias` | See below |

`legacy_alias` keeps storage that was configured through environment variables working. An upgrade creates an alias record so that existing file paths still resolve without moving any data. One workspace may hold only one alias record per provider; manually registered instances are stored independently of it, and an alias record cannot be deleted while old file paths may still reference it.
