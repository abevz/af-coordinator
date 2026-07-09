#!/bin/sh
set -u

status=0
warnings=0

ok() {
	printf '[ok]   %s\n' "$1"
}

warn() {
	warnings=$((warnings + 1))
	printf '[WARN] %s\n' "$1"
}

fail() {
	status=1
	printf '[FAIL] %s\n' "$1"
}

need_cmd() {
	if command -v "$1" >/dev/null 2>&1; then
		ok "$1 found: $(command -v "$1")"
	else
		fail "$1 is required but was not found in PATH"
	fi
}

version_num() {
	awk -v v="$1" 'BEGIN {
		split(v, a, ".")
		printf "%d%03d%03d\n", a[1], a[2], a[3]
	}'
}

need_cmd go
need_cmd make

if command -v go >/dev/null 2>&1; then
	required_go="$(awk '$1 == "go" { print $2; exit }' go.mod)"
	installed_go="$(go version | awk '{ print $3 }' | sed 's/^go//' | sed 's/[-+].*$//')"
	if [ -z "$required_go" ]; then
		fail "could not read required Go version from go.mod"
	elif [ "$(version_num "$installed_go")" -lt "$(version_num "$required_go")" ]; then
		fail "Go $installed_go is too old; go.mod requires $required_go or newer"
	else
		ok "Go version $installed_go satisfies go.mod requirement $required_go"
	fi
fi

if command -v git >/dev/null 2>&1; then
	ok "git found: $(command -v git)"
else
	warn "git not found; source archives can still build, but clone/worktree workflows need git"
fi

bindir="${BINDIR:-$HOME/.local/bin}"
case ":$PATH:" in
	*":$bindir:"*) ok "install directory is already on PATH: $bindir" ;;
	*) warn "install directory is not on PATH: $bindir" ;;
esac

if command -v sqlite3 >/dev/null 2>&1; then
	ok "sqlite3 found for optional backup service: $(command -v sqlite3)"
else
	warn "sqlite3 not found; daemon does not need it, but contrib/systemd backup service does"
fi

os_name="$(uname -s 2>/dev/null || printf unknown)"
case "$os_name" in
	Linux)
		if command -v systemctl >/dev/null 2>&1; then
			ok "systemctl found for Linux user service install"
		else
			warn "systemctl not found; use foreground daemon or install a service manually"
		fi
		;;
	Darwin)
		warn "macOS detected; binaries build, but packaged launchd service install is not shipped yet"
		;;
	*)
		warn "untested OS: $os_name"
		;;
esac

if [ "$status" -eq 0 ]; then
	if [ "$warnings" -gt 0 ]; then
		printf 'Preflight passed with %d warning(s).\n' "$warnings"
	else
		printf 'Preflight passed.\n'
	fi
else
	printf 'Preflight failed.\n'
fi

exit "$status"
