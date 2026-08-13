#!/bin/sh
# Snagarr owns one directory. This script gives it to PUID:PGID and drops to
# that user before the binary starts, so a bind mount from the host ends up
# owned by the person who mounted it instead of by root.
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}
UMASK=${UMASK:-022}
DATA_DIR=${SNAGARR_DATA_DIR:-/data}

umask "$UMASK"

# `user:` in compose already chose an identity, and there is no privilege left
# to drop. Honour the choice and leave ownership alone: a directory this user
# cannot write is the operator's to fix, and guessing would hide the mistake.
if [ "$(id -u)" != "0" ]; then
    mkdir -p "$DATA_DIR" 2>/dev/null || true
    exec "$@"
fi

if ! getent group "$PGID" >/dev/null 2>&1; then
    addgroup -g "$PGID" snagarr
fi
if ! getent passwd "$PUID" >/dev/null 2>&1; then
    adduser -D -H -u "$PUID" -G "$(getent group "$PGID" | cut -d: -f1)" -s /sbin/nologin snagarr
fi

# Only the data directory is touched. Any other mount holds somebody else's
# files and is not ours to rewrite.
mkdir -p "$DATA_DIR"
chown -R "$PUID:$PGID" "$DATA_DIR"

exec su-exec "$PUID:$PGID" "$@"
