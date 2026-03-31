#!/bin/sh
set -e

# Passthrough: run stock Gitea entrypoint when S3 bucket is not configured.
if [ -z "${S3_BUCKET}" ]; then
    exec /usr/bin/entrypoint "$@"
fi

if [ -z "${AWS_ACCESS_KEY_ID}" ] || [ -z "${AWS_SECRET_ACCESS_KEY}" ]; then
    echo "s3 mode: set S3_BUCKET, AWS_ACCESS_KEY_ID, and AWS_SECRET_ACCESS_KEY" >&2
    exit 1
fi

mkdir -p /data/git

if mountpoint -q /data/git 2>/dev/null; then
    fusermount3 -u /data/git 2>/dev/null || umount /data/git 2>/dev/null || true
fi

DBG=""
if [ "${DEBUG_GEESEFS:-${DEBUG_S3FS}}" = "1" ]; then
    DBG="--debug_s3 --debug_fuse"
fi

CERT_OPT=""
if [ "${S3_NO_CHECK_CERT}" = "1" ]; then
    CERT_OPT="--no-verify-ssl"
fi

ENDPOINT_OPTS=""
if [ -n "${S3_ENDPOINT}" ]; then
    ENDPOINT_OPTS="--endpoint ${S3_ENDPOINT}"
fi

REGION_OPT=""
if [ -n "${S3_REGION}" ]; then
    REGION_OPT="--region ${S3_REGION}"
fi

# geesefs uses path-style by default. For virtual-host style, enable --subdomain.
PATH_STYLE_OPT=""
if [ "${S3_USE_PATH_STYLE:-1}" = "0" ]; then
    PATH_STYLE_OPT="--subdomain"
fi

# geesefs stays in foreground with -f; keep it in background
# so this script can continue and start Gitea.
# shellcheck disable=SC2086
geesefs \
    --uid "${S3FS_UID:-1000}" \
    --gid "${S3FS_GID:-1000}" \
    ${ENDPOINT_OPTS} \
    ${CERT_OPT} \
    ${REGION_OPT} \
    ${PATH_STYLE_OPT} \
    ${DBG} \
    ${GEESEFS_EXTRA_OPTS} \
    -f \
    "${S3_BUCKET}" /data/git &
MOUNT_PID=$!

i=0
while ! grep -qE "[[:space:]]/data/git[[:space:]]fuse\\.geesefs[[:space:]]" /proc/mounts; do
    if ! kill -0 "${MOUNT_PID}" 2>/dev/null; then
        echo "geesefs: process exited before mount succeeded (check credentials, S3_ENDPOINT, S3_REGION, path-style). Set DEBUG_GEESEFS=1 for details." >&2
        exit 1
    fi
    i=$((i + 1))
    if [ "$i" -gt 120 ]; then
        echo "geesefs: mount did not become ready at /data/git after 120s" >&2
        exit 1
    fi
    sleep 1
done

exec /usr/bin/entrypoint "$@"
