#!/bin/bash
# Starts and stops Ollama and the docker-compose services on demand

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # no color

# Project root (the parent of the script directory)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

# Version information
VERSION="1.0.1" # version bump

# Show help
show_help() {
    printf "%b\n" "${GREEN}AI KB start script v${VERSION}${NC}"
    printf "%b\n" "${GREEN}Usage:${NC} $0 [options]"
    echo "Options:"
    echo "  -h, --help     show help"
    echo "  -o, --ollama   start the Ollama service"
    echo "  -d, --docker   start the Docker container services"
    echo "  -a, --all      start all services (default)"
    echo "  -s, --stop     stop all services"
    echo "  -c, --check    check the environment and diagnose problems"
    echo "  -r, --restart  rebuild and restart the given container"
    echo "  -l, --list     list all running containers"
    echo "  -p, --pull     pull the latest Docker images"
    echo "  --no-pull      do not pull images on start (pulls by default)"
    echo "  -v, --version  show version information"
    exit "${1:-0}"
}

# Show version information
show_version() {
    printf "%b\n" "${GREEN}AI KB start script v${VERSION}${NC}"
    exit 0
}

# Logging helpers
log_info() {
    printf "%b\n" "${BLUE}[INFO]${NC} $1"
}

log_warning() {
    printf "%b\n" "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    printf "%b\n" "${RED}[ERROR]${NC} $1"
}

log_success() {
    printf "%b\n" "${GREEN}[SUCCESS]${NC} $1"
}

# Inject git short hash into the frontend image when composing from source.
# docker-compose.yml interpolates VITE_FRONTEND_COMMIT; the build context is
# frontend/ (no .git), so Vite cannot discover the commit on its own.
export_frontend_build_args() {
    if [ -n "${VITE_FRONTEND_COMMIT:-}" ]; then
        export VITE_FRONTEND_COMMIT
        return 0
    fi
    eval "$("$PROJECT_ROOT/scripts/get_version.sh" env)"
    export VITE_FRONTEND_COMMIT="${COMMIT_ID:-unknown}"
    log_info "VITE_FRONTEND_COMMIT=${VITE_FRONTEND_COMMIT}"
}

# Pick an available Docker Compose command (docker compose first, then docker-compose)
DOCKER_COMPOSE_BIN=""
DOCKER_COMPOSE_SUBCMD=""

detect_compose_cmd() {
	# Prefer the Docker Compose plugin
	if docker compose version &> /dev/null; then
		DOCKER_COMPOSE_BIN="docker"
		DOCKER_COMPOSE_SUBCMD="compose"
		return 0
	fi

	# Fall back to docker-compose (v1)
	if command -v docker-compose &> /dev/null; then
		if docker-compose version &> /dev/null; then
			DOCKER_COMPOSE_BIN="docker-compose"
			DOCKER_COMPOSE_SUBCMD=""
			return 0
		fi
	fi

	# Neither is available
	return 1
}

# Read .env, keeping any variable the caller already exported (the same precedence docker compose uses)
source_env_file() {
    local exported
    exported=$(export -p)
    # .env is a per-checkout runtime file, so there is nothing for shellcheck to follow.
    # shellcheck source=/dev/null
    source "$PROJECT_ROOT/.env"
    # export, not the declare -x that export -p prints: declare inside a function makes a local.
    # sed anchors at line starts, which ${var//} cannot.
    # shellcheck disable=SC2001
    eval "$(sed 's/^declare -x /export /' <<< "$exported")"
}

# Check for the .env file and create it if missing
check_env_file() {
    log_info "Checking the environment variable configuration..."
    if [ ! -f "$PROJECT_ROOT/.env" ]; then
        log_warning ".env file not found, creating it from the template"
        if [ -f "$PROJECT_ROOT/.env.example" ]; then
            cp "$PROJECT_ROOT/.env.example" "$PROJECT_ROOT/.env"
            log_success "Created .env from .env.example"
        else
            log_error ".env.example template not found, cannot create .env"
            return 1
        fi
    else
        log_info ".env file exists"
    fi

    # Check that the required environment variables are set
    source_env_file
    local missing_vars=()

    # Check the base variables
    if [ -z "$DB_DRIVER" ]; then missing_vars+=("DB_DRIVER"); fi
    if [ -z "$STORAGE_TYPE" ]; then missing_vars+=("STORAGE_TYPE"); fi
    if [ ${#missing_vars[@]} -gt 0 ]; then
        log_warning "Not set in .env: ${missing_vars[*]} (see .env.example)"
    else
        log_success "DB_DRIVER=$DB_DRIVER, STORAGE_TYPE=$STORAGE_TYPE"
    fi

    return 0
}

# Install Ollama (the method depends on the platform)
install_ollama() {
    # Check whether it is a remote service
    get_ollama_base_url

    if [ $IS_REMOTE -eq 1 ]; then
        log_info "Remote Ollama service configured, no local Ollama install needed"
        return 0
    fi

    log_info "Local Ollama is not installed, installing..."

    OS=$(uname)
    local rc
    if [ "$OS" = "Darwin" ]; then
        # macOS install
        if ! command -v brew &> /dev/null; then
            # The release zip holds only Ollama.app; its CLI must stay inside the
            # bundle next to the libraries it loads, so link it onto the PATH.
            log_info "Homebrew is not installed, installing Ollama.app from the release zip..."
            local app="/Applications/Ollama.app"
            rc=0
            if [ ! -d "$app" ]; then
                local tmp
                tmp=$(mktemp -d)
                curl -fsSL https://ollama.com/download/Ollama-darwin.zip -o "$tmp/ollama.zip" &&
                    unzip -q "$tmp/ollama.zip" -d "$tmp" &&
                    mv "$tmp/Ollama.app" /Applications/
                rc=$?
                rm -rf "$tmp"
            fi
            if [ "$rc" -eq 0 ]; then
                mkdir -p /usr/local/bin &&
                    ln -sf "$app/Contents/Resources/ollama" /usr/local/bin/ollama
                rc=$?
            fi
        else
            log_info "macOS detected, installing Ollama with brew..."
            brew install ollama
            rc=$?
        fi
    else
        # Linux install
        log_info "Linux detected, using the install script..."
        curl -fsSL https://ollama.com/install.sh | sh
        rc=$?
    fi

    if [ "$rc" -eq 0 ]; then
        log_success "Local Ollama installed"
        return 0
    else
        log_error "Local Ollama install failed"
        return 1
    fi
}

# Get the Ollama base URL and check whether it is a remote service
get_ollama_base_url() {

    check_env_file

    # Read the Ollama base URL from the environment
    OLLAMA_URL=${OLLAMA_BASE_URL:-"http://host.docker.internal:11434"}
    # Extract the host
    OLLAMA_HOST=$(echo "$OLLAMA_URL" | sed -E 's|^https?://||' | sed -E 's|:[0-9]+$||' | sed -E 's|/.*$||')
    # Extract the port
    OLLAMA_PORT=$(echo "$OLLAMA_URL" | grep -oE ':[0-9]+' | grep -oE '[0-9]+' || echo "11434")
    # Check for localhost or 127.0.0.1
    IS_REMOTE=0
    if [ "$OLLAMA_HOST" = "localhost" ] || [ "$OLLAMA_HOST" = "127.0.0.1" ] || [ "$OLLAMA_HOST" = "host.docker.internal" ]; then
        IS_REMOTE=0  # local service
    else
        IS_REMOTE=1  # remote service
    fi
}

# Start the Ollama service
start_ollama() {
    log_info "Checking the Ollama service..."
    # Extract the host and port
    get_ollama_base_url
    log_info "Ollama service address: $OLLAMA_URL"

    if [ $IS_REMOTE -eq 1 ]; then
        log_info "Remote Ollama service detected. Using it directly, with no local install or start"
        # Check that the remote service is available
        if curl -s "$OLLAMA_URL/api/tags" &> /dev/null; then
            log_success "Remote Ollama service is reachable"
            return 0
        else
            log_warning "Remote Ollama service is unreachable. Check that the address is right and the service is running"
            return 1
        fi
    fi

    # The rest handles a local service
    # Check whether Ollama is installed
    if ! command -v ollama &> /dev/null; then
        if ! install_ollama; then
            return 1
        fi
    fi

    # Check whether the Ollama service is already running
    if curl -s "http://localhost:$OLLAMA_PORT/api/tags" &> /dev/null; then
        log_success "Local Ollama service already running, port: $OLLAMA_PORT"
    else
        log_info "Starting the local Ollama service..."
        # Note: systemctl or launchctl is the official way to manage the service; a background process is only for temporary use
        systemctl restart ollama || (ollama serve > /dev/null 2>&1 < /dev/null &)

        # Wait for the service to start
        MAX_RETRIES=30
        COUNT=0
        while [ $COUNT -lt $MAX_RETRIES ]; do
            if curl -s "http://localhost:$OLLAMA_PORT/api/tags" &> /dev/null; then
                log_success "Local Ollama service started, port: $OLLAMA_PORT"
                break
            fi
            echo -ne "Waiting for the Ollama service to start... ($COUNT/$MAX_RETRIES)\r"
            sleep 1
            COUNT=$((COUNT + 1))
        done
        echo "" # newline

        if [ $COUNT -eq $MAX_RETRIES ]; then
            log_error "Local Ollama service failed to start"
            return 1
        fi
    fi

    log_success "Local Ollama service address: http://localhost:$OLLAMA_PORT"
    return 0
}

# Stop the Ollama service
stop_ollama() {
    log_info "Stopping the Ollama service..."

    # Check whether it is a remote service
    get_ollama_base_url

    if [ $IS_REMOTE -eq 1 ]; then
        log_info "Remote Ollama service detected, nothing to stop locally"
        return 0
    fi

    # Check whether Ollama is installed
    if ! command -v ollama &> /dev/null; then
        log_info "Local Ollama is not installed, nothing to stop"
        return 0
    fi

    # Find and kill the Ollama process
    if pgrep -x "ollama" > /dev/null; then
        # Prefer systemctl
        if command -v systemctl &> /dev/null; then
            sudo systemctl stop ollama
        else
            pkill -f "ollama serve"
        fi
        log_success "Local Ollama service stopped"
    else
        log_info "Local Ollama service is not running"
    fi

    return 0
}

# Check that Docker is installed
check_docker() {
    log_info "Checking Docker..."

    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Install Docker first"
        return 1
    fi

	# Check for and pick an available Docker Compose command
	if detect_compose_cmd; then
		if [ "$DOCKER_COMPOSE_BIN" = "docker" ]; then
			log_info "Docker Compose plugin found (docker compose)"
		else
			log_info "docker-compose found (v1)"
		fi
	else
		log_error "Docker Compose not found (neither docker compose nor docker-compose). Install one of them."
		return 1
	fi

    # Check that the Docker service is running
    if ! docker info &> /dev/null; then
        log_error "The Docker service is not running. Start it first"
        return 1
    fi

    log_success "Docker check passed"
    return 0
}

check_platform() {
     # Detect the platform
    log_info "Detecting the platform..."
    if [ "$(uname -m)" = "x86_64" ]; then
        export PLATFORM="linux/amd64"
    elif [ "$(uname -m)" = "aarch64" ] || [ "$(uname -m)" = "arm64" ]; then
        export PLATFORM="linux/arm64"
    else
        log_warning "Unrecognized platform: $(uname -m). Using the default platform linux/amd64"
        export PLATFORM="linux/amd64"
    fi
    log_info "Platform: $PLATFORM"
}

# Pre-pull the sandbox image (needed to run Agent Skills; pulled, not started)
ensure_sandbox_image() {
    local sandbox_image="ghcr.io/jrmatherly/ai-kb-sandbox:${AIKB_VERSION:-latest}"

    # Check whether the sandbox image already exists locally
    if docker image inspect "$sandbox_image" &> /dev/null; then
        log_success "Sandbox image ready: $sandbox_image"
        return 0
    fi

    log_info "Sandbox image ($sandbox_image) not found, pulling in the background..."
    log_info "Agent Skills depends on this image. The pull must finish before the first run"

    # Pull in the background so the main flow is not blocked
    (
        if PLATFORM=$PLATFORM "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD pull sandbox 2>/dev/null; then
            log_success "Sandbox image pulled: $sandbox_image"
        else
            log_warning "Sandbox image pull failed. Agent Skills may be unavailable"
            log_warning "Pull it by hand later: $DOCKER_COMPOSE_BIN $DOCKER_COMPOSE_SUBCMD pull sandbox"
        fi
    ) &

    return 0
}

# Start the Docker containers
start_docker() {
    log_info "Starting the Docker containers..."

    # Check Docker
    if ! check_docker; then
        return 1
    fi

    # Check the .env file
    check_env_file

    # Read the .env file
    source_env_file

    check_platform

	# Run docker-compose from the project root
    cd "$PROJECT_ROOT" || return 1

    export_frontend_build_args

    # Start the core services
    log_info "Starting the core service containers..."
	local up_args
	if [ "$NO_PULL" = true ]; then
		log_info "Skipping the image pull, using local images..."
		up_args=(up --build -d)
	else
		log_info "Pulling the latest images..."
		up_args=(up --pull always -d)
	fi
    if ! PLATFORM=$PLATFORM "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD "${up_args[@]}"; then
        log_error "Docker containers failed to start"
        return 1
    fi

    log_success "All Docker containers started"

    # Show container status
    log_info "Current container status:"
	"$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD ps

    # Pre-pull the sandbox image (needed to run Agent Skills; pulled, not started)
    ensure_sandbox_image

    return 0
}

# Stop the Docker containers
stop_docker() {
    log_info "Stopping the Docker containers..."

    # Check Docker
    if ! check_docker; then
        # Try to stop anyway even if the check fails, just in case
        log_warning "Docker check failed, still trying to stop the containers..."
    fi

    # Run docker-compose from the project root
    cd "$PROJECT_ROOT" || return 1

    # Stop all containers
	# Without --profile '*', down leaves every profiled service running.
    if ! "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD --profile '*' down --remove-orphans; then
        log_error "Docker containers failed to stop"
        return 1
    fi

    log_success "All Docker containers stopped"
    return 0
}

# List all running containers
list_containers() {
    log_info "Listing all running containers..."

    # Check Docker
    if ! check_docker; then
        return 1
    fi

    # Run docker-compose from the project root
    cd "$PROJECT_ROOT" || return 1

    # List all containers
    printf "%b\n" "${BLUE}Running containers:${NC}"
	"$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD ps --services | sort

    return 0
}

# Pull the latest Docker images
pull_images() {
    log_info "Pulling the latest Docker images..."

    # Check Docker
    if ! check_docker; then
        return 1
    fi

    # Check the .env file
    check_env_file

    # Read the .env file
    source_env_file

    check_platform

    # Run docker-compose from the project root
    cd "$PROJECT_ROOT" || return 1

    # Pull all images
    log_info "Pulling the latest images for all services..."
    if ! PLATFORM=$PLATFORM "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD pull; then
        log_error "Image pull failed"
        return 1
    fi

    # Pull the sandbox image (sandbox is in a profile, so it needs its own pull)
    log_info "Pulling the sandbox image..."
    PLATFORM=$PLATFORM "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD pull sandbox 2>/dev/null || \
        log_warning "Sandbox image pull failed (optional, skipping)"

    log_success "All images pulled to the latest version"

    # Show the pulled images
    log_info "Pulled images:"
    docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.CreatedAt}}\t{{.Size}}" | head -10

    return 0
}

# Restart the given container
restart_container() {
    local container_name="$1"

    if [ -z "$container_name" ]; then
        log_error "No container name given"
        echo "Available containers:"
        list_containers
        return 1
    fi

    log_info "Rebuilding and restarting container: $container_name"

    # Check Docker
    if ! check_docker; then
        return 1
    fi

    check_platform

    # Run docker-compose from the project root
    cd "$PROJECT_ROOT" || return 1

    export_frontend_build_args

    # Check that the container exists
	if ! "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD ps --services | grep -q "^$container_name$"; then
        log_error "Container '$container_name' does not exist or is not running"
        echo "Available containers:"
        list_containers
        return 1
    fi

    # Build and restart the container
    log_info "Rebuilding container '$container_name'..."
    if ! PLATFORM=$PLATFORM "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD build "$container_name"; then
        log_error "Container '$container_name' build failed"
        return 1
    fi

    log_info "Restarting container '$container_name'..."
    if ! PLATFORM=$PLATFORM "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD up -d --no-deps "$container_name"; then
        log_error "Container '$container_name' restart failed"
        return 1
    fi

    log_success "Container '$container_name' rebuilt and restarted"
    return 0
}

# Check the system environment
check_environment() {
    log_info "Starting the environment check..."

    # Check the operating system
    OS=$(uname)
    log_info "Operating system: $OS"

    # Check Docker
    check_docker

    # Check the .env file
    check_env_file

    get_ollama_base_url

    if [ $IS_REMOTE -eq 1 ]; then
        log_info "Remote Ollama service configured"
        if curl -s "$OLLAMA_URL/api/tags" &> /dev/null; then
            version=$(curl -s "$OLLAMA_URL/api/version" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
            log_success "Remote Ollama service is reachable, version: $version"
        else
            log_warning "Remote Ollama service is unreachable. Check that the address is right and the service is running"
        fi
    else
        if command -v ollama &> /dev/null; then
            log_success "Local Ollama is installed"
            if curl -s "http://localhost:$OLLAMA_PORT/api/tags" &> /dev/null; then
                version=$(curl -s "http://localhost:$OLLAMA_PORT/api/version" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
                log_success "Local Ollama service is running, version: $version"
            else
                log_warning "Local Ollama is installed but the service is not running"
            fi
        else
            log_warning "Local Ollama is not installed"
        fi
    fi

    # Check the sandbox image
    log_info "Checking the sandbox image..."
    local sandbox_image="ghcr.io/jrmatherly/ai-kb-sandbox:${AIKB_VERSION:-latest}"
    if docker image inspect "$sandbox_image" &> /dev/null; then
        log_success "Sandbox image ready: $sandbox_image"
    else
        log_warning "Sandbox image not found: $sandbox_image (Agent Skills needs this image)"
        log_info "Pull it with: $0 -p or docker pull $sandbox_image"
    fi

    # Check disk space
    log_info "Checking disk space..."
    df -h | grep -E "(Filesystem|/$)"

    # Check memory
    log_info "Checking memory usage..."
    if [ "$OS" = "Darwin" ]; then
        vm_stat | perl -ne '/page size of (\d+)/ and $size=$1; /Pages free:\s*(\d+)/ and print "Free Memory: ", $1 * $size / 1048576, " MB\n"'
    else
        free -h | grep -E "(total|Mem:)"
    fi

    # Check the CPU
    log_info "CPU info:"
    if [ "$OS" = "Darwin" ]; then
        sysctl -n machdep.cpu.brand_string
        echo "CPU cores: $(sysctl -n hw.ncpu)"
    else
        grep "model name" /proc/cpuinfo | head -1
        echo "CPU cores: $(nproc)"
    fi

    # Check container status
    log_info "Checking container status..."
    if docker info &> /dev/null; then
        docker ps -a
    else
        log_warning "Cannot get container status. Docker may not be running"
    fi

    log_success "Environment check complete"
    return 0
}

# Parse command-line arguments
START_OLLAMA=false
START_DOCKER=false
STOP_SERVICES=false
CHECK_ENVIRONMENT=false
LIST_CONTAINERS=false
RESTART_CONTAINER=false
PULL_IMAGES=false
NO_PULL=false
CONTAINER_NAME=""

# Start all services when no arguments are given
if [ $# -eq 0 ]; then
    START_OLLAMA=true
    START_DOCKER=true
fi

while [ "$1" != "" ]; do
    case $1 in
        -h | --help )       show_help
                            ;;
        -o | --ollama )     START_OLLAMA=true
                            ;;
        -d | --docker )     START_DOCKER=true
                            ;;
        -a | --all )        START_OLLAMA=true
                            START_DOCKER=true
                            ;;
        -s | --stop )       STOP_SERVICES=true
                            ;;
        -c | --check )      CHECK_ENVIRONMENT=true
                            ;;
        -l | --list )       LIST_CONTAINERS=true
                            ;;
        -p | --pull )       PULL_IMAGES=true
                            ;;
        --no-pull )         NO_PULL=true
                            START_OLLAMA=true
                            START_DOCKER=true
                            ;;
        -r | --restart )    RESTART_CONTAINER=true
                            CONTAINER_NAME="$2"
                            shift
                            ;;
        -v | --version )    show_version
                            ;;
        * )                 log_error "Unknown option: $1"
                            show_help 1
                            ;;
    esac
    shift
done

# Run the environment check
if [ "$CHECK_ENVIRONMENT" = true ]; then
    check_environment
    exit $?
fi

# List all containers
if [ "$LIST_CONTAINERS" = true ]; then
    list_containers
    exit $?
fi

# Pull the latest images
if [ "$PULL_IMAGES" = true ]; then
    pull_images
    exit $?
fi

# Restart the given container
if [ "$RESTART_CONTAINER" = true ]; then
    restart_container "$CONTAINER_NAME"
    exit $?
fi

# Run the service operation
if [ "$STOP_SERVICES" = true ]; then
    # Stop the services
    stop_ollama
    OLLAMA_RESULT=$?

    stop_docker
    DOCKER_RESULT=$?

    # Summary
    echo ""
    log_info "=== Stop results ==="
    if [ $OLLAMA_RESULT -eq 0 ]; then
        log_success "✓ Ollama service stopped"
    else
        log_error "✗ Ollama service failed to stop"
    fi

    if [ $DOCKER_RESULT -eq 0 ]; then
        log_success "✓ Docker containers stopped"
    else
        log_error "✗ Docker containers failed to stop"
    fi

    log_success "Services stopped."
    [ $OLLAMA_RESULT -eq 0 ] && [ $DOCKER_RESULT -eq 0 ] || exit 1
else
    # Start the services
    OLLAMA_RESULT=1
    DOCKER_RESULT=1
    if [ "$START_OLLAMA" = true ]; then
        start_ollama
        OLLAMA_RESULT=$?
    fi

    if [ "$START_DOCKER" = true ]; then
        start_docker
        DOCKER_RESULT=$?
    fi

    # Summary
    echo ""
    log_info "=== Start results ==="
    if [ "$START_OLLAMA" = true ]; then
        if [ $OLLAMA_RESULT -eq 0 ]; then
            log_success "✓ Ollama service started"
        else
            log_error "✗ Ollama service failed to start"
        fi
    fi

    if [ "$START_DOCKER" = true ]; then
        if [ $DOCKER_RESULT -eq 0 ]; then
            log_success "✓ Docker containers started"
        else
            log_error "✗ Docker containers failed to start"
        fi
    fi

    if [ "$START_OLLAMA" = true ] && [ "$START_DOCKER" = true ]; then
        if [ $OLLAMA_RESULT -eq 0 ] && [ $DOCKER_RESULT -eq 0 ]; then
            log_success "All services started. Open them at:"
            printf "%b\n" "${GREEN}  - Web UI: http://localhost:${FRONTEND_PORT:-80}${NC}"
            printf "%b\n" "${GREEN}  - API: http://localhost:${APP_PORT:-8080}${NC}"
            echo ""
            log_info "Streaming container logs (Ctrl+C exits the logs, the containers keep running)..."
            "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD logs app docreader postgres --since=10s -f
        else
            log_error "Some services failed to start. Check the logs and fix the problems"
        fi
    elif [ "$START_OLLAMA" = true ] && [ $OLLAMA_RESULT -eq 0 ]; then
        log_success "Ollama service started. Open it at:"
        printf "%b\n" "${GREEN}  - Ollama API: http://localhost:$OLLAMA_PORT${NC}"
    elif [ "$START_DOCKER" = true ] && [ $DOCKER_RESULT -eq 0 ]; then
        log_success "Docker containers started. Open them at:"
        printf "%b\n" "${GREEN}  - Web UI: http://localhost:${FRONTEND_PORT:-80}${NC}"
        printf "%b\n" "${GREEN}  - API: http://localhost:${APP_PORT:-8080}${NC}"
        echo ""
        log_info "Streaming container logs (Ctrl+C exits the logs, the containers keep running)..."
        "$DOCKER_COMPOSE_BIN" $DOCKER_COMPOSE_SUBCMD logs app docreader postgres --since=10s -f
    fi
fi

if { [ "$START_OLLAMA" = true ] && [ $OLLAMA_RESULT -ne 0 ]; } || { [ "$START_DOCKER" = true ] && [ $DOCKER_RESULT -ne 0 ]; }; then
    exit 1
fi
exit 0
