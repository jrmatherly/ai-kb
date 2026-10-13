#!/usr/bin/env bash
# prepare.sh - deploy the AI KB runtime on a clean Linux instance to build a cloud image template.
# Does not clone the whole AI KB repo. It downloads only 4 runtime files (~100KB).
# Supports systemd + Docker distributions such as Ubuntu / Debian / CentOS / Rocky / TencentOS.
# Usage:  sudo bash prepare.sh
# Environment variables:
#   AIKB_REF              git ref to fetch (tag / branch / commit), default main
#   AIKB_DIR              deployment directory, default /opt/ai-kb
#   AIKB_REPO             repository URL, default https://github.com/jrmatherly/ai-kb
#   AIKB_GH_PROXY         GitHub proxy prefix, default empty. Machines in mainland China can set
#                            https://gh-proxy.com/ or https://ghfast.top/
#                            (the download URL becomes ${AIKB_GH_PROXY}${AIKB_REPO}/archive/...)
#   DOCKER_INSTALL_MIRROR    Docker package mirror, default empty (uses get.docker.com).
#                            Set it when overseas CDNs are unreachable from mainland China, for example:
#                              https://mirrors.tencent.com/docker-ce/linux/ubuntu
#                              https://mirrors.aliyun.com/docker-ce/linux/ubuntu
#                            Installs through apt from a mirror of the official docker-ce repository,
#                            including docker-ce / containerd.io / docker-compose-plugin,
#                            and never contacts get.docker.com. apt-based distributions only.
#   DOCKER_REGISTRY_MIRROR   Docker Hub mirror, default empty. On the Tencent Cloud internal network, set
#                            https://mirror.ccs.tencentyun.com
#                            (written to /etc/docker/daemon.json, then docker restarts)
#   PRUNE_OLD_IMAGES         Whether an upgrade removes dangling / old-tag images,
#                            default false. When true, after pulling the new images it runs
#                            `docker image prune -af`, which deletes every image no container references
#                            (including wechatopenai/ai-kb-* at the old AIKB_VERSION)
#                            in one go, to shrink the cloud image.
set -euo pipefail

AIKB_REF="${AIKB_REF:-main}"
AIKB_DIR="${AIKB_DIR:-/opt/ai-kb}"
AIKB_REPO="${AIKB_REPO:-https://github.com/jrmatherly/ai-kb}"
AIKB_GH_PROXY="${AIKB_GH_PROXY:-}"
DOCKER_INSTALL_MIRROR="${DOCKER_INSTALL_MIRROR:-}"
DOCKER_REGISTRY_MIRROR="${DOCKER_REGISTRY_MIRROR:-}"
PRUNE_OLD_IMAGES="${PRUNE_OLD_IMAGES:-false}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

if [[ "${EUID}" -ne 0 ]]; then
  echo "[prepare] Run with sudo or as root" >&2
  exit 1
fi

# Install the docker-ce suite (with compose-plugin) through apt from a mirror.
# For mainland China cloud hosts where direct connections to get.docker.com are reset.
install_docker_via_apt_mirror() {
  local mirror="$1"
  if ! command -v apt-get >/dev/null 2>&1; then
    echo "[prepare] DOCKER_INSTALL_MIRROR supports only apt-based distributions (Ubuntu/Debian)" >&2
    return 1
  fi
  apt-get update -y
  apt-get install -y ca-certificates curl gnupg lsb-release
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL "${mirror%/}/gpg" | gpg --dearmor --yes -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  local arch codename
  arch="$(dpkg --print-architecture)"
  codename="$(lsb_release -cs)"
  echo "deb [arch=${arch} signed-by=/etc/apt/keyrings/docker.gpg] ${mirror%/} ${codename} stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -y
  apt-get install -y docker-ce docker-ce-cli containerd.io \
                     docker-buildx-plugin docker-compose-plugin curl tar
}

echo "[prepare] 1/6 Install Docker and dependencies"
if ! command -v docker >/dev/null 2>&1; then
  if [[ -n "${DOCKER_INSTALL_MIRROR}" ]]; then
    echo "[prepare]   Installing docker-ce through apt from ${DOCKER_INSTALL_MIRROR} (skipping get.docker.com)"
    install_docker_via_apt_mirror "${DOCKER_INSTALL_MIRROR}"
  else
    curl -fsSL https://get.docker.com | bash
  fi
fi
systemctl enable --now docker

if ! docker compose version >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y docker-compose-plugin curl tar
  elif command -v yum >/dev/null 2>&1; then
    yum install -y docker-compose-plugin curl tar
  fi
fi

# Optional: configure a Docker Hub mirror when direct connections to registry-1.docker.io time out
# (typically mainland China cloud hosts or restricted internal networks). daemon.json is touched only when the user passes it.
if [[ -n "${DOCKER_REGISTRY_MIRROR}" ]]; then
  echo "[prepare] 1.5/6 Configure the Docker Hub mirror: ${DOCKER_REGISTRY_MIRROR}"
  mkdir -p /etc/docker
  # Merge an existing daemon.json with python so other user settings are kept
  if [[ -s /etc/docker/daemon.json ]] && command -v python3 >/dev/null 2>&1; then
    python3 - "$DOCKER_REGISTRY_MIRROR" <<'PY'
import json, sys, pathlib
p = pathlib.Path("/etc/docker/daemon.json")
mirror = sys.argv[1]
try:
    cfg = json.loads(p.read_text())
except Exception:
    cfg = {}
mirrors = cfg.get("registry-mirrors") or []
if mirror not in mirrors:
    mirrors.insert(0, mirror)
cfg["registry-mirrors"] = mirrors
p.write_text(json.dumps(cfg, indent=2) + "\n")
PY
  else
    cat >/etc/docker/daemon.json <<EOF
{
  "registry-mirrors": ["${DOCKER_REGISTRY_MIRROR}"]
}
EOF
  fi
  systemctl restart docker
  # Give the docker daemon time to restart so the docker compose pull below does not hit EOF
  for _ in 1 2 3 4 5; do
    docker info >/dev/null 2>&1 && break
    sleep 1
  done
fi

echo "[prepare] 2/6 Fetch the AI KB runtime files (ref=${AIKB_REF})"
# Download only the 4 files needed, no full clone (~MB -> ~KB)
mkdir -p "${AIKB_DIR}/config"

tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT

tarball_url="${AIKB_GH_PROXY}${AIKB_REPO}/archive/${AIKB_REF}.tar.gz"
echo "[prepare]   tarball: ${tarball_url}"
curl -fsSL "${tarball_url}" -o "${tmp}/repo.tar.gz"
# Extract only the needed paths, which is much faster and smaller
tar -xzf "${tmp}/repo.tar.gz" -C "${tmp}" \
  --wildcards \
  '*/docker-compose.yml' \
  '*/.env.example' \
  '*/config/config.yaml'
src=$(find "${tmp}" -maxdepth 1 -mindepth 1 -type d -name 'AI KB-*' | head -1)
if [[ -z "${src}" ]]; then
  echo "[prepare] Extraction failed, no AI KB-* directory found" >&2
  exit 1
fi

cp    "${src}/docker-compose.yml" "${AIKB_DIR}/"
cp    "${src}/.env.example"       "${AIKB_DIR}/"
cp    "${src}/config/config.yaml" "${AIKB_DIR}/config/"

# Record metadata for firstboot and upgrades
cat >"${AIKB_DIR}/.cloud-image-meta" <<EOF
AIKB_REF=${AIKB_REF}
AIKB_REPO=${AIKB_REPO}
PREPARED_AT=$(date -Iseconds)
EOF

echo "[prepare] 3/6 Prepare .env (defaults, firstboot replaces them with random keys)"
cd "${AIKB_DIR}"
[[ -f .env ]] || cp .env.example .env
sed -i 's/^GIN_MODE=.*/GIN_MODE=release/' .env || true

# Align AIKB_VERSION with AIKB_REF so docker compose pulls the image tag that matches
# the ref. Always overwrite, so .env never keeps an old version from a previous prepare.
# Tag naming for wechatopenai/ai-kb-* on Docker Hub:
#   - floating tag: main (always points at the latest build)
#   - fixed release tag: v prefix + semver (such as v0.7.2, v0.5.2)
# So the v is not stripped here and nothing maps to latest.
AIKB_VERSION_VAL="${AIKB_REF}"
if grep -qE '^AIKB_VERSION=' .env; then
  sed -i "s|^AIKB_VERSION=.*|AIKB_VERSION=${AIKB_VERSION_VAL}|" .env
else
  echo "AIKB_VERSION=${AIKB_VERSION_VAL}" >>.env
fi
echo "[prepare]   -> AIKB_VERSION=${AIKB_VERSION_VAL}"

echo "[prepare] 4/6 Pull and start the 5 default long-running containers (frontend/app/docreader/postgres/redis)"
docker compose pull
docker compose up -d

# Pull the sandbox image ahead of time (the app docker-runs the Agent Skills runtime on demand, not long-running)
# Without the pre-pull, a user's first Skill run stalls on the download
echo "[prepare] 4.5/6 Pre-pull the sandbox image (for Agent Skills, not long-running)"
docker compose --profile full pull sandbox || true

# Other vector stores and observability components (qdrant, milvus, weaviate, doris, neo4j, langfuse-*, minio, dex)
# are not pre-pulled, which saves 5-15GB. To enable one:
#   cd /opt/ai-kb && docker compose --profile <name> up -d

# Upgrade: remove wechatopenai/ai-kb-* images with old tags.
# Off by default to keep a rollback path. Turn it on explicitly before building the image to shrink it.
#
# Note: do not use `docker image prune -af`!
# compose only pulls the sandbox image and never brings it up (the app docker-runs Agent Skills on demand),
# so no container references it, and `prune -a` would delete the current sandbox too,
# which defeats the sandbox pre-pull in step 4.5 of prepare.sh.
# This compares tags exactly and removes only wechatopenai/ai-kb-* images whose tag differs from the current
# AIKB_VERSION. Infrastructure images (paradedb / redis) are untouched.
if [[ "${PRUNE_OLD_IMAGES,,}" == "true" || "${PRUNE_OLD_IMAGES}" == "1" ]]; then
  echo "[prepare] 4.6/6 Remove old wechatopenai/ai-kb-* images (PRUNE_OLD_IMAGES=true, keep=${AIKB_VERSION_VAL})"
  docker image ls --format '{{.Repository}}:{{.Tag}}' \
    | grep -E '^wechatopenai/ai-kb-' \
    | grep -vE ":${AIKB_VERSION_VAL}\$" \
    | xargs -r docker rmi -f 2>/dev/null || true
fi

echo "[prepare] 5/6 Install systemd units"
# Find the docker binary; distributions put it in /usr/bin or /usr/local/bin
DOCKER_BIN="$(command -v docker)"
if [[ -z "${DOCKER_BIN}" ]]; then
  echo "[prepare] docker binary not found" >&2
  exit 1
fi
echo "[prepare]   docker binary: ${DOCKER_BIN}"

install -m 0644 "${SCRIPT_DIR}/systemd/aikb.service"           /etc/systemd/system/aikb.service
install -m 0644 "${SCRIPT_DIR}/systemd/ai-kb-firstboot.service" /etc/systemd/system/ai-kb-firstboot.service
install -m 0755 "${SCRIPT_DIR}/firstboot.sh"                      /usr/local/sbin/ai-kb-firstboot.sh

# Replace the docker path placeholder in the systemd units with the real path
sed -i "s|@DOCKER_BIN@|${DOCKER_BIN}|g" /etc/systemd/system/aikb.service

systemctl daemon-reload
systemctl enable aikb.service
systemctl enable ai-kb-firstboot.service

echo "[prepare] 6/6 Done"
echo
echo "  AI KB runtime deployed to ${AIKB_DIR}"
echo "    docker-compose.yml / config/config.yaml / .env"
echo "  Version: ${AIKB_REF}  (see ${AIKB_DIR}/.cloud-image-meta)"
echo
echo "  Open http://<public IP of this host> in a browser to check it works"
echo
echo "  Once it checks out, clean up and build the image:"
echo "      sudo bash ${SCRIPT_DIR}/cleanup.sh"
