#!/bin/sh
set -e

if [ -d /data ]; then
  chown -R app:app /data 2>/dev/null || true
fi

# FreeSWITCH's first mount of the shared gateway volume copies its image's
# external/ contents in as root and roots the dir; ensure app can write it.
mkdir -p "${GATEWAY_DIR:-/fsgw}" 2>/dev/null || true
chown -R app:app "${GATEWAY_DIR:-/fsgw}" 2>/dev/null || true

exec su-exec app:app /app/app "$@"
