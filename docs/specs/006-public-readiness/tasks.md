# Tasks

Task IDs are the live af-coordinator short IDs in project `afc`. One ID scheme,
not two. Use `afctl issue get afc-<N>` for current status, claims,
dependencies, notes, and closure audit.

| ID | Type | Pri | Status | Title | Scope |
|----|------|-----|--------|-------|-------|
| `afc-39` | chore | P3 | done | Document API endpoint map and implementation layout | Added a compact API endpoint inventory and implementation map for contributors. |
| `afc-40` | chore | P2 | done | Add worktree unregister/prune support | Added safe cleanup commands for stale coordinator worktree records. |
| `afc-41` | chore | P3 | done | Cleanup after packet 005 before GPT 5.6 | Removed merged stale worktrees, refreshed roadmap wording, and verified coordinator health. |
| `afc-42` | chore | P3 | done | Clarify public README and add install preflight | Improved public README purpose/platform guidance and added clean-machine preflight checks. |
| `afc-43` | feature | P3 | done | macOS launchd service install | Added macOS launchd install/uninstall plus OS-aware Linux/macOS service diagnostics. |
| `afc-44` | feature | P3 | done | API store boundary cleanup | Made `internal/api` depend on a small store boundary while keeping SQLite as the only backend. |
