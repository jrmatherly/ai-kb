#!/usr/bin/env bash
# firstboot.sh - run by ai-kb-firstboot.service on a new instance's first boot.
# Job: generate random keys into .env -> start the containers -> print credentials -> mark done and disable itself.
#
# Idempotency:
#   1) Write ${MARKER} first, meaning "keys already generated".
#      If docker compose then fails and the script aborts, after a reboot the unit's
#      ConditionPathExists=!${MARKER} stops it, so new keys never overwrite .env.
#      (The old keys are already in the postgres data volume. Generating again would lock the database for good.)
#   2) On failure the unit is marked failed. The user can recover with docker compose up -d;
#      the credentials are still in ${ENV_FILE}.
set -euo pipefail

AIKB_DIR="${AIKB_DIR:-/opt/ai-kb}"
ENV_FILE="${AIKB_DIR}/.env"
ENV_TEMPLATE="${AIKB_DIR}/.env.example"
CRED_FILE="/root/ai-kb-credentials.txt"
LOG_FILE="/var/log/ai-kb-firstboot.log"
MARKER="${AIKB_DIR}/.firstboot.done"

# Open LOG_FILE early and also copy stderr to the systemd journal for debugging
# (with a plain exec >> LOG_FILE, early failures are invisible because stderr is swallowed)
mkdir -p "$(dirname "${LOG_FILE}")"
exec > >(tee -a "${LOG_FILE}") 2>&1
echo "==== firstboot started at $(date -Iseconds) ===="

if [[ -f "${MARKER}" ]]; then
  echo "marker ${MARKER} exists, skip (already initialized)"
  exit 0
fi

# cleanup.sh no longer keeps .env, so copy the template from .env.example and substitute.
# This guarantees no .env with plaintext default passwords exists before firstboot, so aikb.service
# cannot initialize the postgres data volume with the wrong password first.
if [[ ! -f "${ENV_FILE}" ]]; then
  if [[ -f "${ENV_TEMPLATE}" ]]; then
    echo "creating ${ENV_FILE} from ${ENV_TEMPLATE}"
    cp "${ENV_TEMPLATE}" "${ENV_FILE}"
  else
    echo "ERROR: neither ${ENV_FILE} nor ${ENV_TEMPLATE} found"
    exit 1
  fi
fi

DOCKER_BIN="$(command -v docker || true)"
if [[ -z "${DOCKER_BIN}" ]]; then
  echo "ERROR: docker binary not found in PATH"
  exit 1
fi

# 32-byte strong random string (for the AES-256 key, which must be exactly 32 bytes)
# Run in a `() ... ()` subshell with pipefail off: head closes stdin after N bytes,
# tr gets SIGPIPE (exit 141), `set -o pipefail` fails the whole pipeline,
# and the top-level `set -e` kills firstboot.sh with exit 1 within 8ms.
gen32() ( set +o pipefail; LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32 )
# General password: 24 characters, no / + = (they break URLs and sed substitution)
genpw() ( set +o pipefail; LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 24 )

DB_PWD=$(genpw)
REDIS_PWD=$(genpw)
JWT=$(genpw)$(genpw)
SYS_AES=$(gen32)

# Use | as the sed delimiter to avoid conflicts; replace only lines that start with KEY=
replace() {
  local key="$1" val="$2"
  if grep -qE "^${key}=" "${ENV_FILE}"; then
    sed -i "s|^${key}=.*|${key}=${val}|" "${ENV_FILE}"
  else
    echo "${key}=${val}" >>"${ENV_FILE}"
  fi
}

replace DB_PASSWORD     "${DB_PWD}"
replace REDIS_PASSWORD  "${REDIS_PWD}"
replace JWT_SECRET      "${JWT}"
replace SYSTEM_AES_KEY  "${SYS_AES}"
replace GIN_MODE        "release"

# Restore the AIKB_REF that prepare.sh recorded in .cloud-image-meta as AIKB_VERSION in .env.
# Otherwise docker compose falls back to the :latest default and the image version
# differs from the one prepare pulled.
META_FILE="${AIKB_DIR}/.cloud-image-meta"
if [[ -f "${META_FILE}" ]]; then
  META_REF=$(grep -E '^AIKB_REF=' "${META_FILE}" | tail -1 | cut -d= -f2- || true)
  if [[ -n "${META_REF}" ]]; then
    replace AIKB_VERSION "${META_REF}"
    echo "restored AIKB_VERSION=${META_REF} from ${META_FILE}"
  fi
fi

# Key point: write the marker right after .env changes.
# From here on, even if docker compose up fails, a reboot never rewrites .env,
# which keeps it consistent with the password postgres already persisted.
umask 077
touch "${MARKER}"
chmod 0600 "${MARKER}"

echo "env updated, marker written, starting docker compose..."
cd "${AIKB_DIR}"
"${DOCKER_BIN}" compose up -d

# Try the public IP, fall back to the private one
PUB_IP=$(curl -fsS --max-time 5 https://ifconfig.me 2>/dev/null \
  || curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null \
  || hostname -I | awk '{print $1}')

cat >"${CRED_FILE}" <<INFO
========================================
  AI KB instance initialized
  Generated at: $(date -Iseconds)
========================================

URL : http://${PUB_IP}

To disable further sign-ups after you register, edit ${ENV_FILE}:
    DISABLE_REGISTRATION=true
Then run:  cd ${AIKB_DIR} && docker compose up -d

These random credentials are in ${ENV_FILE}. Keep them safe (readable by root only):
  DB_PASSWORD     = ${DB_PWD}
  REDIS_PASSWORD  = ${REDIS_PWD}
  JWT_SECRET      = ${JWT}
  SYSTEM_AES_KEY  = ${SYS_AES}

Notes:
  - This file shows only the keys at first boot. ${ENV_FILE} is the source of truth afterwards.
  - Never expose infrastructure ports such as 5432 / 6379 / 9000 publicly.
  - Serve only on 80 / 443. Put a reverse proxy + HTTPS in front if needed.

INFO

echo "credentials written to ${CRED_FILE}"

# Stop the unit only, do not delete the unit file (otherwise systemd may mark the running oneshot failed).
# On the next boot, ai-kb-firstboot.service skips itself through ConditionPathExists=!${MARKER}.
systemctl disable ai-kb-firstboot.service || true

echo "==== firstboot finished at $(date -Iseconds) ===="
