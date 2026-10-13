#!/bin/bash
set -e

# Get the script directory and project root
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

usage() {
    echo "Usage: $0 {up|down|create <migration_name>|version|force <version>|goto <version>}"
    echo "Connection: DB_URL, or DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME (exported values win over .env)"
}

case "${1:-}" in
    up|down|create|version|force|goto) ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 1 ;;
esac

# Load .env file if it exists (for development mode); variables already exported take precedence
if [ -f "$PROJECT_ROOT/.env" ]; then
    echo "Loading .env file from $PROJECT_ROOT/.env"
    exported=$(export -p)
    set -a
    # The file is a per-checkout runtime file, so there is nothing for shellcheck to follow.
    # shellcheck source=/dev/null
    source "$PROJECT_ROOT/.env"
    set +a
    eval "$exported"
fi

# Database connection details (can be overridden by environment variables)
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-postgres}
DB_NAME=${DB_NAME:-ai_kb}

# Use versioned migrations directory
MIGRATIONS_DIR="${MIGRATIONS_DIR:-$PROJECT_ROOT/migrations/versioned}"

# Check if migrate tool is installed
if ! command -v migrate &> /dev/null; then
    echo "Error: migrate tool is not installed"
    echo "Install the version go.mod pins, from the repository root:"
    echo "  go install -tags postgres \"github.com/golang-migrate/migrate/v4/cmd/migrate@\$(go list -m -f '{{.Version}}' github.com/golang-migrate/migrate/v4)\""
    exit 1
fi

# Construct the database URL
# If DB_URL is already set in .env, use it but ensure sslmode=disable is set
# Otherwise, construct it from individual components
if [ -n "$DB_URL" ]; then
    # If DB_URL already exists, ensure sslmode=disable is set (unless sslmode is already specified)
    if [[ "$DB_URL" != *"sslmode="* ]]; then
        # Add sslmode=disable if not present
        if [[ "$DB_URL" == *"?"* ]]; then
            DB_URL="${DB_URL}&sslmode=disable"
        else
            DB_URL="${DB_URL}?sslmode=disable"
        fi
    elif [[ "$DB_URL" == *"sslmode=require"* ]] || [[ "$DB_URL" == *"sslmode=prefer"* ]]; then
        # Replace sslmode=require/prefer with sslmode=disable for local dev
        DB_URL="${DB_URL//sslmode=require/sslmode=disable}"
        DB_URL="${DB_URL//sslmode=prefer/sslmode=disable}"
    fi
else
    # URL-encode user and password; the values travel as argv, never as Python source
    if command -v python3 &> /dev/null; then
        url_encode() { python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"; }
    else
        echo "Warning: python3 not found, DB_USER and DB_PASSWORD are used without URL encoding"
        url_encode() { printf '%s\n' "$1"; }
    fi
    DB_URL="postgres://$(url_encode "$DB_USER"):$(url_encode "$DB_PASSWORD")@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"
fi

# Execute migration based on command
case "$1" in
    up)
        echo "Running migrations up..."
        echo "DB_URL: $(printf '%s' "$DB_URL" | sed -E 's#(://[^:/@]*:)[^@]*@#\1****@#')"
        echo "DB_USER: ${DB_USER}"
        echo "DB_HOST: ${DB_HOST}"
        echo "DB_PORT: ${DB_PORT}"
        echo "DB_NAME: ${DB_NAME}"
        echo "MIGRATIONS_DIR: ${MIGRATIONS_DIR}"
        migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" up
        ;;
    down)
        echo "Running migrations down..."
        migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" down
        ;;
    create)
        if [ -z "$2" ]; then
            echo "Error: Migration name is required"
            echo "Usage: $0 create <migration_name>"
            exit 1
        fi
        echo "Creating migration files for $2..."
        migrate create -ext sql -dir "${MIGRATIONS_DIR}" -seq "$2"
        ;;
    version)
        echo "Checking current migration version..."
        migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" version
        ;;
    force)
        if [ -z "$2" ]; then
            echo "Error: Version number is required"
            echo "Usage: $0 force <version>"
            echo "Note: Use -1 to reset to no version (allows re-running all migrations)"
            exit 1
        fi
        VERSION="$2"
        echo "Forcing migration version to $VERSION..."
        # Use env to pass the command, avoiding shell flag parsing issues with negative numbers
        env migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" force -- "$VERSION"
        ;;
    goto)
        if [ -z "$2" ]; then
            echo "Error: Version number is required"
            echo "Usage: $0 goto <version>"
            exit 1
        fi
        echo "Migrating to version $2..."
        migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" goto "$2"
        ;;
esac

echo "Migration command completed successfully" 