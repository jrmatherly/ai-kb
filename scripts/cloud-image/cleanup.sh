#!/usr/bin/env bash
# cleanup.sh - remove private data before building a cloud image.
# Warning: this script deletes SSH public keys, wipes the database and logs, and powers the machine off at the end.
# Afterwards, go straight to the cloud console and create the image / snapshot / AMI. Do not SSH back in.
set -euo pipefail

AIKB_DIR="${AIKB_DIR:-/opt/ai-kb}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "[cleanup] Run with sudo or as root" >&2
  exit 1
fi

read -r -p "[cleanup] This cannot be undone. Type YES to continue: " ans
if [[ "${ans}" != "YES" ]]; then
  echo "[cleanup] Cancelled"
  exit 0
fi

echo "[cleanup] 1/8 Stop the AI KB containers"
COMPOSE_PROJECT=""
if [[ -d "${AIKB_DIR}" ]]; then
  cd "${AIKB_DIR}"
  # Prefer compose ls for the real project name (defaults to the lowercase directory name, e.g. aikb)
  COMPOSE_PROJECT="$(docker compose ls --format json 2>/dev/null \
    | grep -oE '"Name":"[^"]+"' | head -1 | cut -d'"' -f4 || true)"
  docker compose down -v --remove-orphans || true
fi

echo "[cleanup] 2/8 Wipe AI KB data, the first-boot marker and logs"
if [[ -d "${AIKB_DIR}" ]]; then
  rm -rf "${AIKB_DIR}/data"/* "${AIKB_DIR}/logs"/* 2>/dev/null || true
  # Deliberately do not recreate .env here. With .env missing from the image, any docker compose
  # started before firstboot fails for lack of .env, so plaintext default passwords
  # (postgres123!@# and the like) never initialize the postgres data volume.
  # firstboot.sh copies .env.example itself and replaces the keys.
  rm -f "${AIKB_DIR}/.env" "${AIKB_DIR}/.firstboot.done"
fi
rm -f /root/ai-kb-credentials.txt /var/log/ai-kb-firstboot.log

echo "[cleanup] 3/8 Remove leftover docker volumes and build cache"
# Match strictly on the compose project name prefix so other postgres/redis volumes on the host are untouched.
if [[ -n "${COMPOSE_PROJECT}" ]]; then
  docker volume ls -q --filter "label=com.docker.compose.project=${COMPOSE_PROJECT}" \
    | xargs -r docker volume rm -f || true
fi
# Note: this clears only volumes, stopped containers and build cache. Never clear images.
# An earlier `docker system prune -af --volumes` also removed the wechatopenai/ai-kb-* images
# prepare.sh pre-pulls, so new instances built from the image had to pull several GB from
# Docker Hub again at firstboot, which defeats the point of preinstalling.
docker container prune -f      || true
docker builder    prune -af    || true
# Remove only unmounted dangling volumes (compose down -v has already removed the data volumes)
docker volume     prune -f     || true

echo "[cleanup] 4/8 Clear system logs"
journalctl --rotate || true
journalctl --vacuum-time=1s || true
find /var/log -type f \( -name '*.log' -o -name '*.gz' -o -name '*.[0-9]' \) -print0 \
  | xargs -0 -r truncate -s 0 || true
find /var/log -type f \( -name '*.gz' -o -name '*.[0-9]' \) -print0 \
  | xargs -0 -r rm -f || true

echo "[cleanup] 5/8 Remove SSH history and authorized keys (SSH access stops working after this)"
rm -f /root/.ssh/authorized_keys /root/.ssh/known_hosts /root/.bash_history
for d in /home/*; do
  [[ -d "$d" ]] || continue
  rm -f "$d/.ssh/authorized_keys" "$d/.ssh/known_hosts" "$d/.bash_history"
done
find / -xdev -type f \( -name 'id_rsa*' -o -name '*.pem' -o -name '*.key' \) \
  -not -path '/etc/ssl/*' -not -path '/usr/*' -not -path '/var/lib/docker/*' 2>/dev/null \
  | tee /tmp/cleanup-secrets-found.txt || true
echo "[cleanup]   ↑ Possible leftover key files above. Check them by hand if needed"

echo "[cleanup] 6/8 Reset cloud-init / machine-id (so new instances get a new ID)"
cloud-init clean --logs --seed 2>/dev/null || true
truncate -s 0 /etc/machine-id || true
rm -f /var/lib/dbus/machine-id || true

echo "[cleanup] 7/8 Clean apt / tmp"
if command -v apt-get >/dev/null 2>&1; then
  apt-get clean
  rm -rf /var/lib/apt/lists/*
fi
rm -rf /tmp/* /var/tmp/* /root/.cache /home/*/.cache 2>/dev/null || true

echo "[cleanup] 8/8 Sync disks and power off"
history -c || true
sync
echo
echo "  Powering off. When it is down, create the image / snapshot / AMI in the cloud console."
echo
sleep 3
poweroff
