#!/usr/bin/env bash
# test-check-govulncheck.sh - exercise scripts/check-govulncheck.sh against a fake govulncheck that prints $FAKE_STREAM.
set -uo pipefail
cd "$(git rev-parse --show-toplevel)" || exit 2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/govulncheck" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$FAKE_STREAM"
FAKE
chmod +x "$tmp/govulncheck"
export PATH="$tmp:$PATH"

config='{"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck","scanner_version":"v1.9.0"}}'
called_ok='{"finding":{"osv":"GO-2025-4251","trace":[{"module":"github.com/ollama/ollama","version":"v0.40.1","package":"github.com/ollama/ollama/api","function":"Init"}]}}'
called_new='{"finding":{"osv":"GO-2099-0001","trace":[{"module":"example.com/x","version":"v1.0.0","package":"example.com/x","function":"Do"}]}}'
module_only='{"finding":{"osv":"GO-2026-5932","trace":[{"module":"golang.org/x/crypto","version":"v0.57.0"}]}}'

fail=0
check() {
  local name=$1 want=$2 stream=$3 out rc
  out=$(FAKE_STREAM="$stream" scripts/check-govulncheck.sh 2>&1); rc=$?
  if [ "$rc" -ne "$want" ]; then echo "FAIL $name: exit $rc, want $want" >&2; echo "$out" >&2; fail=1; else echo "ok   $name" >&2; fi
  printf '%s' "$out"
}
check "allowlisted called finding passes" 0 "$config"$'\n'"$called_ok" >/dev/null
out=$(check "unlisted called finding fails" 1 "$config"$'\n'"$called_ok"$'\n'"$called_new")
case "$out" in *GO-2099-0001*) ;; *) echo "FAIL unlisted finding not named in output"; echo "$out"; fail=1;; esac
check "module-level finding is outside the gate" 0 "$config"$'\n'"$module_only" >/dev/null
check "stream without a config record fails" 1 "$called_ok" >/dev/null
[ "$fail" -eq 0 ] && echo "PASS test-check-govulncheck"
exit "$fail"
