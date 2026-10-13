# Makefile for the AI KB CLI (cli/ Go module).
#
# Usage:
#   make build         compile to ./bin/aikb with version metadata
#   make test          go test ./...
#   make test-coverage same with coverage report
#   make lint          go vet (golangci-lint planned)
#   make tidy          go mod tidy
#   make clean         remove ./bin
#   make help          this list

BINARY      := aikb
BUILD_DIR   := bin
PKG         := github.com/jrmatherly/ai-kb/cli

VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(PKG)/internal/build.Version=$(VERSION) \
	-X $(PKG)/internal/build.Commit=$(COMMIT) \
	-X $(PKG)/internal/build.Date=$(DATE)

.PHONY: build test test-coverage lint tidy clean help

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) .

test:
	go test ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

lint:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR) coverage.out

help:
	@sed -n 's/^#   make /  /p' $(firstword $(MAKEFILE_LIST))
