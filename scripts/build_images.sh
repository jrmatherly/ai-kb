#!/bin/bash
# Builds every AI KB Docker image from source

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # no color

# Project root (the parent of the script directory)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

# Enable BuildKit
export DOCKER_BUILDKIT=1

# Version information
VERSION="1.0.0"
SCRIPT_NAME=$(basename "$0")

# Show help
show_help() {
    echo -e "${GREEN}AI KB image build script v${VERSION}${NC}"
    echo -e "${GREEN}Usage:${NC} $0 [options]"
    echo "Options:"
    echo "  -h, --help     show help"
    echo "  -a, --all      build all images (default)"
    echo "  -p, --app      build only the app image"
    echo "  -d, --docreader build only the docreader image"
    echo "  -f, --frontend build only the frontend image"
    echo "  -s, --sandbox  build only the sandbox image"
    echo "  -c, --clean    remove all local images"
    echo "  -v, --version  show version information"
    exit 0
}

# Show version information
show_version() {
    echo -e "${GREEN}AI KB image build script v${VERSION}${NC}"
    exit 0
}

# Logging helpers
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Check that Docker is installed
check_docker() {
    log_info "Checking Docker..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Install Docker first"
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

# Detect the platform
check_platform() {
    log_info "Detecting the platform..."
    if [ "$(uname -m)" = "x86_64" ]; then
        export PLATFORM="linux/amd64"
        export TARGETARCH="amd64"
    elif [ "$(uname -m)" = "aarch64" ] || [ "$(uname -m)" = "arm64" ]; then
        export PLATFORM="linux/arm64"
        export TARGETARCH="arm64"
    else
        log_warning "Unrecognized platform: $(uname -m). Using the default platform linux/amd64"
        export PLATFORM="linux/amd64"
        export TARGETARCH="amd64"
    fi
    log_info "Platform: $PLATFORM"
    log_info "Architecture: $TARGETARCH"
}

# Version information
get_version_info() {
    # Read the version from the VERSION file
    if [ -f "VERSION" ]; then
        VERSION=$(cat VERSION | tr -d '\n\r')
    else
        VERSION="unknown"
    fi
    
    # Commit ID
    if command -v git >/dev/null 2>&1; then
        COMMIT_ID=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    else
        COMMIT_ID="unknown"
    fi
    
    # Build time
    BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S UTC')
    
    # Go version
    if command -v go >/dev/null 2>&1; then
        GO_VERSION=$(go version 2>/dev/null || echo "unknown")
    else
        GO_VERSION="unknown"
    fi
    
    log_info "Version: $VERSION"
    log_info "Commit ID: $COMMIT_ID"
    log_info "Build time: $BUILD_TIME"
    log_info "Go version: $GO_VERSION"
}

# Build the app image
build_app_image() {
    log_info "Building the app image (ai-kb-app)..."
    
    cd "$PROJECT_ROOT"
    
    # Version information
    get_version_info
    
    docker build \
        --platform $PLATFORM \
        --build-arg GOPRIVATE_ARG=${GOPRIVATE:-""} \
        --build-arg GOPROXY_ARG="${GOPROXY:-}" \
        --build-arg GOSUMDB_ARG=${GOSUMDB:-"off"} \
        --build-arg APK_MIRROR_ARG="${APK_MIRROR_ARG:-}" \
        --build-arg VERSION_ARG="$VERSION" \
        --build-arg COMMIT_ID_ARG="$COMMIT_ID" \
        --build-arg BUILD_TIME_ARG="$BUILD_TIME" \
        --build-arg GO_VERSION_ARG="$GO_VERSION" \
        --build-arg WITH_ANYDOC=${WITH_ANYDOC:-1} \
        -f docker/Dockerfile.app \
        -t ghcr.io/jrmatherly/ai-kb-app:latest \
        .
    
    if [ $? -eq 0 ]; then
        log_success "App image built"
        return 0
    else
        log_error "App image build failed"
        return 1
    fi
}

# Build the docreader image
build_docreader_image() {
    log_info "Building the docreader image (ai-kb-docreader)..."
    
    cd "$PROJECT_ROOT"
    
    docker build \
        --platform $PLATFORM \
        --build-arg PLATFORM=$PLATFORM \
        --build-arg TARGETARCH=$TARGETARCH \
        --build-arg APT_MIRROR="${APT_MIRROR:-}" \
        --build-arg PLAYWRIGHT_DOWNLOAD_HOST="${PLAYWRIGHT_DOWNLOAD_HOST:-}" \
        --build-arg PLAYWRIGHT_WEBKIT_DOWNLOAD_HOST="${PLAYWRIGHT_WEBKIT_DOWNLOAD_HOST:-}" \
        -f docker/Dockerfile.docreader \
        -t ghcr.io/jrmatherly/ai-kb-docreader:latest \
        .
    
    if [ $? -eq 0 ]; then
        log_success "Docreader image built"
        return 0
    else
        log_error "Docreader image build failed"
        return 1
    fi
}

# Build the frontend image (multi-stage: npm runs in the builder, so the host needs no prebuilt dist)
build_frontend_image() {
    log_info "Building the frontend image (ai-kb-ui)..."
    
    cd "$PROJECT_ROOT"
    
    # Version information (to inject the commit hash into the frontend)
    get_version_info

    docker build \
        --platform $PLATFORM \
        --build-arg VITE_FRONTEND_COMMIT="$COMMIT_ID" \
        ${NPM_REGISTRY:+--build-arg NPM_REGISTRY="$NPM_REGISTRY"} \
        ${NODE_MAX_OLD_SPACE_SIZE:+--build-arg NODE_MAX_OLD_SPACE_SIZE="$NODE_MAX_OLD_SPACE_SIZE"} \
        -f frontend/Dockerfile \
        -t ghcr.io/jrmatherly/ai-kb-ui:latest \
        frontend/
    
    if [ $? -eq 0 ]; then
        log_success "Frontend image built"
        return 0
    else
        log_error "Frontend image build failed"
        return 1
    fi
}

# Build the sandbox image
build_sandbox_image() {
    log_info "Building the sandbox image (ai-kb-sandbox)..."

    cd "$PROJECT_ROOT"

    # Also tag main: the default image tracks main (see DefaultDockerImage in sandbox.go),
    # and the Docker backend pulls only when the image is missing locally, so a build without this tag is wasted.
    docker build \
        --platform $PLATFORM \
        --build-arg TARGETPLATFORM=$PLATFORM \
        -f docker/Dockerfile.sandbox \
        --target sandbox \
        -t ghcr.io/jrmatherly/ai-kb-sandbox:latest \
        -t ghcr.io/jrmatherly/ai-kb-sandbox:main \
        .

    if [ $? -ne 0 ]; then
        log_error "Sandbox image build failed"
        return 1
    fi

    # Cube builds its template straight from the image and probes :49983/health, which always fails without envd,
    # so Cube uses a variant image with envd injected. See website-docs/06-development/04-sandbox-deployment.md.
    log_info "Building the sandbox image Cube variant (ai-kb-sandbox:main-cube)..."

    docker build \
        --platform $PLATFORM \
        --build-arg TARGETPLATFORM=$PLATFORM \
        -f docker/Dockerfile.sandbox \
        --target cube \
        -t ghcr.io/jrmatherly/ai-kb-sandbox:latest-cube \
        -t ghcr.io/jrmatherly/ai-kb-sandbox:main-cube \
        .

    if [ $? -ne 0 ]; then
        log_error "Sandbox image Cube variant build failed"
        return 1
    fi

    # Desktop variant: XFCE + x11vnc + websockify. Tagged for E2B template
    # builds; the Docker backend does not consume this image yet.
    log_info "Building the sandbox image desktop variant (ai-kb-sandbox:main-desktop)..."

    docker build \
        --platform $PLATFORM \
        --build-arg TARGETPLATFORM=$PLATFORM \
        -f docker/Dockerfile.sandbox \
        --target desktop \
        -t ghcr.io/jrmatherly/ai-kb-sandbox:latest-desktop \
        -t ghcr.io/jrmatherly/ai-kb-sandbox:main-desktop \
        .

    if [ $? -ne 0 ]; then
        log_error "Sandbox image desktop variant build failed"
        return 1
    fi

    log_info "Building the sandbox image desktop Cube variant (ai-kb-sandbox:main-desktop-cube)..."

    docker build \
        --platform $PLATFORM \
        --build-arg TARGETPLATFORM=$PLATFORM \
        -f docker/Dockerfile.sandbox \
        --target desktop-cube \
        -t ghcr.io/jrmatherly/ai-kb-sandbox:latest-desktop-cube \
        -t ghcr.io/jrmatherly/ai-kb-sandbox:main-desktop-cube \
        .

    if [ $? -eq 0 ]; then
        log_success "Sandbox images built"
        return 0
    else
        log_error "Sandbox image desktop Cube variant build failed"
        return 1
    fi
}

# Build all images
build_all_images() {
    log_info "Building all images..."

    local app_result=0
    local docreader_result=0
    local frontend_result=0
    local sandbox_result=0

    # App image
    build_app_image
    app_result=$?

    # Docreader image
    build_docreader_image
    docreader_result=$?

    # Frontend image
    build_frontend_image
    frontend_result=$?

    # Sandbox image
    build_sandbox_image
    sandbox_result=$?

    # Build results
    echo ""
    log_info "=== Build results ==="
    if [ $app_result -eq 0 ]; then
        log_success "✓ App image built"
    else
        log_error "✗ App image build failed"
    fi

    if [ $docreader_result -eq 0 ]; then
        log_success "✓ Docreader image built"
    else
        log_error "✗ Docreader image build failed"
    fi

    if [ $frontend_result -eq 0 ]; then
        log_success "✓ Frontend image built"
    else
        log_error "✗ Frontend image build failed"
    fi

    if [ $sandbox_result -eq 0 ]; then
        log_success "✓ Sandbox image built"
    else
        log_error "✗ Sandbox image build failed"
    fi

    if [ $app_result -eq 0 ] && [ $docreader_result -eq 0 ] && [ $frontend_result -eq 0 ] && [ $sandbox_result -eq 0 ]; then
        log_success "All images built"
        return 0
    else
        log_error "Some images failed to build"
        return 1
    fi
}

# Remove local images
clean_images() {
    log_info "Removing local AI KB images..."
    
    # Stop the related containers
    log_info "Stopping the related containers..."
    docker stop $(docker ps -q --filter "ancestor=ghcr.io/jrmatherly/ai-kb-app:latest" 2>/dev/null) 2>/dev/null || true
    docker stop $(docker ps -q --filter "ancestor=ghcr.io/jrmatherly/ai-kb-docreader:latest" 2>/dev/null) 2>/dev/null || true
    docker stop $(docker ps -q --filter "ancestor=ghcr.io/jrmatherly/ai-kb-ui:latest" 2>/dev/null) 2>/dev/null || true
    
    # Remove the related containers
    log_info "Removing the related containers..."
    docker rm $(docker ps -aq --filter "ancestor=ghcr.io/jrmatherly/ai-kb-app:latest" 2>/dev/null) 2>/dev/null || true
    docker rm $(docker ps -aq --filter "ancestor=ghcr.io/jrmatherly/ai-kb-docreader:latest" 2>/dev/null) 2>/dev/null || true
    docker rm $(docker ps -aq --filter "ancestor=ghcr.io/jrmatherly/ai-kb-ui:latest" 2>/dev/null) 2>/dev/null || true
    
    # Remove the images
    log_info "Removing local images..."
    docker rmi ghcr.io/jrmatherly/ai-kb-app:latest 2>/dev/null || true
    docker rmi ghcr.io/jrmatherly/ai-kb-docreader:latest 2>/dev/null || true
    docker rmi ghcr.io/jrmatherly/ai-kb-ui:latest 2>/dev/null || true
    docker rmi ghcr.io/jrmatherly/ai-kb-sandbox:latest 2>/dev/null || true
    docker rmi ghcr.io/jrmatherly/ai-kb-sandbox:latest-cube 2>/dev/null || true
    docker rmi ghcr.io/jrmatherly/ai-kb-sandbox:latest-desktop 2>/dev/null || true
    docker rmi ghcr.io/jrmatherly/ai-kb-sandbox:latest-desktop-cube 2>/dev/null || true
    docker rmi ghcr.io/jrmatherly/ai-kb-sandbox:main 2>/dev/null || true
    docker rmi ghcr.io/jrmatherly/ai-kb-sandbox:main-cube 2>/dev/null || true
    docker rmi ghcr.io/jrmatherly/ai-kb-sandbox:main-desktop 2>/dev/null || true
    docker rmi ghcr.io/jrmatherly/ai-kb-sandbox:main-desktop-cube 2>/dev/null || true
    
    docker image prune -f
    
    log_success "Images removed"
    return 0
}

# Parse command-line arguments
BUILD_ALL=false
BUILD_APP=false
BUILD_DOCREADER=false
BUILD_FRONTEND=false
BUILD_SANDBOX=false
CLEAN_IMAGES=false

# Build all images when no arguments are given
if [ $# -eq 0 ]; then
    BUILD_ALL=true
fi

while [ "$1" != "" ]; do
    case $1 in
        -h | --help )       show_help
                            ;;
        -a | --all )        BUILD_ALL=true
                            ;;
        -p | --app )        BUILD_APP=true
                            ;;
        -d | --docreader )  BUILD_DOCREADER=true
                            ;;
        -f | --frontend )   BUILD_FRONTEND=true
                            ;;
        -s | --sandbox )    BUILD_SANDBOX=true
                            ;;
        -c | --clean )      CLEAN_IMAGES=true
                            ;;
        -v | --version )    show_version
                            ;;
        * )                 log_error "Unknown option: $1"
                            show_help
                            ;;
    esac
    shift
done

# Check Docker
check_docker
if [ $? -ne 0 ]; then
    exit 1
fi

# Detect the platform
check_platform

# Run the cleanup
if [ "$CLEAN_IMAGES" = true ]; then
    clean_images
    exit $?
fi

# Run the build
if [ "$BUILD_ALL" = true ]; then
    build_all_images
    exit $?
fi

if [ "$BUILD_APP" = true ]; then
    build_app_image
    exit $?
fi

if [ "$BUILD_DOCREADER" = true ]; then
    build_docreader_image
    exit $?
fi

if [ "$BUILD_FRONTEND" = true ]; then
    build_frontend_image
    exit $?
fi

if [ "$BUILD_SANDBOX" = true ]; then
    build_sandbox_image
    exit $?
fi

exit 0
