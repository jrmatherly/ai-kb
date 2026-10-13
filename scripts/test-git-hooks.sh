#!/usr/bin/env bash
# Verify that app checks cover unchanged callers and propagate failures.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/scripts/git-hooks" "$fixture/bin"
cp "$repo_root/scripts/git-hooks/common.sh" "$fixture/scripts/git-hooks/"
export HOOK_TEST_LOG="$fixture/go.log"
export LOG_FORMAT="developer-custom-template"
cat > "$fixture/bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$HOOK_TEST_LOG"
case "$1" in
  list)
    [[ "${HOOK_TEST_FAIL:-}" != list ]] || exit 1
    printf '%s\n' example/internal/agent/tools example/internal/application/service example/docreader/proto
    ;;
  vet|test)
    if [[ "$1" == test && -n "${LOG_FORMAT:-}" ]]; then
      echo "App tests inherited the developer log template" >&2
      exit 1
    fi
    [[ "${HOOK_TEST_FAIL:-}" != "$1" ]] || exit 1
    ;;
  *) echo "Unexpected go invocation: $*" >&2; exit 1 ;;
esac
EOF
chmod +x "$fixture/bin/go"
export PATH="$fixture/bin:$PATH"
unset HOOK_SKIP_TEST HOOK_TEST_FAIL

run_checks() {
  bash -c 'source "$1"; run_app_vet_test' bash "$fixture/scripts/git-hooks/common.sh"
}

run_checks
cat > "$fixture/expected.log" <<'EOF'
list ./...
vet example/internal/agent/tools example/internal/application/service
test -count=1 example/internal/agent/tools example/internal/application/service
EOF
diff -u "$fixture/expected.log" "$HOOK_TEST_LOG"

# Keep the explicit local opt-out, but it must not skip vet.
: > "$HOOK_TEST_LOG"
HOOK_SKIP_TEST=1 run_checks
head -n 2 "$fixture/expected.log" > "$fixture/skip.log"
diff -u "$fixture/skip.log" "$HOOK_TEST_LOG"

for phase in list vet test; do
  : > "$HOOK_TEST_LOG"
  if HOOK_TEST_FAIL="$phase" run_checks > "$fixture/failure.log" 2>&1; then
    echo "App checks accepted a failing go $phase" >&2
    exit 1
  fi
  case "$phase" in
    list) head -n 1 "$fixture/expected.log" ;;
    vet) head -n 2 "$fixture/expected.log" ;;
    test) cat "$fixture/expected.log" ;;
  esac > "$fixture/failure-expected.log"
  diff -u "$fixture/failure-expected.log" "$HOOK_TEST_LOG"
done

# English-only gate: an added ASCII line passes, an added CJK line fails and names the file.
gate="$fixture/gate"
mkdir -p "$gate" && git -C "$gate" init -q
cp "$repo_root/scripts/check-english-only.sh" "$gate/check.sh"
(cd "$gate" && printf 'ok\n' > a.txt && git add a.txt && ./check.sh --diff)
if (cd "$gate" && printf '\xe4\xb8\xad\n' > b.txt && git add b.txt && ./check.sh --diff > out.log 2>&1); then
  echo "English-only gate accepted a CJK line" >&2
  exit 1
fi
grep -q '^b.txt: ' "$gate/out.log"

# Em dash gate: a clean tree passes, an allowlisted path passes, a planted em dash fails and names its line,
# in the working tree and in the index.
dash="$fixture/dash"
mkdir -p "$dash/scripts" && git -C "$dash" init -q
cp "$repo_root/scripts/check-emdash.sh" "$repo_root/scripts/emdash-allowlist.txt" "$dash/scripts/"
printf 'plain - dash\n' > "$dash/a.txt"
printf 'kept \xe2\x80\x94 history\n' > "$dash/CHANGELOG.md"
git -C "$dash" add -A
(cd "$dash" && scripts/check-emdash.sh && scripts/check-emdash.sh --cached)
printf 'planted \xe2\x80\x94 dash\n' >> "$dash/a.txt"
if (cd "$dash" && scripts/check-emdash.sh > out.log 2>&1); then
  echo "em dash gate accepted a planted em dash" >&2
  exit 1
fi
grep -q '^a.txt:2:' "$dash/out.log"
(cd "$dash" && scripts/check-emdash.sh --cached)
git -C "$dash" add a.txt
if (cd "$dash" && scripts/check-emdash.sh --cached > out.log 2>&1); then
  echo "em dash gate accepted a staged em dash" >&2
  exit 1
fi

# post-edit hook: an em dash already in a tracked file is not reported and an added one is, also for a
# worktree file while CLAUDE_PROJECT_DIR names the main checkout (as in a worktree session).
repo="$fixture/repo"
mkdir -p "$repo/scripts" && git -C "$repo" init -q
cp "$repo_root/scripts/check-english-only.sh" "$repo/scripts/"
printf 'old \xe2\x80\x94 line\n' > "$repo/a.md"
git -C "$repo" add -A
git -C "$repo" -c user.name=t -c user.email=t@example.com -c commit.gpgsign=false commit -qm seed
git -C "$repo" worktree add -q "$fixture/wt" 2>/dev/null
post_edit() { printf '{"tool_input":{"file_path":"%s"}}' "$1" | CLAUDE_PROJECT_DIR="$repo" "$repo_root/.claude/hooks/post-edit.sh"; }
for f in "$repo/a.md" "$fixture/wt/a.md"; do
  printf 'plain line\n' >> "$f"
  post_edit "$f" || { echo "post-edit reported the existing em dash in $f" >&2; exit 1; }
  printf 'added \xe2\x80\x94 dash\n' >> "$f"
  if post_edit "$f" 2> "$fixture/post-edit.log"; then
    echo "post-edit missed the em dash added to $f" >&2
    exit 1
  fi
  grep -q '^3: added' "$fixture/post-edit.log"
done

# markdownlint: pre-commit passes a clean staged .md and fails a bad one; post-edit reports the bad one.
md="$fixture/md"
mkdir -p "$md/scripts/git-hooks" && git -C "$md" init -q
cp "$repo_root/scripts/git-hooks/pre-commit" "$repo_root/scripts/git-hooks/common.sh" "$md/scripts/git-hooks/"
cp "$repo_root/scripts/check-english-only.sh" "$repo_root/scripts/check-emdash.sh" \
  "$repo_root/scripts/emdash-allowlist.txt" "$md/scripts/"
cp "$repo_root/.markdownlint-cli2.jsonc" "$md/"
printf '# Good\n\nText.\n' > "$md/good.md"
git -C "$md" add good.md
if ! "$md/scripts/git-hooks/pre-commit" > "$fixture/md.log" 2>&1; then
  cat "$fixture/md.log" >&2
  echo "pre-commit rejected a clean Markdown file" >&2
  exit 1
fi
grep -q 'markdownlint (1 staged' "$fixture/md.log"
printf '# Bad\n#296 starts a line\n' > "$md/bad.md"
git -C "$md" add bad.md
if "$md/scripts/git-hooks/pre-commit" > "$fixture/md.log" 2>&1; then
  echo "pre-commit accepted a Markdown file with lint errors" >&2
  exit 1
fi
grep -q '^bad.md:2:1 error MD018/' "$fixture/md.log"
if post_edit "$md/bad.md" 2> "$fixture/post-edit.log"; then
  echo "post-edit missed the markdownlint error in bad.md" >&2
  exit 1
fi
grep -q 'bad.md:2:1 error MD018/' "$fixture/post-edit.log"

echo 'Git hook coverage checks passed'
