#!/usr/bin/env bash
set -euo pipefail

# Prove the aikb formula shape locally without any remote write: build the
# CLI, pack it the way release-lite.yml does, write an aikb-test formula into
# the local test tap and install it with Homebrew.
#
# Usage:
#   ./scripts/test-tap-aikb.sh            # build, pack, install, run --version
#   KEEP=1 ./scripts/test-tap-aikb.sh     # leave aikb-test and the tap installed

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TAG="v0.0.0-local"
GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
STAGE="aikb_${TAG}_${GOOS}_${GOARCH}"
WORK="$(mktemp -d)"
TAP_NAME="aikb/test"
TAP_DIR="$(brew --repository)/Library/Taps/aikb/homebrew-test"
FORMULA_NAME="aikb-test"
# Untap as well as uninstall: a remote-less tap left behind makes every later
# brew command warn that it is untrusted and cannot update.
cleanup() {
    rm -rf "${WORK}"
    [ "${KEEP:-}" = "1" ] && return 0
    brew uninstall -q "${TAP_NAME}/${FORMULA_NAME}" 2>/dev/null || true
    brew untap -q "${TAP_NAME}" 2>/dev/null || true
}
trap cleanup EXIT

echo "=== Step 1: Build and pack ${STAGE} ==="
CGO_ENABLED=0 make -s -C "${ROOT_DIR}/cli" build VERSION="${TAG}" BUILD_DIR="${WORK}/${STAGE}" BINARY=aikb
cp "${ROOT_DIR}/LICENSE" "${WORK}/${STAGE}/"
tar czf "${WORK}/${STAGE}.tar.gz" -C "${WORK}/${STAGE}" aikb LICENSE
SHA="$(shasum -a 256 "${WORK}/${STAGE}.tar.gz" | awk '{print $1}')"
echo "  ${WORK}/${STAGE}.tar.gz ${SHA}"

echo "=== Step 2: Write the formula into ${TAP_NAME} ==="
if [ ! -d "${TAP_DIR}" ]; then
    mkdir -p "${TAP_DIR}/Formula"
    (cd "${TAP_DIR}" && git init -q && git commit --allow-empty -m "init" -q)
fi
mkdir -p "${TAP_DIR}/Formula"
cat > "${TAP_DIR}/Formula/${FORMULA_NAME}.rb" << RUBY
class AikbTest < Formula
  desc "Command-line client for AI KB knowledge bases (local test build)"
  homepage "https://ai-kb.matherly.net/docs/05-clients/02-cli"
  url "file://${WORK}/${STAGE}.tar.gz"
  version "0.0.0-local"
  sha256 "${SHA}"
  license "MIT"

  def install
    bin.install "aikb" => "${FORMULA_NAME}"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/${FORMULA_NAME} --version")
  end
end
RUBY
(cd "${TAP_DIR}" && git add -A && git commit -q -m "aikb-test ${TAG}" --allow-empty)

echo "=== Step 3: Install and run ==="
brew reinstall -q "${TAP_NAME}/${FORMULA_NAME}" 2>/dev/null || brew install -q "${TAP_NAME}/${FORMULA_NAME}"
"$(brew --prefix)/bin/${FORMULA_NAME}" --version
brew test "${TAP_NAME}/${FORMULA_NAME}"

if [ "${KEEP:-}" != "1" ]; then
    echo "=== Done: ${FORMULA_NAME} installed and ran; removing it and ${TAP_NAME} ==="
else
    echo "=== Done: ${FORMULA_NAME} left installed ==="
fi
