#!/usr/bin/env bash
# validate.sh - this copy's CI: the checks of the archived upstream workflows (private/OLD-.github/workflows/)
# for every area a change touches.
#
#   scripts/validate.sh                 working tree + untracked files vs HEAD
#   scripts/validate.sh --base <rev>    everything since <rev>: commits, working tree, untracked
#   scripts/validate.sh --list          print the areas and stop
#   scripts/validate.sh --area <name>   run only that area (repeatable), whatever changed
#
# Every touched area runs even when an earlier one fails, so one run reports every failure. Inside an
# area each check is its own command and the first failure ends that area (set -e). The summary lists
# each area as passed, failed or skipped, then every check that did not run and why; the exit status
# is 1 when any area failed. A check whose tool is missing (golangci-lint, cargo, cargo-audit, helm,
# docker, pnpm, npx, shellcheck) is skipped and listed, never silently passed.
#
# golangci-lint reports only issues new since the base, as go-lint.yml did, because the upstream seed
# carries lint debt. HOOK_SKIP_TEST=1 skips the Go tests. AIKB_E2E_HOST and AIKB_E2E_TOKEN enable the
# CLI end-to-end suite (cli-e2e.yml), which needs a live server.
#
# Not run here: docker-image.yml and release-lite.yml build and publish images and release binaries,
# the publish jobs of mcp-server.yml and dsh-plugin.yml upload to PyPI and npm, dsh-plugin.yml's
# weekly drift job against dsh@latest is advisory, and go-lint-cache.yml only warms a CI cache.
# Matrices (cli on three OSes, mcp-server on Python 3.11-3.14) run on this machine's platform only.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=git-hooks/common.sh
source "$ROOT/scripts/git-hooks/common.sh"
cd "$ROOT"

base=HEAD list=0 forced=()
while [ $# -gt 0 ]; do
  case "$1" in
    --base) base="${2:?--base needs a revision}"; shift 2 ;;
    --list) list=1; shift ;;
    --area) forced+=("${2:?--area needs an area name}"); shift 2 ;;
    *) log_fail "unknown option $1"; exit 2 ;;
  esac
done
git rev-parse -q --verify "$base^{commit}" >/dev/null || { log_fail "not a commit: $base"; exit 2; }

# Deleted paths count for area detection; only existing files are formatted.
changed=()
while IFS= read -r f; do [ -n "$f" ] && changed+=("$f"); done < <(
  { git diff --name-only "$base"; git ls-files -o --exclude-standard; } | sort -u)
[ ${#changed[@]} -gt 0 ] || [ ${#forced[@]} -gt 0 ] || { log_ok "no changes since $base"; exit 0; }

# Each argument is a glob (`*` also matches `/`); true when any changed path matches one.
# shellcheck disable=SC2053
has() { local p g; for p in "${changed[@]}"; do for g in "$@"; do [[ "$p" == $g ]] && return 0; done; done; return 1; }
root_go() { [[ "$1" == *.go || "$1" == go.mod || "$1" == go.sum ]] && [[ "$1" != cli/* && "$1" != client/* ]]; }
any_root_go() { local p; for p in "${changed[@]}"; do root_go "$p" && return 0; done; return 1; }

# Trigger paths are the workflows' `paths:` filters. app.yml also triggered on license, hook and
# model-catalog files; those have their own, faster areas below. The locale bundle is a Go trigger
# because internal/datasource/syncreason checks its datasource.syncError keys.
areas=()
if any_root_go || has Makefile 'config/*' 'migrations/*' 'skills/preloaded/*' 'testdata/*' docker/Dockerfile.app \
  'internal/textconv/data/*' 'internal/models/catalog/data/*' 'internal/models/parity/testdata/*' \
  'internal/models/providers/assets/*' cmd/desktop/build/windows/installer/project.nsi .golangci.yml \
  frontend/src/i18n/locales/en-US.ts scripts/check-govulncheck.sh scripts/test-check-govulncheck.sh \
  scripts/govulncheck-allowlist.txt; then areas+=(go); fi
if has 'cli/*' .golangci.yml; then areas+=(cli); fi
if has 'client/*' .golangci.yml; then areas+=(client); fi
if has 'frontend/*' scripts/build_frontend_dist.sh scripts/test_embed_nginx.py scripts/verify_frontend_pr.sh; then areas+=(frontend); fi
if has 'docreader/*' go.mod go.sum 'testdata/*' docker/Dockerfile.docreader docker/Dockerfile.odl-hybrid; then areas+=(docreader); fi
if has 'internal/models/catalog/data/*' 'scripts/model-catalog/*'; then areas+=(model-catalog); fi
if has 'scripts/git-hooks/*' scripts/test-git-hooks.sh scripts/check-english-only.sh scripts/check-emdash.sh \
  scripts/emdash-allowlist.txt .claude/hooks/post-edit.sh; then areas+=(git-hooks); fi
if has scripts/start_all.sh scripts/test-start-all.sh; then areas+=(start-all); fi
if has '*.sh' 'scripts/git-hooks/*'; then areas+=(shell); fi
if has 'licenses/*' 'scripts/*license*' LICENSE THIRD_PARTY_NOTICES.md go.mod; then areas+=(licenses); fi
if has 'third_party/anydoc-go/*' scripts/build-anydoc-lib.sh 'internal/infrastructure/docparser/*'; then areas+=(anydoc); fi
if has 'helm/*'; then areas+=(helm); fi
if has 'mcp-server/*'; then areas+=(mcp-server); fi
if has 'packages/dsh-ai-kb/*'; then areas+=(dsh-plugin); fi
if has 'extension/*'; then areas+=(extension); fi
if has 'dataset/*' .github/workflows/dataset.yml; then areas+=(dataset); fi
if has 'miniprogram/*' 'tests/miniprogram/*'; then areas+=(miniprogram); fi
if has 'website-docs/*'; then areas+=(website-docs); fi
if has '*.md' .markdownlint-cli2.jsonc; then areas+=(markdown); fi
areas+=(english-only emdash version)
if [ ${#forced[@]} -gt 0 ]; then areas=("${forced[@]}"); fi

log_step "${#changed[@]} changed path(s) since $base; areas: ${areas[*]}"
[ "$list" = 1 ] && exit 0

NOTES="$(mktemp)"
trap 'rm -f "$NOTES"' EXIT
SKIPPED=77
area=""

skip() { printf '  %s: %s\n' "$area" "$*" >> "$NOTES"; log_step "skip: $*"; }
need() {
  local t
  for t in "$@"; do command -v "$t" >/dev/null 2>&1 || { skip "$t not installed"; return 1; }; done
}
go_tests() { [ "${HOOK_SKIP_TEST:-}" != 1 ] || { skip "go test (HOOK_SKIP_TEST=1)"; return 1; }; }
port_open() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }

area_go() {
  local p go_files=()
  for p in "${changed[@]}"; do [[ "$p" == *.go && -f "$p" ]] && root_go "$p" && go_files+=("$ROOT/$p"); done
  [ ${#go_files[@]} -eq 0 ] || check_gofmt_files check "${go_files[@]}"
  go mod tidy -diff
  if need golangci-lint; then run_golangci_if_available "$base"; fi
  run_app_vet_test
  if go_tests; then go test -count=1 -tags desktop ./internal/container; fi
  go build -o /dev/null ./cmd/server
  log_ok "go build ./cmd/server"
  if need jq; then bash scripts/test-check-govulncheck.sh; fi
  if need govulncheck jq; then scripts/check-govulncheck.sh; fi
}

area_cli() {
  go -C cli mod tidy -diff
  go -C cli build ./...
  go -C cli vet ./...
  if go_tests; then go -C cli test -race -count=1 ./...; fi
  if need golangci-lint; then run_golangci_if_available "$base" cli; fi
  cli/scripts/check-skill-wire-vocab.sh
  cli/scripts/check-secret-tokens.sh
  if [ -n "${AIKB_E2E_HOST:-}" ] && [ -n "${AIKB_E2E_TOKEN:-}" ]; then
    go -C cli test -tags=acceptance_e2e -count=1 -timeout=8m ./acceptance/e2e/...
  else
    skip "acceptance e2e suite (needs a live server: set AIKB_E2E_HOST and AIKB_E2E_TOKEN)"
  fi
}

area_client() {
  go -C client mod tidy -diff
  go -C client vet ./...
  if go_tests; then go -C client test -count=1 ./...; fi
  if need golangci-lint; then run_golangci_if_available "$base" client; fi
}

area_frontend() {
  scripts/verify_frontend_pr.sh
  if ! need docker python3; then return 0; fi
  if ! docker info >/dev/null 2>&1; then skip "UI image build and nginx route test (Docker is not running)"; return 0; fi
  docker build -q -f frontend/Dockerfile --build-arg VITE_FRONTEND_COMMIT="$(git rev-parse --short HEAD)" \
    -t ai-kb-ui:validate frontend/
  python3 scripts/test_embed_nginx.py ai-kb-ui:validate
}

# Both the Python proxy tests and the Go TestReadURL launch Playwright's WebKit.
install_webkit() {
  local deps=()
  if [ "$(uname -s)" = Linux ]; then
    # --with-deps installs system libraries through apt, which needs root.
    if [ "$(id -u)" = 0 ]; then
      deps=(--with-deps)
    else
      log_step "not root: WebKit system libraries not installed (sudo uv run --project docreader python -m playwright install-deps webkit)"
    fi
  fi
  uv run --project docreader --locked python -m playwright install ${deps[@]+"${deps[@]}"} webkit
}

area_docreader() {
  need uv || return "$SKIPPED"
  uv sync --project docreader --locked
  uv run --project docreader --locked python -m compileall -q -x '/\.venv/' docreader
  install_webkit
  uv run --project docreader --locked python -m unittest discover -s docreader/tests -p "test_*.py"
  # The Go client tests skip without a live server on the port they hardcode, so start one.
  if port_open 50051; then
    skip "own DocReader server (port 50051 is taken; the Go client tests used the server already on it)"
  else
    local i
    # Not local: the EXIT trap runs after this function has returned.
    dr_log="$(mktemp)"
    DOCREADER_GRPC_PORT=50051 uv run --project docreader --locked python -m docreader.main >"$dr_log" 2>&1 &
    dr_pid=$!
    # `|| true`: under set -e the killed server's status (143) would become the area's.
    trap 'kill "$dr_pid" 2>/dev/null || true; wait "$dr_pid" 2>/dev/null || true; rm -f "$dr_log"' EXIT
    for i in $(seq 1 60); do
      port_open 50051 && break
      kill -0 "$dr_pid" 2>/dev/null || break
      sleep 1
    done
    port_open 50051 || { tail -20 "$dr_log" >&2; log_fail "DocReader gRPC server did not start within ${i}s"; return 1; }
    log_ok "DocReader gRPC server on 50051"
  fi
  go test -count=1 ./docreader/client ./docreader/proto
}

area_model-catalog() {
  python3 scripts/model-catalog/generate.py --check
  go test -count=1 ./internal/models/...
}

area_git-hooks() {
  bash scripts/test-git-hooks.sh
}

# Only the changed scripts: the tree is clean at -S warning, so any finding is new.
area_shell() {
  need shellcheck || return "$SKIPPED"
  local p files=()
  for p in "${changed[@]}"; do
    [[ -f "$p" && ( "$p" == *.sh || "$p" == scripts/git-hooks/* ) ]] && files+=("$p")
  done
  [ ${#files[@]} -eq 0 ] || shellcheck -S warning "${files[@]}"
  log_ok "shellcheck -S warning: ${#files[@]} file(s)"
}

area_start-all() {
  bash scripts/test-start-all.sh
}

area_licenses() {
  bash scripts/check-license-bundle.sh
  bash scripts/test-license-bundle.sh
}

area_anydoc() {
  # rustup's default install location, for shells that do not source ~/.cargo/env.
  if [ -d "$HOME/.cargo/bin" ]; then PATH="$HOME/.cargo/bin:$PATH"; fi
  need cargo rustc || return "$SKIPPED"
  scripts/build-anydoc-lib.sh
  # Needs the patched-anydoc/ tree the build just materialized.
  if need cargo-audit; then (cd third_party/anydoc-go && cargo audit); fi
  go vet -tags anydoc ./internal/infrastructure/docparser/...
  if go_tests; then go test -tags anydoc -count=1 ./internal/infrastructure/docparser/...; fi
  go build -tags anydoc -o /dev/null ./cmd/server
}

helm_image() {
  local template="$1"
  shift
  helm template ai-kb ./helm --show-only "$template" --set secrets.dbPassword=test \
    --set secrets.redisPassword=test --set secrets.jwtSecret=test "$@" |
    awk '$1 == "image:" && !n++ { print $2 }'
}

expect_eq() {
  [ "$2" = "$3" ] || { log_fail "$1: got '$2', want '$3'"; return 1; }
}

area_helm() {
  need helm uv || return "$SKIPPED"
  helm lint ./helm
  uv run --no-project --with pyyaml python -m unittest discover -s helm/tests
  local app_tag
  app_tag="$(helm_image templates/app.yaml)"
  app_tag="${app_tag##*:}"
  expect_eq "frontend image tag follows the app tag" "$(helm_image templates/frontend.yaml | sed 's/.*://')" "$app_tag"
  expect_eq "docreader image tag follows the app tag" "$(helm_image templates/docreader.yaml | sed 's/.*://')" "$app_tag"
  expect_eq "frontend image override" "$(helm_image templates/frontend.yaml --set frontend.image.tag=ui-test)" \
    ghcr.io/jrmatherly/ai-kb-ui:ui-test
  expect_eq "docreader image override" "$(helm_image templates/docreader.yaml --set docreader.image.tag=reader-test)" \
    ghcr.io/jrmatherly/ai-kb-docreader:reader-test
  log_ok "helm image tags"
}

area_mcp-server() {
  need uv || return "$SKIPPED"
  # unittest finds the top-level test_*.py; pytest runs tests/ (pyproject testpaths), which CI never ran.
  uv run --directory mcp-server --locked --extra test python -m unittest discover -s . -p "test_*.py"
  uv run --directory mcp-server --locked --extra test python -m pytest
  local dist
  dist="$(mktemp -d)"
  uv build -q mcp-server --out-dir "$dist"
  python3 - "$dist" <<'PY'
import glob, sys, zipfile
names = zipfile.ZipFile(glob.glob(sys.argv[1] + "/*.whl")[0]).namelist()
sys.exit(0 if "upload_paths.py" in names else "upload_paths.py is missing from the wheel (py-modules in pyproject.toml/setup.py)")
PY
  rm -rf "$dist"
  log_ok "mcp-server wheel contains upload_paths.py"
}

area_dataset() {
  need uv || return "$SKIPPED"
  uv run --project dataset --locked python -m unittest discover -s dataset/tests
}

area_dsh-plugin() {
  local dir=packages/dsh-ai-kb
  need node npm || return "$SKIPPED"
  ensure_npm_deps "$dir"
  npm --prefix "$dir" run --silent typecheck
  npm --prefix "$dir" run --silent build
  (cd "$dir" && node --test "test/*.test.mjs")
  go test -count=1 "./$dir/contract/..."
  (cd "$dir" && npm pack --dry-run --json --silent) | node -e '
    const files = JSON.parse(require("fs").readFileSync(0, "utf8"))[0].files.map(f => f.path);
    const missing = ["dist/index.js", "dist/index.d.ts", "cordis.patch.yml", "README.md", "package.json"].filter(n => !files.includes(n));
    const leaked = files.filter(n => n.startsWith("test/") || n.startsWith("src/"));
    if (missing.length) { console.error("missing from the tarball: " + missing.join(", ")); process.exit(1); }
    if (leaked.length) { console.error("unexpected files in the tarball: " + leaked.join(", ")); process.exit(1); }
    console.log("tarball contents verified: " + files.length + " files");'
  # `dsh plugin add` installs through pnpm. The first run installs the pinned harness release into
  # ~/.cache/ai-kb (minutes); later runs reuse it. The pin lives in run-in-dsh.mjs; unset its
  # overrides so this always checks the pinned release.
  if need pnpm; then
    (cd "$dir" && env -u DSH_PACKAGE_SPEC -u npm_config_before -u DSH_INSTALL_DIR node test/e2e/run-in-dsh.mjs)
  fi
}

area_extension() {
  need node npm zip || return "$SKIPPED"
  ensure_npm_deps extension
  npm --prefix extension run --silent test
  npm --prefix extension run --silent type-check
  npm --prefix extension run --silent build
  npm --prefix extension run --silent zip
}

# No archived workflow ran these; they are the package's own tests (no dependencies to install).
area_miniprogram() {
  need node npm || return "$SKIPPED"
  npm --prefix miniprogram test
}

area_website-docs() {
  ensure_npm_deps website-docs
  ensure_npm_deps website-docs/homepage
  node website-docs/scripts/check-anchors.mjs
  npm --prefix website-docs/homepage run --silent lint
  # Builds the homepage and docs, runs the link and mermaid checks, the homepage audit and the site check.
  npm --prefix website-docs run --silent build
}

# The whole tree, not only the changed files: it takes seconds, and a config change can affect any file.
area_markdown() {
  need npx || return "$SKIPPED"
  npx --yes "$MARKDOWNLINT_CLI2"
}

# Lines added since base, committed or not; untracked files are covered by the PostToolUse hook and pre-commit.
area_english-only() {
  scripts/check-english-only.sh --diff "$base"
}

# The whole tree minus scripts/emdash-allowlist.txt: none remain, so any hit is new.
area_emdash() {
  scripts/check-emdash.sh
}

# Every version string against VERSION and each library against itself: milliseconds, and a bump or a hand
# edit anywhere can break it. The tests clone HEAD and drive the make targets, so they run only when the
# tool or the Makefile changes; guards.yml runs them in CI.
area_version() {
  python3 scripts/version.py check
  if has scripts/version.py scripts/test_version.py Makefile && need uv ssh-keygen make; then
    python3 scripts/test_version.py
  fi
}

for area in "${areas[@]}"; do
  declare -F "area_$area" >/dev/null || { log_fail "unknown area $area"; exit 2; }
done
results=() failed=0
for area in "${areas[@]}"; do
  log_step "[$area]"
  start=$SECONDS
  # A subshell tested by `if` or `||` would run with set -e disabled, so capture its status instead.
  set +e
  (
    set -eE
    trap 'rc=$?; [ "$rc" = "$SKIPPED" ] || log_fail "$area: \`${BASH_COMMAND%%$'"'\n'"'*}\` exited $rc"' ERR
    "area_$area"
  )
  rc=$?
  set -e
  case "$rc" in
    0) status=passed ;;
    "$SKIPPED") status=skipped ;;
    *) status=failed failed=1 ;;
  esac
  if [ "$status" = failed ]; then log_fail "[$area] failed"; else log_ok "[$area] $status"; fi
  results+=("$(printf '  %-14s %-8s %4ss' "$area" "$status" $((SECONDS - start)))")
done

printf '\nvalidate summary (changes since %s):\n' "$base"
printf '%s\n' "${results[@]}"
if [ -s "$NOTES" ]; then printf 'not run:\n'; cat "$NOTES"; fi
if [ "$failed" = 1 ]; then log_fail "validate failed"; exit 1; fi
log_ok "validate passed"
