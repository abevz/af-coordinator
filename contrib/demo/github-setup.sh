# shellcheck shell=bash
# Sourced (hidden) by github.tape: an isolated dibs daemon and a fresh clone of
# the sandbox repository. Needs DIBS_BIN_DIR, an authenticated gh, and git.

DEMO_DIR=$(mktemp -d "${TMPDIR:-/tmp}/dibs-gh-demo.XXXXXX")
export DEMO_DIR
export DIBS_SOCKET="${XDG_RUNTIME_DIR:-/tmp}/dibs-gh-demo-$$.sock"
export DIBS_DB="$DEMO_DIR/dibs.db"
export DIBS_ACTOR=demo-agent
export GH_PAGER=cat GH_PROMPT_DISABLED=1
mkdir -p "$DEMO_DIR/bin"
cp "$(dirname "${BASH_SOURCE[0]}")/demo-agent.sh" "$DEMO_DIR/bin/demo-agent"
export PATH="$DIBS_BIN_DIR:$DEMO_DIR/bin:$PATH"
git clone -q "https://github.com/${DEMO_REPO:-abevz/dibs-sandbox}.git" "$DEMO_DIR/sandbox"
cd "$DEMO_DIR/sandbox" || return
dibs init --project sandbox >/dev/null
demo_cleanup() {
	cd /
	dibs daemon stop >/dev/null 2>&1 || true
	rm -rf "$DEMO_DIR" "$DIBS_SOCKET" "$DIBS_SOCKET".*
	echo "dibs github demo cleaned up"
}
PS1='$ '
clear
