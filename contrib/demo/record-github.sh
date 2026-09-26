#!/usr/bin/env bash
# Record the GitHub demo against a real repository: create a fresh issue,
# fill in github.tape, and record it with record.sh. Every run creates one
# issue, one pull request, and one comment in DEMO_REPO (a throwaway repo).
# Requires DIBS_BIN_DIR (use published release binaries), an authenticated gh,
# vhs (with ttyd), ffmpeg, and git.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
repo="${DEMO_REPO:-abevz/dibs-sandbox}"
: "${DIBS_BIN_DIR:?set DIBS_BIN_DIR to the dibs binaries to record}"

url=$(gh issue create -R "$repo" --title "Add a changelog entry" \
	--body "Add one line to CHANGELOG.md and open a pull request. Created by the dibs GitHub demo.")
number=${url##*/}
echo "demo issue: $url"

tmp="$root/.demo-tmp-github"
rm -rf "$tmp"
mkdir -p "$tmp"

# The tape's own cleanup does not run when a recording fails, so stop any
# demo daemon here, and close the issue if no GIF was produced.
finish() {
	status=$?
	for sock in "${XDG_RUNTIME_DIR:-/tmp}"/dibs-gh-demo-*.sock; do
		[ -e "$sock.pid" ] && kill "$(cat "$sock.pid")" 2>/dev/null
		rm -f "$sock" "$sock".*
	done
	rm -rf "$tmp"
	if [ "$status" -ne 0 ]; then
		gh issue close "$number" -R "$repo" --comment "Unused: the dibs demo recording failed." >/dev/null 2>&1 || true
		echo "recording failed; closed $url" >&2
	fi
}
trap finish EXIT
sed -e "s|@ISSUE_URL@|$url|g" -e "s|@ISSUE_NUMBER@|$number|g" \
	"$root/contrib/demo/github.tape" >"$tmp/github.tape"

# gh reads its token from the desktop keyring over D-Bus; pass the session
# addresses instead of the token itself.
export DEMO_REPO="$repo"
DEMO_TAPE="$tmp/github.tape" \
	DEMO_ENV_VARS="DEMO_REPO HOME XDG_RUNTIME_DIR DBUS_SESSION_BUS_ADDRESS" \
	"$root/contrib/demo/record.sh" "${1:-$root/docs/assets/dibs-github-demo.gif}"
