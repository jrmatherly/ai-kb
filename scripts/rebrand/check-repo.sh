#!/usr/bin/env bash
# check-repo.sh - run check.sh over every tracked file scope.txt selects; exit 1 prints each leftover line.
set -uo pipefail
cd "$(git rev-parse --show-toplevel)" || exit 2
scope=()
while IFS= read -r line; do [ -n "$line" ] && [ "${line:0:1}" != "#" ] && scope+=("$line"); done < scripts/rebrand/scope.txt
git ls-files -z -- "${scope[@]}" | xargs -0 scripts/rebrand/check.sh 2>/dev/null
[ "${PIPESTATUS[1]}" -eq 0 ] || exit 1
