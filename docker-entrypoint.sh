#!/bin/sh
# Docker creates a missing bind-mount directory (./data) owned by root. Fix the
# ownership once, then drop to the unprivileged user.
set -e
if [ "$(id -u)" = "0" ]; then
	if [ "$(stat -c %u /data)" != "$(id -u gunta)" ]; then
		chown -R gunta:gunta /data
	fi
	exec su-exec gunta "$@"
fi
exec "$@"
