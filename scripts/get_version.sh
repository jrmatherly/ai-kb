#!/bin/bash
# Shared script that reports version information
# Works for local builds and CI builds

# Defaults
VERSION="unknown"
EDITION="${EDITION:-standard}"
COMMIT_ID="unknown"
GO_VERSION="unknown"

# Version
if [ -f "VERSION" ]; then
    VERSION=$(cat VERSION | tr -d '\n\r')
fi

# Commit ID
if [ -n "$GITHUB_SHA" ]; then
    # GitHub Actions
    COMMIT_ID="${GITHUB_SHA:0:7}"
elif command -v git >/dev/null 2>&1; then
    # Local
    COMMIT_ID=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
fi

BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S UTC')

# Go version
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version 2>/dev/null || echo "unknown")
fi

# Output format depends on the argument
case "${1:-env}" in
    "env")
        # Environment variable format, escaping values that contain spaces
        echo "VERSION=$VERSION"
        echo "EDITION=$EDITION"
        echo "COMMIT_ID=$COMMIT_ID"
        echo "BUILD_TIME=\"$BUILD_TIME\""
        echo "GO_VERSION=\"$GO_VERSION\""
        ;;
    "json")
        # JSON format
        cat << EOF
{
  "version": "$VERSION",
  "edition": "$EDITION",
  "commit_id": "$COMMIT_ID",
  "build_time": "$BUILD_TIME",
  "go_version": "$GO_VERSION"
}
EOF
        ;;
    "ldflags")
        # Go ldflags format
        echo "-X 'github.com/jrmatherly/ai-kb/internal/handler.Version=$VERSION' -X 'github.com/jrmatherly/ai-kb/internal/handler.Edition=$EDITION' -X 'github.com/jrmatherly/ai-kb/internal/handler.CommitID=$COMMIT_ID' -X 'github.com/jrmatherly/ai-kb/internal/handler.BuildTime=$BUILD_TIME' -X 'github.com/jrmatherly/ai-kb/internal/handler.GoVersion=$GO_VERSION'"
        ;;
    "info")
        # Human-readable format
        echo "Version: $VERSION"
        echo "Edition: $EDITION"
        echo "Commit ID: $COMMIT_ID"
        echo "Build time: $BUILD_TIME"
        echo "Go version: $GO_VERSION"
        ;;
    *)
        echo "Usage: $0 [env|json|ldflags|info]"
        echo "  env        - environment variable format (default)"
        echo "  json       - JSON format"
        echo "  ldflags    - Go ldflags format"
        echo "  info       - human-readable format"
        exit 1
        ;;
esac
