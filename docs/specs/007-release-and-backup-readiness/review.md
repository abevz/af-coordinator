# Review

Status: completed

## Current Branch Verification

### `afc-45`

- `go test ./...`
- `go build -buildvcs=false ./...`
- `VERSION=v0.0.0-test make build`
- local release packaging smoke test created archive plus `checksums.txt`
- release workflow smoke check for required build/publish fragments
- `sh -n contrib/install/install-release.sh`

### `afc-46`

- `go test ./...`
- `make preflight`
- `make -n install-backup`
- `make -n uninstall-backup`
- `make -n install-backup-launchd`
- `make -n uninstall-backup-launchd`
- `bash -n contrib/systemd/af-coordinator-backup.sh`
- `afctl doctor`

## Outcome

- Added `.github/workflows/release.yml` for tag-triggered Linux/macOS release
  archives and checksum manifest publishing.
- Added `contrib/install/install-release.sh` for checksum-verified install from
  GitHub Releases.
- Made `internal/build.Version` overridable through Go ldflags while preserving
  the source default for local builds.
- Added a macOS backup LaunchAgent template and Makefile backup target dispatch
  for Linux/systemd and macOS/launchd.
- Made the shared backup script portable across Linux and macOS pruning tools.
- Updated `afctl doctor` to recognize macOS launchd backup automation and reuse
  the existing backup integrity checks.
