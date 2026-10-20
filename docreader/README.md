# DocReader Service

DocReader is the AI KB gRPC service that parses documents into Markdown and extracts their images. It does no OCR itself: it renders scanned PDF pages to JPEG images, and the Go app runs OCR/VLM on them.

## Supported formats

The builtin engine (`parser/registry.py` is the source of truth) handles:

| Formats | Notes |
| --- | --- |
| `docx`, `doc` | `doc` is converted to `docx` first, with antiword as the text-only fallback |
| `pdf` | scanned pages are rendered to JPEG for the app's OCR step |
| `xlsx`, `xls`, `xlsb` | spreadsheets |
| `pptx`, `ppt` | via MarkItDown |
| `md`, `markdown`, `html`, `htm`, `mhtml`, `epub`, `xmind` | |
| `jpg`, `jpeg`, `png`, `gif`, `bmp`, `tiff`, `webp` | images |

Other engines: `markitdown` (also `csv`) and `opendataloader` (`pdf`, needs Java 11+).

## Run locally

DocReader needs Python 3.14 and [uv](https://docs.astral.sh/uv/).

```bash
uv sync --project docreader   # install the locked dependencies
make -C docreader proto       # regenerate the gRPC stubs
make -C docreader run         # start the server on :50051
```

## Configuration

DocReader reads only environment variables. `config.py` and `auth.py` define most of them, and the docreader block in `docker-compose.yml` sets every one with a comment. `parser/pdf_parser.py` reads the `DOCREADER_PDF_*` parsing tunables (scan detection, embedded images, layout, headings, charts), `utils/ssrf.py` the `SSRF_*` variables, and `main.py` `LOG_LEVEL`. On start the service logs the effective values of the `config.py` set only.

### gRPC

- `DOCREADER_GRPC_PORT`: listen port (default: 50051)
- `DOCREADER_GRPC_MAX_WORKERS`: worker threads (default: 4)
- `DOCREADER_GRPC_MAX_FILE_SIZE_MB`: largest accepted file in MB (default: `MAX_FILE_SIZE_MB`, else 50). Keep it in line with the app and frontend limits.

### gRPC TLS and auth

- `GRPC_TLS_ENABLED`, `GRPC_TLS_CERT`, `GRPC_TLS_KEY`, `GRPC_TLS_CA`: serve gRPC over TLS
- `GRPC_MTLS_REQUIRE_CLIENT_CERT`: require a client certificate (mTLS)
- `GRPC_AUTH_TOKEN`: setting it enables token authentication; clients send the token in gRPC metadata

Compose does not publish port 50051 to the host. Only the app reaches it over `ai-kb-network`.

### Outbound HTTP

- `DOCREADER_EXTERNAL_HTTP_PROXY`, `DOCREADER_EXTERNAL_HTTPS_PROXY`: proxy for fetching web pages and remote images (`EXTERNAL_HTTP(S)_PROXY` are accepted as aliases)
- `SSRF_WHITELIST`, `SSRF_WHITELIST_EXTRA`, `SSRF_DNS_WHITELIST_ONLY`: SSRF allowlist, same names as the app

### Parser resource limits

- `DOCREADER_MARKITDOWN_MAX_WORKERS`: concurrent MarkItDown parses (default: 1, 0 disables the limit)
- `DOCREADER_PDF_RENDER_MAX_WORKERS`: concurrent scanned-PDF renders (default: 1, 0 disables the limit)
- `DOCREADER_PDF_RENDER_PARALLELISM`: processes rendering the pages of one PDF (default: min(4, CPU count))
- `DOCREADER_PDF_RENDER_DPI`: render DPI for scanned PDFs (default: 200)
- `DOCREADER_PDF_RENDER_MAX_EDGE`: longest edge of a rendered page in pixels (default: 2000)
- `DOCREADER_PDF_JPEG_QUALITY`: JPEG quality (default: 85, clamped to 1-95)

If importing several large PDFs uses too many resources, lower `DOCREADER_PDF_RENDER_MAX_WORKERS` or `DOCREADER_MARKITDOWN_MAX_WORKERS` first.

### Storage

DocReader does not write to object storage and reads no storage variables (`STORAGE_TYPE`, `MINIO_*`, `COS_*`, `OSS_*`). It returns extracted images as inline bytes in the gRPC response, and the Go app persists them to its configured backend. Configure storage on the Go app; with `STORAGE_TYPE=minio` it requires `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY_ID`, `MINIO_SECRET_ACCESS_KEY` and `MINIO_BUCKET_NAME` and has no built-in default for any of them. `docker-compose.yml` fills in the endpoint and the `minioadmin` keys but leaves `MINIO_BUCKET_NAME` empty, so set it yourself. See [Storage backends](../website-docs/03-features/19-storage-backends.md).

MinerU and the other external parsing services are configured on the Go app too, not here.

## FAQ

### 1. The DocReader service does not start?

Check the container logs for missing dependencies or permission errors, and compare the logged effective config with what you set.

### 2. Images do not display?

DocReader only returns image bytes. The app stores them and builds their URLs, so check the app's storage backend configuration.

### 3. File upload fails?

Check that `DOCREADER_GRPC_MAX_FILE_SIZE_MB` (or `MAX_FILE_SIZE_MB`) is large enough. The frontend, app and docreader limits must match.

## Service health check

```yaml
healthcheck:
  test: ["CMD", "grpc_health_probe", "-addr=localhost:50051"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 60s
```

```bash
docker ps | grep docreader
docker logs ai-kb-docreader
```

Compose runs it as container `ai-kb-docreader` on network `ai-kb-network` with restart policy `unless-stopped`.
