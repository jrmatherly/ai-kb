#!/bin/bash
# Check the development environment configuration

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # no color

# Project root
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

log_info() {
    printf "%b\n" "${BLUE}[INFO]${NC} $1"
}

log_success() {
    printf "%b\n" "${GREEN}[✓]${NC} $1"
}

log_error() {
    printf "%b\n" "${RED}[✗]${NC} $1"
}

log_warning() {
    printf "%b\n" "${YELLOW}[!]${NC} $1"
}

echo ""
printf "%b\n" "${GREEN}========================================${NC}"
printf "%b\n" "${GREEN}  AI KB development environment check${NC}"
printf "%b\n" "${GREEN}========================================${NC}"
echo ""

cd "$PROJECT_ROOT"

# Check the .env file
log_info "Checking the .env file..."
if [ -f ".env" ]; then
    log_success ".env file exists"
else
    log_error ".env file not found"
    echo ""
    log_info "Fix:"
    echo "  1. Copy the example file: cp .env.example .env"
    echo "  2. Edit .env and set the required environment variables"
    exit 1
fi

echo ""
log_info "Checking required environment variables..."

# Load the .env file
set -a
source .env
set +a

# Check required environment variables
errors=0

check_var() {
    local var_name=$1
    local var_value="${!var_name}"
    
    if [ -z "$var_value" ]; then
        log_error "$var_name is not set"
        errors=$((errors + 1))
    else
        log_success "$var_name = $var_value"
    fi
}

# Database configuration
log_info "Database configuration:"
check_var "DB_DRIVER"
check_var "DB_HOST"
check_var "DB_PORT"
check_var "DB_USER"
check_var "DB_PASSWORD"
check_var "DB_NAME"

echo ""
log_info "Storage configuration:"
check_var "STORAGE_TYPE"

if [ "$STORAGE_TYPE" = "minio" ]; then
    check_var "MINIO_BUCKET_NAME"
fi

if [ "$STORAGE_TYPE" = "tos" ]; then
    check_var "TOS_ENDPOINT"
    check_var "TOS_REGION"
    check_var "TOS_ACCESS_KEY"
    check_var "TOS_SECRET_KEY"
    check_var "TOS_BUCKET_NAME"
fi

if [ "$STORAGE_TYPE" = "s3" ]; then
    check_var "S3_REGION"
    check_var "S3_BUCKET_NAME"
fi

echo ""
log_info "Redis configuration:"
check_var "REDIS_ADDR"

echo ""
log_info "Ollama configuration:"
check_var "OLLAMA_BASE_URL"

echo ""
log_info "Model configuration:"
if [ -n "$INIT_LLM_MODEL_NAME" ]; then
    log_success "INIT_LLM_MODEL_NAME = $INIT_LLM_MODEL_NAME"
else
    log_warning "INIT_LLM_MODEL_NAME is not set (optional)"
fi

if [ -n "$INIT_EMBEDDING_MODEL_NAME" ]; then
    log_success "INIT_EMBEDDING_MODEL_NAME = $INIT_EMBEDDING_MODEL_NAME"
else
    log_warning "INIT_EMBEDDING_MODEL_NAME is not set (optional)"
fi

# Check Go
echo ""
log_info "Checking Go..."
if command -v go &> /dev/null; then
    go_version=$(go version)
    log_success "Go installed: $go_version"
else
    log_error "Go is not installed"
    errors=$((errors + 1))
fi

# Check Air
if command -v air &> /dev/null; then
    log_success "Air installed (hot reload supported)"
else
    log_warning "Air is not installed (optional, used for hot reload)"
    log_info "Install with: go install github.com/air-verse/air@latest"
fi

# Check npm
echo ""
log_info "Checking Node.js..."
if command -v npm &> /dev/null; then
    npm_version=$(npm --version)
    log_success "npm installed: $npm_version"
else
    log_error "npm is not installed"
    errors=$((errors + 1))
fi

# Check Docker
echo ""
log_info "Checking Docker..."
if command -v docker &> /dev/null; then
    docker_version=$(docker --version)
    log_success "Docker installed: $docker_version"
    
    if docker info &> /dev/null; then
        log_success "Docker service is running"
    else
        log_error "Docker service is not running"
        errors=$((errors + 1))
    fi
else
    log_error "Docker is not installed"
    errors=$((errors + 1))
fi

# Check Docker Compose
if docker compose version &> /dev/null; then
    compose_version=$(docker compose version)
    log_success "Docker Compose installed: $compose_version"
elif command -v docker-compose &> /dev/null; then
    compose_version=$(docker-compose --version)
    log_success "docker-compose installed: $compose_version"
else
    log_error "Docker Compose is not installed"
    errors=$((errors + 1))
fi

# Summary
echo ""
printf "%b\n" "${GREEN}========================================${NC}"
if [ $errors -eq 0 ]; then
    log_success "All checks passed. The environment is configured"
    echo ""
    log_info "Next steps:"
    echo "  1. Start the dev environment: make dev-start"
    echo "  2. Start the backend: make dev-app"
    echo "  3. Start the frontend: make dev-frontend"
else
    log_error "Found $errors problem(s). Fix them before starting the dev environment"
    echo ""
    log_info "Common issues:"
    echo "  - If .env is missing, copy .env.example"
    echo "  - Make sure DB_DRIVER is set to 'postgres' or 'mysql'"
    echo "  - Make sure the Docker service is running"
fi
printf "%b\n" "${GREEN}========================================${NC}"
echo ""

exit $errors
