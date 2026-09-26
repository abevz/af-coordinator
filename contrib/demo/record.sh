#!/usr/bin/env bash
# Record the README demo: run race.tape with vhs, then assemble the GIF.
# Run from anywhere; requires vhs (with ttyd), ffmpeg, tmux, jq, git, and Go.
# Set DIBS_BIN_DIR to record with prebuilt (for example published) binaries.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
out="${1:-$root/docs/assets/dibs-race-demo.gif}"
tape_src="${DEMO_TAPE:-contrib/demo/race.tape}"
frames="$root/.demo-frames"
tmp="$root/.demo-tmp"

cd "$root"
rm -rf "$frames" "$tmp"
mkdir -p "$tmp"
trap 'rm -rf "$frames" "$tmp"' EXIT
# vhs does not pass the caller's environment to the recorded shell, so
# DIBS_BIN_DIR (for example published release binaries) and any variables
# named in DEMO_ENV_VARS go in as tape Env lines. vhs ignores Set lines after
# any other command, so the Env lines go just before the first Hide.
tape="$tmp/demo.tape"
env_lines=""
for var in DIBS_BIN_DIR ${DEMO_ENV_VARS:-}; do
	if [ -n "${!var:-}" ]; then
		env_lines+="Env $var \"${!var}\""$'\n'
	fi
done
DEMO_ENV_LINES="$env_lines" awk '!done && /^Hide$/ { printf "%s", ENVIRON["DEMO_ENV_LINES"]; done = 1 } { print }' \
	"$tape_src" >"$tape"
# vhs moves its frame directory out of TMPDIR with a rename, which fails
# silently across filesystems (for example tmpfs /tmp), so keep it local.
TMPDIR="$tmp" vhs "$tape"

# vhs stores terminal text and cursor as separate layers at 50 fps. Merge
# them, pad like a terminal window, drop to 10 fps, and use one palette
# without dithering so text stays sharp.
ffmpeg -v error -y \
	-framerate 50 -i "$frames/frame-text-%05d.png" \
	-framerate 50 -i "$frames/frame-cursor-%05d.png" \
	-filter_complex "[0][1]overlay,fps=10,pad=iw+32:ih+32:16:16:color=#171717,split[a][b];[a]palettegen=max_colors=64:stats_mode=full[p];[b][p]paletteuse=dither=none" \
	"$out"
rm -rf "$frames" "$tmp"
echo "wrote $out"
