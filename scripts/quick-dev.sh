#!/bin/bash
# One-command quick start for the development environment
# Starts every required service from one terminal

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
    printf "%b\n" "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    printf "%b\n" "${RED}[ERROR]${NC} $1"
}

log_warning() {
    printf "%b\n" "${YELLOW}[WARNING]${NC} $1"
}

echo ""
printf "%b\n" "${GREEN}========================================${NC}"
printf "%b\n" "${GREEN}  AI KB quick dev environment start${NC}"
printf "%b\n" "${GREEN}========================================${NC}"
echo ""

# Make sure we are in the project root
cd "$PROJECT_ROOT"

# 1. Start the infrastructure
log_info "Step 1/3: starting infrastructure services..."
./scripts/dev.sh start
if [ $? -ne 0 ]; then
    log_error "Infrastructure failed to start"
    exit 1
fi

# Wait for the services to be ready
log_info "Waiting for the services to start..."
sleep 5

# 2. Ask whether to start the backend
echo ""
log_info "Step 2/3: start the backend"
printf "%b" "${YELLOW}Start the backend from this terminal? (y/N): ${NC}"
read -r start_backend

if [ "$start_backend" = "y" ] || [ "$start_backend" = "Y" ]; then
    log_info "Starting the backend..."
    # Start the backend in the background
    nohup bash -c 'cd "'$PROJECT_ROOT'" && ./scripts/dev.sh app' > "$PROJECT_ROOT/logs/backend.log" 2>&1 &
    BACKEND_PID=$!
    echo $BACKEND_PID > "$PROJECT_ROOT/tmp/backend.pid"
    log_success "Backend started in the background (PID: $BACKEND_PID)"
    log_info "Backend logs: tail -f $PROJECT_ROOT/logs/backend.log"
else
    log_warning "Skipping the backend"
    log_info "Later, in a new terminal, run: make dev-app or ./scripts/dev.sh app"
fi

# 3. Ask whether to start the frontend
echo ""
log_info "Step 3/3: start the frontend"
printf "%b" "${YELLOW}Start the frontend from this terminal? (y/N): ${NC}"
read -r start_frontend

if [ "$start_frontend" = "y" ] || [ "$start_frontend" = "Y" ]; then
    log_info "Starting the frontend..."
    # Start the frontend in the background
    nohup bash -c 'cd "'$PROJECT_ROOT'/frontend" && npm run dev' > "$PROJECT_ROOT/logs/frontend.log" 2>&1 &
    FRONTEND_PID=$!
    echo $FRONTEND_PID > "$PROJECT_ROOT/tmp/frontend.pid"
    log_success "Frontend started in the background (PID: $FRONTEND_PID)"
    log_info "Frontend logs: tail -f $PROJECT_ROOT/logs/frontend.log"
else
    log_warning "Skipping the frontend"
    log_info "Later, in a new terminal, run: make dev-frontend or ./scripts/dev.sh frontend"
fi

# Summary
echo ""
printf "%b\n" "${GREEN}========================================${NC}"
printf "%b\n" "${GREEN}  Startup complete${NC}"
printf "%b\n" "${GREEN}========================================${NC}"
echo ""

log_info "URLs:"
echo "  - Frontend: http://localhost:5173"
echo "  - Backend API: http://localhost:8080"
echo "  - MinIO Console: http://localhost:9001"
echo ""

log_info "Management commands:"
echo "  - Service status: make dev-status"
echo "  - Logs: make dev-logs"
echo "  - Stop all services: make dev-stop"
echo ""

if [ -f "$PROJECT_ROOT/tmp/backend.pid" ] || [ -f "$PROJECT_ROOT/tmp/frontend.pid" ]; then
    log_warning "Stop the background processes:"
    if [ -f "$PROJECT_ROOT/tmp/backend.pid" ]; then
        echo "  - Stop the backend: kill \$(cat $PROJECT_ROOT/tmp/backend.pid)"
    fi
    if [ -f "$PROJECT_ROOT/tmp/frontend.pid" ]; then
        echo "  - Stop the frontend: kill \$(cat $PROJECT_ROOT/tmp/frontend.pid)"
    fi
fi

echo ""
log_success "The dev environment is ready. Happy coding."
echo ""

