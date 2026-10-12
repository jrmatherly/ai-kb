#!/usr/bin/env bash
# check-english-only.sh - find CJK text (Han, kana, Hangul, fullwidth forms). This copy is English only.
# Exit 1 when anything matches, so every mode works as a gate; 2 on a bad pathspec or option.
# UTF-8 text only: legacy encodings (GBK, Big5) are not detected.
#
#   check-english-only.sh [pathspec...]     tracked files containing CJK text (default: whole repo)
#   check-english-only.sh -n [pathspec...]  matching lines with line numbers
#   check-english-only.sh --summary         file counts per top-level directory
#   check-english-only.sh --diff [<range>]  lines a diff adds: the staged diff by default (the pre-commit
#                                           gate), or a range such as <sha>..HEAD after a cherry-pick batch
set -uo pipefail
export LC_ALL=C.UTF-8   # git grep -P rejects \x{...} above 0xff under a non-UTF-8 locale
CJK='[\x{3040}-\x{30ff}\x{3400}-\x{4dbf}\x{4e00}-\x{9fff}\x{ac00}-\x{d7af}\x{ff00}-\x{ffef}]'
root="$(git rev-parse --show-toplevel)" || exit 2
# A path marked `-english-only` in .gitattributes may hold CJK text (the translation glossary).
EXEMPT=':(exclude,attr:-english-only)'

# Whole repo with no pathspec, else the given pathspecs relative to the caller's directory.
# A pathspec that matches no tracked file is an error, not a clean result.
scan() {
  local flag="$1"; shift
  if [ $# -eq 0 ]; then
    git -C "$root" grep -I "$flag" -P "$CJK" -- . "$EXEMPT"
  else
    git ls-files --error-unmatch -- "$@" >/dev/null || exit 2
    git grep -I --full-name "$flag" -P "$CJK" -- "$@" "$EXEMPT"
  fi
}

mode=files
case "${1:-}" in
  --diff) mode=diff; shift ;;
  --summary) mode=summary; shift ;;
  -n) mode=lines; shift ;;
  -?*) echo "check-english-only.sh: unknown option $1" >&2; exit 2 ;;
esac

case "$mode" in
  diff)
    range=(--cached); [ $# -eq 0 ] || range=("$@")
    hits=$(git -C "$root" diff -U0 --no-color --src-prefix=a/ --dst-prefix=b/ "${range[@]}" -- . "$EXEMPT" | perl -CSD -ne '
      if (m{^\+\+\+ b/(.*)}) { $f = $1; next }
      next unless /^\+/ && /'"$CJK"'/;
      s/^\+//; print "$f: $_"')
    [ -z "$hits" ] && exit 0
    printf '%s\n' "$hits"
    echo "CJK text in added lines; this repo is English only (scripts/check-english-only.sh)." >&2
    exit 1 ;;
  summary) scan -l "$@" | awk -F/ '{c[NF>1 ? $1 : "(root)"]++} END {for (k in c) print c[k], k}' | sort -rn; rc=${PIPESTATUS[0]} ;;
  lines)   scan -n "$@"; rc=$? ;;
  files)   scan -l "$@"; rc=$? ;;
esac
case "$rc" in
  0) exit 1 ;;   # matches: not English only
  1) exit 0 ;;   # clean
  *) exit "$rc" ;;
esac
