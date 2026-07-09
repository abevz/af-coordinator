# Tasks

Task IDs are the live af-coordinator short IDs in project `afc`. One ID scheme,
not two. Use `afctl issue get afc-<N>` for current status, claims,
dependencies, notes, and closure audit.

| ID | Type | Pri | Status | Title | Scope |
|----|------|-----|--------|-------|-------|
| `afc-45` | feature | P3 | done | Release packaging readiness | Added GitHub release packaging, checksums, release install docs, install script, and tag-driven version injection. |
| `afc-46` | feature | P3 | done | macOS launchd backup parity | Added launchd-backed automated backups on macOS with doctor and operations docs. |
