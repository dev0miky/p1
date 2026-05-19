#!/bin/sh
set -e

if [ -d /data ]; then
  chown -R app:app /data 2>/dev/null || true
fi

exec su-exec app:app /app/app "$@"
