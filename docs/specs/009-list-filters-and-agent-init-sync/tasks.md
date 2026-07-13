# Tasks

Task IDs are live af-coordinator short IDs in project `afc`. This packet owns
scope and verification; coordinator records own claims, notes, links, and
closure audit.

| ID | Type | Pri | Status | Title | Depends on |
|----|------|-----|--------|-------|------------|
| `afc-54` | feature | P2 | done | Add multi-value issue list filters and init handoff sync | - |

## `afc-54`: Multi-Value List Filters And Managed Init Sync

Scope:

- normalize repeated and comma-separated project/type/status list filters;
- preserve legacy single-value client and API behavior while adding an explicit
  multi-value client path;
- implement safe `IN (...)` store predicates and deterministic ordering;
- make list CLI parsing strict and document `ls` filter usage;
- update the init snippet to route unfinished sessions through handoff/close;
- dry-run then synchronize the managed integration block in registered
  repositories after the installed CLI is verified.

Verification:

- store/API/client/CLI tests cover single, CSV, repeated, intersection,
  whitespace, empty-element, and invalid-type cases;
- `afctl ls --help` and `afctl issue list --help` do not call the daemon;
- init dry-run and managed-block replacement preserve surrounding content;
- `go test ./...`, `go build -buildvcs=false ./...`, and `make test` pass;
- installed CLI verifies both list forms against the live daemon and dry-run
  precedes every real registered-repository `afctl init` sync;
- documentation and the packet review record the deployed behavior.
