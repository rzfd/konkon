#!/bin/sh
set -e
if [ "$(id -u)" = "0" ]; then
	chown -R konkon:konkon /data 2>/dev/null || true
	exec su-exec konkon:konkon /usr/local/bin/konkon
fi
exec /usr/local/bin/konkon
