# Requirements

- Public docs SHALL explain what af-coordinator is for and which platforms are
  supported.
- Clean-machine setup SHALL have a preflight check for required build/install
  tools.
- Linux worktree cleanup SHALL be possible through supported CLI/API paths, not
  direct database edits.
- macOS users SHALL have a documented `launchd` install/uninstall path for
  `af-coordinatord`.
- Linux service Makefile targets SHALL work from non-interactive agent
  environments without manual `XDG_RUNTIME_DIR`/`DBUS_SESSION_BUS_ADDRESS`
  exports when the user bus is available under `/run/user/$(id -u)/bus`.
- `internal/api` SHALL depend on a minimal store interface, not
  `internal/store/sqlite`.
- SQLite SHALL remain the only implemented storage backend.

