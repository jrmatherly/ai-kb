#!/usr/bin/env bash
# check-govulncheck.sh [package-pattern] - fail on any symbol-level govulncheck finding not in
# scripts/govulncheck-allowlist.txt. Symbol-level findings only (a trace[0].function), the same set the text output
# calls affected; module-level entries for required modules are outside the gate. Exit 1 on an unlisted finding or a
# stream with no config record, 2 when govulncheck or jq is missing.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
for t in govulncheck jq; do
  command -v "$t" >/dev/null 2>&1 || { echo "check-govulncheck: $t not installed" >&2; exit 2; }
done
stream=$(mktemp)
trap 'rm -f "$stream"' EXIT
govulncheck -format json "${1:-./...}" > "$stream"
jq -e 'select(.config)' "$stream" >/dev/null || { echo "check-govulncheck: no config record in the govulncheck stream" >&2; exit 1; }
unlisted=$(LC_ALL=C comm -23 \
  <(jq -r 'select(.finding) | select(.finding.trace[0].function != null) | .finding.osv' "$stream" | LC_ALL=C sort -u) \
  <(grep -v '^#' scripts/govulncheck-allowlist.txt | cut -f1 | LC_ALL=C sort -u))
if [ -n "$unlisted" ]; then
  echo "check-govulncheck: called findings not in scripts/govulncheck-allowlist.txt:" >&2
  printf '  %s\n' "$unlisted" >&2
  exit 1
fi
echo "check-govulncheck: ok"
