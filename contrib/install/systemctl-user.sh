#!/bin/sh
set -u

if [ "$(uname -s 2>/dev/null || printf unknown)" != "Linux" ]; then
	printf 'systemctl-user.sh is Linux/systemd-only\n' >&2
	exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
	printf 'systemctl is required for Linux user service operations\n' >&2
	exit 1
fi

uid="$(id -u)"
runtime_dir="${XDG_RUNTIME_DIR:-/run/user/$uid}"

if [ -z "${XDG_RUNTIME_DIR:-}" ] && [ -d "$runtime_dir" ]; then
	export XDG_RUNTIME_DIR="$runtime_dir"
fi

bus_path="$runtime_dir/bus"
if [ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ] && [ -S "$bus_path" ]; then
	export DBUS_SESSION_BUS_ADDRESS="unix:path=$bus_path"
fi

if [ ! -S "$bus_path" ]; then
	printf 'systemd user bus socket was not found at %s\n' "$bus_path" >&2
	printf 'Run from a logged-in user session or start the user manager first.\n' >&2
fi

exec systemctl --user "$@"
