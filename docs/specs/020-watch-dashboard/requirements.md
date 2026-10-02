# Requirements

- The all-project view starts with one summary row per project: ready,
  blocked, in_progress, deferred, and the last retained issue-event timestamp.
  Projects with no non-deferred work sort last.
- STALE lists non-terminal in_progress issues without an active lease and
  issues with an expired lease. Show elapsed time since expiry when exposed;
  otherwise since the issue's last update, labelled explicitly.
- Interactive navigation selects projects then issues, displays description,
  dependencies and the last five notes, and supports back, q and r.
- --once prints a complete summary and STALE lane without terminal padding.
- The board uses only read APIs. It never claims or changes coordinator state.
- Counts and last activity cover retained history, including events outside
  the board's recent-event tail. Unknown timestamps remain unknown.
