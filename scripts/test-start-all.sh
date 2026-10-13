#!/usr/bin/env bash
# Verify start_all.sh installs Ollama on macOS without Homebrew from the real
# release layout (a zip holding only Ollama.app), with the download stubbed.
# Usage: scripts/test-start-all.sh [path/to/start_all.sh]
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="${1:-$repo_root/scripts/start_all.sh}"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/bin" "$fixture/cwd" "$fixture/usr/local/bin" "$fixture/release/Ollama.app/Contents/Resources" "$fixture/Applications"

printf '#!/bin/sh\necho "ollama version is 0.0.0-test"\n' > "$fixture/release/Ollama.app/Contents/Resources/ollama"
chmod +x "$fixture/release/Ollama.app/Contents/Resources/ollama"
(cd "$fixture/release" && python3 -c 'import shutil; shutil.make_archive("Ollama-darwin", "zip", ".", "Ollama.app")')

cat > "$fixture/bin/curl" <<EOF
#!/bin/sh
echo "\$*" >> "$fixture/curl.log"
while [ \$# -gt 0 ]; do
  if [ "\$1" = -o ]; then cp "$fixture/release/Ollama-darwin.zip" "\$2"; exit 0; fi
  shift
done
exit 1
EOF
printf '#!/bin/sh\necho Darwin\n' > "$fixture/bin/uname"
chmod +x "$fixture/bin/curl" "$fixture/bin/uname"

# Run the script's install_ollama with the system paths moved into the fixture.
run_install() {
  local body
  body="$(sed -n '/^install_ollama() {/,/^}/p' "$script" |
    sed -e "s|/Applications|$fixture/Applications|g" -e "s|/usr/local/bin|$fixture/usr/local/bin|g")"
  PATH="$fixture/bin:/usr/bin:/bin" bash -c '
    log_info() { :; }; log_success() { :; }; log_error() { echo "$*" >&2; }
    get_ollama_base_url() { IS_REMOTE=0; }
    eval "$1"
    cd "$2" && install_ollama' bash "$body" "$fixture/cwd"
}

fail() { echo "FAIL: $*" >&2; exit 1; }

run_install || fail "install_ollama returned non-zero"
[ -L "$fixture/usr/local/bin/ollama" ] || fail "no ollama symlink on the PATH dir"
[ "$("$fixture/usr/local/bin/ollama")" = "ollama version is 0.0.0-test" ] || fail "linked CLI does not run the bundled binary"
[ "$(wc -l < "$fixture/curl.log" | tr -d ' ')" = 1 ] || fail "expected one download"

rm "$fixture/usr/local/bin/ollama"
run_install || fail "relink with Ollama.app present returned non-zero"
[ "$("$fixture/usr/local/bin/ollama")" = "ollama version is 0.0.0-test" ] || fail "relink did not restore the CLI"
[ "$(wc -l < "$fixture/curl.log" | tr -d ' ')" = 1 ] || fail "downloaded again although Ollama.app exists"

echo "start_all.sh macOS Ollama install: ok"
