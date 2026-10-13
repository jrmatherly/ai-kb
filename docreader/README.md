# DocReader Service

DocReader is the AI KB gRPC service that parses and processes documents. It reads many document formats and supports OCR and multimodal processing.

## Docker Compose environment variables

In `docker-compose.yml`, the docreader service sets these environment variables:

```yaml
docreader:
  image: ghcr.io/jrmatherly/ai-kb-docreader:${AIKB_VERSION:-latest}
  environment:
    - MINIO_ENDPOINT=minio:9000
    - MINIO_PUBLIC_ENDPOINT=http://localhost:${MINIO_PORT:-9000}
    - MINERU_ENDPOINT=${MINERU_ENDPOINT:-}
    - MAX_FILE_SIZE_MB=${MAX_FILE_SIZE_MB:-}
```

### Environment variable reference

#### 1. MINIO_ENDPOINT

- **Description**: internal address of the MinIO service (container to container)
- **Default**: `minio:9000`
- **Purpose**: DocReader connects to MinIO object storage at this address to read and store files during document processing
- **Example**:

  ```yaml
  - MINIO_ENDPOINT=minio:9000  # address inside the Docker network
  ```

#### 2. MINIO_PUBLIC_ENDPOINT

- **Description**: public address of the MinIO service (external access)
- **Default**: `http://localhost:9000`
- **Purpose**: builds externally reachable file URLs, for example image links returned after parsing a document
- **Important**:
  - To access it from other devices or containers, replace `localhost` with the real IP address
  - Set `MINIO_PORT` in `.env` to change the port
- **Example**:

  ```bash
  # .env file
  MINIO_PORT=9000
  ```

  Or edit docker-compose.yml directly:

  ```yaml
  - MINIO_PUBLIC_ENDPOINT=http://192.168.1.100:9000  # use the real IP
  ```

#### 3. MINERU_ENDPOINT

- **Description**: address of the MinerU service (optional)
- **Default**: empty (MinerU not used)
- **Purpose**: MinerU is an advanced document parsing service that recognizes and processes more complex document structures. With this variable set, DocReader can call MinerU to parse documents
- **Example**:

  ```bash
  # .env file
  MINERU_ENDPOINT=http://mineru-service:8080
  ```

#### 4. MAX_FILE_SIZE_MB

- **Description**: maximum upload file size (in MB)
- **Default**: `50` MB
- **Purpose**: limits the file size the gRPC service accepts, so oversized files cannot crash the service or degrade performance
- **Example**:

  ```bash
  # .env file
  MAX_FILE_SIZE_MB=100  # allow files up to 100MB
  ```

## Other environment variables

Besides the variables set in docker-compose.yml, DocReader supports these (add them as needed):

### gRPC

- `DOCREADER_GRPC_MAX_WORKERS`: maximum gRPC worker threads (default: 4)
- `DOCREADER_GRPC_PORT`: gRPC listen port (default: 50051)

### Parser resource limits

- `DOCREADER_MARKITDOWN_MAX_WORKERS`: maximum concurrent MarkItDown parses (default: 1, 0 disables the limit)
- `DOCREADER_PDF_RENDER_MAX_WORKERS`: maximum concurrent renders of scanned PDFs to images (default: 1, 0 disables the limit)
- `DOCREADER_PDF_RENDER_DPI`: render DPI for scanned PDFs (default: 200)
- `DOCREADER_PDF_JPEG_QUALITY`: JPEG quality for scanned PDF output (default: 85, clamped to 1-95)

### OCR / VLM

DocReader no longer has built-in OCR or VLM backends. Scanned PDFs are rendered to JPEG images, and the Go app calls the OCR/VLM services on them. See the main project docs for that configuration.

### Storage

DocReader does not write to object storage and reads no storage variables (`STORAGE_TYPE`, `MINIO_*`, `COS_*`, `OSS_*`). It returns extracted images as inline bytes in the gRPC response, and the Go app persists them to its configured backend. Configure storage on the Go app; with `STORAGE_TYPE=minio` it requires `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY_ID`, `MINIO_SECRET_ACCESS_KEY` and `MINIO_BUCKET_NAME` and has no built-in default for any of them. `docker-compose.yml` fills in the endpoint and the `minioadmin` keys but leaves `MINIO_BUCKET_NAME` empty, so set it yourself. See [Storage backends](../website-docs/03-features/19-storage-backends.md).

### Proxy

To reach external services through a proxy:

- `EXTERNAL_HTTP_PROXY`: HTTP proxy address
- `EXTERNAL_HTTPS_PROXY`: HTTPS proxy address

### Image processing

Scanned PDFs are rendered to JPEG images and handed to the Go app for OCR. If importing several large PDFs uses too many resources,
lower `DOCREADER_PDF_RENDER_MAX_WORKERS` or `DOCREADER_MARKITDOWN_MAX_WORKERS` first.

## Configuration examples

### Basic (MinIO)

```yaml
docreader:
  environment:
    - MINIO_ENDPOINT=minio:9000
    - MINIO_PUBLIC_ENDPOINT=http://localhost:9000
    - MAX_FILE_SIZE_MB=50
```

### Advanced (MinerU enabled)

```yaml
docreader:
  environment:
    - MINIO_ENDPOINT=minio:9000
    - MINIO_PUBLIC_ENDPOINT=http://192.168.1.100:9000
    - MINERU_ENDPOINT=http://mineru:8080
    - MAX_FILE_SIZE_MB=100
```

### Using Tencent Cloud COS

```yaml
docreader:
  environment:
    - STORAGE_TYPE=cos
    - COS_SECRET_ID=your_secret_id
    - COS_SECRET_KEY=your_secret_key
    - COS_REGION=ap-guangzhou
    - COS_BUCKET_NAME=your-bucket
    - COS_APP_ID=your_app_id
    - MAX_FILE_SIZE_MB=50
```

### Using Alibaba Cloud OSS

```yaml
docreader:
  environment:
    - STORAGE_TYPE=oss
    - OSS_ACCESS_KEY_ID=your_access_key_id
    - OSS_ACCESS_KEY_SECRET=your_access_key_secret
    - OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com
    - OSS_BUCKET_NAME=your-bucket
    - OSS_REGION=cn-hangzhou
    - MAX_FILE_SIZE_MB=50
```

## FAQ

### 1. The DocReader service does not start?

Check the container logs for missing dependencies or permission errors. If needed, confirm that `MINIO_ENDPOINT` and the storage environment variables are set correctly.

### 2. Images do not display?

Check `MINIO_PUBLIC_ENDPOINT`:

- Make sure the address is reachable from the browser
- When accessing from another device, use the real IP address, not `localhost`

### 3. File upload fails?

Check that `MAX_FILE_SIZE_MB` is large enough. The frontend and backend file size limits must also match.

## Service health check

The DocReader service has a health check:

```yaml
healthcheck:
  test: ["CMD", "grpc_health_probe", "-addr=localhost:50051"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 60s
```

Check the service status with:

```bash
docker ps | grep docreader
docker logs ai-kb-docreader
```

## More information

- Service port: 50051 (gRPC)
- Container name: ai-kb-docreader
- Network: ai-kb-network
- Restart policy: unless-stopped
