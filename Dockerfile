# syntax=docker/dockerfile:1.7-labs
# Full image: webpack + go inside Docker (~15–25 min cold). Fast image: see target gitea-fast + docker-compose.local-fast.yml
#
# Build context must include the sibling Go module github.com/gitjet-ru/git-storage (go.mod replace => ../git-storage).
# docker-compose.yml uses context: .. (monorepo root: core-scm + git-storage) and args SRC_ROOT=core-scm — no git-storage *service* required.
# Manual: docker build -f core-scm/Dockerfile --target gitea -t core-scm:gitea --build-arg SRC_ROOT=core-scm ..
#
# Build frontend on the native platform to avoid QEMU-related issues with esbuild/webpack
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.26-alpine3.23 AS frontend-build
RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache build-base git nodejs pnpm
WORKDIR /src
ARG SRC_ROOT=.
COPY ${SRC_ROOT}/package.json ${SRC_ROOT}/pnpm-lock.yaml ${SRC_ROOT}/.npmrc ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile
COPY --exclude=.git/ ${SRC_ROOT}/ ./
RUN make frontend

# Shared backend prep (no assets, no compile yet)
FROM docker.io/library/golang:1.26-alpine3.23 AS build-env-base

ARG GITEA_VERSION
ARG TAGS=""
ENV TAGS="bindata timetzdata $TAGS"
ARG CGO_EXTRA_CFLAGS

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache \
    build-base \
    git

WORKDIR ${GOPATH}/src/github.com/gitjet-ru/core-scm
ARG SRC_ROOT=.
COPY ${SRC_ROOT}/go.mod ${SRC_ROOT}/go.sum ./
# Monorepo dev mode: provide replaced git-storage module if present in build context.
COPY git-storage /go/src/github.com/gitjet-ru/git-storage
RUN go mod download
# Use COPY instead of bind mount as read-only one breaks makefile state tracking and read-write one needs binary to be moved as it's discarded.
# ".git" directory is mounted separately later only for version data extraction.
COPY --exclude=.git/ ${SRC_ROOT}/ ./

# Default path: assets from webpack stage above
FROM build-env-base AS build-env
ARG SRC_ROOT=.
COPY --from=frontend-build /src/public/assets public/assets

# Build gitea, .git mount is required for version data
RUN --mount=type=cache,target="/root/.cache/go-build" \
    make backend

COPY ${SRC_ROOT}/docker/root /tmp/local

# Set permissions for builds that made under windows which strips the executable bit from file
RUN chmod 755 /tmp/local/usr/bin/entrypoint \
              /tmp/local/usr/local/bin/* \
              /tmp/local/etc/s6/gitea/* \
              /tmp/local/etc/s6/openssh/* \
              /tmp/local/etc/s6/.s6-svscan/* \
              /go/src/github.com/gitjet-ru/core-scm/gitea

# Fast local path: skip webpack in Docker — build assets on host first: (cd core-scm && make frontend)
# Build: docker compose ... -f docker-compose.local-fast.yml build gitea
#   or: docker build --build-context prebuilt=./core-scm/public/assets --target gitea-fast ...
FROM build-env-base AS build-env-fast
ARG SRC_ROOT=.
COPY --from=prebuilt / public/assets
RUN --mount=type=cache,target="/root/.cache/go-build" \
    make backend
COPY ${SRC_ROOT}/docker/root /tmp/local
RUN chmod 755 /tmp/local/usr/bin/entrypoint \
              /tmp/local/usr/local/bin/* \
              /tmp/local/etc/s6/gitea/* \
              /tmp/local/etc/s6/openssh/* \
              /tmp/local/etc/s6/.s6-svscan/* \
              /go/src/github.com/gitjet-ru/core-scm/gitea

FROM docker.io/library/alpine:3.23 AS gitea

EXPOSE 22 3000

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache \
    bash \
    ca-certificates \
    curl \
    gettext \
    git \
    linux-pam \
    openssh \
    s6 \
    su-exec \
    gnupg

RUN addgroup \
    -S -g 1000 \
    git && \
  adduser \
    -S -H -D \
    -h /data/git \
    -s /bin/bash \
    -u 1000 \
    -G git \
    git && \
  echo "git:*" | chpasswd -e

COPY --from=build-env /tmp/local /
COPY --from=build-env /go/src/github.com/gitjet-ru/core-scm/gitea /app/gitea/gitea

ENV USER=git
ENV GITEA_CUSTOM=/data/gitea

VOLUME ["/data"]

# HINT: HEALTH-CHECK-ENDPOINT: don't use HEALTHCHECK, search this hint keyword for more information
ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/usr/bin/s6-svscan", "/etc/s6"]

# Same runtime as gitea; binary/assets from build-env-fast (host-built public/assets)
FROM docker.io/library/alpine:3.23 AS gitea-fast

EXPOSE 22 3000

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache \
    bash \
    ca-certificates \
    curl \
    gettext \
    git \
    linux-pam \
    openssh \
    s6 \
    su-exec \
    gnupg

RUN addgroup \
    -S -g 1000 \
    git && \
  adduser \
    -S -H -D \
    -h /data/git \
    -s /bin/bash \
    -u 1000 \
    -G git \
    git && \
  echo "git:*" | chpasswd -e

COPY --from=build-env-fast /tmp/local /
COPY --from=build-env-fast /go/src/github.com/gitjet-ru/core-scm/gitea /app/gitea/gitea

ENV USER=git
ENV GITEA_CUSTOM=/data/gitea

VOLUME ["/data"]

ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/usr/bin/s6-svscan", "/etc/s6"]

# Build geesefs once (for local/dev S3 mount image)
FROM docker.io/library/alpine:3.23 AS geesefs-build
ARG GEESEFS_VERSION=0.43.5
ARG TARGETARCH
RUN apk add --no-cache curl && \
    case "${TARGETARCH}" in \
      amd64) ARCH=amd64 ;; \
      arm64) ARCH=arm64 ;; \
      *) echo "Unsupported TARGETARCH: ${TARGETARCH}" && exit 1 ;; \
    esac && \
    curl -fsSL "https://github.com/yandex-cloud/geesefs/releases/download/v${GEESEFS_VERSION}/geesefs-linux-${ARCH}" \
      -o /usr/local/bin/geesefs && \
    chmod +x /usr/local/bin/geesefs

# Local / dev image: mount S3 over /data/git via geesefs before starting Gitea.
# Build: docker build --target gitea-s3fs -t core-scm:s3fs .
FROM gitea AS gitea-s3fs

RUN apk add --no-cache fuse3 util-linux

COPY --from=geesefs-build /usr/local/bin/geesefs /usr/local/bin/geesefs

ARG SRC_ROOT=.
COPY ${SRC_ROOT}/docker/s3fs-local/entrypoint-s3fs.sh /usr/local/bin/entrypoint-s3fs.sh
RUN chmod 755 /usr/local/bin/entrypoint-s3fs.sh

ENTRYPOINT ["/usr/local/bin/entrypoint-s3fs.sh"]
CMD ["/usr/bin/s6-svscan", "/etc/s6"]
