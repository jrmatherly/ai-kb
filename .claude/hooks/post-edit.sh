#!/usr/bin/env bash
# PostToolUse (Edit|Write|MultiEdit): gofmt Go files, then report CJK text or em dashes the edit added
# and markdownlint findings in an edited Markdown file.
# Feedback only (exit 2 shows stderr to Claude); the pre-commit hook is the enforcing gate.
set -uo pipefail
f="$(jq -r '.tool_input.file_path // empty')"
[ -n "$f" ] && [ -f "$f" ] || exit 0
case "$f" in *.go) gofmt -w "$f" 2>/dev/null ;; esac
# The file's own checkout, not CLAUDE_PROJECT_DIR: in a worktree session that is the main checkout,
# which cannot see the worktree's index and would report every line as added.
cd "$(dirname "$f")" && root="$(git rev-parse --show-toplevel 2>/dev/null)" && cd "$root" || exit 0
[ -x scripts/check-english-only.sh ] || exit 0   # another repository
# markdownlint findings (stderr; the banner goes to stdout). Never --fix: some fixers damage content.
md=""
if [[ "$f" == *.md ]] && [ -f .markdownlint-cli2.jsonc ]; then
  pin="$(sed -n 's/^MARKDOWNLINT_CLI2=//p' scripts/git-hooks/common.sh)"
  [ -n "$pin" ] && md="$(npx --yes "$pin" --no-globs -- "$f" 2>&1 >/dev/null)"
fi
# Lines added against HEAD as "<line>: <text>" (the whole file when untracked); a file keeps
# being reported until its added CJK or em dash is gone. --diff-algorithm=minimal overrides a
# diff.algorithm=histogram config, which can report an untouched line as re-added when lines are
# inserted around it; a minimal diff of pure insertions adds only the inserted lines.
if git ls-files --error-unmatch -- "$f" >/dev/null 2>&1; then
  added="$(git diff --no-color --diff-algorithm=minimal -U0 HEAD -- "$f" |
    perl -ne 'if (/^@@ -\S+ \+(\d+)/) { $n = $1; next } next if /^\+\+\+/; if (s/^\+//) { print "$n: $_"; $n++ }')"
else
  added="$(perl -ne 'print "$.: $_"' -- "$f")"
fi
cjk="" dash=""
if [ -n "$added" ]; then
  # New CJK only: for a tracked file the English-only gate ignores Chinese a line already carried
  # (a rename inside a Chinese line); an untracked file is all new. The gate honours -english-only paths.
  if git ls-files --error-unmatch -- "$f" >/dev/null 2>&1; then
    cjk="$(scripts/check-english-only.sh --diff HEAD -- "$f" 2>/dev/null | head -5)"
  elif [ "$(git check-attr english-only -- "$f" | awk '{print $NF}')" != unset ]; then
    cjk="$(printf '%s\n' "$added" | perl -CSD -ne 'print if /[\x{3040}-\x{30ff}\x{3400}-\x{4dbf}\x{4e00}-\x{9fff}\x{ac00}-\x{d7af}\x{ff00}-\x{ffef}]/' | head -5)"
  fi
  dash="$(printf '%s\n' "$added" | grep $'\xe2\x80\x94' | head -5)"
fi
[ -z "$cjk$dash$md" ] && exit 0
[ -n "$cjk" ] && printf '%s: added CJK text (this copy is English only):\n%s\n' "$f" "$cjk" >&2
[ -n "$dash" ] && printf '%s: added an em dash (use a plain dash, comma or new sentence):\n%s\n' "$f" "$dash" >&2
[ -n "$md" ] && printf '%s: markdownlint findings (fix by hand; --fix breaks MD018, MD029, MD037 and MD038 cases):\n%s\n' "$f" "$md" >&2
exit 2
