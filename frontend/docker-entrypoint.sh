#!/bin/sh

# Only emit whitelisted locale tags to avoid config.js injection from env values.
RUNTIME_DEFAULT_LOCALE=""
case "${DEFAULT_LOCALE:-}" in
  en-US) RUNTIME_DEFAULT_LOCALE="${DEFAULT_LOCALE}" ;;
esac

# Write the runtime config that exposes env values to the frontend.
FILE_MB=${MAX_FILE_SIZE_MB:-50}
SKILL_MB=${MAX_SKILL_BUNDLE_SIZE_MB:-256}
if [ "$SKILL_MB" -lt "$FILE_MB" ] 2>/dev/null; then
  SKILL_MB=$FILE_MB
fi
if [ "$SKILL_MB" -gt 512 ] 2>/dev/null; then
  SKILL_MB=512
fi

cat > /usr/share/nginx/html/config.js << EOF
window.__RUNTIME_CONFIG__ = {
  MAX_FILE_SIZE_MB: ${FILE_MB},
  MAX_SKILL_BUNDLE_SIZE_MB: ${SKILL_MB},
  DEFAULT_LOCALE: "${RUNTIME_DEFAULT_LOCALE}"
};
EOF

# nginx config. The two limits are injected separately: the whole site keeps the
# knowledge base MAX_FILE_SIZE, and only the two skill zip upload collection routes
# are raised to MAX_SKILL_BUNDLE_SIZE (not sub-paths such as /install or PATCH).
# One merged site-wide limit would let every upload endpoint accept a skill-bundle-sized body.
export MAX_FILE_SIZE=${FILE_MB}M
export MAX_SKILL_BUNDLE_SIZE=${SKILL_MB}M
export APP_HOST=${APP_HOST:-app}
export APP_PORT=${APP_PORT:-8080}
export APP_SCHEME=${APP_SCHEME:-http}
envsubst '${MAX_FILE_SIZE} ${MAX_SKILL_BUNDLE_SIZE} ${APP_HOST} ${APP_PORT} ${APP_SCHEME}' \
  < /etc/nginx/templates/default.conf.template > /etc/nginx/conf.d/default.conf

exec nginx -g 'daemon off;'
