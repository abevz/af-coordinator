# Design

## Public And Operator Surface

Keep README and operations docs focused on public users: purpose, supported
platforms, preflight checks, service install commands, diagnostics, and safe
worktree hygiene.

Linux service targets call `contrib/install/systemctl-user.sh` instead of raw
`systemctl --user`. The helper fills `XDG_RUNTIME_DIR` and
`DBUS_SESSION_BUS_ADDRESS` from `/run/user/$(id -u)/bus` when they are missing,
then delegates to `systemctl --user`.

macOS service support ships as a `launchd` plist template under
`contrib/launchd/`. The Makefile renders it into `~/Library/LaunchAgents/` and
uses `launchctl bootstrap`, `enable`, and `kickstart`.

## Store Boundary

Add `internal/store.CoordinatorStore` as the API-facing persistence contract.
`internal/store/sqlite.Store` wraps the existing SQLite functions and remains
the only implementation.

The API layer receives the interface and calls store methods. SQLite-specific
opening, pragmas, and migrations stay in `internal/store/sqlite` and
`cmd/af-coordinatord`.

JSONL export is expressed through an export source interface that the SQLite
store implements. The API transport calls the store boundary; it does not reach
into SQLite directly.

This is an architectural seam, not a promise that other databases are
supported.

