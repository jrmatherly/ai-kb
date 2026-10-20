#!/usr/bin/env bash
# PreToolUse (Edit|Write|MultiEdit): block hand edits to generated files and lockfiles.
# Each is rebuilt by a generator; a hand edit is lost on the next run or fails its --check gate.
set -euo pipefail
f="$(jq -r '.tool_input.file_path // empty')"
[ -n "$f" ] || exit 0
root="${CLAUDE_PROJECT_DIR:-$PWD}"
[[ "$f" == "$root"/* ]] || exit 0
rel="${f#"$root"/}"
rel="${rel#.claude/worktrees/*/}"   # a worktree's generated files are just as generated
case "$rel" in
  docs/docs.go|docs/swagger.json|docs/swagger.yaml)
    gen="make docs (swag over cmd/server/main.go; edit the handler annotations instead)" ;;
  internal/models/catalog/data/models.generated.json)
    gen="make model-catalog-generate (edit seed.json or overrides.json instead)" ;;
  frontend/src/components.d.ts)
    gen="any Vite build or dev run (unplugin-vue-components writes it from the templates)" ;;
  docreader/proto/*.pb.go|docreader/proto/*_pb2.py|docreader/proto/*_pb2.pyi|docreader/proto/*_pb2_grpc.py)
    gen="make -C docreader proto (edit docreader/proto/docreader.proto instead)" ;;
  go.sum|*/go.sum|package-lock.json|*/package-lock.json|uv.lock|*/uv.lock|*/Cargo.lock)
    gen="the package manager (go mod tidy, npm install, uv lock, cargo update)" ;;
  private/OLD-.git/*)
    gen="nothing: it is the archived upstream clone, read it with git --git-dir" ;;
  *) exit 0 ;;
esac
echo "$rel is generated; regenerate it with $gen." >&2
exit 2
