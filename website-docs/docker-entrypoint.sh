#!/bin/sh
set -eu

export WEBSITE_NGINX_PORT="${WEBSITE_NGINX_PORT:-8080}"

# Render under /tmp so the root filesystem can stay read-only.
# Substitute only the port; preserve Nginx variables such as $uri.
mkdir -p /tmp/nginx/conf.d
envsubst '${WEBSITE_NGINX_PORT}' \
  < /etc/nginx/templates/default.conf.template \
  > /tmp/nginx/conf.d/default.conf

# Match the frontend entrypoint: configure and start Nginx even with no arguments.
exec nginx -c /etc/nginx/nginx-runtime.conf -g 'daemon off;'
