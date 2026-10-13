#!/usr/bin/env bash
# The frontend.yml verify job: install, test, type-check, build.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=git-hooks/common.sh
source "$ROOT/scripts/git-hooks/common.sh"
ensure_npm_deps "$ROOT/frontend"
cd "$ROOT/frontend"

echo "[verify_frontend_pr] npm test"
npm test

echo "[verify_frontend_pr] npm run type-check"
npm run type-check

echo "[verify_frontend_pr] npm run build"
npm run build

echo "[verify_frontend_pr] OK"
