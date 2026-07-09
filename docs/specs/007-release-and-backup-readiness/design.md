# Design

## Release Packaging

Add a tag-triggered GitHub Actions workflow that cross-builds the three Go
binaries for `linux/{amd64,arm64}` and `darwin/{amd64,arm64}`. Each matrix job
packages the binaries into one `tar.gz`; the publish job creates a checksum
manifest and uploads the archives to the GitHub release.

`internal/build.Version` remains the runtime version source, but it becomes an
overridable string variable so release builds can set it with Go `-ldflags -X`.
Local builds keep the source default unless `VERSION` is passed to Make.

Add a small release install script under `contrib/install/` that downloads the
matching archive and checksum manifest from GitHub Releases and installs the
three binaries into `~/.local/bin` by default.

## macOS Backup

Reuse `contrib/systemd/af-coordinator-backup.sh` as the shared backup script.
Make the script portable across Linux and macOS shell tools, then install it
for macOS through a `launchd` plist template under `contrib/launchd/`.

Root Makefile backup targets dispatch by OS:

- Linux: install/remove the existing systemd timer.
- macOS: install/remove the new launchd backup job.

`afctl doctor` keeps one backup file/integrity check path and swaps only the
automation probe: Linux checks the systemd timer; macOS checks the launchd job.

