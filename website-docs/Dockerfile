# Build from the repository root with:
#   docker build --platform=linux/amd64 \
#     --build-arg WEBSITE_NGINX_PORT=8080 \
#     -t ai-kb-site website-docs
# No host Node.js installation or prebuilt static-site/ is required.
# WEBSITE_NGINX_PORT (default 8080, unprivileged) is written into the baked
# default.conf at build time. The intranet platform starts `nginx; sleep infinity`
# and does not run docker-entrypoint.sh; the entrypoint renders a runtime
# override under /tmp instead (see deploy/nginx-main.conf).
# The output is static files, so the builder runs on the build host and multi-arch CI never runs Node under QEMU.
FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS builder
RUN apt-get update \
    && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /build
ENV NEXT_TELEMETRY_DISABLED=1

# Install both locked dependency sets before copying source for layer caching.
COPY package.json package-lock.json ./
COPY homepage/package.json homepage/package-lock.json ./homepage/
RUN npm run setup

COPY . .
RUN npm run build

# Runtime base pinned by index digest, never the floating `nginx:stable-alpine`
# tag: nginx 1.30.5 on Alpine 3.24.2, the same base as the frontend image.
# Hosts on kernel 3.10 (CentOS 7) are not a target.
FROM nginx:1.30.5-alpine@sha256:0985e772fb9f729e6fa0980da05fca5d9c468e870eed43071545afa9d2e27d94 AS runtime
# Some deployment platforms wrap the startup command with /bin/bash -c.
RUN apk add --no-cache bash
# Baked into default.conf below. Entrypoint still honors a runtime override
# when it actually runs; the platform path does not.
ARG WEBSITE_NGINX_PORT=8080
ENV WEBSITE_NGINX_PORT=${WEBSITE_NGINX_PORT}
COPY deploy/nginx-main.conf /etc/nginx/nginx.conf
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
# The template keeps a literal ${WEBSITE_NGINX_PORT} for the nginx entrypoint's envsubst.
# hadolint ignore=SC2016
RUN sed 's#include /etc/nginx/conf.d/\*.conf;#include /tmp/nginx/conf.d/*.conf;#' \
      /etc/nginx/nginx.conf > /etc/nginx/nginx-runtime.conf \
    && grep -q 'include /tmp/nginx/conf.d/\*.conf;' /etc/nginx/nginx-runtime.conf \
    && mkdir -p /etc/nginx/templates \
    && sed 's/listen 80;/listen ${WEBSITE_NGINX_PORT};/' /etc/nginx/conf.d/default.conf \
      > /etc/nginx/templates/default.conf.template \
    && sed "s/listen 80;/listen ${WEBSITE_NGINX_PORT};/" /etc/nginx/conf.d/default.conf \
      > /etc/nginx/conf.d/default.conf.baked \
    && mv /etc/nginx/conf.d/default.conf.baked /etc/nginx/conf.d/default.conf \
    && grep -q "listen ${WEBSITE_NGINX_PORT};" /etc/nginx/conf.d/default.conf \
    && rm -rf /usr/share/nginx/html/*
COPY --from=builder /build/static-site/ /usr/share/nginx/html/
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
EXPOSE ${WEBSITE_NGINX_PORT}
# The base image's nginx user. Writes go to /tmp only (see deploy/nginx-main.conf).
USER 101:101
ENTRYPOINT ["/docker-entrypoint.sh"]
