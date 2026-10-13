#!/usr/bin/env bash
# check-emdash.sh - find em dashes (U+2014) in tracked files. Prose here uses a plain dash or a new sentence.
# Exit 1 when any line outside scripts/emdash-allowlist.txt has one, 2 on a bad option or a git error.
# Code that must keep the character as data spells it as an escape (\u2014).
#
#   check-emdash.sh            tracked files in the working tree (validate.sh)
#   check-emdash.sh --cached   the index (the pre-commit gate)
set -uo pipefail
root="$(git rev-parse --show-toplevel)" || exit 2
case "${1:-}" in
  ''|--cached) ;;
  *) echo "check-emdash.sh: unknown option $1" >&2; exit 2 ;;
esac
allow="$root/scripts/emdash-allowlist.txt"
[ -f "$allow" ] || { echo "check-emdash.sh: $allow is missing" >&2; exit 2; }
excludes=()
while IFS= read -r p || [ -n "$p" ]; do
  case "$p" in ''|'#'*) continue ;; esac
  excludes+=(":(exclude)$p")
done < "$allow"
# The byte sequence, so this file does not match itself and bash 3.2 needs no \u support.
git -C "$root" grep -n -I -F ${1:+"$1"} -e $'\xe2\x80\x94' -- . ${excludes[@]+"${excludes[@]}"}
case "$?" in
  0) echo "em dash in the lines above; use a plain dash or a new sentence (scripts/check-emdash.sh)." >&2; exit 1 ;;
  1) exit 0 ;;
  *) exit 2 ;;
esac
