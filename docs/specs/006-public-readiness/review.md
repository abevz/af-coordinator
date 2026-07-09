# Review

Status: completed

## Completed Before This Branch

- `afc-39`: documented the API endpoint map and implementation layout.
- `afc-40`: added worktree unregister/prune support.
- `afc-41`: cleaned up after packet 005 before the next phase.
- `afc-42`: clarified the public README and added install preflight.

## Current Branch Verification

### `afc-43`

- `make preflight`
- `GOOS=darwin GOARCH=arm64 go build -buildvcs=false ./...`
- `GOOS=darwin GOARCH=amd64 go build -buildvcs=false ./...`
- `make -n install-launchd`
- `make build-install`
- `make restart-service`
- `afctl health`
- `afctl doctor`

### `afc-44`

- `go test ./...`
- `go build -buildvcs=false ./...`
- `rg -n "internal/store/sqlite|sqlite\\." internal/api -g '*.go' -g '!*_test.go'`

## Outcome

- Added a supported macOS LaunchAgent install path.
- Routed Linux service Makefile targets through an env-aware systemd user
  helper.
- Made preflight and doctor service guidance OS-aware.
- Added `internal/store.CoordinatorStore` and `internal/store/sqlite.Store`.
- Updated `internal/api` handlers to depend on the store interface instead of
  importing `internal/store/sqlite`.
- Kept SQLite as the only storage implementation.
