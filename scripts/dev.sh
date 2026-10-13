#!/bin/bash
# Dev environment script - starts only the infrastructure; run app and frontend locally by hand

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # no color

# Project root
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

# Logging helpers
log_info() {
    printf "%b\n" "${BLUE}[INFO]${NC} $1"
}

log_success() {
    printf "%b\n" "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    printf "%b\n" "${RED}[ERROR]${NC} $1"
}

log_warning() {
    printf "%b\n" "${YELLOW}[WARNING]${NC} $1"
}

# Pick an available Docker Compose command
DOCKER_COMPOSE_BIN=""
DOCKER_COMPOSE_SUBCMD=""

detect_compose_cmd() {
    if docker compose version &> /dev/null; then
        DOCKER_COMPOSE_BIN="docker"
        DOCKER_COMPOSE_SUBCMD="compose"
        return 0
    fi
    if command -v docker-compose &> /dev/null; then
        if docker-compose version &> /dev/null; then
            DOCKER_COMPOSE_BIN="docker-compose"
            DOCKER_COMPOSE_SUBCMD=""
            return 0
        fi
    fi
    return 1
}

# Show help
show_help() {
    printf "%b\n" "${GREEN}AI KB dev environment script${NC}"
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  start      start the infrastructure services (postgres, redis, docreader, langfuse)"
    echo "  stop       stop all services"
    echo "  restart    restart all services"
    echo "  logs       show service logs"
    echo "  status     show service status"
    echo "  app        start the backend (runs locally)"
    echo "  frontend   start the frontend dev server (runs locally)"
    echo "  help       show this help"
    echo ""
    echo "Optional profiles (for the start command):"
    echo "  --minio       start MinIO object storage"
    echo "  --qdrant      start the Qdrant vector database"
    echo "  --neo4j       start the Neo4j graph database"
    echo "  --dex         start Dex (OIDC authentication)"
    echo "  --langfuse    start Langfuse (on by default)"
    echo "  --no-langfuse do not start Langfuse"
    echo "  --odl-hybrid  start OpenDataLoader hybrid (Docling, large image, enable when needed)"
    echo "  --full        start all optional services (excludes odl-hybrid, add --odl-hybrid for it)"
    echo ""
    echo "Examples:"
    echo "  $0 start                    # start the base services"
    echo "  $0 start --qdrant           # base services + Qdrant"
    echo "  $0 start --dex             # base services + Dex"
    echo "  $0 start --odl-hybrid       # base services + OpenDataLoader hybrid"
    echo "  $0 start --full             # start all services"
    echo "  make dev-start DEV_ARGS=--odl-hybrid   # same as above (argument passed through the Makefile)"
    echo "  $0 app                      # start the backend in another terminal"
    echo "  $0 frontend                 # start the frontend in another terminal"
}

# Load .env and the optional .env.local (the latter overrides the former)
# Strip trailing \r while reading, to accept Windows-style (CRLF) line endings.
# Otherwise bash source treats the leftover \r as a command: "...: $'\r': command not found".
# Note: source <(sed ...) does not work. With the Bash 3.2 that ships with macOS, sourcing a process substitution
# does not import variables into the current shell, so write to a seekable temp file and source that.
_source_env_file() {
    local src="$1"
    local tmp
    tmp="$(mktemp)" || return 1
    sed -e 's/\r$//' "$src" > "$tmp"
    set -a
    # shellcheck source=/dev/null
    source "$tmp"
    set +a
    rm -f "$tmp"
}

load_env_files() {
    if [ -f ".env" ]; then
        _source_env_file .env || return 1
    else
        return 1
    fi

    if [ -f ".env.local" ]; then
        log_info "Loading .env.local overrides..."
        _source_env_file .env.local || return 1
    fi
    return 0
}

# Check Docker
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Install Docker first"
        return 1
    fi
    
    if ! detect_compose_cmd; then
        log_error "Docker Compose not found"
        return 1
    fi
    
    if ! docker info &> /dev/null; then
        log_error "The Docker service is not running"
        return 1
    fi
    
    return 0
}

# Check whether .env enables hybrid mode (to rebuild docreader after starting --odl-hybrid)
_should_enable_odl_hybrid_from_env() {
    local hybrid="${DOCREADER_ODL_HYBRID:-off}"
    hybrid=$(printf '%s' "$hybrid" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
    case "$hybrid" in
        off|"") return 1 ;;
        *) return 0 ;;
    esac
}

_enable_odl_hybrid_profile() {
    PROFILES="$PROFILES --profile odl-hybrid"
    ENABLED_SERVICES="$ENABLED_SERVICES odl-hybrid"
}

# Wait for the odl-hybrid HTTP health check (after compose starts, the service may still be pulling dependencies)
_wait_odl_hybrid_ready() {
    local port="${ODL_HYBRID_PORT:-5002}"
    local max_wait="${ODL_HYBRID_STARTUP_WAIT_SEC:-180}"
    local waited=0
    local interval=5

    if ! command -v curl &> /dev/null; then
        log_warning "curl is not installed, skipping the odl-hybrid readiness wait. Check http://localhost:${port}/health by hand"
        return 0
    fi

    log_info "Waiting for odl-hybrid to be ready (up to ${max_wait}s; the first run builds the image: docker compose ... build odl-hybrid)..."
    while [ "$waited" -lt "$max_wait" ]; do
        if curl -sf "http://127.0.0.1:${port}/health" >/dev/null 2>&1; then
            log_success "odl-hybrid is ready (http://localhost:${port}/health)"
            return 0
        fi
        sleep "$interval"
        waited=$((waited + interval))
    done
    log_warning "odl-hybrid was not ready within ${max_wait}s. Check: docker logs ai-kb-odl-hybrid"
    return 1
}

# Start the infrastructure services
start_services() {
    log_info "Starting the dev infrastructure services..."
    
    check_docker
    if [ $? -ne 0 ]; then
        return 1
    fi

    cd "$PROJECT_ROOT"
    
    # Check the .env file
    if [ ! -f ".env" ]; then
        log_error ".env file not found. Create it first"
        return 1
    fi

    load_env_files
    if [ $? -ne 0 ]; then
        log_error ".env file not found. Create it first"
        return 1
    fi

    if [ -n "${DEV_REMOTE_HOST:-}" ]; then
        log_warning "DEV_REMOTE_HOST=${DEV_REMOTE_HOST} is set, skipping the local Docker infrastructure"
        log_info "Remote services: PostgreSQL/Redis/DocReader/Langfuse → ${DEV_REMOTE_HOST}"
        log_info "Next: make dev-app (local backend) or make dev-frontend (frontend)"
        return 0
    fi
    
    # Parse the profile arguments
    shift  # drop the "start" command itself
    # Starts the infrastructure (postgres / redis / docreader) + langfuse by default;
    # other optional services are enabled with --minio / --qdrant / --neo4j / --dex / --full.
    PROFILES="--profile langfuse"
    ENABLED_SERVICES="langfuse"
    while [ $# -gt 0 ]; do
        case "$1" in
            --minio)
                PROFILES="$PROFILES --profile minio"
                ENABLED_SERVICES="$ENABLED_SERVICES minio"
                ;;
            --qdrant)
                PROFILES="$PROFILES --profile qdrant"
                ENABLED_SERVICES="$ENABLED_SERVICES qdrant"
                ;;
            --neo4j)
                PROFILES="$PROFILES --profile neo4j"
                ENABLED_SERVICES="$ENABLED_SERVICES neo4j"
                ;;
            --dex)
                PROFILES="$PROFILES --profile dex"
                ENABLED_SERVICES="$ENABLED_SERVICES dex"
                ;;
            --langfuse)
                PROFILES="$PROFILES --profile langfuse"
                ENABLED_SERVICES="$ENABLED_SERVICES langfuse"
                ;;
            --no-langfuse)
                PROFILES="${PROFILES//--profile langfuse/}"
                ENABLED_SERVICES="${ENABLED_SERVICES//langfuse/}"
                ;;
            --odl-hybrid)
                if [[ "$ENABLED_SERVICES" != *"odl-hybrid"* ]]; then
                    _enable_odl_hybrid_profile
                fi
                ;;
            --full)
                PROFILES="--profile full"
                ENABLED_SERVICES="minio qdrant neo4j dex"
                break
                ;;
            *)
                log_warning "Unknown argument: $1"
                ;;
        esac
        shift
    done

    # Start the services (odl-hybrid gets its own --build so docreader is not rebuilt every time)
    "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD -f docker-compose.dev.yml $PROFILES up -d
    local compose_rc=$?
    if [ "$compose_rc" -eq 0 ] && [[ "$ENABLED_SERVICES" == *"odl-hybrid"* ]]; then
        log_info "Building/updating the odl-hybrid image..."
        "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD -f docker-compose.dev.yml $PROFILES up -d --build odl-hybrid
        _wait_odl_hybrid_ready || true
        # docreader reads DOCREADER_ODL_HYBRID; if .env just changed, force a rebuild to inject it
        if _should_enable_odl_hybrid_from_env; then
            log_info "Rebuilding docreader to apply DOCREADER_ODL_HYBRID=${DOCREADER_ODL_HYBRID} ..."
            "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD -f docker-compose.dev.yml up -d --force-recreate docreader
        fi
    fi

    if [ "$compose_rc" -eq 0 ]; then
        log_success "Infrastructure services started"
        echo ""
        log_info "Service URLs:"
        echo "  - PostgreSQL:    localhost:5432"
        echo "  - Redis:         localhost:6379"
        echo "  - DocReader:     localhost:50051"
        
        # Show the extra services for the enabled profiles
        if [[ "$ENABLED_SERVICES" == *"minio"* ]]; then
            echo "  - MinIO:         localhost:9000 (Console: localhost:9001)"
        fi
        if [[ "$ENABLED_SERVICES" == *"qdrant"* ]]; then
            echo "  - Qdrant:        localhost:6333 (gRPC: localhost:6334)"
        fi
        if [[ "$ENABLED_SERVICES" == *"neo4j"* ]]; then
            echo "  - Neo4j:         localhost:7474 (Bolt: localhost:7687)"
        fi
        if [[ "$ENABLED_SERVICES" == *"dex"* ]]; then
            echo "  - Dex:           localhost:5556"
        fi
        if [[ "$ENABLED_SERVICES" == *"langfuse"* ]]; then
            echo "  - Langfuse:      http://localhost:${LANGFUSE_WEB_PORT:-3000}"
        fi
        if [[ "$ENABLED_SERVICES" == *"odl-hybrid"* ]]; then
            echo "  - ODL Hybrid:    http://localhost:${ODL_HYBRID_PORT:-5002} (health: /health)"
            echo "                   docreader needs DOCREADER_ODL_HYBRID=docling-fast"
        fi
        
        echo ""
        log_info "Next steps:"
        printf "%b\n" "${YELLOW}1. Run the backend in a new terminal:${NC} make dev-app"
        printf "%b\n" "${YELLOW}2. Run the frontend in a new terminal:${NC} make dev-frontend"
        return 0
    else
        log_error "Services failed to start"
        return 1
    fi
}

# Stop the services
stop_services() {
    log_info "Stopping the dev environment services..."
    
    check_docker
    if [ $? -ne 0 ]; then
        return 1
    fi
    
    cd "$PROJECT_ROOT"
    "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD -f docker-compose.dev.yml down
    
    if [ $? -eq 0 ]; then
        log_success "All services stopped"
        return 0
    else
        log_error "Services failed to stop"
        return 1
    fi
}

# Restart the services
restart_services() {
    stop_services
    sleep 2
    start_services
}

# Show logs
show_logs() {
    check_docker
    if [ $? -ne 0 ]; then
        return 1
    fi

    cd "$PROJECT_ROOT"
    "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD -f docker-compose.dev.yml logs -f
}

# Show status
show_status() {
    check_docker
    if [ $? -ne 0 ]; then
        return 1
    fi

    cd "$PROJECT_ROOT"
    "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD -f docker-compose.dev.yml ps
}

# In remote dev mode, check that the infrastructure ports are reachable
check_remote_dev_connectivity() {
    local host="${DEV_REMOTE_HOST:-}"
    if [ -z "$host" ]; then
        return 0
    fi

    local db_port="${DB_PORT:-5432}"
    local redis_port
    redis_port="${REDIS_ADDR#*:}"
    if [ "$redis_port" = "$REDIS_ADDR" ]; then
        redis_port=6379
    fi
    local docreader_port="${DOCREADER_PORT:-50051}"

    log_info "Checking connectivity to the remote infrastructure (${host})..."
    local failed=0
    for spec in "PostgreSQL:${host}:${db_port}" "Redis:${host}:${redis_port}" "DocReader:${host}:${docreader_port}"; do
        local name="${spec%%:*}"
        local rest="${spec#*:}"
        local h="${rest%%:*}"
        local p="${rest##*:}"
        if command -v nc &> /dev/null; then
            if nc -z -w 3 "$h" "$p" 2>/dev/null; then
                log_success "${name} ${h}:${p} reachable"
            else
                log_error "${name} ${h}:${p} unreachable (no route / connection refused)"
                failed=1
            fi
        else
            log_warning "nc is not installed, skipping the ${name} connectivity check"
        fi
    done

    if [ "$failed" -ne 0 ]; then
        echo ""
        log_error "Cannot reach the remote dev environment ${host}"
        log_info "Troubleshooting:"
        echo "  1. Make sure the Docker containers are running on the remote machine (postgres/redis/docreader)"
        echo "  2. Make sure this machine and ${host} are on the same LAN (this machine: $(ipconfig getifaddr en0 2>/dev/null || echo 'unknown'))"
        echo "  3. Check the port mappings on the remote: docker ps --format 'table {{.Names}}\t{{.Ports}}'"
        echo "  4. Check that the remote firewall allows 5432/6379/50051"
        return 1
    fi
    return 0
}

# Host-platform path of the anydoc static archive (built by `make anydoc-lib`).
anydoc_host_archive() {
    case "$(uname -s)-$(uname -m)" in
        Darwin-arm64) echo "$PROJECT_ROOT/third_party/anydoc-go/lib/darwin_arm64/libanydoc_go.a" ;;
        Darwin-x86_64) echo "$PROJECT_ROOT/third_party/anydoc-go/lib/darwin_amd64/libanydoc_go.a" ;;
        Linux-x86_64)
            if [ -f "$PROJECT_ROOT/third_party/anydoc-go/lib/linux_amd64_gnu/libanydoc_go.a" ]; then
                echo "$PROJECT_ROOT/third_party/anydoc-go/lib/linux_amd64_gnu/libanydoc_go.a"
            else
                echo "$PROJECT_ROOT/third_party/anydoc-go/lib/linux_amd64_musl/libanydoc_go.a"
            fi
            ;;
        Linux-aarch64)
            if [ -f "$PROJECT_ROOT/third_party/anydoc-go/lib/linux_arm64_gnu/libanydoc_go.a" ]; then
                echo "$PROJECT_ROOT/third_party/anydoc-go/lib/linux_arm64_gnu/libanydoc_go.a"
            else
                echo "$PROJECT_ROOT/third_party/anydoc-go/lib/linux_arm64_musl/libanydoc_go.a"
            fi
            ;;
        *) echo "" ;;
    esac
}

# Enable the in-process anydoc engine when the archive is present, unless the
# caller already set GO_BUILD_TAGS (including empty, which opts out).
enable_anydoc_build_tag() {
    if [ -n "${GO_BUILD_TAGS+x}" ]; then
        export GO_BUILD_TAGS
        return
    fi
    local archive
    archive="$(anydoc_host_archive)"
    if [ -n "$archive" ] && [ -f "$archive" ]; then
        export GO_BUILD_TAGS=anydoc
        log_info "anydoc static library found, enabling -tags anydoc"
    else
        log_info "anydoc static library not found, so that parsing engine is unavailable. When needed, run: make anydoc-lib"
    fi
}

# Start the backend (local)
start_app() {
    log_info "Starting the backend (local dev mode)..."
    
    cd "$PROJECT_ROOT"
    
    # Check that Go is installed
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        return 1
    fi
    
    log_info "Loading the environment configuration..."
    if ! load_env_files; then
        log_error ".env file not found. Create the config file first"
        return 1
    fi
    
    # Local docker-compose.dev mode: map container service names to the host loopback address
    # Remote dev mode (DEV_REMOTE_HOST, or addresses set in .env.local) keeps the values from .env/.env.local
    if [ -n "${DEV_REMOTE_HOST:-}" ]; then
        log_info "Remote dev mode: infrastructure → ${DEV_REMOTE_HOST}"
        export DB_HOST="${DB_HOST:-$DEV_REMOTE_HOST}"
        export REDIS_ADDR="${REDIS_ADDR:-$DEV_REMOTE_HOST:6379}"
        export DOCREADER_ADDR="${DOCREADER_ADDR:-$DEV_REMOTE_HOST:50051}"
        export MINIO_ENDPOINT="${MINIO_ENDPOINT:-$DEV_REMOTE_HOST:9000}"
        export MILVUS_ADDRESS="${MILVUS_ADDRESS:-$DEV_REMOTE_HOST:19530}"
        export NEO4J_URI="${NEO4J_URI:-bolt://$DEV_REMOTE_HOST:7687}"
        export QDRANT_HOST="${QDRANT_HOST:-$DEV_REMOTE_HOST}"
        if [ -z "${LANGFUSE_HOST:-}" ] || [ "$LANGFUSE_HOST" = "http://langfuse-web:3000" ]; then
            export LANGFUSE_HOST="http://${DEV_REMOTE_HOST}:3000"
        fi
    else
        export DB_HOST=127.0.0.1
        export DOCREADER_ADDR=127.0.0.1:50051
        export MINIO_ENDPOINT=127.0.0.1:9000
        export REDIS_ADDR=127.0.0.1:6379
        export MILVUS_ADDRESS=127.0.0.1:19530
        export NEO4J_URI=bolt://127.0.0.1:7687
        export QDRANT_HOST=127.0.0.1
    fi
    export DOCREADER_TRANSPORT="${DOCREADER_TRANSPORT:-grpc}"

    if ! check_remote_dev_connectivity; then
        return 1
    fi

    # .env.example uses /data/files for the Docker app container, where a
    # volume is mounted at that path. When the backend runs directly on the
    # host via dev-app, /data is often read-only or missing, so use a repo-local
    # writable directory unless the developer explicitly configured another
    # local storage path.
    if [ -z "${LOCAL_STORAGE_BASE_DIR:-}" ] || [ "$LOCAL_STORAGE_BASE_DIR" = "/data/files" ]; then
        export LOCAL_STORAGE_BASE_DIR="$PROJECT_ROOT/.local-data/files"
    fi
    mkdir -p "$LOCAL_STORAGE_BASE_DIR"
    
    # Make sure the required environment variables are set
    if [ -z "$DB_DRIVER" ]; then
        log_error "DB_DRIVER is not set. Check the .env file"
        return 1
    fi
    
    log_info "Environment variables set, starting the app..."
    log_info "Database address: $DB_HOST:${DB_PORT:-5432}"
    
    export CGO_CFLAGS="-Wno-deprecated-declarations -Wno-gnu-folding-constant"
    if [[ "$(uname)" == "Darwin" ]]; then
      export CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries"
    fi

    enable_anydoc_build_tag

    # Check whether Air (hot reload tool) is installed
    if command -v air &> /dev/null; then
        log_success "Air found, starting with hot reload..."
        log_info "Go code changes are recompiled and restarted automatically"
        air
    else
        log_info "Air not found, starting in normal mode"
        log_warning "Tip: install Air to restart automatically on code changes"
        log_info "Install with: go install github.com/air-verse/air@latest"
        LDFLAGS="$(./scripts/get_version.sh ldflags) -X 'google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'"
        go run -tags "${GO_BUILD_TAGS:-}" -ldflags="$LDFLAGS" ./cmd/server
    fi
}

# Start the frontend (local)
start_frontend() {
    log_info "Starting the frontend dev server..."

    cd "$PROJECT_ROOT"
    if [ -f ".env" ] || [ -f ".env.local" ]; then
        load_env_files >/dev/null 2>&1 || true
    fi
    
    cd "$PROJECT_ROOT/frontend"
    
    # Check that npm is installed
    if ! command -v npm &> /dev/null; then
        log_error "npm is not installed"
        return 1
    fi
    
    # Check whether the dependencies are installed
    if [ ! -d "node_modules" ]; then
        log_warning "node_modules not found, installing dependencies..."
        npm install
    fi
    
    log_info "Starting the Vite dev server..."
    log_info "The frontend runs at http://localhost:5173"
    log_info "Frontend API proxy target: ${VITE_DEV_PROXY_TARGET:-${FRONTEND_BACKEND_URL:-http://localhost:8080}}"
    
    # Run the dev server
    npm run dev
}

# Parse the command
CMD="${1:-help}"
case "$CMD" in
    start)
        start_services "$@"
        ;;
    stop)
        stop_services
        ;;
    restart)
        restart_services
        ;;
    logs)
        show_logs
        ;;
    status)
        show_status
        ;;
    app)
        start_app
        ;;
    frontend)
        start_frontend
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        log_error "Unknown command: $CMD"
        show_help
        exit 1
        ;;
esac
