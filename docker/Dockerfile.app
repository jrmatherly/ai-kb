# Node 24 for the BrowserSkill build.
FROM node:24-bookworm-slim@sha256:a5f4366e834695d5f363b920c104953a535d3a9864d1bdf74566a0ebe01daee5 AS node-base

# Build extension and daemon from the same pinned source on the runtime architecture.
FROM node-base AS browserskill
# A failed curl must fail the curl | sh steps, not pipe an empty script.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
WORKDIR /build
# Use the same optional Debian mirror as the Go builder and runtime stages.
ARG APK_MIRROR_ARG
# This base image has no CA certificates. Install them from the original HTTP
# Debian source before swapping in a mirror: mirrors.tencent.com redirects
# apt to HTTPS, and that handshake fails without a trust store.
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
    apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    sed -i "s@deb.debian.org@${APK_MIRROR_ARG}@g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends git python3 ca-certificates curl build-essential cmake pkg-config && \
    rm -rf /var/lib/apt/lists/*
ENV RUSTUP_HOME=/usr/local/rustup CARGO_HOME=/usr/local/cargo
ENV PATH=/usr/local/cargo/bin:$PATH
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal --default-toolchain stable
COPY scripts/build_browserskill.sh scripts/browserskill-release.json ./scripts/
ARG TARGETOS
ARG TARGETARCH
RUN bash scripts/build_browserskill.sh /opt/ai-kb/browserskill "${TARGETOS}/${TARGETARCH}"

# Build stage
FROM golang:1.26.10-bookworm@sha256:5b1f78db1f54cc8cb46e652c51d3f854441c4310a449161b6fe9fb553edab505 AS builder
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

WORKDIR /app

# Go module settings for private modules or a mirror. Not for secrets: build args
# are visible in the build history. Empty values keep Go's defaults
# (proxy.golang.org, sum.golang.org); pass GOSUMDB_ARG=off only where the
# checksum database is unreachable.
# hadolint ignore=DL3064
ARG GOPRIVATE_ARG
ARG GOPROXY_ARG
ARG GOSUMDB_ARG
ARG APK_MIRROR_ARG

# hadolint ignore=DL3064
ENV GOPRIVATE=${GOPRIVATE_ARG}
ENV GOPROXY=${GOPROXY_ARG}
ENV GOSUMDB=${GOSUMDB_ARG}

# Install dependencies
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
    sed -i "s@deb.debian.org@${APK_MIRROR_ARG}@g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends git build-essential libsqlite3-dev curl

# Copy go mod files. go.mod replace-points anydoc at ./third_party/anydoc-go,
# so that module's go.mod must exist before `go mod download`.
COPY go.mod go.sum ./
COPY third_party/anydoc-go/go.mod third_party/anydoc-go/go.mod
RUN --mount=type=cache,target=/go/pkg/mod go mod download
# The migrate CLI must match the golang-migrate library version the module builds against.
# hadolint ignore=DL3062
RUN --mount=type=cache,target=/go/pkg/mod \
    go install -tags 'postgres' \
    "github.com/golang-migrate/migrate/v4/cmd/migrate@$(go list -m -f '{{.Version}}' github.com/golang-migrate/migrate/v4)"
COPY cmd/download cmd/download
# hadolint ignore=DL3062
RUN go run cmd/download/duckdb/duckdb.go
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod bash ./scripts/copy-licenses.sh /license-bundle

# Get version and commit info for build injection
ARG VERSION_ARG
ARG COMMIT_ID_ARG
ARG BUILD_TIME_ARG
ARG GO_VERSION_ARG

# Set build-time variables
ENV VERSION=${VERSION_ARG}
ENV COMMIT_ID=${COMMIT_ID_ARG}
ENV BUILD_TIME=${BUILD_TIME_ARG}
ENV GO_VERSION=${GO_VERSION_ARG}

# Link the anydoc parser engine (office docs converted in-process, no
# Python docreader). Default on so Hub / compose images ship a working
# engine; pass WITH_ANYDOC=0 to skip the Rust toolchain (~few minutes and
# ~1 GB of build-stage layers).
ARG WITH_ANYDOC=1
ENV RUSTUP_HOME=/usr/local/rustup CARGO_HOME=/usr/local/cargo
ENV PATH=/usr/local/cargo/bin:$PATH
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/usr/local/cargo/git \
    if [ "$WITH_ANYDOC" = "1" ]; then \
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
    | sh -s -- -y --profile minimal --default-toolchain stable && \
    ./scripts/build-anydoc-lib.sh; \
    fi

# Build the application with version info
RUN --mount=type=cache,target=/go/pkg/mod \
    if [ "$WITH_ANYDOC" = "1" ]; then \
    make build-prod GO_BUILD_TAGS=anydoc; \
    else \
    make build-prod; \
    fi
RUN --mount=type=cache,target=/go/pkg/mod cp -r /go/pkg/mod/github.com/yanyiwu/ /app/yanyiwu/

# Final stage
FROM debian:12.15-slim
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

WORKDIR /app

ARG APK_MIRROR_ARG

# Pairing derives the gateway URL from the user's page origin by default.
ENV BROWSERSKILL_BINARY=/opt/ai-kb/browserskill/bsk \
    BROWSERSKILL_EXTENSION_PATH=/opt/ai-kb/browserskill/browser-skill-ai-kb-0.3.1.zip
COPY --from=browserskill /opt/ai-kb/browserskill /opt/ai-kb/browserskill

# Fixed uid/gid 1000: the Helm chart's runAsUser/runAsGroup/fsGroup rely on it.
RUN groupadd -g 1000 appuser && \
    useradd -m -u 1000 -g 1000 -s /bin/bash appuser

# First, install ca-certificates without mirror to ensure HTTPS works
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Then switch to mirror if specified and install other packages
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
    sed -i "s@deb.debian.org@${APK_MIRROR_ARG}@g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends ca-certificates curl gosu tzdata && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create data directories and set permissions
RUN mkdir -p /data/files && \
    chown -R appuser:appuser /app /data/files

# Copy migrate tool from builder stage
COPY --from=builder /go/bin/migrate /usr/local/bin/
COPY --from=builder /app/yanyiwu/ /go/pkg/mod/github.com/yanyiwu/

# Copy the binary from the builder stage
COPY --from=builder /app/config ./config
COPY --from=builder /app/scripts ./scripts
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/dataset/samples ./dataset/samples
COPY --from=builder /root/.duckdb /home/appuser/.duckdb
COPY --from=builder /app/ai-kb .
COPY --from=builder /license-bundle/ ./

# Copy and make entrypoint script executable
COPY --from=builder /app/scripts/docker-entrypoint.sh ./scripts/docker-entrypoint.sh

# Make scripts executable. DuckDB spills larger-than-memory queries to ".tmp"
# under the cwd; point it at /tmp so a read-only /app still works.
RUN chmod +x ./scripts/*.sh && \
    ln -s /tmp /app/.tmp

# Pin HOME so DuckDB finds the prebaked ~/.duckdb extensions under any uid
# (an unknown uid gets HOME=/, and INSTALL then needs network and a writable root).
ENV HOME=/home/appuser

# No USER: compose starts as root for the entrypoint's chown and gosu step. An orchestrator must set
# runAsUser 1000 (the chart does); runAsNonRoot alone fails with CreateContainerConfigError.

# Expose ports
EXPOSE 8080


ENTRYPOINT ["./scripts/docker-entrypoint.sh"]
CMD ["./ai-kb"]
