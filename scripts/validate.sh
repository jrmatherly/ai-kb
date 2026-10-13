#!/usr/bin/env bash
# validate.sh - run the checks for each area the change touches (the archived upstream CI per area).
#
#   scripts/validate.sh                 working tree + untracked files vs HEAD
#   scripts/validate.sh --base <rev>    everything since <rev>: commits, working tree, untracked
#   scripts/validate.sh --list [...]    print the areas and stop
#
# Reuses scripts/git-hooks/common.sh. golangci-lint reports only issues new since the base, because the
# upstream seed carries lint debt a full run would always report. HOOK_SKIP_TEST=1 skips go test.
# Every check is its own command so `set -e` stops at the first failure.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=scripts/git-hooks/common.sh
source "$ROOT/scripts/git-hooks/common.sh"
cd "$ROOT"

base=HEAD list=0
while [ $# -gt 0 ]; do
  case "$1" in
    --base) base="${2:?--base needs a revision}"; shift 2 ;;
    --list) list=1; shift ;;
    *) log_fail "unknown option $1"; exit 2 ;;
  esac
done
git rev-parse -q --verify "$base^{commit}" >/dev/null || { log_fail "not a commit: $base"; exit 2; }

# Deleted paths count for area detection; only existing files are formatted.
changed=()
while IFS= read -r f; do [ -n "$f" ] && changed+=("$f"); done < <(
  { git diff --name-only "$base"; git ls-files -o --exclude-standard; } | sort -u)
[ ${#changed[@]} -gt 0 ] || { log_ok "no changes since $base"; exit 0; }

# shellcheck disable=SC2053  # $1 is a glob on purpose
has() { local p; for p in "${changed[@]}"; do [[ "$p" == $1 ]] && return 0; done; return 1; }
root_go() { [[ "$1" == *.go || "$1" == go.mod || "$1" == go.sum ]] && [[ "$1" != cli/* && "$1" != client/* ]]; }

areas=()
for p in "${changed[@]}"; do if root_go "$p"; then areas+=(go); break; fi; done
if has 'cli/*'; then areas+=(cli); fi
if has 'client/*'; then areas+=(client); fi
if has 'frontend/*'; then areas+=(frontend); fi
if has 'docreader/*'; then areas+=(docreader); fi
if has 'internal/models/catalog/data/*' || has 'scripts/model-catalog/*'; then areas+=(model-catalog); fi
if has 'scripts/git-hooks/*'; then areas+=(git-hooks); fi
if has 'scripts/start_all.sh' || has 'scripts/test-start-all.sh'; then areas+=(start-all); fi
if has 'licenses/*' || has 'scripts/*license*'; then areas+=(licenses); fi
if has 'website-docs/*'; then areas+=(website-docs); fi

log_step "${#changed[@]} changed path(s) since $base; areas: ${areas[*]:-none}"
[ "$list" = 1 ] && exit 0

for area in ${areas[@]+"${areas[@]}"}; do
  case "$area" in
    go)
      go_files=()
      for p in "${changed[@]}"; do [[ "$p" == *.go && -f "$p" ]] && root_go "$p" && go_files+=("$ROOT/$p"); done
      [ ${#go_files[@]} -eq 0 ] || check_gofmt_files check "${go_files[@]}"
      run_golangci_if_available "$base"
      run_app_vet_test
      if [ "${HOOK_SKIP_TEST:-}" != 1 ]; then go test -count=1 -tags desktop ./internal/container; fi
      go build ./cmd/server
      log_ok "go build ./cmd/server" ;;
    cli)
      go -C cli vet ./...
      if [ "${HOOK_SKIP_TEST:-}" != 1 ]; then go -C cli test -race -count=1 ./...; fi
      cli/scripts/check-skill-wire-vocab.sh
      cli/scripts/check-secret-tokens.sh
      log_ok "cli" ;;
    client)
      go -C client vet ./...
      go -C client test -count=1 ./...
      log_ok "client" ;;
    frontend)
      run_frontend_checks ;;
    docreader)
      uv run --project docreader python -m unittest discover -s docreader/tests -p "test_*.py"
      go test -count=1 ./docreader/client ./docreader/proto
      log_ok "docreader" ;;
    model-catalog)
      python3 scripts/model-catalog/generate.py --check
      go test -count=1 ./internal/models/...
      log_ok "model catalog" ;;
    git-hooks)
      bash scripts/test-git-hooks.sh
      log_ok "git hooks" ;;
    start-all)
      bash scripts/test-start-all.sh
      log_ok "start_all.sh" ;;
    licenses)
      bash scripts/check-license-bundle.sh
      bash scripts/test-license-bundle.sh
      log_ok "license bundle" ;;
    website-docs)
      node website-docs/scripts/check-links.mjs
      node website-docs/scripts/check-anchors.mjs
      npm --prefix website-docs run --silent check:mermaid
      npm --prefix website-docs run --silent build:docs
      log_ok "website-docs" ;;
  esac
done

# Lines added since base, committed or not; untracked files are covered by the PostToolUse hook and pre-commit.
scripts/check-english-only.sh --diff "$base"
log_ok "english-only on changes since $base"
log_ok "validate passed (${areas[*]:-no checked areas})"
