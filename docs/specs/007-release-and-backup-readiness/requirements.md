# Requirements

- Tagged GitHub releases SHALL publish Linux and macOS archives for
  `afctl`, `af-coordinatord`, and `afc-mcp`.
- Release artifacts SHALL include a SHA-256 checksum manifest.
- Release builds SHALL be able to set the runtime version from the tag without
  editing source constants.
- Public docs SHALL describe release creation and release installation.
- macOS users SHALL have a launchd-backed automated backup install/uninstall
  path.
- macOS backup jobs SHALL reuse the same SQLite `VACUUM INTO`, integrity check,
  and retention semantics as Linux backups.
- `afctl doctor` SHALL recognize macOS launchd backup state instead of warning
  that no packaged macOS backup job exists.

