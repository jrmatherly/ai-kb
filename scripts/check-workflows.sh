#!/usr/bin/env bash
# check-workflows.sh - enforce the .github/workflows conventions: every remote action pinned to a full
# commit SHA with a `# <tag>` comment, a top-level `concurrency`, every workflow, job and step
# named with a leading emoji, and no bash 4 builtin (mapfile, readarray) in a `run` step of a job
# that can land on a macOS runner, whose bash is 3.2. Needs yq v4. Exit 1 on any violation, printing
# each one.
#
#   check-workflows.sh [dir]   default: .github/workflows under the repo root
set -uo pipefail
export LC_ALL=C
dir="${1:-$(git rev-parse --show-toplevel)/.github/workflows}"
fail=0
bad() { echo "$1" >&2; fail=1; }

for f in "$dir"/*.yml "$dir"/*.yaml; do
  [ -f "$f" ] || continue
  while IFS= read -r line; do bad "$f:$line: not pinned as @<40-hex sha> # <tag>"; done < <(
    grep -nE '^[[:space:]-]*uses:' "$f" |
      grep -vE "uses:[[:space:]]*['\"]?\./" |
      grep -vE 'uses:[[:space:]]*[^[:space:]@]+@[0-9a-f]{40}[[:space:]]+#[[:space:]]*[^[:space:]]')
  [ "$(yq 'has("concurrency")' "$f")" = true ] || bad "$f: no top-level concurrency"
  # Absent names come out as "", so a missing job or step name fails like a plain one.
  while IFS=$'\t' read -r path name; do
    [[ "$name" == [$'\x80'-$'\xff']* ]] || bad "$f: $path must start with an emoji: '$name'"
  done < <(yq -r '(.name, .jobs[].name, .jobs[].steps[]?.name) | [path | join("."), (. // "")] | @tsv' "$f")
  # A job can run on macOS when "macos" appears anywhere in it (runs-on or a matrix). Only a builtin
  # at the start of a line is caught; one behind `;` or `&&` is not.
  while IFS= read -r line; do [ -n "$line" ] && bad "$f: $line: bash 4 builtin, the macOS runner has bash 3.2"; done < <(
    yq -r '.jobs | to_entries[] | select((.value | tostring) | test("macos")) | .key as $j |
      .value.steps[]?.run // "" | split("\n")[] | select(test("^ *(mapfile|readarray) ")) | "job " + $j + ": " + .' "$f")
done
exit "$fail"
