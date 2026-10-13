.PHONY: help build run test clean docker-build-app docker-build-docreader docker-build-frontend docker-build-all docker-run migrate-up migrate-down docker-restart docker-stop start-all stop-all start-ollama build-images build-images-app build-images-docreader build-images-frontend clean-images check-env list-containers pull-images show-platform dev-start dev-stop dev-restart dev-logs dev-status dev-app dev-frontend docs install-swagger build-lite run-lite package-lite anydoc-lib build-anydoc

# Show help
help:
	@echo "AI KB Makefile help"
	@echo ""
	@echo "Basic commands:"
	@echo "  build             Build the application"
	@echo "  run               Run the application"
	@echo "  test              Run tests"
	@echo "  anydoc-lib        Build the anydoc static library (needs the Rust toolchain)"
	@echo "  build-anydoc      Build the application with the anydoc parsing engine"
	@echo "  clean             Remove build files"
	@echo ""
	@echo "Docker commands:"
	@echo "  docker-build-app       Build the application Docker image (ghcr.io/jrmatherly/ai-kb-app)"
	@echo "  docker-build-docreader Build the document reader image (ghcr.io/jrmatherly/ai-kb-docreader)"
	@echo "  docker-build-frontend  Build the frontend image (ghcr.io/jrmatherly/ai-kb-ui)"
	@echo "  docker-build-all       Build all Docker images"
	@echo "  docker-run            Run the Docker containers"
	@echo "  docker-stop           Stop the Docker containers"
	@echo "  docker-restart        Restart the Docker containers"
	@echo ""
	@echo "Service management:"
	@echo "  start-all         Start all services"
	@echo "  stop-all          Stop all services"
	@echo "  start-ollama      Start only the Ollama service"
	@echo ""
	@echo "Image builds:"
	@echo "  build-images      Build all images from source"
	@echo "  build-images-app  Build the application image from source"
	@echo "  build-images-docreader Build the document reader image from source"
	@echo "  build-images-frontend  Build the frontend image from source"
	@echo "  clean-images      Remove local images"
	@echo ""
	@echo "Database:"
	@echo "  migrate-up        Run database migrations"
	@echo "  migrate-down      Roll back database migrations"
	@echo ""
	@echo "Developer tools:"
	@echo "  fmt               Format code"
	@echo "  lint              Lint code"
	@echo "  deps              Install dependencies"
	@echo "  docs              Generate the Swagger API docs"
	@echo "  install-swagger   Install the swag tool"
	@echo ""
	@echo "Model vendor catalog:"
	@echo "  model-catalog-check   Check the vendor catalog (invariants + old/new behavior comparison + vendor tests)"
	@echo "  model-catalog-diff    Compare with models.dev and report model metadata differences (needs human review)"
	@echo "                        Optional: make model-catalog-diff VENDOR=deepseek"
	@echo ""
	@echo "Environment checks:"
	@echo "  check-env         Check the environment configuration"
	@echo "  list-containers   List running containers"
	@echo "  pull-images       Pull the latest images"
	@echo "  show-platform     Show the current build platform"
	@echo ""
	@echo "Development mode (recommended):"
	@echo "  dev-start         Start the development infrastructure (dependency services only)"
	@echo "                    Optional: make dev-start DEV_ARGS=--odl-hybrid"
	@echo "  dev-stop          Stop the development environment"
	@echo "  dev-restart       Restart the development environment"
	@echo "  dev-logs          Show development environment logs"
	@echo "  dev-status        Show development environment status"
	@echo "  dev-app           Start the backend locally (run dev-start first)"
	@echo "                    Links the anydoc engine automatically after make anydoc-lib"
	@echo "  dev-frontend      Start the frontend locally (run dev-start first)"
	@echo ""
	@echo "Lite mode (no external dependencies):"
	@echo "  build-lite        Build the Lite version (frontend into web/ first, then Go; SKIP_FRONTEND=1 skips the frontend)"
	@echo "  run-lite          Build and start the Lite version"
	@echo "  package-lite      Build and package the Lite release (tarball)"
	@echo "  package-mac-app   Build and package the macOS desktop app (.app)"

# Go related variables
BINARY_NAME=ai-kb
MAIN_PATH=./cmd/server

# Docker related variables
DOCKER_IMAGE=ghcr.io/jrmatherly/ai-kb-app
DOCKER_TAG=latest

# Platform detection
ifeq ($(shell uname -m),x86_64)
    PLATFORM=linux/amd64
else ifeq ($(shell uname -m),aarch64)
    PLATFORM=linux/arm64
else ifeq ($(shell uname -m),arm64)
    PLATFORM=linux/arm64
else
    PLATFORM=linux/amd64
endif

# Build the application
build:
	go build -o $(BINARY_NAME) $(MAIN_PATH)

# Build the anydoc static archive (Rust) that the `anydoc` build tag links.
# Override the platform with TARGET=<rust-target-triple>.
anydoc-lib:
	./scripts/build-anydoc-lib.sh

# Build the application with the in-process anydoc parser engine linked in.
build-anydoc: anydoc-lib
	go build -tags anydoc -o $(BINARY_NAME) $(MAIN_PATH)

# Run the application
run: build
	./$(BINARY_NAME)

# Run tests
test:
	go test -v ./...

# Generate reviewed metadata + protocol overrides, then verify every model.
.PHONY: model-catalog-generate model-catalog-check
model-catalog-generate:
	python3 scripts/model-catalog/generate.py

model-catalog-check:
	python3 scripts/model-catalog/generate.py --check
	go test ./internal/models/...

# Vendor catalog: report where our model metadata differs from models.dev.
# Development aid only: nothing is fetched at runtime and nothing is written
# automatically; review each line against the vendor's own documentation.
.PHONY: model-catalog-diff
model-catalog-diff:
	@python3 scripts/model_catalog_diff.py $(if $(VENDOR),--vendor $(VENDOR),)

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)

# Build Docker image
docker-build-app:
	@echo "Getting version info..."
	@eval $$(./scripts/get_version.sh env); \
	./scripts/get_version.sh info; \
	docker build --platform $(PLATFORM) \
		--build-arg VERSION_ARG="$$VERSION" \
		--build-arg COMMIT_ID_ARG="$$COMMIT_ID" \
		--build-arg BUILD_TIME_ARG="$$BUILD_TIME" \
		--build-arg GO_VERSION_ARG="$$GO_VERSION" \
		--build-arg WITH_ANYDOC=$${WITH_ANYDOC:-1} \
		-f docker/Dockerfile.app -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Build docreader Docker image
docker-build-docreader:
	docker build --platform $(PLATFORM) -f docker/Dockerfile.docreader -t ghcr.io/jrmatherly/ai-kb-docreader:latest .

# Build frontend Docker image (multi-stage: npm runs inside the builder stage)
docker-build-frontend:
	@eval $$(./scripts/get_version.sh env); \
	docker build --platform $(PLATFORM) \
		--build-arg VITE_FRONTEND_COMMIT="$$COMMIT_ID" \
		-f frontend/Dockerfile -t ghcr.io/jrmatherly/ai-kb-ui:latest frontend/

# Build all Docker images
docker-build-all: docker-build-app docker-build-docreader docker-build-frontend

# Run Docker container (legacy method)
# Touch .env if missing: docker-compose.yml's `env_file: [.env]` is required
# for ${ENV} interpolation in builtin_models.yaml and would otherwise refuse
# to parse on fresh clones. `start-all` handles this via check_env_file; this
# direct path needs its own guard.
docker-run:
	@[ -f .env ] || ([ -f .env.example ] && cp .env.example .env || touch .env)
	docker-compose up

# Start all services with the new script
start-all:
	./scripts/start_all.sh

# Start only the Ollama service with the new script
start-ollama:
	./scripts/start_all.sh --ollama

# Start only the Docker containers with the new script
start-docker:
	./scripts/start_all.sh --docker

# Stop all services with the new script
stop-all:
	./scripts/start_all.sh --stop

# Stop Docker container (legacy method)
docker-stop:
	docker-compose --profile '*' down

# Build images from source
build-images:
	./scripts/build_images.sh

build-images-app:
	./scripts/build_images.sh --app

build-images-docreader:
	./scripts/build_images.sh --docreader

build-images-frontend:
	./scripts/build_images.sh --frontend

clean-images:
	./scripts/build_images.sh --clean

# Restart Docker container (stop, start)
docker-restart:
	@[ -f .env ] || ([ -f .env.example ] && cp .env.example .env || touch .env)
	docker-compose stop -t 60
	docker-compose up

# Database migrations
migrate-up:
	./scripts/migrate.sh up

migrate-down:
	./scripts/migrate.sh down

migrate-version:
	./scripts/migrate.sh version

migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "Error: migration name is required"; \
		echo "Usage: make migrate-create name=your_migration_name"; \
		exit 1; \
	fi
	./scripts/migrate.sh create $(name)

migrate-force:
	@if [ -z "$(version)" ]; then \
		echo "Error: version is required"; \
		echo "Usage: make migrate-force version=4"; \
		exit 1; \
	fi
	./scripts/migrate.sh force $(version)

migrate-goto:
	@if [ -z "$(version)" ]; then \
		echo "Error: version is required"; \
		echo "Usage: make migrate-goto version=3"; \
		exit 1; \
	fi
	./scripts/migrate.sh goto $(version)

# Generate API documentation (Swagger)
#
# internal/localsandbox is excluded because cmd/server does not depend on it
# (its importers are behind the `desktop` build tag). Without the exclude,
# swag walks into the directory, follows its `const X = core.X` aliases and
# loads internal/localsandbox/core a second time through go/loader; that
# second load re-registers every model under its short name without enum
# values, so definitions lose their package-qualified names and
# docs/swagger_contract_test.go fails.
docs:
	@echo "Generating Swagger API docs..."
	swag init -g $(MAIN_PATH)/main.go -o ./docs --parseDependency --parseInternal \
		--exclude ./internal/localsandbox
	@echo "Docs generated in ./docs"
	@echo "After starting the service, open http://localhost:8080/swagger/index.html to view the docs"

# Install swagger tool
install-swagger:
	go install github.com/swaggo/swag/cmd/swag@v1.16.6

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Install dependencies
deps:
	go mod download

# Build for production
# google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn for qdrant milvus proto conflict
# GO_BUILD_TAGS adds optional build tags, e.g. GO_BUILD_TAGS=anydoc to link the
# in-process office document parser (run `make anydoc-lib` first).
build-prod:
	VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "$${VERSION:-$$(cat VERSION 2>/dev/null || echo unknown)}"); \
	COMMIT_ID=$${COMMIT_ID:-unknown}; \
	BUILD_TIME=$${BUILD_TIME:-unknown}; \
	GO_VERSION=$${GO_VERSION:-unknown}; \
	LDFLAGS="-X 'github.com/jrmatherly/ai-kb/internal/handler.Version=$$VERSION' -X 'github.com/jrmatherly/ai-kb/internal/handler.Edition=standard' -X 'github.com/jrmatherly/ai-kb/internal/handler.CommitID=$$COMMIT_ID' -X 'github.com/jrmatherly/ai-kb/internal/handler.BuildTime=$$BUILD_TIME' -X 'github.com/jrmatherly/ai-kb/internal/handler.GoVersion=$$GO_VERSION' -X 'google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'"; \
	CGO_ENABLED=1 \
	CGO_CFLAGS="-Wno-deprecated-declarations" \
	CGO_LDFLAGS="$$(if [ "$$(uname)" = 'Darwin' ]; then echo '-Wl,-no_warn_duplicate_libraries'; fi)" \
	go build -tags "$(GO_BUILD_TAGS)" -ldflags="-w -s $$LDFLAGS" -o $(BINARY_NAME) $(MAIN_PATH)

# Build Lite version (single binary, SQLite + in-memory queue)
# Builds the frontend into web/ first, then the Go binary; SKIP_FRONTEND=1 skips the frontend
build-lite:
	@if [ -f frontend/package.json ] && [ "$${SKIP_FRONTEND:-}" != "1" ]; then \
		echo ">> Building frontend for Lite..."; \
		(cd frontend && npm ci --prefer-offline && npm run build) && \
		rm -rf web && cp -r frontend/dist web; \
	elif [ "$${SKIP_FRONTEND:-}" = "1" ]; then \
		echo ">> Skipping frontend (SKIP_FRONTEND=1)"; \
	else \
		echo ">> No frontend/package.json, skipping frontend"; \
	fi
	export EDITION=lite; \
	eval "$$(./scripts/get_version.sh env)"; \
	LDFLAGS="$$(./scripts/get_version.sh ldflags) -X 'google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'"; \
	CGO_ENABLED=1 \
	CGO_CFLAGS="-Wno-deprecated-declarations" \
	CGO_LDFLAGS="$$(if [ "$$(uname)" = 'Darwin' ]; then echo '-Wl,-no_warn_duplicate_libraries'; fi)" \
	go build -tags "sqlite_fts5" -ldflags="-w -s $$LDFLAGS" -o $(BINARY_NAME)-lite $(MAIN_PATH)

# Run Lite version with .env.lite defaults
run-lite: build-lite
	@if [ ! -f .env.lite ]; then echo "Error: .env.lite not found"; exit 1; fi
	@set -a && . ./.env.lite && set +a && ./$(BINARY_NAME)-lite

# Package Lite version into distributable tarball
package-lite:
	./scripts/package-lite.sh

# Package Mac App
package-mac-app:
	./scripts/package-mac-app.sh

download_spatial:
	go run cmd/download/duckdb/duckdb.go

# Compose prefixes volume names with the project name: COMPOSE_PROJECT_NAME, else
# the checkout directory name, lowercased with anything outside [a-z0-9_-] dropped.
clean-db:
	@echo "Cleaning database..."
	@project="$${COMPOSE_PROJECT_NAME:-$$(basename "$(CURDIR)" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_-')}"; \
	for v in postgres-data minio_data; do \
		if docker volume inspect "$${project}_$$v" >/dev/null 2>&1; then \
			docker volume rm "$${project}_$$v"; \
		fi; \
	done

# Environment check
check-env:
	./scripts/start_all.sh --check

# List containers
list-containers:
	./scripts/start_all.sh --list

# Pull latest images
pull-images:
	./scripts/start_all.sh --pull

# Show current platform
show-platform:
	@echo "Current system architecture: $(shell uname -m)"
	@echo "Docker build platform: $(PLATFORM)"

# Development mode commands
dev-start:
	./scripts/dev.sh start $(DEV_ARGS)

dev-stop:
	./scripts/dev.sh stop

dev-restart:
	./scripts/dev.sh restart $(DEV_ARGS)

dev-logs:
	./scripts/dev.sh logs

dev-status:
	./scripts/dev.sh status

dev-app:
	./scripts/dev.sh app

dev-frontend:
	./scripts/dev.sh frontend
